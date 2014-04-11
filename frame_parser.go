// Copyright 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package http2

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
)

const (
	kFrameLengthReservedMask   uint16   = 0xc000
	kPriorityGroupReservedMask uint32   = 0x80000000
	kStreamIdReservedMask      StreamId = 0x80000000
	kWindowSizeReservedMask    uint32   = 0x80000000
)

func ParseFrame(in io.Reader) (Frame, *Error) {
	var length uint16
	if err := read(in, &length); err != nil {
		return nil, err
	} else if length&kFrameLengthReservedMask != 0 {
		return nil, ProtocolError(fmt.Errorf("Reserved length bits are non-zero"))
	}

	var frameType FrameType
	if err := read(in, &frameType); err != nil {
		return nil, err
	} else if frameType > LAST_FRAME_TYPE {
		return nil, ProtocolError(fmt.Errorf("Invalid frame type %#x", frameType))
	}

	frame := NewFrame(frameType)

	// Bound the reader to the frame length, adding five to account for
	// frame flags and stream ID (which are not part of the length).
	lIn := io.LimitedReader{N: int64(length) + 5, R: in}
	if err := frame.Parse(&lIn); err != nil {
		return nil, err
	}
	return frame, nil
}

func (s *FramePrefix) Parse(in io.Reader, frameType FrameType) *Error {
	// Type has already been set.
	if err := read(in, &s.Flags); err != nil {
		return err
	} else if s.Flags&(^kValidFlags[frameType]) != 0 {
		return ProtocolError(fmt.Errorf(
			"Invalid flags %#x for frame type %#x",
			s.Flags&(^kValidFlags[frameType]), frameType))
	}
	if err := read(in, &s.Id); err != nil {
		return err
	} else if s.Id&kStreamIdReservedMask != 0 {
		return ProtocolError(fmt.Errorf("Reserved stream ID bit is non-zero"))
	}
	return nil
}

func (s *FramePadding) Parse(flags Flags, in *io.LimitedReader) *Error {
	if flags&PAD_HIGH != 0 && flags&PAD_LOW == 0 {
		return ProtocolError(fmt.Errorf("PAD_HIGH set without PAD_LOW"))
	}
	if flags&PAD_HIGH != 0 {
		var padHigh uint8
		if err := read(in, &padHigh); err != nil {
			return err
		}
		s.PaddingLength += uint(padHigh) << 8
	}
	if flags&PAD_LOW != 0 {
		var padLow uint8
		if err := read(in, &padLow); err != nil {
			return err
		}
		s.PaddingLength += uint(padLow)
	}
	if int64(s.PaddingLength) > in.N {
		return FrameSizeError(fmt.Errorf(
			"Padding of %v is longer than remaining frame length %v",
			s.PaddingLength, in.N))
	}
	return nil
}

func (f *FramePadding) ReadRemainder(in *io.LimitedReader) ([]byte, *Error) {
	// Read and buffer frame data.
	out := make([]byte, in.N-int64(f.PaddingLength))
	if _, err := io.ReadFull(in, out); err != nil {
		return nil, InternalError(err)
	}
	// Read and discard padding.
	if _, err := io.Copy(ioutil.Discard, in); err != nil {
		return nil, InternalError(err)
	}
	return out, nil
}

func (s *FramePriority) Parse(flags Flags, in io.Reader) *Error {
	if flags&PRIORITY_GROUP != 0 && flags&PRIORITY_DEPENDENCY != 0 {
		return ProtocolError(fmt.Errorf(
			"Both PRIORITY_GROUP and PRIORITY_DEPENDENCY set"))
	}
	if flags&PRIORITY_GROUP != 0 {
		if err := read(in, &s.PriorityGroup); err != nil {
			return err
		} else if s.PriorityGroup&kPriorityGroupReservedMask != 0 {
			return ProtocolError(fmt.Errorf(
				"Reserved priority group bit is non-zero"))
		}
		if err := read(in, &s.PriorityWeight); err != nil {
			return err
		}
	}
	if flags&PRIORITY_DEPENDENCY != 0 {
		if err := read(in, &s.StreamDependency); err != nil {
			return err
		}
		if s.StreamDependency&kStreamIdReservedMask != 0 {
			s.ExclusiveDependency = true
			s.StreamDependency = s.StreamDependency ^ kStreamIdReservedMask
		}
	}
	return nil
}

func (f *DataFrame) Parse(in *io.LimitedReader) *Error {
	var err *Error
	if err = f.FramePrefix.Parse(in, f.GetType()); err != nil {
		return err
	}
	if err = f.FramePadding.Parse(f.Flags, in); err != nil {
		return err
	}
	if f.Data, err = f.FramePadding.ReadRemainder(in); err != nil {
		return err
	}
	return expectEOF(in)
}

func (f *HeadersFrame) Parse(in *io.LimitedReader) *Error {
	var err *Error
	if err = f.FramePrefix.Parse(in, f.GetType()); err != nil {
		return err
	}
	if err = f.FramePadding.Parse(f.Flags, in); err != nil {
		return err
	}
	if err = f.FramePriority.Parse(f.Flags, in); err != nil {
		return err
	}
	if f.Fragment, err = f.FramePadding.ReadRemainder(in); err != nil {
		return err
	}
	return expectEOF(in)
}

func (f *PriorityFrame) Parse(in *io.LimitedReader) *Error {
	if err := f.FramePrefix.Parse(in, f.GetType()); err != nil {
		return err
	}
	if f.Flags&PRIORITY_GROUP == 0 && f.Flags&PRIORITY_DEPENDENCY == 0 {
		return ProtocolError(fmt.Errorf(
			"PRIORITY must have PRIORITY_GROUP or PRIORITY_DEPENDENCY set"))
	}
	if err := f.FramePriority.Parse(f.Flags, in); err != nil {
		return err
	}
	return expectEOF(in)
}

func (f *RstStreamFrame) Parse(in *io.LimitedReader) *Error {
	if err := f.FramePrefix.Parse(in, f.GetType()); err != nil {
		return err
	}
	if f.Id == 0 {
		return ProtocolError(fmt.Errorf(
			"RST_STREAM must have non-zero stream ID"))
	}
	if err := read(in, &f.Code); err != nil {
		return err
	}
	return expectEOF(in)
}

func (f *SettingsFrame) Parse(in *io.LimitedReader) *Error {
	if err := f.FramePrefix.Parse(in, f.GetType()); err != nil {
		return err
	}
	if f.Id != 0 {
		return ProtocolError(fmt.Errorf("Invalid SETTINGS StreamId %#x", f.Id))
	}
	if f.Flags&ACK != 0 && in.N != 0 {
		return FrameSizeError(fmt.Errorf(
			"SETTINGS with ACK must have empty payload"))
	}
	if in.N%5 != 0 {
		return FrameSizeError(fmt.Errorf(
			"Invalid SETTINGS payload size (not modulo 5)"))
	}
	f.Settings = make(map[SettingId]uint32)

	for in.N != 0 {
		var key SettingId
		if err := read(in, &key); err != nil {
			return err
		}
		if key < SETTINGS_MIN_SETTING_ID || key > SETTINGS_MAX_SETTING_ID {
			return ProtocolError(fmt.Errorf("Invalid setting ID %#x", key))
		}

		var value uint32
		if err := read(in, &value); err != nil {
			return err
		}
		if key == SETTINGS_ENABLE_PUSH && value != 0 && value != 1 {
			return ProtocolError(fmt.Errorf(
				"Invalid setting for SETTINGS_ENABLE_PUSH (must be 0 or 1)"))
		}
		f.Settings[key] = value
	}
	return expectEOF(in)
}

func (f *PushPromiseFrame) Parse(in *io.LimitedReader) *Error {
	var err *Error
	if err = f.FramePrefix.Parse(in, f.GetType()); err != nil {
		return err
	}
	if err = f.FramePadding.Parse(f.Flags, in); err != nil {
		return err
	}
	if err = read(in, &f.PromisedId); err != nil {
		return err
	}
	if f.PromisedId == 0 {
		return ProtocolError(fmt.Errorf("Promised stream ID must be nonzero"))
	}
	if f.Fragment, err = f.FramePadding.ReadRemainder(in); err != nil {
		return err
	}
	return expectEOF(in)
}

func (f *PingFrame) Parse(in *io.LimitedReader) *Error {
	if err := f.FramePrefix.Parse(in, f.GetType()); err != nil {
		return err
	}
	if err := read(in, &f.OpaqueData); err != nil {
		return err
	}
	return expectEOF(in)
}

func (f *GoAwayFrame) Parse(in *io.LimitedReader) *Error {
	if err := f.FramePrefix.Parse(in, f.GetType()); err != nil {
		return err
	}
	if f.Id != 0 {
		return ProtocolError(fmt.Errorf("Invalid GOAWAY StreamId %#x", f.Id))
	}
	if err := read(in, &f.LastStream); err != nil {
		return err
	}
	if f.LastStream&kStreamIdReservedMask != 0 {
		return ProtocolError(fmt.Errorf("Reserved stream ID bit is non-zero"))
	}
	if err := read(in, &f.Code); err != nil {
		return err
	}
	f.Debug = make([]byte, in.N)
	if _, err := io.ReadFull(in, f.Debug); err != nil {
		return InternalError(err)
	}
	return expectEOF(in)
}

func (f *WindowUpdateFrame) Parse(in *io.LimitedReader) *Error {
	if err := f.FramePrefix.Parse(in, f.GetType()); err != nil {
		return err
	}
	if err := read(in, &f.SizeDelta); err != nil {
		return err
	}
	if f.SizeDelta&kWindowSizeReservedMask != 0 {
		return ProtocolError(fmt.Errorf("Reserved bit is non-zero"))
	}
	return expectEOF(in)
}

func (f *ContinuationFrame) Parse(in *io.LimitedReader) *Error {
	var err *Error
	if err = f.FramePrefix.Parse(in, f.GetType()); err != nil {
		return err
	}
	if err = f.FramePadding.Parse(f.Flags, in); err != nil {
		return err
	}
	if f.Fragment, err = f.FramePadding.ReadRemainder(in); err != nil {
		return err
	}
	return expectEOF(in)
}

func read(in io.Reader, out interface{}) *Error {
	if err := binary.Read(in, binary.BigEndian, out); err != nil {
		if lIn, ok := in.(*io.LimitedReader); ok && lIn.N == 0 {
			return FrameSizeError(
				fmt.Errorf("Reached frame end while reading fixed-size payload"))
		} else {
			return InternalError(err)
		}
	}
	return nil
}

func expectEOF(in *io.LimitedReader) *Error {
	if in.N != 0 {
		return FrameSizeError(fmt.Errorf(
			"Unexpected %v bytes of frame remainder", in.N))
	}
	return nil
}
