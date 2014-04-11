// Copyright 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package http2

import (
	"bytes"

	. "gopkg.in/check.v1"
)

type ParserTest struct{}

func (t *ParserTest) TestInvalidFrameLength(c *C) {
	input := bytes.NewBuffer([]byte{
		0xff, 0xff, 0x00, 0x00,
	})
	frame, err := ParseFrame(input)
	c.Check(err, ErrorMatches, "Reserved length bits are non-zero")
	c.Check(err.Code, Equals, PROTOCOL_ERROR)
	c.Check(frame, IsNil)
}

func (t *ParserTest) TestInvalidFrameType(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0xff, 0xff, 0x00,
	})
	frame, err := ParseFrame(input)
	c.Check(err, ErrorMatches, "Invalid frame type 0xff")
	c.Check(err.Code, Equals, PROTOCOL_ERROR)
	c.Check(frame, IsNil)
}

func (t *ParserTest) TestInvalidStreamId(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x00, 0x00, 0x00,
		0xff, 0x02, 0x03, 0x04,
	})
	frame, err := ParseFrame(input)
	c.Check(err, ErrorMatches, "Reserved stream ID bit is non-zero")
	c.Check(err.Code, Equals, PROTOCOL_ERROR)
	c.Check(frame, IsNil)
}

func (t *ParserTest) TestValidPrefixNoFlags(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x00, 0x00, 0x00,
		0x01, 0x02, 0x03, 0x04,
	})
	frame, err := ParseFrame(input)
	c.Check(err, IsNil)

	data := frame.(*DataFrame)
	c.Check(data.Flags, Equals, NO_FLAGS)
	c.Check(data.Id, Equals, StreamId(0x01020304))
}

func (t *ParserTest) TestValidPrefixWithFlags(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x00, 0x00, byte(END_STREAM | END_SEGMENT),
		0x01, 0x02, 0x03, 0x04,
	})
	frame, err := ParseFrame(input)
	c.Check(err, IsNil)

	data := frame.(*DataFrame)
	c.Check(frame.GetFlags(), Equals, END_STREAM|END_SEGMENT)
	c.Check(data.Id, Equals, StreamId(0x01020304))
}

func (t *ParserTest) TestInvalidPrefixFlags(c *C) {
	input := bytes.NewBuffer([]byte{
		// PRIORITY_GROUP is not a valid DATA frame flag.
		0x00, 0x00, 0x00, byte(END_STREAM | PRIORITY_GROUP),
		0x01, 0x02, 0x03, 0x04,
	})
	frame, err := ParseFrame(input)
	c.Check(err, ErrorMatches, "Invalid flags 0x\\w+ for frame type 0x\\w+")
	c.Check(err.Code, Equals, PROTOCOL_ERROR)
	c.Check(frame, IsNil)
}

func (t *ParserTest) TestValidPaddingLowIsZero(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x01, byte(DATA), byte(PAD_LOW),
		0x01, 0x02, 0x03, 0x04,
		0x00,
	})
	frame, err := ParseFrame(input)
	c.Check(err, IsNil)

	data := frame.(*DataFrame)
	c.Check(frame.GetFlags(), Equals, PAD_LOW)
	c.Check(data.PaddingLength, Equals, uint(0))
}

func (t *ParserTest) TestValidPadLowAndHighAreZero(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x02, byte(DATA), byte(PAD_LOW | PAD_HIGH),
		0x01, 0x02, 0x03, 0x04,
		0x00, 0x00,
	})
	frame, err := ParseFrame(input)
	c.Check(err, IsNil)

	data := frame.(*DataFrame)
	c.Check(frame.GetFlags(), Equals, PAD_LOW|PAD_HIGH)
	c.Check(data.PaddingLength, Equals, uint(0))
}

func (t *ParserTest) TestValidPadLowIsNonzero(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x04, byte(DATA), byte(PAD_LOW),
		0x01, 0x02, 0x03, 0x04,
		0x03, 0xa1, 0xa2, 0xa3,
	})
	frame, err := ParseFrame(input)
	c.Check(err, IsNil)

	data := frame.(*DataFrame)
	c.Check(frame.GetFlags(), Equals, PAD_LOW)
	c.Check(data.PaddingLength, Equals, uint(3))
	// Remaining frame payload was discarded.
	c.Check(data.Data, DeepEquals, []byte{})
}

func (t *ParserTest) TestValidPadLowIsNonzeroAndPadHighIsZero(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x04, byte(DATA), byte(PAD_LOW | PAD_HIGH),
		0x01, 0x02, 0x03, 0x04,
		0x00, 0x02, 0xa1, 0xa2,
	})
	frame, err := ParseFrame(input)
	c.Check(err, IsNil)

	data := frame.(*DataFrame)
	c.Check(frame.GetFlags(), Equals, PAD_LOW|PAD_HIGH)
	c.Check(data.PaddingLength, Equals, uint(2))
	// Remaining frame payload was discarded.
	c.Check(data.Data, DeepEquals, []byte{})
}

func (t *ParserTest) TestValidPadLowAndPadHighAreNonzero(c *C) {
	input := bytes.NewBuffer([]byte{
		0x01, 0x05, byte(DATA), byte(PAD_LOW | PAD_HIGH),
		0x01, 0x02, 0x03, 0x04,
		0x01, 0x03,
	})
	for i := 0; i != 259; i++ {
		input.WriteByte(0xff)
	}

	frame, err := ParseFrame(input)
	c.Check(err, IsNil)

	data := frame.(*DataFrame)
	c.Check(frame.GetFlags(), Equals, PAD_LOW|PAD_HIGH)
	c.Check(data.PaddingLength, Equals, uint(259))
	// Remaining frame payload was discarded.
	c.Check(data.Data, DeepEquals, []byte{})
}

func (t *ParserTest) TestInvalidPadHighWithoutLow(c *C) {
	input := bytes.NewBuffer([]byte{
		0x01, 0x01, byte(DATA), byte(PAD_HIGH),
		0x01, 0x02, 0x03, 0x04,
		0x00,
	})
	frame, err := ParseFrame(input)
	c.Check(err, ErrorMatches, "PAD_HIGH set without PAD_LOW")
	c.Check(err.Code, Equals, PROTOCOL_ERROR)
	c.Check(frame, IsNil)
}

func (t *ParserTest) TestInvalidPadLength(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x04, byte(DATA), byte(PAD_LOW),
		0x01, 0x02, 0x03, 0x04,
		0x04, 0xa1, 0xa2, 0xa3,
		0xa4,
	})
	frame, err := ParseFrame(input)
	// Though |input| is sufficiently sized, the frame length
	// limit is hit before all padding can be read.
	c.Check(err, ErrorMatches,
		"Padding of 4 is longer than remaining frame length 3")
	c.Check(err.Code, Equals, FRAME_SIZE_ERROR)
	c.Check(frame, IsNil)
}

func (t *ParserTest) TestInvalidFixedPayloadOverflow(c *C) {
	// All fixed-sized frame types check that the complete frame was consumed.
	input := bytes.NewBuffer([]byte{
		0x00, 0x05, byte(RST_STREAM), byte(NO_FLAGS),
		0x01, 0x02, 0x03, 0x04,
		0xaa, 0xaa, 0xaa, 0xaa,
		0xff,
	})
	frame, err := ParseFrame(input)
	c.Check(err, ErrorMatches, "Unexpected 1 bytes of frame remainder")
	c.Check(err.Code, Equals, FRAME_SIZE_ERROR)
	c.Check(frame, IsNil)
}

func (t *ParserTest) TestInvalidDynamicPayloadUnderflow(c *C) {
	// Simulates a broken connection.
	input := bytes.NewBuffer([]byte{
		0x00, 0x06, byte(DATA), byte(NO_FLAGS),
		0x01, 0x02, 0x03, 0x04,
		0xd1, 0xd2, 0xd3, 0xd4,
		0xd5,
	})
	frame, err := ParseFrame(input)
	c.Check(err, ErrorMatches, "unexpected EOF")
	c.Check(err.Code, Equals, INTERNAL_ERROR)
	c.Check(frame, IsNil)
}

func (t *ParserTest) TestValidPriorityGroupId(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x05, byte(PRIORITY), byte(PRIORITY_GROUP),
		0x01, 0x02, 0x03, 0x04,
		0x10, 0x20, 0x30, 0x40,
		0x50,
	})
	frame, err := ParseFrame(input)
	c.Check(err, IsNil)

	priority := frame.(*PriorityFrame)
	c.Check(priority.PriorityGroup, Equals, uint32(0x10203040))
	c.Check(priority.PriorityWeight, Equals, uint8(0x50))
	c.Check(priority.ExclusiveDependency, Equals, false)
	c.Check(priority.StreamDependency, Equals, StreamId(0))
}

func (t *ParserTest) TestInvalidPriorityGroupId(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x05, byte(PRIORITY), byte(PRIORITY_GROUP),
		0x01, 0x02, 0x03, 0x04,
		0xff, 0x20, 0x30, 0x40,
		0x50,
	})
	frame, err := ParseFrame(input)
	c.Check(err, ErrorMatches, "Reserved priority group bit is non-zero")
	c.Check(err.Code, Equals, PROTOCOL_ERROR)
	c.Check(frame, IsNil)
}

func (t *ParserTest) TestValidNonexclusiveStreamDependency(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x04, byte(PRIORITY), byte(PRIORITY_DEPENDENCY),
		0x01, 0x02, 0x03, 0x04,
		0x10, 0x20, 0x30, 0x40,
	})
	frame, err := ParseFrame(input)
	c.Check(err, IsNil)

	priority := frame.(*PriorityFrame)
	c.Check(priority.PriorityGroup, Equals, uint32(0))
	c.Check(priority.PriorityWeight, Equals, uint8(0))
	c.Check(priority.ExclusiveDependency, Equals, false)
	c.Check(priority.StreamDependency, Equals, StreamId(0x10203040))
}

func (t *ParserTest) TestValidExclusiveStreamDependency(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x04, byte(PRIORITY), byte(PRIORITY_DEPENDENCY),
		0x01, 0x02, 0x03, 0x04,
		0x90, 0x20, 0x30, 0x40,
	})
	frame, err := ParseFrame(input)
	c.Check(err, IsNil)

	priority := frame.(*PriorityFrame)
	c.Check(priority.PriorityGroup, Equals, uint32(0))
	c.Check(priority.PriorityWeight, Equals, uint8(0))
	c.Check(priority.ExclusiveDependency, Equals, true)
	c.Check(priority.StreamDependency, Equals, StreamId(0x10203040))
}

func (t *ParserTest) TestInvalidPriorityGroupIdAndDependency(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x09, byte(PRIORITY), byte(PRIORITY_GROUP | PRIORITY_DEPENDENCY),
		0x01, 0x02, 0x03, 0x04,
		0xff, 0x20, 0x30, 0x40,
		0x50,
		0x90, 0x20, 0x30, 0x40,
	})
	frame, err := ParseFrame(input)
	c.Check(err, ErrorMatches,
		"Both PRIORITY_GROUP and PRIORITY_DEPENDENCY set")
	c.Check(err.Code, Equals, PROTOCOL_ERROR)
	c.Check(frame, IsNil)
}

func (t *ParserTest) TestValidDataFrame(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x0a, byte(DATA), byte(PAD_LOW | END_SEGMENT),
		0x01, 0x02, 0x03, 0x04,
		0x04, 0xd1, 0xd2, 0xd3,
		0xd4, 0xd5, 0xa1, 0xa2,
		0xa3, 0xa4,
	})
	frame, err := ParseFrame(input)
	c.Check(err, IsNil)
	data := frame.(*DataFrame)

	c.Check(data.Flags, Equals, PAD_LOW|END_SEGMENT)
	c.Check(data.Id, Equals, StreamId(0x01020304))
	c.Check(data.PaddingLength, Equals, uint(4))
	c.Check(data.Data, DeepEquals, []byte{0xd1, 0xd2, 0xd3, 0xd4, 0xd5})
}

func (t *ParserTest) TestValidHeadersFrame(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x10, byte(HEADERS), byte(PAD_LOW | PRIORITY_GROUP),
		0x01, 0x02, 0x03, 0x04,
		0x05, 0x10, 0x20, 0x30,
		0x40, 0x50, 0xf1, 0xf2,
		0xf3, 0xf4, 0xf5, 0xa1,
		0xa2, 0xa3, 0xa4, 0xa5,
	})
	frame, err := ParseFrame(input)
	c.Check(err, IsNil)
	headers := frame.(*HeadersFrame)

	c.Check(headers.Flags, Equals, PAD_LOW|PRIORITY_GROUP)
	c.Check(headers.Id, Equals, StreamId(0x01020304))
	c.Check(headers.PaddingLength, Equals, uint(5))
	c.Check(headers.PriorityGroup, Equals, uint32(0x10203040))
	c.Check(headers.PriorityWeight, Equals, uint8(0x50))
	c.Check(headers.Fragment, DeepEquals, []byte{0xf1, 0xf2, 0xf3, 0xf4, 0xf5})
}

func (t *ParserTest) TestInvalidHeadersUnderflow(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x01, byte(HEADERS), byte(PAD_LOW | PAD_HIGH),
		0x01, 0x02, 0x03, 0x04,
		0x00, 0x00,
	})
	frame, err := ParseFrame(input)
	c.Check(err, ErrorMatches,
		"Reached frame end while reading fixed-size payload")
	c.Check(err.Code, Equals, FRAME_SIZE_ERROR)
	c.Check(frame, IsNil)
}

func (t *ParserTest) TestInvalidPriorityFrameWithoutFlags(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x00, byte(PRIORITY), byte(NO_FLAGS),
		0x01, 0x02, 0x03, 0x04,
	})
	frame, err := ParseFrame(input)
	c.Check(err, ErrorMatches,
		"PRIORITY must have PRIORITY_GROUP or PRIORITY_DEPENDENCY set")
	c.Check(err.Code, Equals, PROTOCOL_ERROR)
	c.Check(frame, IsNil)
}

func (t *ParserTest) TestValidRstStreamFrame(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x04, byte(RST_STREAM), byte(NO_FLAGS),
		0x01, 0x02, 0x03, 0x04,
		0x00, 0x00, 0x00, 0x11,
	})
	frame, err := ParseFrame(input)
	c.Check(err, IsNil)
	rstStream := frame.(*RstStreamFrame)

	c.Check(rstStream.Flags, Equals, NO_FLAGS)
	c.Check(rstStream.Id, Equals, StreamId(0x01020304))
	c.Check(rstStream.Code, Equals, ENHANCE_YOUR_CALM)
}

func (t *ParserTest) TestInvalidRstStreamWithStreamIdZero(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x04, byte(RST_STREAM), byte(NO_FLAGS),
		0x00, 0x00, 0x00, 0x00,
	})
	frame, err := ParseFrame(input)
	c.Check(err, ErrorMatches, "RST_STREAM must have non-zero stream ID")
	c.Check(err.Code, Equals, PROTOCOL_ERROR)
	c.Check(frame, IsNil)
}

func (t *ParserTest) TestInvalidRstStreamUnderflow(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x03, byte(RST_STREAM), byte(NO_FLAGS),
		0x01, 0x02, 0x03, 0x04,
		0xaa, 0xaa, 0xaa, 0xaa,
	})
	frame, err := ParseFrame(input)
	c.Check(err, ErrorMatches,
		"Reached frame end while reading fixed-size payload")
	c.Check(err.Code, Equals, FRAME_SIZE_ERROR)
	c.Check(frame, IsNil)
}

func (t *ParserTest) TestValidSettingsFrameWithPayload(c *C) {
	input := bytes.NewBuffer([]byte{
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
	frame, err := ParseFrame(input)
	c.Check(err, IsNil)
	settings := frame.(*SettingsFrame)

	c.Check(settings.Flags, Equals, NO_FLAGS)
	c.Check(settings.Id, Equals, StreamId(0x0))
	c.Check(settings.Settings, DeepEquals, map[SettingId]uint32{
		SETTINGS_HEADER_TABLE_SIZE:      uint32(0x01234567),
		SETTINGS_INITIAL_WINDOW_SIZE:    uint32(0x891abcde),
		SETTINGS_ENABLE_PUSH:            uint32(0x1),
		SETTINGS_MAX_CONCURRENT_STREAMS: uint32(4096),
	})
}

func (t *ParserTest) TestValidSettingsFrameWithAck(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x00, byte(SETTINGS), byte(ACK),
		0x00, 0x00, 0x00, 0x00,
	})
	frame, err := ParseFrame(input)
	c.Check(err, IsNil)
	settings := frame.(*SettingsFrame)

	c.Check(settings.Flags, Equals, ACK)
	c.Check(settings.Id, Equals, StreamId(0x0))
	c.Check(settings.Settings, DeepEquals, map[SettingId]uint32{})
}

func (t *ParserTest) TestInvalidSettingsStreamId(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x00, byte(SETTINGS), byte(ACK),
		0x00, 0x00, 0x00, 0x01,
	})
	frame, err := ParseFrame(input)
	c.Check(err, ErrorMatches, "Invalid SETTINGS StreamId 0x1")
	c.Check(err.Code, Equals, PROTOCOL_ERROR)
	c.Check(frame, IsNil)
}

func (t *ParserTest) TestInvalidSettingsWithAckAndPayload(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x05, byte(SETTINGS), byte(ACK),
		0x00, 0x00, 0x00, 0x00,
		byte(SETTINGS_HEADER_TABLE_SIZE),
		0x01, 0x23, 0x45, 0x67,
	})
	frame, err := ParseFrame(input)
	c.Check(err, ErrorMatches, "SETTINGS with ACK must have empty payload")
	c.Check(err.Code, Equals, FRAME_SIZE_ERROR)
	c.Check(frame, IsNil)
}

func (t *ParserTest) TestInvalidSettingsUnderflow(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x04, byte(SETTINGS), byte(NO_FLAGS),
		0x00, 0x00, 0x00, 0x00,
		byte(SETTINGS_HEADER_TABLE_SIZE),
		0x01, 0x23, 0x45, 0x67,
	})
	frame, err := ParseFrame(input)
	c.Check(err, ErrorMatches,
		"Invalid SETTINGS payload size \\(not modulo 5\\)")
	c.Check(err.Code, Equals, FRAME_SIZE_ERROR)
	c.Check(frame, IsNil)
}

func (t *ParserTest) TestInvalidSettingsWithUnknownSettingId(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x05, byte(SETTINGS), byte(NO_FLAGS),
		0x00, 0x00, 0x00, 0x00,
		byte(SETTINGS_MAX_SETTING_ID + 1),
		0x01, 0x23, 0x45, 0x67,
	})
	frame, err := ParseFrame(input)
	c.Check(err, ErrorMatches, "Invalid setting ID 0x\\w+")
	c.Check(err.Code, Equals, PROTOCOL_ERROR)
	c.Check(frame, IsNil)

	input = bytes.NewBuffer([]byte{
		0x00, 0x05, byte(SETTINGS), byte(NO_FLAGS),
		0x00, 0x00, 0x00, 0x00,
		0x00,
		0x01, 0x23, 0x45, 0x67,
	})
	frame, err = ParseFrame(input)
	c.Check(err, ErrorMatches, "Invalid setting ID 0x0")
	c.Check(err.Code, Equals, PROTOCOL_ERROR)
	c.Check(frame, IsNil)
}

func (t *ParserTest) TestInvalidSettingsWithBadEnablePushValue(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x05, byte(SETTINGS), byte(NO_FLAGS),
		0x00, 0x00, 0x00, 0x00,
		byte(SETTINGS_ENABLE_PUSH),
		0x00, 0x00, 0x00, 0x02,
	})
	frame, err := ParseFrame(input)
	c.Check(err, ErrorMatches,
		"Invalid setting for SETTINGS_ENABLE_PUSH \\(must be 0 or 1\\)")
	c.Check(err.Code, Equals, PROTOCOL_ERROR)
	c.Check(frame, IsNil)
}

func (t *ParserTest) TestValidPushPromiseFrame(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x0c, byte(PUSH_PROMISE), byte(PAD_LOW | END_HEADERS),
		0x01, 0x02, 0x03, 0x04,
		0x02, 0x10, 0x20, 0x30,
		0x40, 0xf1, 0xf2, 0xf3,
		0xf4, 0xf5, 0xa1, 0xa2,
	})
	frame, err := ParseFrame(input)
	c.Check(err, IsNil)
	promise := frame.(*PushPromiseFrame)

	c.Check(promise.Flags, Equals, PAD_LOW|END_HEADERS)
	c.Check(promise.Id, Equals, StreamId(0x01020304))
	c.Check(promise.PromisedId, Equals, StreamId(0x10203040))
	c.Check(promise.Fragment, DeepEquals, []byte{0xf1, 0xf2, 0xf3, 0xf4, 0xf5})
}

func (t *ParserTest) TestInvalidPushPromiseUnderflow(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x03, byte(PUSH_PROMISE), byte(NO_FLAGS),
		0x01, 0x02, 0x03, 0x04,
		0x02, 0x10, 0x20, 0x30,
	})
	frame, err := ParseFrame(input)
	c.Check(err, ErrorMatches,
		"Reached frame end while reading fixed-size payload")
	c.Check(err.Code, Equals, FRAME_SIZE_ERROR)
	c.Check(frame, IsNil)
}

func (t *ParserTest) TestInvalidPushPromiseFrameWithZeroPromisedId(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x04, byte(PUSH_PROMISE), byte(NO_FLAGS),
		0x01, 0x02, 0x03, 0x04,
		0x00, 0x00, 0x00, 0x00,
	})
	frame, err := ParseFrame(input)
	c.Check(err, ErrorMatches, "Promised stream ID must be nonzero")
	c.Check(err.Code, Equals, PROTOCOL_ERROR)
	c.Check(frame, IsNil)
}

func (t *ParserTest) TestValidPingFrame(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x08, byte(PING), byte(ACK),
		0x01, 0x02, 0x03, 0x04,
		0x55, 0x66, 0x77, 0x88,
		0x99, 0xaa, 0xbb, 0xcc,
	})
	frame, err := ParseFrame(input)
	c.Check(err, IsNil)
	ping := frame.(*PingFrame)

	c.Check(ping.Flags, Equals, ACK)
	c.Check(ping.Id, Equals, StreamId(0x01020304))
	c.Check(ping.OpaqueData, Equals, uint64(0x5566778899aabbcc))
}

func (t *ParserTest) TestInvalidPingFrameUnderflow(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x07, byte(PING), byte(ACK),
		0x01, 0x02, 0x03, 0x04,
		0x55, 0x66, 0x77, 0x88,
		0x99, 0xaa, 0xbb, 0xcc,
	})
	frame, err := ParseFrame(input)
	c.Check(err, ErrorMatches,
		"Reached frame end while reading fixed-size payload")
	c.Check(err.Code, Equals, FRAME_SIZE_ERROR)
	c.Check(frame, IsNil)
}

func (t *ParserTest) TestValidGoAwayFrame(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x0b, byte(GOAWAY), byte(NO_FLAGS),
		0x00, 0x00, 0x00, 0x00,
		0x10, 0x20, 0x30, 0x40,
		0x00, 0x00, 0x00, 0x11,
		0xd1, 0xd2, 0xd3,
	})
	frame, err := ParseFrame(input)
	c.Check(err, IsNil)
	goAway := frame.(*GoAwayFrame)

	c.Check(goAway.Flags, Equals, NO_FLAGS)
	c.Check(goAway.Id, Equals, StreamId(0x0))
	c.Check(goAway.LastStream, Equals, StreamId(0x10203040))
	c.Check(goAway.Code, Equals, ENHANCE_YOUR_CALM)
	c.Check(goAway.Debug, DeepEquals, []byte{0xd1, 0xd2, 0xd3})
}

func (t *ParserTest) TestInvalidGoAwayStreamId(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x0b, byte(GOAWAY), byte(NO_FLAGS),
		0x00, 0x00, 0x00, 0x01,
	})
	frame, err := ParseFrame(input)
	c.Check(err, ErrorMatches, "Invalid GOAWAY StreamId 0x1")
	c.Check(err.Code, Equals, PROTOCOL_ERROR)
	c.Check(frame, IsNil)
}

func (t *ParserTest) TestInvalidGoAwayLastStreamId(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x0b, byte(GOAWAY), byte(NO_FLAGS),
		0x00, 0x00, 0x00, 0x00,
		0xff, 0x20, 0x30, 0x40,
	})
	frame, err := ParseFrame(input)
	c.Check(err, ErrorMatches, "Reserved stream ID bit is non-zero")
	c.Check(err.Code, Equals, PROTOCOL_ERROR)
	c.Check(frame, IsNil)
}

func (t *ParserTest) TestInvalidGoAwayFixedPayloadUnderflow(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x07, byte(GOAWAY), byte(NO_FLAGS),
		0x00, 0x00, 0x00, 0x00,
		0x10, 0x20, 0x30, 0x40,
		0x00, 0x00, 0x00, 0x11,
	})
	frame, err := ParseFrame(input)
	c.Check(err, ErrorMatches,
		"Reached frame end while reading fixed-size payload")
	c.Check(err.Code, Equals, FRAME_SIZE_ERROR)
	c.Check(frame, IsNil)
}

func (t *ParserTest) TestInvalidGoAwayDynamicPayloadUnderflow(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x08, byte(GOAWAY), byte(NO_FLAGS),
		0x00, 0x00, 0x00, 0x00,
		0x10, 0x20, 0x30, 0x40,
		0x00, 0x00, 0x00,
	})
	frame, err := ParseFrame(input)
	c.Check(err, ErrorMatches, "unexpected EOF")
	c.Check(err.Code, Equals, INTERNAL_ERROR)
	c.Check(frame, IsNil)
}

func (t *ParserTest) TestValidWindowUpdate(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x04, byte(WINDOW_UPDATE), byte(NO_FLAGS),
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x10, 0x00,
	})
	frame, err := ParseFrame(input)
	c.Check(err, IsNil)
	windowUpdate := frame.(*WindowUpdateFrame)

	c.Check(windowUpdate.Flags, Equals, NO_FLAGS)
	c.Check(windowUpdate.Id, Equals, StreamId(0x0))
	c.Check(windowUpdate.SizeDelta, Equals, uint32(4096))
}

func (t *ParserTest) TestInvalidWindowUpdateWithBadSizeDelta(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x04, byte(WINDOW_UPDATE), byte(NO_FLAGS),
		0x00, 0x00, 0x00, 0x00,
		0xff, 0x00, 0x10, 0x00,
	})
	frame, err := ParseFrame(input)
	c.Check(err, ErrorMatches, "Reserved bit is non-zero")
	c.Check(err.Code, Equals, PROTOCOL_ERROR)
	c.Check(frame, IsNil)
}

func (t *ParserTest) TestInvalidWindowUpdateUnderflow(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x03, byte(WINDOW_UPDATE), byte(NO_FLAGS),
		0x00, 0x00, 0x00, 0x00,
		0xff, 0x00, 0x10, 0x00,
	})
	frame, err := ParseFrame(input)
	c.Check(err, ErrorMatches,
		"Reached frame end while reading fixed-size payload")
	c.Check(err.Code, Equals, FRAME_SIZE_ERROR)
	c.Check(frame, IsNil)
}

func (t *ParserTest) TestValidContinuationFrame(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x08, byte(CONTINUATION), byte(PAD_LOW | END_HEADERS),
		0x01, 0x02, 0x03, 0x04,
		0x02, 0xf1, 0xf2, 0xf3,
		0xf4, 0xf5, 0xa1, 0xa2,
	})
	frame, err := ParseFrame(input)
	c.Check(err, IsNil)
	continuation := frame.(*ContinuationFrame)

	c.Check(continuation.Flags, Equals, PAD_LOW|END_HEADERS)
	c.Check(continuation.Id, Equals, StreamId(0x01020304))
	c.Check(continuation.Fragment, DeepEquals,
		[]byte{0xf1, 0xf2, 0xf3, 0xf4, 0xf5})
}

func (t *ParserTest) TestInvalidContinuationUnderflow(c *C) {
	input := bytes.NewBuffer([]byte{
		0x00, 0x00, byte(CONTINUATION), byte(PAD_LOW),
		0x01, 0x02, 0x03, 0x04,
		0x00,
	})
	frame, err := ParseFrame(input)
	c.Check(err, ErrorMatches,
		"Reached frame end while reading fixed-size payload")
	c.Check(err.Code, Equals, FRAME_SIZE_ERROR)
	c.Check(frame, IsNil)
}

var _ = Suite(&ParserTest{})
