// Copyright 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package http2

import (
  "io"
)

type BitSequence struct {
	Bits   uint64
	Length uint
}

type Reader struct {
  reader io.Reader
  bits BitSequence
}

func (r *Reader) Read(p []byte) (int, error) {
  if r.bits.Length % 8 != 0 {
    panic("Unread byte remainder")
  }
  wIndex := 0
  for r.bits.Length != 0 {
    p[wIndex] = byte(r.bits.Bits >> 56)
    r.bits.Bits <<= 8
    r.bits.Length -= 8
    wIndex += 1
  }
  n, err := r.reader.Read(p[wIndex:])
  return n + wIndex, err
}

func (r *Reader) PeekBits() (BitSequence, error) {
  var buffer [8]byte

  count := 8 - (r.bits.Length / 8)
  if r.bits.Length % 8 != 0 {
    count -= 1
  }
  if count == 0 {
    return r.bits, nil
  }
  n, err := r.reader.Read(buffer[:count])
  for i := 0; i != n; i++ {
    r.bits.Bits |= uint64(buffer[i]) << (64 - 8 - r.bits.Length)
    r.bits.Length += 8
  }
  return r.bits, err
}

func (r *Reader) ConsumeBits(length uint) {
  if length > r.bits.Length {
    panic("Invalid length")
  }
  r.bits.Bits <<= length
  r.bits.Length -= length
}

func (r *Reader) ByteRemainder() uint {
  return r.bits.Length % 8
}
