// Copyright 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package http2

type StreamId uint32

type FrameType uint8

const (
	DATA            FrameType = 0x00
	HEADERS         FrameType = 0x01
	PRIORITY        FrameType = 0x02
	RST_STREAM      FrameType = 0x03
	SETTINGS        FrameType = 0x04
	PUSH_PROMISE    FrameType = 0x05
	PING            FrameType = 0x06
	GOAWAY          FrameType = 0x07
	WINDOW_UPDATE   FrameType = 0x08
	CONTINUATION    FrameType = 0x09
	LAST_FRAME_TYPE FrameType = CONTINUATION
)

type Flags uint8

const (
	NO_FLAGS            Flags = 0x00
	ACK                 Flags = 0x01
	END_STREAM          Flags = 0x01
	END_SEGMENT         Flags = 0x02
	END_HEADERS         Flags = 0x04
	PAD_LOW             Flags = 0x08
	PAD_HIGH            Flags = 0x10
	PRIORITY_GROUP      Flags = 0x20
	PRIORITY_DEPENDENCY Flags = 0x40
)

var kValidFlags = [...]Flags{
	// DATA
	END_STREAM | END_SEGMENT | PAD_LOW | PAD_HIGH,
	// HEADERS
	END_STREAM | END_SEGMENT | END_HEADERS | PAD_LOW |
		PAD_HIGH | PRIORITY_GROUP | PRIORITY_DEPENDENCY,
	// PRIORITY
	PRIORITY_GROUP | PRIORITY_DEPENDENCY,
	// RST_STREAM
	NO_FLAGS,
	// SETTINGS
	ACK,
	// PUSH_PROMISE
	END_HEADERS | PAD_LOW | PAD_HIGH,
	// PING
	ACK,
	// GOAWAY
	NO_FLAGS,
	// WINDOW_UPDATE
	NO_FLAGS,
	// CONTINUATION
	END_HEADERS | PAD_LOW | PAD_HIGH,
}
