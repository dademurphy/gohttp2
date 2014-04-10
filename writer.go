// Copyright 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package http2

import (
  "io"
)

type Writer struct {
  writer io.Writer
  bits BitSequence
}

func (w *Writer) WriteBits(bits BitSequence) (n int, err error) {
  if bits.Length + w.bits.Length > 64 {
    n, err = w.FlushBits()
  }
  if bits.Length + w.bits.Length > 64 {
    panic("Too many bits to write")
  }
  w.bits.Bits |= bits.Bits << (64 - 8 - w.bits.Length)
  w.bits.Length += bits.Length
  return
}

func (w *Writer) FlushBits() (int, error) {
  buffer := [...]byte{
    byte(w.bits.Bits >> 56),
    byte(w.bits.Bits >> 48),
    byte(w.bits.Bits >> 40),
    byte(w.bits.Bits >> 32),
    byte(w.bits.Bits >> 24),
    byte(w.bits.Bits >> 16),
    byte(w.bits.Bits >> 8),
    byte(w.bits.Bits)}
  n, err := w.writer.Write(buffer[:w.bits.Length / 8])
  w.bits.Length -= uint(n) * 8
  return n, err
}

func (w *Writer) ByteRemainder() uint {
  return 8 - w.bits.Length % 8
}

func (w *Writer) Write(p []byte) (int, error) {
  if w.bits.Length != 0 {
    panic("Unflushed written bits")
  }
  return w.writer.Write(p)
}
