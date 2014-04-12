// Copyright 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package http2

import (
	"io"
)

type HeaderDecoder interface {
	DecodeHeaderBlockFragment(in *io.LimitedReader) ([]HeaderField, *Error)
	HeaderBlockComplete() ([]HeaderField, *Error)
}

/*
type Connection struct {
	reader io.ReadCloser
	writer io.WriteCloser

	writeQueue chan Frame

	decoder  HeaderDecoder

	settings map[SettingId]uint32
}

func (c *Connection) readLoop() {
	defer c.reader.Close()

	var lastStream StreamId
	for {
		parsedFrame, err := ParseFrame(c.reader, c.decoder)

		if err != nil {
			c.GoAway(lastStream, err)
			return
		}

		switch parsedFrame.(type) {
		case *DataFrame:
			//c.onDataFrame(frame)
		case *HeadersFrame:
			//c.onHeadersFrame(frame)
		case *PriorityFrame:
			//c.onPriorityFrame(frame)
		case *RstStreamFrame:
			//c.onRstStreamFrame(frame)
		case *SettingsFrame:
			//c.onSettingsFrame(frame)
		case *PushPromiseFrame:
			//c.onPushPromiseFrame(frame)
		case *PingFrame:
			//c.onPingFrame(frame)
		case *GoAwayFrame:
			//c.onGoAwayFrame(frame)
		case *WindowUpdateFrame:
			//c.onWindowUpdateFrame(frame)
		}
	}
}

func (c *Connection) GoAway(lastStream StreamId, err *Error) {
	c.writeQueue <- &GoAwayFrame{
		LastStream: lastStream,
		Code:       err.Code,
		Debug:      []byte(err.Error()),
	}
	c.writeQueue <- nil // EOF
}
*/
