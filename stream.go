// Copyright 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package http2

type StreamState uint8

type SendOrReceive bool

const (
	Idle             StreamState = 0
	ReservedLocal    StreamState = iota
	ReservedRemote   StreamState = iota
	Open             StreamState = iota
	HalfClosedLocal  StreamState = iota
	HalfClosedRemote StreamState = iota
	Closed           StreamState = iota

	Send    SendOrReceive = true
	Receive SendOrReceive = false
)

type Stream struct {
	ID    StreamID
	State StreamState

	SawRemoteRst bool
	SawRemoteFin bool

	RecvFlow RecieveFlow

	SendFlowAvailable int
	SendFlowPump      chan<- int
}

func (s *Stream) frameError(dir SendOrReceive, frameName string) *Error {
	// Note that errors are Level CONNECTION by default.
	var err *Error

	if dir == Send {
		err = internalError("Attempt to send %v on %v stream %v",
			frameName, s.State, s.ID)
	} else {
		err = protocolError("Recieved %v on %v stream %v",
			frameName, s.State, s.ID)
	}

	// Special error cases.
	if dir == Send && s.SawRemoteRst {
		// Local send raced with remote reset.
		err.Code = STREAM_CLOSED
		err.Level = RecoverableError
	} else if dir == Receive && !s.SawRemoteRst && !s.SawRemoteFin {
		// Remote send raced with local reset.
		err.Code = STREAM_CLOSED
		err.Level = RecoverableError
	} else if dir == Receive && s.SawRemoteRst {
		// Remote reset followed by remote send. Mandated as a stream error.
		err.Code = STREAM_CLOSED
		err.Level = StreamError
	} else if s.State == Closed && !s.SawRemoteFin {
		// Remote fin followed by remote send. Also mandated as a stream error.
		err.Code = STREAM_CLOSED
		err.Level = StreamError
	}
	return err
}

func (s *Stream) onPushPromise(dir SendOrReceive) *Error {
	if s.State != Idle {
		return s.frameError(dir, "PUSH_PROMISE")
	}

	if dir == Send {
		s.State = ReservedLocal
		s.SawRemoteFin = true
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
		return s.frameError(dir, "HEADERS")
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

func (s *Stream) onData(dir SendOrReceive) *Error {
	if s.State != Open &&
		!(s.State == HalfClosedLocal && dir == Receive) &&
		!(s.State == HalfClosedRemote && dir == Send) {
		return s.frameError(dir, "DATA")
	}
	return nil
}

func (s *Stream) onReset(dir SendOrReceive) *Error {
	if s.State == Idle {
		return s.frameError(dir, "RST_STREAM")
	}

	if s.State != HalfClosedLocal && s.State != Closed {
		close(s.SendFlowPump)
	}
	if dir == Receive {
		s.SawRemoteRst = true
	}
	s.State = Closed
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
	s.SawRemoteFin = true
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
