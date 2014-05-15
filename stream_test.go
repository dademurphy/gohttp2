// Copyright 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package http2

import (
	gc "gopkg.in/check.v1"
)

type StreamTest struct {
	pump   chan int
	stream *Stream
}

type errCase struct {
	level ErrorLevel
	code  ErrorCode
}

type successCase struct {
	state        StreamState
	sawRemoteFin bool
	sawRemoteRst bool
	pumpOpened   bool
	pumpClosed   bool
}

func (t *StreamTest) from(state StreamState) *Stream {
	t.pump = make(chan int, 1)
	t.stream = &Stream{
		State:             state,
		SendFlowAvailable: 4096,
		SendFlowPump:      t.pump,
	}
	return t.stream
}

func (t *StreamTest) verify(outcome interface{}, err *Error, c *gc.C) {
	if expected, ok := outcome.(*errCase); ok {
		c.Check(err, gc.NotNil)
		c.Check(err.Level, gc.Equals, expected.level)
		c.Check(err.Code, gc.Equals, expected.code)
		return
	}
	expected := outcome.(*successCase)
	c.Check(t.stream.State, gc.Equals, expected.state)
	c.Check(t.stream.SawRemoteRst, gc.Equals, expected.sawRemoteRst)
	c.Check(t.stream.SawRemoteFin, gc.Equals, expected.sawRemoteFin)

	select {
	case r, ok := <-t.pump:
		if expected.pumpOpened {
			c.Check(ok, gc.Equals, true)
			c.Check(r, gc.Equals, t.stream.SendFlowAvailable)
		} else if expected.pumpClosed {
			c.Check(ok, gc.Equals, false)
		} else {
			c.Error("unexpected pump update: ", r, ok)
		}
	default:
		c.Check(expected.pumpOpened, gc.Equals, false)
		c.Check(expected.pumpClosed, gc.Equals, false)
	}
}

func (t *StreamTest) TestPushPromiseTransitions(c *gc.C) {
	testCases := []struct {
		initial StreamState
		dir     SendOrReceive
		outcome interface{}
	}{
		{Idle, Send,
			&successCase{ReservedLocal, true, false, false, false}},
		{Idle, Receive,
			&successCase{ReservedRemote, false, false, false, true}},
		{ReservedLocal, Send,
			&errCase{ConnectionError, INTERNAL_ERROR}},
		{ReservedLocal, Receive,
			&errCase{ConnectionError, PROTOCOL_ERROR}},
		{ReservedRemote, Send,
			&errCase{ConnectionError, INTERNAL_ERROR}},
		{ReservedRemote, Receive,
			&errCase{ConnectionError, PROTOCOL_ERROR}},
		{Open, Send,
			&errCase{ConnectionError, INTERNAL_ERROR}},
		{Open, Receive,
			&errCase{ConnectionError, PROTOCOL_ERROR}},
		{HalfClosedLocal, Send,
			&errCase{ConnectionError, INTERNAL_ERROR}},
		{HalfClosedLocal, Receive,
			&errCase{ConnectionError, PROTOCOL_ERROR}},
		{HalfClosedRemote, Send,
			&errCase{ConnectionError, INTERNAL_ERROR}},
		{HalfClosedRemote, Receive,
			&errCase{ConnectionError, PROTOCOL_ERROR}},
		{Closed, Send,
			&errCase{ConnectionError, INTERNAL_ERROR}},
		{Closed, Receive,
			&errCase{RecoverableError, STREAM_CLOSED}},
	}
	for _, testCase := range testCases {
		t.verify(testCase.outcome,
			t.from(testCase.initial).onPushPromise(testCase.dir), c)
	}
}

func (t *StreamTest) TestHeadersTransitions(c *gc.C) {
	testCases := []struct {
		initial StreamState
		dir     SendOrReceive
		fin     bool
		outcome interface{}
	}{
		{Idle, Send, false,
			&successCase{Open, false, false, true, false}},
		{Idle, Send, true,
			&successCase{HalfClosedLocal, false, false, false, true}},
		{Idle, Receive, false,
			&successCase{Open, false, false, true, false}},
		{Idle, Receive, true,
			&successCase{HalfClosedRemote, true, false, true, false}},
		{ReservedLocal, Send, false,
			&successCase{HalfClosedRemote, false, false, true, false}},
		{ReservedLocal, Send, true,
			&successCase{Closed, false, false, false, true}},
		{ReservedLocal, Receive, false,
			&errCase{ConnectionError, PROTOCOL_ERROR}},
		{ReservedRemote, Send, false,
			&errCase{ConnectionError, INTERNAL_ERROR}},
		{ReservedRemote, Receive, false,
			&successCase{HalfClosedLocal, false, false, false, false}},
		{ReservedRemote, Receive, true,
			&successCase{Closed, true, false, false, false}},
		{Open, Send, false,
			&successCase{Open, false, false, false, false}},
		{Open, Send, true,
			&successCase{HalfClosedLocal, false, false, false, true}},
		{Open, Receive, false,
			&successCase{Open, false, false, false, false}},
		{Open, Receive, true,
			&successCase{HalfClosedRemote, true, false, false, false}},
		{HalfClosedLocal, Send, false,
			&errCase{ConnectionError, INTERNAL_ERROR}},
		{HalfClosedLocal, Receive, false,
			&successCase{HalfClosedLocal, false, false, false, false}},
		{HalfClosedLocal, Receive, true,
			&successCase{Closed, true, false, false, false}},
		{HalfClosedRemote, Send, false,
			&successCase{HalfClosedRemote, false, false, false, false}},
		{HalfClosedRemote, Send, true,
			&successCase{Closed, false, false, false, true}},
		{HalfClosedRemote, Receive, false,
			&errCase{ConnectionError, PROTOCOL_ERROR}},
		{Closed, Send, false,
			&errCase{ConnectionError, INTERNAL_ERROR}},
		{Closed, Receive, false,
			&errCase{RecoverableError, STREAM_CLOSED}},
	}
	for _, testCase := range testCases {
		t.verify(testCase.outcome,
			t.from(testCase.initial).onHeaders(testCase.dir, testCase.fin), c)
	}
}

var _ = gc.Suite(&StreamTest{})
