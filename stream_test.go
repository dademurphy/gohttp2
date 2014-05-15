// Copyright 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package http2

import (
	gc "gopkg.in/check.v1"
)

type StreamTest struct {
	pump chan int
}

func (t *StreamTest) Stream(state StreamState) Stream {
	t.pump = make(chan int)
	return Stream{
		State:             state,
		SendFlowAvailable: 4096,
		SendFlowPump:      t.pump,
	}
}

// Test every possible state transition.
func (t *StreamTest) TestIdleSendTransitions(c *gc.C) {
	s := t.Stream(Idle)

	c.Check(s.onPushPromise(true), gc.IsNil)
	c.Check(s.State, gc.Equals, ReservedLocal)
}
