// Copyright 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package http2

import (
	"bytes"
	"io"
	"io/ioutil"

	gc "gopkg.in/check.v1"
)

type ParserTest struct {
	input  *bytes.Buffer
	parser *FrameParser
}

func (t *ParserTest) SetUpTest(c *gc.C) {
	t.input = new(bytes.Buffer)
	t.parser = NewFrameParser(t.input, t)
}

// HeaderDecoder implementation. Sets the entire
// fragment as the value of header "fragment".
func (t *ParserTest) DecodeHeaderBlockFragment(
	in *io.LimitedReader) ([]HeaderField, *Error) {
	if value, err := ioutil.ReadAll(in); err != nil {
		return nil, internalError(err)
	} else if in.N != 0 {
		return nil, protocolError("decoder fragment underflow")
	} else {
		return append([]HeaderField{},
			HeaderField{Name: "fragment", Values: string(value)}), nil
	}
}

// HeaderDecoder implementation. Returns a final "cookie" header.
func (t *ParserTest) HeaderBlockComplete() ([]HeaderField, *Error) {
	return append([]HeaderField{},
		HeaderField{Name: "cookie", Values: "bar=baz; bing;"}), nil
}

func (t *ParserTest) TestInvalidFrameLength(c *gc.C) {
	t.input.Write([]byte{
		0xff, 0xff, byte(DATA), 0x00,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.ErrorMatches, "reserved length bits are non-zero")
	c.Check(err.Code, gc.Equals, PROTOCOL_ERROR)
	c.Check(frame, gc.IsNil)
}

func (t *ParserTest) TestInvalidFrameType(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0xff, 0xff, 0x00,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.ErrorMatches, "invalid frame type 0xff")
	c.Check(err.Code, gc.Equals, PROTOCOL_ERROR)
	c.Check(frame, gc.IsNil)
}

func (t *ParserTest) TestInvalidStreamID(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x00, byte(DATA), 0x00,
		0xff, 0x02, 0x03, 0x04,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.ErrorMatches, "reserved StreamID bit is non-zero")
	c.Check(err.Code, gc.Equals, PROTOCOL_ERROR)
	c.Check(frame, gc.IsNil)
}

func (t *ParserTest) TestValidPrefixNoFlags(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x00, byte(DATA), 0x00,
		0x01, 0x02, 0x03, 0x04,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.IsNil)

	data := frame.(*DataFrame)
	c.Check(data.Flags, gc.Equals, NO_FLAGS)
	c.Check(data.StreamID, gc.Equals, StreamID(0x01020304))
}

func (t *ParserTest) TestValidPrefixWithFlags(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x00, byte(DATA), byte(END_STREAM | END_SEGMENT),
		0x01, 0x02, 0x03, 0x04,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.IsNil)

	data := frame.(*DataFrame)
	c.Check(frame.GetFlags(), gc.Equals, END_STREAM|END_SEGMENT)
	c.Check(data.StreamID, gc.Equals, StreamID(0x01020304))
}

func (t *ParserTest) TestInvalidPrefixFlags(c *gc.C) {
	t.input.Write([]byte{
		// PRIORITY_GROUP is not a valid DATA frame flag.
		0x00, 0x00, byte(DATA), byte(END_STREAM | PRIORITY_GROUP),
		0x01, 0x02, 0x03, 0x04,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.ErrorMatches, "invalid flags 0x\\w+ for frame type 0x\\w+")
	c.Check(err.Code, gc.Equals, PROTOCOL_ERROR)
	c.Check(frame, gc.IsNil)
}

func (t *ParserTest) TestValidPaddingLowIsZero(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x01, byte(DATA), byte(PAD_LOW),
		0x01, 0x02, 0x03, 0x04,
		0x00,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.IsNil)

	data := frame.(*DataFrame)
	c.Check(frame.GetFlags(), gc.Equals, PAD_LOW)
	c.Check(data.PaddingLength, gc.Equals, uint(0))
}

func (t *ParserTest) TestValidPadLowAndHighAreZero(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x02, byte(DATA), byte(PAD_LOW | PAD_HIGH),
		0x01, 0x02, 0x03, 0x04,
		0x00, 0x00,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.IsNil)

	data := frame.(*DataFrame)
	c.Check(frame.GetFlags(), gc.Equals, PAD_LOW|PAD_HIGH)
	c.Check(data.PaddingLength, gc.Equals, uint(0))
}

func (t *ParserTest) TestValidPadLowIsNonzero(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x04, byte(DATA), byte(PAD_LOW),
		0x01, 0x02, 0x03, 0x04,
		0x03, 0xa1, 0xa2, 0xa3,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.IsNil)

	data := frame.(*DataFrame)
	c.Check(frame.GetFlags(), gc.Equals, PAD_LOW)
	c.Check(data.PaddingLength, gc.Equals, uint(3))
	// Remaining frame payload was discarded.
	c.Check(data.Data, gc.DeepEquals, []byte{})
}

func (t *ParserTest) TestValidPadLowIsNonzeroAndPadHighIsZero(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x04, byte(DATA), byte(PAD_LOW | PAD_HIGH),
		0x01, 0x02, 0x03, 0x04,
		0x00, 0x02, 0xa1, 0xa2,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.IsNil)

	data := frame.(*DataFrame)
	c.Check(frame.GetFlags(), gc.Equals, PAD_LOW|PAD_HIGH)
	c.Check(data.PaddingLength, gc.Equals, uint(2))
	// Remaining frame payload was discarded.
	c.Check(data.Data, gc.DeepEquals, []byte{})
}

func (t *ParserTest) TestValidPadLowAndPadHighAreNonzero(c *gc.C) {
	t.input.Write([]byte{
		0x01, 0x05, byte(DATA), byte(PAD_LOW | PAD_HIGH),
		0x01, 0x02, 0x03, 0x04,
		0x01, 0x03,
	})
	for i := 0; i != 259; i++ {
		t.input.WriteByte(0xff)
	}
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.IsNil)

	data := frame.(*DataFrame)
	c.Check(frame.GetFlags(), gc.Equals, PAD_LOW|PAD_HIGH)
	c.Check(data.PaddingLength, gc.Equals, uint(259))
	// Remaining frame payload was discarded.
	c.Check(data.Data, gc.DeepEquals, []byte{})
}

func (t *ParserTest) TestInvalidPadHighWithoutLow(c *gc.C) {
	t.input.Write([]byte{
		0x01, 0x01, byte(DATA), byte(PAD_HIGH),
		0x01, 0x02, 0x03, 0x04,
		0x00,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.ErrorMatches, "PAD_HIGH set without PAD_LOW")
	c.Check(err.Code, gc.Equals, PROTOCOL_ERROR)
	c.Check(frame, gc.IsNil)
}

func (t *ParserTest) TestInvalidPadLength(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x04, byte(DATA), byte(PAD_LOW),
		0x01, 0x02, 0x03, 0x04,
		0x04, 0xa1, 0xa2, 0xa3,
		0xa4,
	})
	frame, err := t.parser.ParseFrame()
	// Though |input| is sufficiently sized, the frame length
	// limit is hit before all padding can be read.
	c.Check(err, gc.ErrorMatches,
		"padding of 4 is longer than remaining frame length 3")
	c.Check(err.Code, gc.Equals, FRAME_SIZE_ERROR)
	c.Check(frame, gc.IsNil)
}

func (t *ParserTest) TestInvalidFixedPayloadOverflow(c *gc.C) {
	// All fixed-sized frame types check that the complete frame was consumed.
	t.input.Write([]byte{
		0x00, 0x05, byte(RST_STREAM), byte(NO_FLAGS),
		0x01, 0x02, 0x03, 0x04,
		0xaa, 0xaa, 0xaa, 0xaa,
		0xff,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.ErrorMatches, "1 bytes of extra frame payload")
	c.Check(err.Code, gc.Equals, FRAME_SIZE_ERROR)
	c.Check(frame, gc.IsNil)
}

func (t *ParserTest) TestInvalidDynamicPayloadUnderflow(c *gc.C) {
	// Simulates a broken connection.
	t.input.Write([]byte{
		0x00, 0x06, byte(DATA), byte(NO_FLAGS),
		0x01, 0x02, 0x03, 0x04,
		0xd1, 0xd2, 0xd3, 0xd4,
		0xd5,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.ErrorMatches, "unexpected EOF")
	c.Check(err.Code, gc.Equals, INTERNAL_ERROR)
	c.Check(frame, gc.IsNil)
}

func (t *ParserTest) TestValidPriorityGroupId(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x05, byte(PRIORITY), byte(PRIORITY_GROUP),
		0x01, 0x02, 0x03, 0x04,
		0x10, 0x20, 0x30, 0x40,
		0x50,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.IsNil)

	priority := frame.(*PriorityFrame)
	c.Check(priority.PriorityGroup, gc.Equals, uint32(0x10203040))
	c.Check(priority.PriorityWeight, gc.Equals, uint8(0x50))
	c.Check(priority.ExclusiveDependency, gc.Equals, false)
	c.Check(priority.StreamDependency, gc.Equals, StreamID(0))
}

func (t *ParserTest) TestInvalidPriorityGroupId(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x05, byte(PRIORITY), byte(PRIORITY_GROUP),
		0x01, 0x02, 0x03, 0x04,
		0xff, 0x20, 0x30, 0x40,
		0x50,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.ErrorMatches, "reserved priority group bit is non-zero")
	c.Check(err.Code, gc.Equals, PROTOCOL_ERROR)
	c.Check(frame, gc.IsNil)
}

func (t *ParserTest) TestValidNonexclusiveStreamDependency(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x04, byte(PRIORITY), byte(PRIORITY_DEPENDENCY),
		0x01, 0x02, 0x03, 0x04,
		0x10, 0x20, 0x30, 0x40,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.IsNil)

	priority := frame.(*PriorityFrame)
	c.Check(priority.PriorityGroup, gc.Equals, uint32(0))
	c.Check(priority.PriorityWeight, gc.Equals, uint8(0))
	c.Check(priority.ExclusiveDependency, gc.Equals, false)
	c.Check(priority.StreamDependency, gc.Equals, StreamID(0x10203040))
}

func (t *ParserTest) TestValidExclusiveStreamDependency(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x04, byte(PRIORITY), byte(PRIORITY_DEPENDENCY),
		0x01, 0x02, 0x03, 0x04,
		0x90, 0x20, 0x30, 0x40,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.IsNil)

	priority := frame.(*PriorityFrame)
	c.Check(priority.PriorityGroup, gc.Equals, uint32(0))
	c.Check(priority.PriorityWeight, gc.Equals, uint8(0))
	c.Check(priority.ExclusiveDependency, gc.Equals, true)
	c.Check(priority.StreamDependency, gc.Equals, StreamID(0x10203040))
}

func (t *ParserTest) TestInvalidPriorityGroupIdAndDependency(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x09, byte(PRIORITY), byte(PRIORITY_GROUP | PRIORITY_DEPENDENCY),
		0x01, 0x02, 0x03, 0x04,
		0xff, 0x20, 0x30, 0x40,
		0x50,
		0x90, 0x20, 0x30, 0x40,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.ErrorMatches,
		"both PRIORITY_GROUP and PRIORITY_DEPENDENCY set")
	c.Check(err.Code, gc.Equals, PROTOCOL_ERROR)
	c.Check(frame, gc.IsNil)
}

func (t *ParserTest) TestValidHeadersFragmentWithoutEndHeaders(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x04, byte(HEADERS), byte(NO_FLAGS),
		0x01, 0x02, 0x03, 0x04,
		0xf1, 0xf2, 0xf3, 0xf4,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.IsNil)
	headers := frame.(*HeadersFrame)

	// Expect the fragment was read by the ParserTest decoder mock.
	c.Check(headers.Fields, gc.DeepEquals, []HeaderField{
		HeaderField{Name: "fragment", Values: "\xf1\xf2\xf3\xf4"}})

	c.Check(t.parser.expectContinuation, gc.Equals, true)
}

func (t *ParserTest) TestValidHeadersFragmentWithEndHeaders(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x04, byte(HEADERS), byte(END_HEADERS),
		0x01, 0x02, 0x03, 0x04,
		0xf1, 0xf2, 0xf3, 0xf4,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.IsNil)
	headers := frame.(*HeadersFrame)

	// Expect the fragment was read, and the header block was completed.
	c.Check(headers.Fields, gc.DeepEquals, []HeaderField{
		HeaderField{Name: "fragment", Values: "\xf1\xf2\xf3\xf4"},
		HeaderField{Name: "cookie", Values: "bar=baz; bing;"}})

	c.Check(t.parser.expectContinuation, gc.Equals, false)
}

func (t *ParserTest) TestInvalidHeadersFragmentUnderflow(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x04, byte(HEADERS), byte(NO_FLAGS),
		0x01, 0x02, 0x03, 0x04,
		0xf1, 0xf2, 0xf3,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.ErrorMatches, "decoder fragment underflow")
	c.Check(err.Code, gc.Equals, PROTOCOL_ERROR)
	c.Check(frame, gc.IsNil)
}

func (t *ParserTest) TestValidDataFrame(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x0a, byte(DATA), byte(PAD_LOW | END_SEGMENT),
		0x01, 0x02, 0x03, 0x04,
		0x04, 0xd1, 0xd2, 0xd3,
		0xd4, 0xd5, 0xa1, 0xa2,
		0xa3, 0xa4,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.IsNil)
	data := frame.(*DataFrame)

	c.Check(data.Flags, gc.Equals, PAD_LOW|END_SEGMENT)
	c.Check(data.StreamID, gc.Equals, StreamID(0x01020304))
	c.Check(data.PaddingLength, gc.Equals, uint(4))
	c.Check(data.Data, gc.DeepEquals, []byte{0xd1, 0xd2, 0xd3, 0xd4, 0xd5})
}

func (t *ParserTest) TestValidHeadersFrame(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x10, byte(HEADERS), byte(PAD_LOW | PRIORITY_GROUP | END_HEADERS),
		0x01, 0x02, 0x03, 0x04,
		0x05, 0x10, 0x20, 0x30,
		0x40, 0x50, 0xf1, 0xf2,
		0xf3, 0xf4, 0xf5, 0xa1,
		0xa2, 0xa3, 0xa4, 0xa5,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.IsNil)
	headers := frame.(*HeadersFrame)

	c.Check(headers.Flags, gc.Equals, PAD_LOW|PRIORITY_GROUP|END_HEADERS)
	c.Check(headers.StreamID, gc.Equals, StreamID(0x01020304))
	c.Check(headers.PaddingLength, gc.Equals, uint(5))
	c.Check(headers.PriorityGroup, gc.Equals, uint32(0x10203040))
	c.Check(headers.PriorityWeight, gc.Equals, uint8(0x50))
	c.Check(headers.Fields, gc.DeepEquals, []HeaderField{
		HeaderField{Name: "fragment", Values: "\xf1\xf2\xf3\xf4\xf5"},
		HeaderField{Name: "cookie", Values: "bar=baz; bing;"}})
}

func (t *ParserTest) TestInvalidHeadersFrameUnderflow(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x01, byte(HEADERS), byte(PAD_LOW | PAD_HIGH),
		0x01, 0x02, 0x03, 0x04,
		0x00, 0x00,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.ErrorMatches,
		"reached premature frame end reading \\*uint8")
	c.Check(err.Code, gc.Equals, FRAME_SIZE_ERROR)
	c.Check(frame, gc.IsNil)
}

func (t *ParserTest) TestInvalidPriorityFrameWithoutFlags(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x00, byte(PRIORITY), byte(NO_FLAGS),
		0x01, 0x02, 0x03, 0x04,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.ErrorMatches,
		"PRIORITY must have PRIORITY_GROUP or PRIORITY_DEPENDENCY set")
	c.Check(err.Code, gc.Equals, PROTOCOL_ERROR)
	c.Check(frame, gc.IsNil)
}

func (t *ParserTest) TestValidRstStreamFrame(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x04, byte(RST_STREAM), byte(NO_FLAGS),
		0x01, 0x02, 0x03, 0x04,
		0x00, 0x00, 0x00, 0x11,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.IsNil)
	rstStream := frame.(*RstStreamFrame)

	c.Check(rstStream.Flags, gc.Equals, NO_FLAGS)
	c.Check(rstStream.StreamID, gc.Equals, StreamID(0x01020304))
	c.Check(rstStream.Code, gc.Equals, ENHANCE_YOUR_CALM)
}

func (t *ParserTest) TestInvalidRstStreamWithStreamIDZero(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x04, byte(RST_STREAM), byte(NO_FLAGS),
		0x00, 0x00, 0x00, 0x00,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.ErrorMatches, "RST_STREAM must have non-zero StreamID")
	c.Check(err.Code, gc.Equals, PROTOCOL_ERROR)
	c.Check(frame, gc.IsNil)
}

func (t *ParserTest) TestInvalidRstStreamFrameUnderflow(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x03, byte(RST_STREAM), byte(NO_FLAGS),
		0x01, 0x02, 0x03, 0x04,
		0xaa, 0xaa, 0xaa, 0xaa,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.ErrorMatches,
		"reached premature frame end reading \\*http2.ErrorCode")
	c.Check(err.Code, gc.Equals, FRAME_SIZE_ERROR)
	c.Check(frame, gc.IsNil)
}

func (t *ParserTest) TestValidSettingsFrameWithPayload(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x14, byte(SETTINGS), byte(NO_FLAGS),
		0x00, 0x00, 0x00, 0x00,
		byte(SETTINGS_HEADER_TABLE_SIZE),
		0x01, 0x23, 0x45, 0x67,
		byte(SETTINGS_INITIAL_WINDOW_SIZE),
		0x89, 0x1a, 0xbc, 0xde,
		byte(SETTINGS_ENABLE_PUSH),
		0x00, 0x00, 0x00, 0x01,
		byte(SETTINGS_MAX_CONCURRENT_STREAMS),
		0x00, 0x00, 0x10, 0x00,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.IsNil)
	settings := frame.(*SettingsFrame)

	c.Check(settings.Flags, gc.Equals, NO_FLAGS)
	c.Check(settings.StreamID, gc.Equals, StreamID(0x0))
	c.Check(settings.Settings, gc.DeepEquals, map[SettingID]uint32{
		SETTINGS_HEADER_TABLE_SIZE:      uint32(0x01234567),
		SETTINGS_INITIAL_WINDOW_SIZE:    uint32(0x891abcde),
		SETTINGS_ENABLE_PUSH:            uint32(0x1),
		SETTINGS_MAX_CONCURRENT_STREAMS: uint32(4096),
	})
}

func (t *ParserTest) TestValidSettingsFrameWithAck(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x00, byte(SETTINGS), byte(ACK),
		0x00, 0x00, 0x00, 0x00,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.IsNil)
	settings := frame.(*SettingsFrame)

	c.Check(settings.Flags, gc.Equals, ACK)
	c.Check(settings.StreamID, gc.Equals, StreamID(0x0))
	c.Check(settings.Settings, gc.DeepEquals, map[SettingID]uint32{})
}

func (t *ParserTest) TestInvalidSettingsStreamID(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x00, byte(SETTINGS), byte(ACK),
		0x00, 0x00, 0x00, 0x01,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.ErrorMatches, "invalid SETTINGS StreamID 0x1")
	c.Check(err.Code, gc.Equals, PROTOCOL_ERROR)
	c.Check(frame, gc.IsNil)
}

func (t *ParserTest) TestInvalidSettingsWithAckAndPayload(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x05, byte(SETTINGS), byte(ACK),
		0x00, 0x00, 0x00, 0x00,
		byte(SETTINGS_HEADER_TABLE_SIZE),
		0x01, 0x23, 0x45, 0x67,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.ErrorMatches, "SETTINGS with ACK must have empty payload")
	c.Check(err.Code, gc.Equals, FRAME_SIZE_ERROR)
	c.Check(frame, gc.IsNil)
}

func (t *ParserTest) TestInvalidSettingsFrameUnderflow(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x04, byte(SETTINGS), byte(NO_FLAGS),
		0x00, 0x00, 0x00, 0x00,
		byte(SETTINGS_HEADER_TABLE_SIZE),
		0x01, 0x23, 0x45, 0x67,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.ErrorMatches,
		"invalid SETTINGS payload \\(length % 5 != 0\\)")
	c.Check(err.Code, gc.Equals, FRAME_SIZE_ERROR)
	c.Check(frame, gc.IsNil)
}

func (t *ParserTest) TestInvalidSettingsWithHighSettingID(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x05, byte(SETTINGS), byte(NO_FLAGS),
		0x00, 0x00, 0x00, 0x00,
		byte(SETTINGS_MAX_SETTING_ID + 1),
		0x01, 0x23, 0x45, 0x67,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.ErrorMatches, "invalid setting ID 0x\\w+")
	c.Check(err.Code, gc.Equals, PROTOCOL_ERROR)
	c.Check(frame, gc.IsNil)
}

func (t *ParserTest) TestInvalidSettingsWithLowSettingID(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x05, byte(SETTINGS), byte(NO_FLAGS),
		0x00, 0x00, 0x00, 0x00,
		0x00,
		0x01, 0x23, 0x45, 0x67,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.ErrorMatches, "invalid setting ID 0x0")
	c.Check(err.Code, gc.Equals, PROTOCOL_ERROR)
	c.Check(frame, gc.IsNil)
}

func (t *ParserTest) TestInvalidSettingsWithBadEnablePushValue(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x05, byte(SETTINGS), byte(NO_FLAGS),
		0x00, 0x00, 0x00, 0x00,
		byte(SETTINGS_ENABLE_PUSH),
		0x00, 0x00, 0x00, 0x02,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.ErrorMatches,
		"invalid setting for SETTINGS_ENABLE_PUSH \\(must be 0 or 1\\)")
	c.Check(err.Code, gc.Equals, PROTOCOL_ERROR)
	c.Check(frame, gc.IsNil)
}

func (t *ParserTest) TestValidPushPromiseFrame(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x0c, byte(PUSH_PROMISE), byte(PAD_LOW | END_HEADERS),
		0x01, 0x02, 0x03, 0x04,
		0x02, 0x10, 0x20, 0x30,
		0x40, 0xf1, 0xf2, 0xf3,
		0xf4, 0xf5, 0xa1, 0xa2,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.IsNil)
	promise := frame.(*PushPromiseFrame)

	c.Check(promise.Flags, gc.Equals, PAD_LOW|END_HEADERS)
	c.Check(promise.StreamID, gc.Equals, StreamID(0x01020304))
	c.Check(promise.PromisedID, gc.Equals, StreamID(0x10203040))
	c.Check(promise.Fields, gc.DeepEquals, []HeaderField{
		HeaderField{Name: "fragment", Values: "\xf1\xf2\xf3\xf4\xf5"},
		HeaderField{Name: "cookie", Values: "bar=baz; bing;"}})
}

func (t *ParserTest) TestInvalidPushPromiseFrameUnderflow(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x03, byte(PUSH_PROMISE), byte(NO_FLAGS),
		0x01, 0x02, 0x03, 0x04,
		0x02, 0x10, 0x20, 0x30,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.ErrorMatches,
		"reached premature frame end reading \\*http2.StreamID")
	c.Check(err.Code, gc.Equals, FRAME_SIZE_ERROR)
	c.Check(frame, gc.IsNil)
}

func (t *ParserTest) TestInvalidPushPromiseFrameWithZeroPromisedId(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x04, byte(PUSH_PROMISE), byte(NO_FLAGS),
		0x01, 0x02, 0x03, 0x04,
		0x00, 0x00, 0x00, 0x00,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.ErrorMatches, "promised StreamID must be nonzero")
	c.Check(err.Code, gc.Equals, PROTOCOL_ERROR)
	c.Check(frame, gc.IsNil)
}

func (t *ParserTest) TestInvalidPushPromiseFrameWithBadPromisedId(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x04, byte(PUSH_PROMISE), byte(NO_FLAGS),
		0x01, 0x02, 0x03, 0x04,
		0xff, 0x00, 0x00, 0x00,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.ErrorMatches, "promised StreamID has reserved bit set")
	c.Check(err.Code, gc.Equals, PROTOCOL_ERROR)
	c.Check(frame, gc.IsNil)
}

func (t *ParserTest) TestValidPingFrame(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x08, byte(PING), byte(ACK),
		0x01, 0x02, 0x03, 0x04,
		0x55, 0x66, 0x77, 0x88,
		0x99, 0xaa, 0xbb, 0xcc,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.IsNil)
	ping := frame.(*PingFrame)

	c.Check(ping.Flags, gc.Equals, ACK)
	c.Check(ping.StreamID, gc.Equals, StreamID(0x01020304))
	c.Check(ping.OpaqueData, gc.Equals, uint64(0x5566778899aabbcc))
}

func (t *ParserTest) TestInvalidPingFrameUnderflow(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x07, byte(PING), byte(ACK),
		0x01, 0x02, 0x03, 0x04,
		0x55, 0x66, 0x77, 0x88,
		0x99, 0xaa, 0xbb, 0xcc,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.ErrorMatches,
		"reached premature frame end reading \\*uint64")
	c.Check(err.Code, gc.Equals, FRAME_SIZE_ERROR)
	c.Check(frame, gc.IsNil)
}

func (t *ParserTest) TestValidGoAwayFrame(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x0b, byte(GOAWAY), byte(NO_FLAGS),
		0x00, 0x00, 0x00, 0x00,
		0x10, 0x20, 0x30, 0x40,
		0x00, 0x00, 0x00, 0x11,
		0xd1, 0xd2, 0xd3,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.IsNil)
	goAway := frame.(*GoAwayFrame)

	c.Check(goAway.Flags, gc.Equals, NO_FLAGS)
	c.Check(goAway.StreamID, gc.Equals, StreamID(0x0))
	c.Check(goAway.LastID, gc.Equals, StreamID(0x10203040))
	c.Check(goAway.Code, gc.Equals, ENHANCE_YOUR_CALM)
	c.Check(goAway.Debug, gc.DeepEquals, []byte{0xd1, 0xd2, 0xd3})
}

func (t *ParserTest) TestInvalidGoAwayStreamID(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x0b, byte(GOAWAY), byte(NO_FLAGS),
		0x00, 0x00, 0x00, 0x01,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.ErrorMatches, "invalid GOAWAY StreamID 0x1")
	c.Check(err.Code, gc.Equals, PROTOCOL_ERROR)
	c.Check(frame, gc.IsNil)
}

func (t *ParserTest) TestInvalidGoAwayLastStreamID(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x0b, byte(GOAWAY), byte(NO_FLAGS),
		0x00, 0x00, 0x00, 0x00,
		0xff, 0x20, 0x30, 0x40,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.ErrorMatches, "last StreamID has reserved bit set")
	c.Check(err.Code, gc.Equals, PROTOCOL_ERROR)
	c.Check(frame, gc.IsNil)
}

func (t *ParserTest) TestInvalidGoAwayFixedPayloadUnderflow(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x07, byte(GOAWAY), byte(NO_FLAGS),
		0x00, 0x00, 0x00, 0x00,
		0x10, 0x20, 0x30, 0x40,
		0x00, 0x00, 0x00, 0x11,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.ErrorMatches,
		"reached premature frame end reading \\*http2.ErrorCode")
	c.Check(err.Code, gc.Equals, FRAME_SIZE_ERROR)
	c.Check(frame, gc.IsNil)
}

func (t *ParserTest) TestInvalidGoAwayDynamicPayloadUnderflow(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x08, byte(GOAWAY), byte(NO_FLAGS),
		0x00, 0x00, 0x00, 0x00,
		0x10, 0x20, 0x30, 0x40,
		0x00, 0x00, 0x00,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.ErrorMatches, "unexpected EOF")
	c.Check(err.Code, gc.Equals, INTERNAL_ERROR)
	c.Check(frame, gc.IsNil)
}

func (t *ParserTest) TestValidWindowUpdate(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x04, byte(WINDOW_UPDATE), byte(NO_FLAGS),
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x10, 0x00,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.IsNil)
	windowUpdate := frame.(*WindowUpdateFrame)

	c.Check(windowUpdate.Flags, gc.Equals, NO_FLAGS)
	c.Check(windowUpdate.StreamID, gc.Equals, StreamID(0x0))
	c.Check(windowUpdate.SizeDelta, gc.Equals, uint32(4096))
}

func (t *ParserTest) TestInvalidWindowUpdateWithBadSizeDelta(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x04, byte(WINDOW_UPDATE), byte(NO_FLAGS),
		0x00, 0x00, 0x00, 0x00,
		0xff, 0x00, 0x10, 0x00,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.ErrorMatches, "reserved size delta bit is non-zero")
	c.Check(err.Code, gc.Equals, PROTOCOL_ERROR)
	c.Check(frame, gc.IsNil)
}

func (t *ParserTest) TestInvalidWindowUpdateUnderflow(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x03, byte(WINDOW_UPDATE), byte(NO_FLAGS),
		0x00, 0x00, 0x00, 0x00,
		0xff, 0x00, 0x10, 0x00,
	})
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.ErrorMatches, "reached premature frame end reading \\*uint32")
	c.Check(err.Code, gc.Equals, FRAME_SIZE_ERROR)
	c.Check(frame, gc.IsNil)
}

func (t *ParserTest) TestValidContinuationFrame(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x08, byte(CONTINUATION), byte(PAD_LOW | END_HEADERS),
		0x01, 0x02, 0x03, 0x04,
		0x02, 0xf1, 0xf2, 0xf3,
		0xf4, 0xf5, 0xa1, 0xa2,
	})
	t.parser.expectContinuation = true
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.IsNil)
	continuation := frame.(*ContinuationFrame)

	c.Check(continuation.Flags, gc.Equals, PAD_LOW|END_HEADERS)
	c.Check(continuation.StreamID, gc.Equals, StreamID(0x01020304))
	c.Check(continuation.Fields, gc.DeepEquals, []HeaderField{
		HeaderField{Name: "fragment", Values: "\xf1\xf2\xf3\xf4\xf5"},
		HeaderField{Name: "cookie", Values: "bar=baz; bing;"}})
}

func (t *ParserTest) TestInvalidContinuationUnexpected(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x00, byte(CONTINUATION), byte(NO_FLAGS),
	})
	t.parser.expectContinuation = false
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.ErrorMatches, "unexpected CONTINUATION")
	c.Check(err.Code, gc.Equals, PROTOCOL_ERROR)
	c.Check(frame, gc.IsNil)
}

func (t *ParserTest) TestInvalidContinuationExpected(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x00, byte(DATA), 0x00,
	})
	t.parser.expectContinuation = true
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.ErrorMatches, "expected CONTINUATION")
	c.Check(err.Code, gc.Equals, PROTOCOL_ERROR)
	c.Check(frame, gc.IsNil)
}

func (t *ParserTest) TestInvalidContinuationUnderflow(c *gc.C) {
	t.input.Write([]byte{
		0x00, 0x00, byte(CONTINUATION), byte(PAD_LOW),
		0x01, 0x02, 0x03, 0x04,
		0x00,
	})
	t.parser.expectContinuation = true
	frame, err := t.parser.ParseFrame()
	c.Check(err, gc.ErrorMatches,
		"reached premature frame end reading \\*uint8")
	c.Check(err.Code, gc.Equals, FRAME_SIZE_ERROR)
	c.Check(frame, gc.IsNil)
}

var _ = gc.Suite(&ParserTest{})
