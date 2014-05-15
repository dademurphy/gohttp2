// Copyright 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package http2

import (
	"container/heap"
)

type queuedFrame struct {
	frame    Frame
	priority int64
}
type queuedFrameHeap []queuedFrame

// TODO: Use priorities. Prioritize control frames, etc. Fair scheduling.
type writeQueue struct {
	frames      queuedFrameHeap
	queuedCount int64
}

func (q *writeQueue) enqueueBack(frame Frame) {
	heap.Push(&q.frames, queuedFrame{frame, q.queuedCount})
	q.queuedCount += 1
}

func (q *writeQueue) enqueueFront(frame Frame) {
	heap.Push(&q.frames, queuedFrame{frame, -q.queuedCount})
	q.queuedCount += 1
}

func (q *writeQueue) deque() (Frame, bool) {
	if len(q.frames) > 0 {
		result := q.frames[0].frame
		heap.Remove(&q.frames, 0)
		return result, true
	}
	return nil, false
}

// Functions for sort.Interface.
func (h queuedFrameHeap) Len() int {
	return len(h)
}
func (h queuedFrameHeap) Less(i, j int) bool {
	return h[i].priority < h[j].priority
}
func (h queuedFrameHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

// Functions for heap.Interface.
func (h *queuedFrameHeap) Push(x interface{}) {
	*h = append(*h, x.(queuedFrame))
}
func (h *queuedFrameHeap) Pop() interface{} {
	n := len(*h)
	result := (*h)[n-1]
	*h = (*h)[0 : n-1]
	return result
}
