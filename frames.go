// Copyright 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package http2

import (
	"io"
)

type Frame interface {
	GetType() FrameType
	GetFlags() Flags
	GetId() StreamId

	Parse(in *io.LimitedReader) *Error
}

// Models the Flags and StreamID fields common to all frame types.
type FramePrefix struct {
	Flags Flags
	Id    StreamId
}

func (f *FramePrefix) GetFlags() Flags {
	return f.Flags
}
func (f *FramePrefix) GetId() StreamId {
	return f.Id
}

// Models frames carrying padding (DATA, HEADERS, PUSH_PROMISE,
// and CONTINUATION).
type FramePadding struct {
	PaddingLength uint
}

// Models frames carrying a priority update (HEADERS, PRIORITY)
type FramePriority struct {
	PriorityGroup  uint32
	PriorityWeight uint8

	ExclusiveDependency bool
	StreamDependency    StreamId
}

type DataFrame struct {
	FramePrefix
	FramePadding

	Data []byte
}

type HeadersFrame struct {
	FramePrefix
	FramePadding
	FramePriority

	Fragment []byte
}

type PriorityFrame struct {
	FramePrefix
	FramePriority
}

type RstStreamFrame struct {
	FramePrefix

	Code ErrorCode
}

type SettingsFrame struct {
	FramePrefix

	Settings map[SettingId]uint32
}

type PushPromiseFrame struct {
	FramePrefix
	FramePadding

	PromisedId StreamId
	Fragment   []byte
}

type PingFrame struct {
	FramePrefix

	OpaqueData uint64
}

type GoAwayFrame struct {
	FramePrefix

	LastId StreamId
	Code   ErrorCode
	Debug  []byte
}

type WindowUpdateFrame struct {
	FramePrefix

	SizeDelta uint32
}

type ContinuationFrame struct {
	FramePrefix
	FramePadding

	Fragment []byte
}

func NewFrame(frameType FrameType) Frame {
	if frameType == DATA {
		return &DataFrame{}
	}
	if frameType == HEADERS {
		return &HeadersFrame{}
	}
	if frameType == PRIORITY {
		return &PriorityFrame{}
	}
	if frameType == RST_STREAM {
		return &RstStreamFrame{}
	}
	if frameType == SETTINGS {
		return &SettingsFrame{}
	}
	if frameType == PUSH_PROMISE {
		return &PushPromiseFrame{}
	}
	if frameType == PING {
		return &PingFrame{}
	}
	if frameType == GOAWAY {
		return &GoAwayFrame{}
	}
	if frameType == WINDOW_UPDATE {
		return &WindowUpdateFrame{}
	}
	if frameType == CONTINUATION {
		return &ContinuationFrame{}
	}
	return nil
}

func (f *DataFrame) GetType() FrameType {
	return DATA
}
func (f *HeadersFrame) GetType() FrameType {
	return HEADERS
}
func (f *PriorityFrame) GetType() FrameType {
	return PRIORITY
}
func (f *RstStreamFrame) GetType() FrameType {
	return RST_STREAM
}
func (f *SettingsFrame) GetType() FrameType {
	return SETTINGS
}
func (f *PushPromiseFrame) GetType() FrameType {
	return PUSH_PROMISE
}
func (f *PingFrame) GetType() FrameType {
	return PING
}
func (f *GoAwayFrame) GetType() FrameType {
	return GOAWAY
}
func (f *WindowUpdateFrame) GetType() FrameType {
	return WINDOW_UPDATE
}
func (f *ContinuationFrame) GetType() FrameType {
	return CONTINUATION
}
