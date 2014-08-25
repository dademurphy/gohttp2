// Copyright 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package http2

import (
	"github.com/petar/GoLLRB/llrb"
)

type EntryType int

const (
	// Values affect entry ordering. Lookup entries are ordered before
	// Dynamic entries, which are ordered before Static entries.
	LookupEntry  EntryType = 0
	DynamicEntry EntryType = 1
	StaticEntry  EntryType = 2

	kInitialHpackEntryQueueSize int = 16
)

type HpackEntry struct {
	Name, Value string

	Type           EntryType
	InsertionIndex int
}

func HpackEntrySize(name, value string) int {
	return len(name) + len(value) + HpackOverheadPerEntry
}

func (e *HpackEntry) Size() int {
	return len(e.Name) + len(e.Value) + HpackOverheadPerEntry
}

func (lhs *HpackEntry) Less(item llrb.Item) bool {
	rhs := item.(*HpackEntry)

	if lhs.Name < rhs.Name {
		return true
	} else if lhs.Name > rhs.Name {
		return false
	}

	if lhs.Value < rhs.Value {
		return true
	} else if lhs.Value > rhs.Value {
		return false
	}

	if lhs.Type < rhs.Type {
		return true
	} else if lhs.Type > rhs.Type {
		return false
	}

	// Most-recent entry is ordered first.
	return lhs.InsertionIndex > rhs.InsertionIndex
}

type HpackEntryQueue struct {
	entries []HpackEntry
	head, tail, size int
}

// Inserts and returns a nil HpackEntry at the queue head.
func (q *HpackEntryQueue) PushFront() *HpackEntry {
	if q.head == q.tail && q.size > 0 {
		entries = make([]HpackEntry, len(q.entries)*2)
		copy(entries, q.entries[q.head:])
		copy(entries[len(q.entries)-q.head:], q.entries[:q.head])
		q.head = 0
		q.tail = len(q.nodes)
		q.entries = entries
	}


}



