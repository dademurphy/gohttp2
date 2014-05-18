// Copyright 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package http2

import (
	"errors"
	"log"
)

var (
	kConnectionStallError error = errors.New("Connection stall")
	kStreamStallError     error = errors.New("Stream stall")
)

// Manages the state of a Connection. Owned and only accessible
// from within the Connection.mainLoop() goroutine.
type connection struct {
	recvFlow          RecieveFlow
	sendFlowAvailable int

	streams map[StreamID]*Stream

	// Muxed together.
	recvMux  <-chan Frame // Frames read by the read loop.
	sendMux  chan<- Frame // Frames written to the write loop.
	queueMux <-chan Frame // Frames to write, queued by clients.

	writeQueue writeQueue
}

func (c *connection) mainLoop() {
	var pendingSend Frame

	maybeSendMux := func() chan<- Frame {
		if pendingSend != nil {
			return c.sendMux
		}
		return nil
	}

	for {
		// Deque the next frame to write.
		if pendingSend == nil {
			if next, ok := c.writeQueue.deque(); ok {
				if err := c.prepareToSendFrame(next); err == nil {
					pendingSend = next
				} else {
					c.handleError(err, next)
				}
			}
		}

		// Connection loop makes progress when:
		//  * A frame to write is queued, OR
		//  * A frame is written, OR
		//  * A frame is recieved.
		select {
		case maybeSendMux() <- pendingSend:
			pendingSend = nil
		case frame := <-c.queueMux:
			c.writeQueue.enqueueBack(frame)
		case frame := <-c.recvMux:
			if err := c.recieveFrame(frame); err != nil {
				c.handleError(err, frame)
			}
			// TODO(johng): consumeMux
		}
	}
}

func (c *connection) prepareToSendHeadersFrame(headers *HeadersFrame) *Error {
	stream := c.getOrCreateStream(headers.StreamID)
	return stream.onHeaders(Send, headers.Flags&END_STREAM != 0)
}

func (c *connection) recieveHeadersFrame(headers *HeadersFrame) *Error {
	stream := c.getOrCreateStream(headers.StreamID)
	return stream.onHeaders(Receive, headers.Flags&END_STREAM != 0)
}

func (c *connection) prepareToSendDataFrame(data *DataFrame) *Error {
	stream := c.getOrCreateStream(data.StreamID)
	if err := stream.onData(Send, data.Flags&END_STREAM != 0); err != nil {
		return err
	}

	// Determine how much of the frame we're allowed to send.
	bound := int(^kFrameLengthReservedMask) // Frame payload maximum size.

	if c.sendFlowAvailable == 0 {
		// We're stalled on connection flow control.
		c.writeQueue.enqueueFront(data)
		//c.writeQueue.stallAllStreams()
		//c.writeQueue.enqueueFront(&BlockedFrame{})
		return &Error{Code: FLOW_CONTROL_ERROR, Level: RecoverableError,
			Err: kConnectionStallError}
	} else if c.sendFlowAvailable < bound {
		bound = c.sendFlowAvailable
	}

	if stream.SendFlowAvailable == 0 {
		// We're stalled on stream flow control.
		c.writeQueue.enqueueFront(data)
		//c.writeQueue.stallStream(stream.ID)
		//c.writeQueue.enqueueFront(
		//  &BlockedFrame{FramePrefix{StreamID: data.StreamID}})
		return &Error{Code: FLOW_CONTROL_ERROR, Level: RecoverableError,
			Err: kStreamStallError}
	} else if stream.SendFlowAvailable < bound {
		bound = stream.SendFlowAvailable
	}

	// Split the frame if needed, and update flow control state.
	if bound < data.PayloadLength() {
		remainder := data.SplitAt(bound)
		c.writeQueue.enqueueFront(remainder)
	}

	// TODO(johng): Compress payload iff a) allowed, b) uncompressed length
	// is above-threshold, and c) compressed version is shorter.

	c.sendFlowAvailable -= data.PayloadLength()
	stream.SendFlowAvailable -= data.PayloadLength()

	// Inform delegate of window decrease from the send.
	stream.SendFlowPump <- -data.PayloadLength()

	// Update stream state.
	if data.Flags&END_STREAM != 0 {
		stream.onLocalFin()
	}
	return nil
}

func (c *connection) recieveDataFrame(data *DataFrame) *Error {
	if err := c.recvFlow.ApplyDataRecieved(data); err != nil {
		return err
	}

	stream := c.getOrCreateStream(data.StreamID)
	if err := stream.onData(Receive, data.Flags&END_STREAM != 0); err != nil {
		c.recvFlow.ApplyDataConsumed(data)
		return err
	}
	if err := stream.RecvFlow.ApplyDataRecieved(data); err != nil {
		c.recvFlow.ApplyDataConsumed(data)
		return err
	}

	if data.Flags&END_STREAM != 0 {
		stream.onRemoteFin()
	}
	return nil
}

func (c *connection) prepareToSendFrame(frame Frame) *Error {
	switch f := frame.(type) {
	case *DataFrame:
		return c.prepareToSendDataFrame(f)
	case *HeadersFrame:
		return c.prepareToSendHeadersFrame(f)
	default:
		return internalError("unknown frame type %v", frame)
	}
}

func (c *connection) recieveFrame(frame Frame) *Error {
	switch f := frame.(type) {
	case *DataFrame:
		return c.recieveDataFrame(f)
	case *HeadersFrame:
		return c.recieveHeadersFrame(f)
	default:
		return internalError("unknown frame type %v", frame)
	}
}

func (c *connection) handleError(err *Error, frame Frame) {
	log.Println("%v error (%v-level): %v", err.Code, err.Level, err)

	if err.Level == StreamError {
		c.writeQueue.enqueueFront(
			&RstStreamFrame{
				FramePrefix{StreamID: frame.GetStreamID()},
				*err,
			})
	} else if err.Level == ConnectionError {
		c.writeQueue.enqueueFront(
			&GoAwayFrame{
				LastID: 0, // TODO(johng): Report last stream.
				Error:  *err,
			})
	}
}

func (c *connection) getOrCreateStream(id StreamID) *Stream {
	stream, ok := c.streams[id]
	if !ok {
		// TODO(johng): Fill this out.
		stream = &Stream{
			ID: id,
		}
		c.streams[id] = stream
	}
	return stream
}
