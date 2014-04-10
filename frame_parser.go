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
	kPriorityGroupReservedMask uint32   = 0x8000
	kStreamIdReservedMask      StreamId = 0x8000
	kWindowSizeReservedMask    uint32   = 0x8000
)

func ParseFrame(in io.Reader) (Frame, error) {
	var length uint16
	if err := read(in, &length); err != nil {
		return nil, err
	} else if length&kFrameLengthReservedMask != 0 {
		return nil, fmt.Errorf("Reserved length bits are non-zero")
	}

	var frameType FrameType
	if err := read(in, &frameType); err != nil {
		return nil, err
	} else if frameType > LAST_FRAME_TYPE {
		return nil, fmt.Errorf("Invalid frame type %#v", frameType)
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

func (s *FramePrefix) Parse(in io.Reader, frameType FrameType) error {
	// Type has already been set.
	if err := read(in, &s.Flags); err != nil {
		return err
	} else if s.Flags&(^kValidFlags[frameType]) != 0 {
		return fmt.Errorf("Invalid flags %x for frame type %v",
			s.Flags&(^kValidFlags[frameType]), frameType)
	}
	if err := read(in, &s.Id); err != nil {
		return err
	} else if s.Id&kStreamIdReservedMask != 0 {
		return fmt.Errorf("Reserved stream ID bit is non-zero")
	}
	return nil
}

func (s *FramePadding) Parse(flags Flags, in *io.LimitedReader) error {
	if flags&PAD_HIGH != 0 && flags&PAD_LOW == 0 {
		return fmt.Errorf("PAD_HIGH set without PAD_LOW")
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
		return fmt.Errorf("Padding %v is longer than remaining frame length %v",
			s.PaddingLength, in.N)
	}
	return nil
}

func (f *FramePadding) ReadRemainder(in *io.LimitedReader) ([]byte, error) {
	// Read and buffer frame data.
	out := make([]byte, in.N-int64(f.PaddingLength))
	if _, err := io.ReadFull(in, out); err != nil {
		return nil, err
	}
	// Read and discard padding.
	if _, err := io.Copy(ioutil.Discard, in); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *FramePriority) Parse(flags Flags, in io.Reader) error {
	if flags&PRIORITY_GROUP != 0 && flags&PRIORITY_DEPENDENCY != 0 {
		return fmt.Errorf("Both PRIORITY_GROUP and PRIORITY_DEPENDENCY set")
	}
	if flags&PRIORITY_GROUP != 0 {
		if err := read(in, &s.PriorityGroup); err != nil {
			return err
		} else if s.PriorityGroup&kPriorityGroupReservedMask != 0 {
			return fmt.Errorf("Reserved priority group bit is non-zero")
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

func (f *DataFrame) Parse(in *io.LimitedReader) error {
	var err error
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

func (f *HeadersFrame) Parse(in *io.LimitedReader) error {
	var err error
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

func (f *PriorityFrame) Parse(in *io.LimitedReader) error {
	if err := f.FramePrefix.Parse(in, f.GetType()); err != nil {
		return err
	}
	if err := f.FramePriority.Parse(f.Flags, in); err != nil {
		return err
	}
	return expectEOF(in)
}

func (f *RstStreamFrame) Parse(in *io.LimitedReader) error {
	if err := f.FramePrefix.Parse(in, f.GetType()); err != nil {
		return err
	}
	if err := read(in, &f.ErrorCode); err != nil {
		return err
	}
	return expectEOF(in)
}

func (f *SettingsFrame) Parse(in *io.LimitedReader) error {
	if err := f.FramePrefix.Parse(in, f.GetType()); err != nil {
		return err
	}
	if f.Id != 0 {
		return fmt.Errorf("Invalid SETTINGS StreamId %v", f.Id)
	}
	if f.Flags&ACK != 0 {
		return expectEOF(in)
	}
	for in.N != 0 {
		var key uint8
		if err := read(in, &key); err != nil {
			return err
		}
		var value uint32
		if err := read(in, &value); err != nil {
			return err
		}
		f.Settings[key] = value
	}
	return expectEOF(in)
}

func (f *PushPromiseFrame) Parse(in *io.LimitedReader) error {
	var err error
	if err = f.FramePrefix.Parse(in, f.GetType()); err != nil {
		return err
	}
	if err = f.FramePadding.Parse(f.Flags, in); err != nil {
		return err
	}
	if err = read(in, &f.PromisedId); err != nil {
		return err
	}
	if f.Fragment, err = f.FramePadding.ReadRemainder(in); err != nil {
		return err
	}
	return expectEOF(in)
}

func (f *PingFrame) Parse(in *io.LimitedReader) error {
	if err := f.FramePrefix.Parse(in, f.GetType()); err != nil {
		return err
	}
	if err := read(in, &f.OpaqueData); err != nil {
		return err
	}
	return expectEOF(in)
}

func (f *GoAwayFrame) Parse(in *io.LimitedReader) error {
	if err := f.FramePrefix.Parse(in, f.GetType()); err != nil {
		return err
	}
	if err := read(in, &f.LastId); err != nil {
		return err
	}
	if err := read(in, &f.ErrorCode); err != nil {
		return err
	}
	if f.LastId&kStreamIdReservedMask != 0 {
		return fmt.Errorf("Reserved stream ID bit is non-zero")
	}
	f.Debug = make([]byte, in.N)
	if _, err := io.ReadFull(in, f.Debug); err != nil {
		return err
	}
	return expectEOF(in)
}

func (f *WindowUpdateFrame) Parse(in *io.LimitedReader) error {
	if err := f.FramePrefix.Parse(in, f.GetType()); err != nil {
		return err
	}
	if err := read(in, &f.SizeDelta); err != nil {
		return err
	}
	if f.SizeDelta&kWindowSizeReservedMask != 0 {
		return fmt.Errorf("Reserved bit is non-zero")
	}
	return expectEOF(in)
}

func (f *ContinuationFrame) Parse(in *io.LimitedReader) error {
	var err error
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

func read(in io.Reader, out interface{}) error {
	return binary.Read(in, binary.BigEndian, out)
}

func expectEOF(in *io.LimitedReader) error {
	if in.N != 0 {
		return fmt.Errorf("Unexpected %v bytes of frame remainder", in.N)
	}
	return nil
}

