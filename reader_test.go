// Copyright 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package http2

import (
	"bytes"
	"io/ioutil"
	"testing/iotest"

	. "gopkg.in/check.v1"
)

type ReaderTest struct{}

func (t *ReaderTest) TestReadsWithVaryingPrefixSizes(c *C) {
	table := [...]struct {
		bits   BitSequence
		expect string
	}{
		{BitSequence{0x0000000000000000, 0},
			"\xff"},
		{BitSequence{0x1100000000000000, 8},
			"\x11\xff"},
		{BitSequence{0x1122000000000000, 16},
			"\x11\x22\xff"},
		{BitSequence{0x1122330000000000, 24},
			"\x11\x22\x33\xff"},
		{BitSequence{0x1122334400000000, 32},
			"\x11\x22\x33\x44\xff"},
		{BitSequence{0x1122334455000000, 40},
			"\x11\x22\x33\x44\x55\xff"},
		{BitSequence{0x1122334455660000, 48},
			"\x11\x22\x33\x44\x55\x66\xff"},
		{BitSequence{0x1122334455667700, 56},
			"\x11\x22\x33\x44\x55\x66\x77\xff"},
		{BitSequence{0x1122334455667788, 64},
			"\x11\x22\x33\x44\x55\x66\x77\x88\xff"},
	}
	for i, fixture := range table {
		r := &Reader{
			reader: bytes.NewReader([]byte("\xff")),
			bits:   fixture.bits}

		buffer, err := ioutil.ReadAll(r)
		c.Check(string(buffer), Equals, fixture.expect, Commentf("i=%v", i))
		c.Check(err, IsNil)
	}
}

func (t *ReaderTest) TestPeekBitsWithVaryingPrefixSizes(c *C) {
	table := [...]struct {
		bits  BitSequence
		input string
	}{
		{BitSequence{0x0000000000000000, 0},
			"\x11\x22\x33\x44\x55\x66\x77\x88"},
		{BitSequence{0x1000000000000000, 4},
			"\x12\x23\x34\x45\x56\x67\x78\x80"},
		{BitSequence{0x1100000000000000, 8},
			"\x22\x33\x44\x55\x66\x77\x88"},
		{BitSequence{0x1120000000000000, 12},
			"\x23\x34\x45\x56\x67\x78\x80"},
		{BitSequence{0x1122000000000000, 16},
			"\x33\x44\x55\x66\x77\x88"},
		{BitSequence{0x1122300000000000, 20},
			"\x34\x45\x56\x67\x78\x80"},
		{BitSequence{0x1122330000000000, 24},
			"\x44\x55\x66\x77\x88"},
		{BitSequence{0x1122334000000000, 28},
			"\x45\x56\x67\x78\x80"},
		{BitSequence{0x1122334400000000, 32},
			"\x55\x66\x77\x88"},
		{BitSequence{0x1122334450000000, 36},
			"\x56\x67\x78\x80"},
		{BitSequence{0x1122334455000000, 40},
			"\x66\x77\x88"},
		{BitSequence{0x1122334455600000, 44},
			"\x67\x78\x80"},
		{BitSequence{0x1122334455660000, 48},
			"\x77\x88"},
		{BitSequence{0x1122334455667000, 52},
			"\x78\x80"},
		{BitSequence{0x1122334455667700, 56},
			"\x88"},
		{BitSequence{0x1122334455667780, 60},
			"\x80"},
		{BitSequence{0x1122334455667788, 64},
			""},
	}
	for i, fixture := range table {
		r := &Reader{
			reader: bytes.NewReader([]byte(fixture.input)),
			bits:   fixture.bits}

		bits, _ := r.PeekBits()

		if fixture.bits.Length%8 == 4 {
			c.Check(bits, Equals, BitSequence{0x1122334455667780, 60},
				Commentf("i=%d", i))
		} else {
			c.Check(bits, Equals, BitSequence{0x1122334455667788, 64},
				Commentf("i=%d", i))
		}
	}
}

func (t *ReaderTest) TestPeekBitsWithShortReads(c *C) {
	r := &Reader{
		reader: iotest.HalfReader(
			bytes.NewReader([]byte("\x12\x23\x34\x45\x56\x67\x78"))),
		bits: BitSequence{0x1000000000000000, 4}}

	bits, _ := r.PeekBits()
	c.Check(bits, Equals, BitSequence{0x1122334450000000, 36})
	bits, _ = r.PeekBits()
	c.Check(bits, Equals, BitSequence{0x1122334455667000, 52})
	bits, _ = r.PeekBits()
	c.Check(bits, Equals, BitSequence{0x1122334455667780, 60})
}

func (t *ReaderTest) TestConsumeBits(c *C) {
	r := &Reader{
		reader: nil,
		bits:   BitSequence{0x1122334455667788, 64}}

	r.ConsumeBits(4)
	c.Check(r.bits, Equals, BitSequence{0x1223344556677880, 60})

	r.ConsumeBits(32)
	c.Check(r.bits, Equals, BitSequence{0x5667788000000000, 28})

	r.ConsumeBits(16)
	c.Check(r.bits, Equals, BitSequence{0x7880000000000000, 12})

	r.ConsumeBits(12)
	c.Check(r.bits, Equals, BitSequence{0x0000000000000000, 0})
}

var _ = Suite(&ReaderTest{})
