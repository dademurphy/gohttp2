// Copyright 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package http2

import (
	"encoding/binary"
	"io"
	"io/ioutil"
)

const (
	kFrameLengthReservedMask   uint16   = 0xc000
	kPriorityGroupReservedMask uint32   = 0x80000000
	kStreamIDReservedMask      StreamID = 0x80000000
	kWindowSizeReservedMask    uint32   = 0x80000000
)

type FrameParser struct {
	decoder HeaderDecoder
	in      *io.LimitedReader

	expectContinuation bool

	frameType FrameType
	prefix    FramePrefix
}

func NewFrameParser(in io.Reader, decoder HeaderDecoder) *FrameParser {
	parser := new(FrameParser)
	parser.decoder = decoder
	parser.in = &io.LimitedReader{N: 0, R: in}
	return parser
}

func (p *FrameParser) ParseFrame() (Frame, *Error) {
	var frame Frame
	var err *Error

	if err = p.parsePrefix(); err != nil {
		*p = FrameParser{} // Reset parser.
		return nil, err
	}

	switch p.frameType {
	case DATA:
		frame, err = p.parseDataFrame()
	case HEADERS:
		frame, err = p.parseHeadersFrame()
	case PRIORITY:
		frame, err = p.parsePriorityFrame()
	case RST_STREAM:
		frame, err = p.parseRstStreamFrame()
	case SETTINGS:
		frame, err = p.parseSettingsFrame()
	case PUSH_PROMISE:
		frame, err = p.parsePushPromiseFrame()
	case PING:
		frame, err = p.parsePingFrame()
	case GOAWAY:
		frame, err = p.parseGoAwayFrame()
	case WINDOW_UPDATE:
		frame, err = p.parseWindowUpdateFrame()
	case CONTINUATION:
		frame, err = p.parseContinuationFrame()
	default:
		// Frame type has already been validated.
		panic(p.frameType)
	}

	// Expect the complete frame length to have been consumed.
	if err == nil && p.in.N != 0 {
		err = frameSizeError("%v bytes of extra frame payload", p.in.N)
		frame = nil
	}
	if err != nil {
		*p = FrameParser{} // Reset parser.
	}
	return frame, err
}

func (p *FrameParser) parsePrefix() *Error {
	var length uint16

	// Read two-byte frame length field.
	p.in.N = 2
	if err := p.read(&length); err != nil {
		return err
	} else if length&kFrameLengthReservedMask != 0 {
		return protocolError("reserved length bits are non-zero")
	}

	// Bound the reader to the frame length. Add six to account for frame type,
	// flags, and stream ID, which are not considered part of the payload length.
	p.in.N = int64(length + 6)

	if err := p.read(&p.frameType); err != nil {
		return err
	} else if p.frameType > LAST_FRAME_TYPE {
		return protocolError("invalid frame type %#x", p.frameType)
	} else if p.expectContinuation && p.frameType != CONTINUATION {
		return protocolError("expected CONTINUATION")
	} else if !p.expectContinuation && p.frameType == CONTINUATION {
		return protocolError("unexpected CONTINUATION")
	}

	// Parse and validate flags against allowed flags of the frame type.
	if err := p.read(&p.prefix.Flags); err != nil {
		return err
	} else if p.prefix.Flags&(^kValidFlags[p.frameType]) != 0 {
		return protocolError("invalid flags %#x for frame type %#x",
			p.prefix.Flags&(^kValidFlags[p.frameType]), p.frameType)
	}

	// Parse and validate the stream ID against the reserved bits mask.
	if err := p.read(&p.prefix.StreamID); err != nil {
		return err
	} else if p.prefix.StreamID&kStreamIDReservedMask != 0 {
		return protocolError("reserved StreamID bit is non-zero")
	}
	return nil
}

func (p *FrameParser) parseFramePadding() (FramePadding, *Error) {
	var high, low uint8

	// Expect that HIGH is not set without LOW.
	if p.prefix.Flags&PAD_HIGH != 0 && p.prefix.Flags&PAD_LOW == 0 {
		return FramePadding{}, protocolError("PAD_HIGH set without PAD_LOW")
	}
	// Parse the HIGH and LOW bytes.
	if p.prefix.Flags&PAD_HIGH != 0 {
		if err := p.read(&high); err != nil {
			return FramePadding{}, err
		}
	}
	if p.prefix.Flags&PAD_LOW != 0 {
		if err := p.read(&low); err != nil {
			return FramePadding{}, err
		}
	}
	// Expect that the combined value doesn't overflow remaining input.
	length := uint(high)<<8 + uint(low)
	if int64(length) > p.in.N {
		return FramePadding{}, frameSizeError(
			"padding of %v is longer than remaining frame length %v",
			length, p.in.N)
	}
	return FramePadding{length}, nil
}

func (p *FrameParser) read(out interface{}) *Error {
	if err := binary.Read(p.in, binary.BigEndian, out); err != nil {
		if p.in.N == 0 {
			return frameSizeError("reached premature frame end reading %T", out)
		} else {
			return internalError(err)
		}
	}
	return nil
}

func (p *FrameParser) readData(padLength uint) ([]byte, *Error) {
	// Read and buffer frame data.
	out := make([]byte, p.in.N-int64(padLength))
	if _, err := io.ReadFull(p.in, out); err != nil {
		return nil, internalError(err)
	}
	// Read and discard padding.
	if _, err := io.Copy(ioutil.Discard, p.in); err != nil {
		return nil, internalError(err)
	}
	return out, nil
}

func (p *FrameParser) readFragment(padLength uint) ([]HeaderField, *Error) {
	var fields []HeaderField
	var err *Error

	// Bound the header decoder to the frame payload length, and read the fragment.
	p.in.N -= int64(padLength)
	if fields, err = p.decoder.DecodeHeaderBlockFragment(p.in); err != nil {
		return nil, err
	}
	if p.in.N != 0 {
		return nil, internalError("header decoder left %v bytes of input", p.in.N)
	}
	// Read and discard padding.
	p.in.N = int64(padLength)
	if _, err := io.Copy(ioutil.Discard, p.in); err != nil {
		return nil, internalError(err)
	}
	// If flagged, complete the header block.
	if p.prefix.Flags&END_HEADERS != 0 {
		var finalFields []HeaderField
		if finalFields, err = p.decoder.HeaderBlockComplete(); err != nil {
			return nil, err
		}
		fields = append(fields, finalFields...)
	}
	// Set CONTINUATION expectation for the next parsed frame.
	p.expectContinuation = p.prefix.Flags&END_HEADERS == 0

	return fields, err
}

func (p *FrameParser) parseFramePriority() (FramePriority, *Error) {
	var priority FramePriority

	// Expect either GROUP or DEPENDENCY is set.
	if p.prefix.Flags&PRIORITY_GROUP != 0 && p.prefix.Flags&PRIORITY_DEPENDENCY != 0 {
		return priority, protocolError(
			"both PRIORITY_GROUP and PRIORITY_DEPENDENCY set")
	}
	// Read the group and weight, expecting reserved bits to be clear.
	if p.prefix.Flags&PRIORITY_GROUP != 0 {
		if err := p.read(&priority.PriorityGroup); err != nil {
			return priority, err
		} else if priority.PriorityGroup&kPriorityGroupReservedMask != 0 {
			return priority, protocolError("reserved priority group bit is non-zero")
		}
		if err := p.read(&priority.PriorityWeight); err != nil {
			return priority, err
		}
	}
	// Read the depedency, masking out the exlusivity bit.
	if p.prefix.Flags&PRIORITY_DEPENDENCY != 0 {
		if err := p.read(&priority.StreamDependency); err != nil {
			return priority, err
		}
		if priority.StreamDependency&kStreamIDReservedMask != 0 {
			priority.ExclusiveDependency = true
			priority.StreamDependency =
				priority.StreamDependency ^ kStreamIDReservedMask
		}
	}
	return priority, nil
}

func (p *FrameParser) parseDataFrame() (*DataFrame, *Error) {
	var err *Error
	frame := &DataFrame{FramePrefix: p.prefix}

	if frame.FramePadding, err = p.parseFramePadding(); err != nil {
		return nil, err
	}
	if frame.Data, err = p.readData(frame.PaddingLength); err != nil {
		return nil, err
	}
	return frame, nil
}

func (p *FrameParser) parseHeadersFrame() (*HeadersFrame, *Error) {
	var err *Error
	frame := &HeadersFrame{FramePrefix: p.prefix}

	if frame.FramePadding, err = p.parseFramePadding(); err != nil {
		return nil, err
	}
	if frame.FramePriority, err = p.parseFramePriority(); err != nil {
		return nil, err
	}
	if frame.Fields, err = p.readFragment(frame.PaddingLength); err != nil {
		return nil, err
	}
	return frame, nil
}

func (p *FrameParser) parsePriorityFrame() (*PriorityFrame, *Error) {
	var err *Error
	frame := &PriorityFrame{FramePrefix: p.prefix}

	// Expect a priority to be included in the frame payload.
	if frame.Flags&PRIORITY_GROUP == 0 && frame.Flags&PRIORITY_DEPENDENCY == 0 {
		return nil, protocolError(
			"PRIORITY must have PRIORITY_GROUP or PRIORITY_DEPENDENCY set")
	}
	if frame.FramePriority, err = p.parseFramePriority(); err != nil {
		return nil, err
	}
	return frame, nil
}

func (p *FrameParser) parseRstStreamFrame() (*RstStreamFrame, *Error) {
	frame := &RstStreamFrame{FramePrefix: p.prefix}

	if frame.StreamID == 0 {
		return nil, protocolError("RST_STREAM must have non-zero StreamID")
	}
	if err := p.read(&frame.Code); err != nil {
		return nil, err
	}
	return frame, nil
}

func (p *FrameParser) parseSettingsFrame() (*SettingsFrame, *Error) {
	frame := &SettingsFrame{FramePrefix: p.prefix}

	if frame.StreamID != 0 {
		return nil, protocolError("invalid SETTINGS StreamID %#x", frame.StreamID)
	}
	if frame.Flags&ACK != 0 && p.in.N != 0 {
		return nil, frameSizeError("SETTINGS with ACK must have empty payload")
	}
	if p.in.N%5 != 0 {
		return nil, frameSizeError("invalid SETTINGS payload (length %% 5 != 0)")
	}
	frame.Settings = make(map[SettingID]uint32)

	for p.in.N != 0 {
		var key SettingID
		if err := p.read(&key); err != nil {
			return nil, err
		}
		if key < SETTINGS_MIN_SETTING_ID || key > SETTINGS_MAX_SETTING_ID {
			return nil, protocolError("invalid setting ID %#x", key)
		}

		var value uint32
		if err := p.read(&value); err != nil {
			return nil, err
		}
		if key == SETTINGS_ENABLE_PUSH && value != 0 && value != 1 {
			return nil, protocolError(
				"invalid setting for SETTINGS_ENABLE_PUSH (must be 0 or 1)")
		}
		frame.Settings[key] = value
	}
	return frame, nil
}

func (p *FrameParser) parsePushPromiseFrame() (*PushPromiseFrame, *Error) {
	var err *Error
	frame := &PushPromiseFrame{FramePrefix: p.prefix}

	if frame.FramePadding, err = p.parseFramePadding(); err != nil {
		return nil, err
	}
	if err = p.read(&frame.PromisedID); err != nil {
		return nil, err
	}
	if frame.PromisedID == 0 {
		return nil, protocolError("promised StreamID must be nonzero")
	}
	if frame.PromisedID&kStreamIDReservedMask != 0 {
		return nil, protocolError("promised StreamID has reserved bit set")
	}
	if frame.Fields, err = p.readFragment(frame.PaddingLength); err != nil {
		return nil, err
	}
	return frame, nil
}

func (p *FrameParser) parsePingFrame() (*PingFrame, *Error) {
	frame := &PingFrame{FramePrefix: p.prefix}

	if err := p.read(&frame.OpaqueData); err != nil {
		return nil, err
	}
	return frame, nil
}

func (p *FrameParser) parseGoAwayFrame() (*GoAwayFrame, *Error) {
	var err *Error
	frame := &GoAwayFrame{FramePrefix: p.prefix}

	if frame.StreamID != 0 {
		return nil, protocolError("invalid GOAWAY StreamID %#x", frame.StreamID)
	}
	if err := p.read(&frame.LastID); err != nil {
		return nil, err
	}
	if frame.LastID&kStreamIDReservedMask != 0 {
		return nil, protocolError("last StreamID has reserved bit set")
	}
	if err := p.read(&frame.Code); err != nil {
		return nil, err
	}
	if frame.Debug, err = p.readData(0); err != nil {
		return nil, err
	}
	return frame, nil
}

func (p *FrameParser) parseWindowUpdateFrame() (*WindowUpdateFrame, *Error) {
	frame := &WindowUpdateFrame{FramePrefix: p.prefix}

	if err := p.read(&frame.SizeDelta); err != nil {
		return nil, err
	}
	if frame.SizeDelta&kWindowSizeReservedMask != 0 {
		return nil, protocolError("reserved size delta bit is non-zero")
	}
	return frame, nil
}

func (p *FrameParser) parseContinuationFrame() (*ContinuationFrame, *Error) {
	var err *Error
	frame := &ContinuationFrame{FramePrefix: p.prefix}

	if frame.FramePadding, err = p.parseFramePadding(); err != nil {
		return nil, err
	}
	if frame.Fields, err = p.readFragment(frame.PaddingLength); err != nil {
		return nil, err
	}
	return frame, nil
}
