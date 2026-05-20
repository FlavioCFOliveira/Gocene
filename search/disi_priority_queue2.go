// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

package search

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/DisiPriorityQueue2.java

// DisiPriorityQueue2 is a DisiPriorityQueue of two entries or less.
// It avoids heap overhead by storing up to two wrappers directly as fields.
//
// Mirrors org.apache.lucene.search.DisiPriorityQueue2 (Lucene 10.4.0).
type DisiPriorityQueue2 struct {
	top  *DisiWrapper
	top2 *DisiWrapper
}

// NewDisiPriorityQueue2 returns an empty DisiPriorityQueue2.
func NewDisiPriorityQueue2() *DisiPriorityQueue2 { return &DisiPriorityQueue2{} }

// Size returns the number of entries (0, 1, or 2).
func (pq *DisiPriorityQueue2) Size() int {
	if pq.top2 != nil {
		return 2
	}
	if pq.top != nil {
		return 1
	}
	return 0
}

// Top returns the entry with the smallest doc, or nil when empty.
func (pq *DisiPriorityQueue2) Top() *DisiWrapper { return pq.top }

// Top2 returns the second-smallest entry, or nil when size < 2.
func (pq *DisiPriorityQueue2) Top2() *DisiWrapper { return pq.top2 }

// TopList returns the linked list of wrappers that share the current top doc.
// Only top is included unless top2 has the same doc, in which case top2 is
// prepended.
func (pq *DisiPriorityQueue2) TopList() *DisiWrapper {
	if pq.top == nil {
		return nil
	}
	pq.top.next = nil
	topList := pq.top
	if pq.top2 != nil && pq.top.doc == pq.top2.doc {
		pq.top2.next = topList
		topList = pq.top2
	}
	return topList
}

// Add inserts entry into the queue (max 2 entries; panics if a third is added).
func (pq *DisiPriorityQueue2) Add(entry *DisiWrapper) *DisiWrapper {
	if pq.top == nil {
		pq.top = entry
		return pq.top
	}
	if pq.top2 == nil {
		pq.top2 = entry
		return pq.UpdateTop()
	}
	panic("DisiPriorityQueue2: cannot add a 3rd element (max size is 2)")
}

// Pop removes and returns the top entry; the former top2 becomes the new top.
func (pq *DisiPriorityQueue2) Pop() *DisiWrapper {
	ret := pq.top
	pq.top = pq.top2
	pq.top2 = nil
	return ret
}

// UpdateTop reorders top and top2 so that top has the smaller doc.
func (pq *DisiPriorityQueue2) UpdateTop() *DisiWrapper {
	if pq.top2 != nil && pq.top2.doc < pq.top.doc {
		pq.top, pq.top2 = pq.top2, pq.top
	}
	return pq.top
}

// UpdateTopWith replaces the top entry with topReplacement and rebalances.
func (pq *DisiPriorityQueue2) UpdateTopWith(topReplacement *DisiWrapper) *DisiWrapper {
	pq.top = topReplacement
	return pq.UpdateTop()
}

// Clear removes all entries.
func (pq *DisiPriorityQueue2) Clear() {
	pq.top = nil
	pq.top2 = nil
}

// Slice returns a slice over all current entries for range-iteration, ordered
// by doc (top first).
func (pq *DisiPriorityQueue2) Slice() []*DisiWrapper {
	switch pq.Size() {
	case 0:
		return nil
	case 1:
		return []*DisiWrapper{pq.top}
	default:
		return []*DisiWrapper{pq.top, pq.top2}
	}
}
