// Copyright 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package http2

import (
	. "gopkg.in/check.v1"
)

type WriterTest struct{}

func (t *WriterTest) TestFlushBits(c *C) {
  //c.Check(1, Equals, 2)
}

var _ = Suite(&WriterTest{})
