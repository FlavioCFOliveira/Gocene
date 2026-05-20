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
//   lucene/core/src/java/org/apache/lucene/search/MultiPhraseQuery.java
//   (inner classes UnionPostingsEnum and PositionsQueue)

import (
	"sort"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// UnionPostingsEnum is a union of PostingsEnum instances that presents them
// as a single merged sequence ordered by docID and then position.
//
// Mirrors MultiPhraseQuery.UnionPostingsEnum (Lucene 10.4.0).
//
// Implementation note: Lucene uses a min-heap ordered by docID. Gocene's
// index.NO_MORE_DOCS = -1 (vs Java's Integer.MAX_VALUE), which would cause
// exhausted subs to float to the top of a min-heap and corrupt ordering.
// A linear scan across a small sub-slice (typical phrase matching uses 1–4
// subs) is equivalent in practice and avoids the sentinel mismatch entirely.
type UnionPostingsEnum struct {
	subs        []index.PostingsEnum
	posQueue    *positionsQueue
	posQueueDoc int
	curDoc      int
	cost        int64
}

// NewUnionPostingsEnum creates a UnionPostingsEnum over the supplied subs.
// Each sub must be in its pre-iteration state (DocID() == -1 before the
// first NextDoc call).
func NewUnionPostingsEnum(subs []index.PostingsEnum) *UnionPostingsEnum {
	var cost int64
	for _, sub := range subs {
		cost += sub.Cost()
	}
	return &UnionPostingsEnum{
		subs:        subs,
		posQueue:    &positionsQueue{array: make([]int, 0, 16)},
		posQueueDoc: -2,
		curDoc:      -1, // pre-iteration sentinel (Java convention)
		cost:        cost,
	}
}

// DocID returns the current document ID. Returns -1 before the first NextDoc
// and index.NO_MORE_DOCS when exhausted.
func (u *UnionPostingsEnum) DocID() int {
	return u.curDoc
}

// NextDoc advances to the next document. Returns the new docID or
// index.NO_MORE_DOCS when exhausted.
//
// All subs currently at curDoc (including pre-iteration subs at -1) are
// advanced, then the minimum valid docID across all subs is returned.
//
// Mirrors UnionPostingsEnum.nextDoc (Lucene 10.4.0).
func (u *UnionPostingsEnum) NextDoc() (int, error) {
	curDoc := u.curDoc
	for _, sub := range u.subs {
		if sub.DocID() == curDoc {
			if _, err := sub.NextDoc(); err != nil {
				return 0, err
			}
		}
	}
	next := u.minValidDoc()
	u.curDoc = next
	return next, nil
}

// Advance moves to the first document >= target.
//
// Mirrors UnionPostingsEnum.advance (Lucene 10.4.0).
func (u *UnionPostingsEnum) Advance(target int) (int, error) {
	for _, sub := range u.subs {
		if sub.DocID() < target {
			if _, err := sub.Advance(target); err != nil {
				return 0, err
			}
		}
	}
	next := u.minValidDoc()
	u.curDoc = next
	return next, nil
}

// Freq returns the total number of positions across all matching subs for
// the current document. Positions are collected and sorted on first call.
//
// Mirrors UnionPostingsEnum.freq (Lucene 10.4.0).
func (u *UnionPostingsEnum) Freq() (int, error) {
	doc := u.curDoc
	if doc != u.posQueueDoc {
		u.posQueue.clear()
		for _, sub := range u.subs {
			if sub.DocID() == doc {
				freq, err := sub.Freq()
				if err != nil {
					return 0, err
				}
				for i := 0; i < freq; i++ {
					pos, err := sub.NextPosition()
					if err != nil {
						return 0, err
					}
					u.posQueue.add(pos)
				}
			}
		}
		u.posQueue.sortQueue()
		u.posQueueDoc = doc
	}
	return u.posQueue.size(), nil
}

// NextPosition returns the next position for the current document in
// ascending order. Must only be called after Freq().
func (u *UnionPostingsEnum) NextPosition() (int, error) {
	return u.posQueue.next(), nil
}

// StartOffset returns -1 (offsets are not supported).
func (u *UnionPostingsEnum) StartOffset() (int, error) { return -1, nil }

// EndOffset returns -1 (offsets are not supported).
func (u *UnionPostingsEnum) EndOffset() (int, error) { return -1, nil }

// GetPayload returns nil (payloads are not supported).
func (u *UnionPostingsEnum) GetPayload() ([]byte, error) { return nil, nil }

// Cost returns the total cost (sum of sub costs).
func (u *UnionPostingsEnum) Cost() int64 { return u.cost }

// minValidDoc returns the minimum docID across all subs that are not
// exhausted (DocID() != index.NO_MORE_DOCS). Returns index.NO_MORE_DOCS
// when all subs are exhausted.
func (u *UnionPostingsEnum) minValidDoc() int {
	found := false
	minDoc := 0
	for _, sub := range u.subs {
		d := sub.DocID()
		if d != index.NO_MORE_DOCS {
			if !found || d < minDoc {
				minDoc = d
				found = true
			}
		}
	}
	if !found {
		return index.NO_MORE_DOCS
	}
	return minDoc
}

// ─── positionsQueue ──────────────────────────────────────────────────────────

// positionsQueue is a sorted array of positions for a single document.
// Mirrors MultiPhraseQuery.UnionPostingsEnum.PositionsQueue (Lucene 10.4.0).
type positionsQueue struct {
	array []int
	index int
}

func (q *positionsQueue) add(pos int) {
	q.array = append(q.array, pos)
}

func (q *positionsQueue) next() int {
	v := q.array[q.index]
	q.index++
	return v
}

func (q *positionsQueue) sortQueue() {
	sort.Ints(q.array[q.index:])
}

func (q *positionsQueue) clear() {
	q.array = q.array[:0]
	q.index = 0
}

func (q *positionsQueue) size() int {
	return len(q.array) - q.index
}

// Compile-time assertion: UnionPostingsEnum implements index.PostingsEnum.
var _ index.PostingsEnum = (*UnionPostingsEnum)(nil)
