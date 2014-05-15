// Copyright 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package http2

import (
	"fmt"
)

type StreamID uint32

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

type ErrorCode uint32

const (
	NO_ERROR            ErrorCode = 0x00
	PROTOCOL_ERROR      ErrorCode = 0x01
	INTERNAL_ERROR      ErrorCode = 0x02
	FLOW_CONTROL_ERROR  ErrorCode = 0x03
	SETTINGS_TIMEOUT    ErrorCode = 0x04
	STREAM_CLOSED       ErrorCode = 0x05
	FRAME_SIZE_ERROR    ErrorCode = 0x06
	REFUSED_STREAM      ErrorCode = 0x07
	CANCEL              ErrorCode = 0x08
	COMPRESSION_ERROR   ErrorCode = 0x09
	CONNECT_ERROR       ErrorCode = 0x10
	ENHANCE_YOUR_CALM   ErrorCode = 0x11
	INADEQUATE_SECURITY ErrorCode = 0x12
)

type ErrorLevel uint8

const (
	// Default. Error must be handled by breaking the connection.
	ConnectionError ErrorLevel = 0
	// Connection may continue, but stream must be reset.
	StreamError ErrorLevel = iota
	// No explicit error handling required. Eg, DATA
	// received shortly after sending a RST_STREAM.
	RecoverableError ErrorLevel = iota
)

// Wrapper around error, satisfying the error interface
// but additionally capturing an ErrorCode.
type Error struct {
	Code  ErrorCode
	Level ErrorLevel
	Err   error
}

func NewError(code ErrorCode, errArgs ...interface{}) *Error {
	if len(errArgs) == 0 {
		return &Error{Code: code}
	}

	var err error
	switch t := errArgs[0].(type) {
	case error:
		err = t
	case string:
		err = fmt.Errorf(t, errArgs[1:]...)
	default:
		err = fmt.Errorf("%#v", errArgs)
	}
	return &Error{
		Code:  code,
		Level: ConnectionError,
		Err:   err,
	}
}

func (e *Error) Error() string {
	return e.Err.Error()
}

func protocolError(errArgs ...interface{}) *Error {
	return NewError(PROTOCOL_ERROR, errArgs...)
}
func internalError(errArgs ...interface{}) *Error {
	return NewError(INTERNAL_ERROR, errArgs...)
}
func flowControlError(errArgs ...interface{}) *Error {
	return NewError(FLOW_CONTROL_ERROR, errArgs...)
}
func frameSizeError(errArgs ...interface{}) *Error {
	return NewError(FRAME_SIZE_ERROR, errArgs...)
}

type SettingID uint8

const (
	SETTINGS_HEADER_TABLE_SIZE      SettingID = 0x01
	SETTINGS_ENABLE_PUSH            SettingID = 0x02
	SETTINGS_MAX_CONCURRENT_STREAMS SettingID = 0x03
	SETTINGS_INITIAL_WINDOW_SIZE    SettingID = 0x04

	// For range-tests of SettingID validity.
	SETTINGS_MIN_SETTING_ID SettingID = SETTINGS_HEADER_TABLE_SIZE
	SETTINGS_MAX_SETTING_ID SettingID = SETTINGS_INITIAL_WINDOW_SIZE
)

var kSettingDefaults = [...]uint32{
	// (Not a setting)
	0,
	// SETTINGS_HEADER_TABLE_SIZE
	0x00001000, // 4096.
	// SETTINGS_ENABLE_PUSH
	1,
	// SETTINGS_MAX_CONCURRENT_STREAMS
	0xffffffff,
	// SETTINGS_INITIAL_WINDOW_SIZE
	0x0000ffff, // 65,535
}
