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
	state      StreamState
	pumpOpened bool
	pumpClosed bool
}

type transitionCases struct {
	onRecvData           interface{}
	onRecvDataWithFin    interface{}
	onRecvHeaders        interface{}
	onRecvHeadersWithFin interface{}
	onRecvPushPromise    interface{}
	onRecvReset          interface{}
	onSendData           interface{}
	onSendDataWithFin    interface{}
	onSendHeaders        interface{}
	onSendHeadersWithFin interface{}
	onSendPushPromise    interface{}
	onSendReset          interface{}
}

func defaultTransitionOutcomes() transitionCases {
	return transitionCases{
		onRecvData:           &errCase{ConnectionError, PROTOCOL_ERROR},
		onRecvDataWithFin:    &errCase{ConnectionError, PROTOCOL_ERROR},
		onRecvHeaders:        &errCase{ConnectionError, PROTOCOL_ERROR},
		onRecvHeadersWithFin: &errCase{ConnectionError, PROTOCOL_ERROR},
		onRecvPushPromise:    &errCase{ConnectionError, PROTOCOL_ERROR},
		onRecvReset:          &successCase{Closed, false, true},
		onSendData:           &errCase{ConnectionError, INTERNAL_ERROR},
		onSendDataWithFin:    &errCase{ConnectionError, INTERNAL_ERROR},
		onSendHeaders:        &errCase{ConnectionError, INTERNAL_ERROR},
		onSendHeadersWithFin: &errCase{ConnectionError, INTERNAL_ERROR},
		onSendPushPromise:    &errCase{ConnectionError, INTERNAL_ERROR},
		onSendReset:          &successCase{ClosedWithSentReset, false, true},
	}
}

func verifyTransitions(model Stream, outcomes transitionCases, c *gc.C) {
	var pump chan int
	var underTest *Stream

	from := func() *Stream {
		pump = make(chan int, 1)
		underTest = &Stream{
			ID:                model.ID,
			State:             model.State,
			SendFlowAvailable: 4096,
			SendFlowPump:      pump,
		}
		return underTest
	}

	verify := func(outcome interface{}, err *Error) {
		if expected, ok := outcome.(*errCase); ok {
			c.Check(err, gc.NotNil)
			c.Check(err.Level, gc.Equals, expected.level)
			c.Check(err.Code, gc.Equals, expected.code)
			return
		}
		expected := outcome.(*successCase)
		c.Check(underTest.State, gc.Equals, expected.state)

		select {
		case r, ok := <-pump:
			if expected.pumpOpened {
				c.Check(ok, gc.Equals, true)
				c.Check(r, gc.Equals, underTest.SendFlowAvailable)
			} else if expected.pumpClosed {
				c.Check(ok, gc.Equals, false)
			} else {
				c.Error("unexpected pump update: ", r, ok)
			}
		default:
			c.Check(expected.pumpOpened, gc.Equals, false)
			c.Check(expected.pumpClosed, gc.Equals, false)
		}
		if c.Failed() {
			panic(false) // Generate a callstack.
		}
	}

	verify(outcomes.onRecvData, from().onData(Receive, false))
	verify(outcomes.onRecvDataWithFin, from().onData(Receive, true))
	verify(outcomes.onRecvHeaders, from().onHeaders(Receive, false))
	verify(outcomes.onRecvHeadersWithFin, from().onHeaders(Receive, true))
	verify(outcomes.onRecvPushPromise, from().onPushPromise(Receive))
	verify(outcomes.onRecvReset, from().onReset(Receive))
	verify(outcomes.onSendData, from().onData(Send, false))
	verify(outcomes.onSendDataWithFin, from().onData(Send, true))
	verify(outcomes.onSendHeaders, from().onHeaders(Send, false))
	verify(outcomes.onSendHeadersWithFin, from().onHeaders(Send, true))
	verify(outcomes.onSendPushPromise, from().onPushPromise(Send))
	verify(outcomes.onSendReset, from().onReset(Send))
}

func (t *StreamTest) TestTransitionsFromIdle(c *gc.C) {
	cases := defaultTransitionOutcomes()

	cases.onRecvHeaders = &successCase{Open, true, false}
	cases.onRecvHeadersWithFin = &successCase{HalfClosedRemote, true, false}
	cases.onRecvPushPromise = &successCase{ReservedRemote, false, true}
	cases.onRecvReset = &errCase{ConnectionError, PROTOCOL_ERROR}
	cases.onSendHeaders = &successCase{Open, true, false}
	cases.onSendHeadersWithFin = &successCase{HalfClosedLocal, false, true}
	cases.onSendPushPromise = &successCase{ReservedLocal, false, false}
	cases.onSendReset = &errCase{ConnectionError, INTERNAL_ERROR}

	verifyTransitions(Stream{State: Idle}, cases, c)
}

func (t *StreamTest) TestTransitionsFromReservedLocal(c *gc.C) {
	cases := defaultTransitionOutcomes()

	cases.onSendHeaders = &successCase{HalfClosedRemote, true, false}
	cases.onSendHeadersWithFin = &successCase{Closed, false, true}

	verifyTransitions(Stream{State: ReservedLocal}, cases, c)
}

func (t *StreamTest) TestTransitionsFromReservedRemote(c *gc.C) {
	cases := defaultTransitionOutcomes()

	cases.onRecvHeaders = &successCase{HalfClosedLocal, false, false}
	cases.onRecvHeadersWithFin = &successCase{Closed, false, false}
	cases.onRecvReset = &successCase{Closed, false, false}
	cases.onSendReset = &successCase{ClosedWithSentReset, false, false}

	verifyTransitions(Stream{State: ReservedRemote}, cases, c)
}

func (t *StreamTest) TestTransitionsFromOpen(c *gc.C) {
	cases := defaultTransitionOutcomes()

	cases.onRecvData = &successCase{Open, false, false}
	cases.onRecvDataWithFin = &successCase{HalfClosedRemote, false, false}
	cases.onRecvHeaders = &successCase{Open, false, false}
	cases.onRecvHeadersWithFin = &successCase{HalfClosedRemote, false, false}
	cases.onSendData = &successCase{Open, false, false}
	cases.onSendDataWithFin = &successCase{HalfClosedLocal, false, true}
	cases.onSendHeaders = &successCase{Open, false, false}
	cases.onSendHeadersWithFin = &successCase{HalfClosedLocal, false, true}

	verifyTransitions(Stream{State: Open}, cases, c)
}

func (t *StreamTest) TestTransitionsFromHalfClosedRemote(c *gc.C) {
	cases := defaultTransitionOutcomes()

	cases.onSendData = &successCase{HalfClosedRemote, false, false}
	cases.onSendDataWithFin = &successCase{Closed, false, true}
	cases.onSendHeaders = &successCase{HalfClosedRemote, false, false}
	cases.onSendHeadersWithFin = &successCase{Closed, false, true}

	verifyTransitions(Stream{State: HalfClosedRemote}, cases, c)
}

func (t *StreamTest) TestTransitionsFromHalfClosedLocal(c *gc.C) {
	cases := defaultTransitionOutcomes()

	cases.onRecvData = &successCase{HalfClosedLocal, false, false}
	cases.onRecvDataWithFin = &successCase{Closed, false, false}
	cases.onRecvHeaders = &successCase{HalfClosedLocal, false, false}
	cases.onRecvHeadersWithFin = &successCase{Closed, false, false}
	cases.onRecvReset = &successCase{Closed, false, false}
	cases.onSendReset = &successCase{ClosedWithSentReset, false, false}

	verifyTransitions(Stream{State: HalfClosedLocal}, cases, c)
}

func (t *StreamTest) TestTransitionsFromClosed(c *gc.C) {
	cases := defaultTransitionOutcomes()

	cases.onRecvData = &errCase{StreamError, STREAM_CLOSED}
	cases.onRecvDataWithFin = &errCase{StreamError, STREAM_CLOSED}
	cases.onRecvHeaders = &errCase{StreamError, STREAM_CLOSED}
	cases.onRecvHeadersWithFin = &errCase{StreamError, STREAM_CLOSED}
	cases.onRecvPushPromise = &errCase{StreamError, STREAM_CLOSED}
	cases.onRecvReset = &successCase{Closed, false, false}
	cases.onSendReset = &successCase{ClosedWithSentReset, false, false}

	verifyTransitions(Stream{State: Closed}, cases, c)
}

func (t *StreamTest) TestTransitionsFromClosedWithSentReset(c *gc.C) {
	cases := defaultTransitionOutcomes()

	cases.onRecvData = &errCase{RecoverableError, STREAM_CLOSED}
	cases.onRecvDataWithFin = &errCase{RecoverableError, STREAM_CLOSED}
	cases.onRecvHeaders = &errCase{RecoverableError, STREAM_CLOSED}
	cases.onRecvHeadersWithFin = &errCase{RecoverableError, STREAM_CLOSED}
	cases.onRecvPushPromise = &errCase{RecoverableError, STREAM_CLOSED}
	cases.onRecvReset = &errCase{RecoverableError, STREAM_CLOSED}
	cases.onSendReset = &errCase{ConnectionError, INTERNAL_ERROR}

	verifyTransitions(Stream{State: ClosedWithSentReset}, cases, c)
}

var _ = gc.Suite(&StreamTest{})
