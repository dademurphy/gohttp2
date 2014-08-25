// Copyright 2014 The Chromium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.
package http2

import (
	"github.com/petar/GoLLRB/llrb"
)

type HpackTable struct {
	DynamicEntries map[int]*HpackEntry

	// Items are *HpackEntry.
	Index        *llrb.LLRB
	ReferenceSet *llrb.LLRB

	TotalInsertions int

	Size, MaxSize, SettingMaxSize int
}

func NewHpackTable() *HpackTable {
	table := &HpackTable{
		Index:           llrb.New(),
		TotalInsertions: len(kHpackStaticTable) + 1,
		MaxSize:         HpackInitialTableSize,
		SettingMaxSize:  HpackInitialTableSize,
	}
	for i := range kHpackStaticTable {
		table.Index.InsertNoReplace(&kHpackStaticTable[i])
	}
	return table
}

// Returns the entry matching |index|, or nil.
func (t *HpackTable) GetByIndex(index int) *HpackEntry {
	if index == 0 {
		return nil
	}
	index -= 1
	if deLen := len(t.DynamicEntries); index < deLen {
		return t.DynamicEntries[t.TotalInsertions-index-1]
	} else {
		index -= deLen
	}
	if index < len(kHpackStaticTable) {
		return &kHpackStaticTable[index]
	}
	return nil
}

// Returns an entry having |name|, or nil.
func (t *HpackTable) GetByName(name string) *HpackEntry {
	var result *HpackEntry
	t.Index.AscendGreaterOrEqual(&HpackEntry{Name: name},
		func(item llrb.Item) bool {
			result = item.(*HpackEntry)
			return false
		})
	return result
}

// Returns the lowest-index entry having |name| and |value|, or nil.
func (t *HpackTable) GetByNameAndValue(name, value string) *HpackEntry {
	var result *HpackEntry
	t.Index.AscendGreaterOrEqual(&HpackEntry{Name: name, Value: value},
		func(item llrb.Item) bool {
			result = item.(*HpackEntry)
			return false
		})
	return result
}

// Calls |iter| with each of the set of entries which would be evicted by an
// insertion of |name| and |value|. No eviction actually occurs.
func (t *HpackTable) EvictionSet(name, value string, iter func(*HpackEntry)) {
	reclaimSize := HpackEntrySize(name, value) - (t.MaxSize - t.Size)

	t.DynamicEntries.WalkTail(func(e *HpackEntry) bool {
		if reclaimSize <= 0 {
			return false
		}
		reclaimSize -= e.Size()
		iter(e)
		return true
	})
}
