// Copyright 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package http2

type HeaderField struct {
	// Header name. Must be lower-case.
	Name string
	// Header value, or values if delimited by '\0'.
	Values string
	// Whether value may participate in delta encoding.
	NeverDeltaEncode bool
}

type Frame interface {
	GetType() FrameType
	GetFlags() Flags
	GetStreamID() StreamID
}

// Models the Flags and StreamID fields common to all frame types.
type FramePrefix struct {
	Flags    Flags
	StreamID StreamID
}

func (f *FramePrefix) GetFlags() Flags {
	return f.Flags
}
func (f *FramePrefix) GetStreamID() StreamID {
	return f.StreamID
}

// Models frames carrying padding (DATA, HEADERS, PUSH_PROMISE,
// and CONTINUATION).
type FramePadding struct {
	PaddingLength uint16
}

// Models frames carrying a priority update (HEADERS, PRIORITY)
type FramePriority struct {
	PriorityGroup  uint32
	PriorityWeight uint8

	ExclusiveDependency bool
	StreamDependency    StreamID
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

	Fields []HeaderField
}

type PriorityFrame struct {
	FramePrefix
	FramePriority
}

type RstStreamFrame struct {
	FramePrefix

	Error Error
}

type SettingsFrame struct {
	FramePrefix

	Settings map[SettingID]uint32
}

type PushPromiseFrame struct {
	FramePrefix
	FramePadding

	PromisedID StreamID
	Fields     []HeaderField
}

type PingFrame struct {
	FramePrefix

	OpaqueData uint64
}

type GoAwayFrame struct {
	FramePrefix

	LastID StreamID
	Error  Error
}

type WindowUpdateFrame struct {
	FramePrefix

	SizeDelta uint32
}

type ContinuationFrame struct {
	FramePrefix
	FramePadding

	Fields []HeaderField
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

func (f *DataFrame) PayloadLength() int {
	return len(f.Data) + int(f.PaddingLength)
}
func (f *DataFrame) SplitAt(bound int) *DataFrame {
	// TODO(johng)
	panic(bound)
	return nil
}
