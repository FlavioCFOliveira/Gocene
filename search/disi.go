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
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package search

// DisiPriorityQueue is a priority queue of DisiWrapper instances that orders by current doc ID.
//
// Mirrors org.apache.lucene.search.DisiPriorityQueue (Lucene 10.5.0).
type DisiPriorityQueue interface {
	// Size returns the number of entries in this heap.
	Size() int
	// Top returns the top value in this heap, or nil if the heap is empty.
	Top() *DisiWrapper
	// Top2 returns the 2nd least value in this heap, or nil if the heap contains less than 2 values.
	Top2() *DisiWrapper
	// TopList returns the list of scorers which are on the current doc.
	TopList() *DisiWrapper
	// Add inserts a DisiWrapper to this queue and returns the top entry.
	Add(entry *DisiWrapper) *DisiWrapper
	// AddAll bulk-inserts len entries from entries[offset:offset+len].
	AddAll(entries []*DisiWrapper, offset, length int)
	// Pop removes the top entry and returns it.
	Pop() *DisiWrapper
	// UpdateTop rebalances this heap and returns the top entry.
	UpdateTop() *DisiWrapper
	// UpdateTopWith replaces the top entry with the given entry, rebalances the heap, and returns the new top entry.
	UpdateTopWith(topReplacement *DisiWrapper) *DisiWrapper
	// Clear removes all entries from the heap.
	Clear()
}

// OfMaxSize creates a DisiPriorityQueue of the given maximum size.
func OfMaxSize(maxSize int) DisiPriorityQueue {
	if maxSize <= 2 {
		return NewDisiPriorityQueue2()
	}
	return NewDisiPriorityQueueN(maxSize)
}
