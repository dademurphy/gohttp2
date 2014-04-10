// Copyright 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package http2

import (
	"bytes"

	. "gopkg.in/check.v1"
)

type ParserTest struct{}

func (t *ParserTest) TestPrefixNoFlags(c *C) {
	input := bytes.NewBuffer([]byte{
    0x00, 0x05, 0x00, 0x00,
		0x01, 0x02, 0x03, 0x04,
  }
	frame, err := ParseFrame(input)
  data := frame.(*DataFrame)

  c.Check(data.Flags, Equals, NO_FLAGS)
  c.Check(data.Id, Equals, StreamId(0x01020304))
}

func (t *ParserTest) TestPrefixValidFlags(c *C) {
	input := bytes.NewBuffer([]byte{
    0x00, 0x05, 0x00, 0x01,
		0x01, 0x02, 0x03, 0x04,
  }
	frame, err := ParseFrame(input)
  data := frame.(*DataFrame)

  c.Check(frame.GetFlags(), Equals, END_STREAM)
  c.Check(data.Id, Equals, StreamId(0x01020304))
}

func (t *ParserTest) TestPrefixValidFlags(c *C) {
	input := bytes.NewBuffer([]byte{
    0x00, 0x05, 0x00, 0x01,
		0x01, 0x02, 0x03, 0x04,
  }
	frame, err := ParseFrame(input)
  data := frame.(*DataFrame)

  c.Check(data.Flags, Equals, END_STREAM)
  c.Check(data.Id, Equals, StreamId(0x01020304))
}


func (t *ParserTest) TestParseDataFrame(c *C) {
	input := bytes.NewBuffer([]byte{
    0x00, 0x05, 0x00, 0x00,
		0x01, 0x02, 0x03, 0x04,
		0xd1, 0xd2, 0xd3, 0xd4,
		0xd5})
	frame, err := ParseFrame(input)
  c.Check(err, IsNil)
  data := frame.(*DataFrame)
  c.Check(data, Not(IsNil))

  c.Check(data.Flags, Equals, NO_FLAGS)
  c.Check(data.Id, Equals, StreamId(0x01020304))
  c.Check(data.PaddingLength, DeepEquals, uint(0))
  c.Check(data.Data, DeepEquals, []byte{0xd1, 0xd2, 0xd3, 0xd4, 0xd5})
}

func (t *ParserTest) TestParseDataFrameEndStreamPadLow(c *C) {
	input := bytes.NewBuffer([]byte{
    0x00, 0x09, 0x00, 0x09,
		0x01, 0x02, 0x03, 0x04,
    0x03,  // Pad length.
		0xd1, 0xd2, 0xd3, 0xd4,
		0xd5, 0xf1, 0xf2, 0xf3})
	frame, err := ParseFrame(input)
  c.Check(err, IsNil)
  data := frame.(*DataFrame)
  c.Check(data, Not(IsNil))

  c.Check(data.Flags, Equals, END_STREAM | PAD_LOW)
  c.Check(data.Id, Equals, StreamId(0x01020304))
  c.Check(data.PaddingLength, DeepEquals, uint(3))
  c.Check(data.Data, DeepEquals, []byte{0xd1, 0xd2, 0xd3, 0xd4, 0xd5})
}



var _ = Suite(&ParserTest{})
