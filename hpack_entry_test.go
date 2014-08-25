// Copyright 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package http2

import (
	gc "gopkg.in/check.v1"
)

type HpackEntryTest struct {
}
var _ = gc.Suite(&HpackEntryTest{})

// TODO(johng): HpackEntry tests (particularly ordering).

