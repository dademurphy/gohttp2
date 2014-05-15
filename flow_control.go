// Copyright 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package http2

type RecieveFlow struct {
	// Read bytes which have not been acknowledge by a sent WINDOW_UPDATE.
	WinUsed int
	// Read and consumed bytes which have not
	// been acknowledged by a sent WINDOW_UPDATE.
	WinUnacked int
	// Total size of the receive window.
	WinSize int
}

func (f *RecieveFlow) ApplyDataRecieved(data *DataFrame) *Error {
	f.WinUsed += len(data.Data) + int(data.PaddingLength)
	if f.WinUsed > f.WinSize {
		return flowControlError("DATA exceeded available window (%v vs %v)",
			f.WinUsed, f.WinSize)
	}
	return nil
}
func (f *RecieveFlow) ApplyDataConsumed(data *DataFrame) {
	f.WinUnacked += len(data.Data) + int(data.PaddingLength)
}
func (f *RecieveFlow) OverUnackedThreshold() bool {
	return f.WinUnacked*2 > f.WinSize
}
func (f *RecieveFlow) BuildWindowUpdate(id StreamID) *WindowUpdateFrame {
	update := &WindowUpdateFrame{
		FramePrefix: FramePrefix{
			StreamID: id,
			Flags:    NO_FLAGS,
		},
		SizeDelta: uint32(f.WinUnacked),
	}
	f.WinUnacked = 0
	return update
}
