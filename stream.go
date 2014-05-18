// Copyright 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package http2

type StreamState uint8

type SendOrReceive bool

const (
	Idle                StreamState = 0
	ReservedLocal       StreamState = iota
	ReservedRemote      StreamState = iota
	Open                StreamState = iota
	HalfClosedLocal     StreamState = iota
	HalfClosedRemote    StreamState = iota
	Closed              StreamState = iota
	ClosedWithSentReset StreamState = iota // Substate of Closed.

	Send    SendOrReceive = true
	Receive SendOrReceive = false
)

func (s StreamState) String() string {
	switch s {
	case Idle:
		return "Idle"
	case ReservedLocal:
		return "ReservedLocal"
	case ReservedRemote:
		return "ReservedRemote"
	case Open:
		return "Open"
	case HalfClosedLocal:
		return "HalfClosedLocal"
	case HalfClosedRemote:
		return "HalfClosedRemote"
	case Closed:
		return "Closed"
	case ClosedWithSentReset:
		return "ClosedWithSentReset"
	default:
		return "(unknown StreamState)"
	}
}

type Stream struct {
	ID    StreamID
	State StreamState

	RecvFlow RecieveFlow

	SendFlowAvailable int
	SendFlowPump      chan<- int
}

func (s *Stream) frameError(dir SendOrReceive, frameType FrameType) *Error {
	// Note that errors are Level CONNECTION by default.
	var err *Error

	if dir == Send {
		err = internalError("attempt to send %v on %v stream %v",
			frameType, s.State, s.ID)
	} else {
		err = protocolError("recieved %v on %v stream %v",
			frameType, s.State, s.ID)
	}

	if dir == Receive && s.State == Closed {
		// Remote close followed by remote send. Manadated as a stream error.
		err.Code = STREAM_CLOSED
		err.Level = StreamError
	}
	if dir == Receive && s.State == ClosedWithSentReset {
		// Ignore further receive errors on this stream.
		err.Code = STREAM_CLOSED
		err.Level = RecoverableError
	}
	return err
}

func (s *Stream) onPushPromise(dir SendOrReceive) *Error {
	if s.State != Idle {
		return s.frameError(dir, PUSH_PROMISE)
	}

	if dir == Send {
		s.State = ReservedLocal
	} else {
		s.State = ReservedRemote
		close(s.SendFlowPump)
	}
	return nil
}

func (s *Stream) onHeaders(dir SendOrReceive, fin bool) *Error {
	if s.State != Idle &&
		s.State != Open &&
		!(s.State == ReservedLocal && dir == Send) &&
		!(s.State == ReservedRemote && dir == Receive) &&
		!(s.State == HalfClosedLocal && dir == Receive) &&
		!(s.State == HalfClosedRemote && dir == Send) {
		return s.frameError(dir, HEADERS)
	}

	localOpen := false

	if s.State == Idle {
		s.State = Open
		localOpen = true
	} else if s.State == ReservedLocal {
		s.State = HalfClosedRemote
		localOpen = true
	} else if s.State == ReservedRemote {
		s.State = HalfClosedLocal
	}

	if fin && dir == Send {
		s.onLocalFin()
		localOpen = false
	} else if fin {
		s.onRemoteFin()
	}

	if localOpen {
		// Stream was locally opened, and remains open.
		s.SendFlowPump <- s.SendFlowAvailable
	}
	return nil
}

func (s *Stream) onData(dir SendOrReceive, fin bool) *Error {
	if s.State != Open &&
		!(s.State == HalfClosedLocal && dir == Receive) &&
		!(s.State == HalfClosedRemote && dir == Send) {
		return s.frameError(dir, DATA)
	}

	if dir == Send && fin {
		s.onLocalFin()
	} else if fin {
		s.onRemoteFin()
	}
	return nil
}

func (s *Stream) onReset(dir SendOrReceive) *Error {
	if s.State == Idle ||
		s.State == ClosedWithSentReset {
		return s.frameError(dir, RST_STREAM)
	}

	if s.State != ReservedRemote &&
		s.State != HalfClosedLocal &&
		s.State != Closed &&
		s.State != ClosedWithSentReset {
		close(s.SendFlowPump)
	}
	if dir == Receive {
		s.State = Closed
	} else {
		s.State = ClosedWithSentReset
	}
	return nil
}

func (s *Stream) onRemoteFin() {
	if s.State == Open {
		s.State = HalfClosedRemote
	} else if s.State == HalfClosedLocal {
		s.State = Closed
	} else {
		panic(s.State)
	}
}

func (s *Stream) onLocalFin() {
	if s.State == Open {
		s.State = HalfClosedLocal
	} else if s.State == HalfClosedRemote {
		s.State = Closed
	} else {
		panic(s.State)
	}
	close(s.SendFlowPump)
}
