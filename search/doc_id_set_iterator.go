// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed under the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License. You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package search

import (
	"fmt"
	"io"
	"math"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// NO_MORE_DOCS is the sentinel value meaning the iterator has exhausted.
const NO_MORE_DOCS = math.MaxInt32

// DocIdSetIterator defines methods to iterate over a set of non-decreasing doc ids.
type DocIdSetIterator interface {
	// docID returns the current doc ID.
	// -1 if not yet positioned, NO_MORE_DOCS if exhausted.
	DocID() int

	// NextDoc advances to the next document and returns its ID.
	NextDoc() (int, error)

	// Advance advances to the first document >= target.
	Advance(target int) (int, error)

	// Cost returns an estimated cost of this iterator.
	Cost() int64

	// IntoBitSet loads doc IDs into a FixedBitSet.
	IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error

	// DocIDRunEnd returns the end of the current run of consecutive doc IDs.
	DocIDRunEnd() (int, error)
}

// AbstractDocIdSetIterator provides a base implementation that tracks the current doc ID.
type AbstractDocIdSetIterator struct {
	Doc int
}

func (a *AbstractDocIdSetIterator) DocID() int {
	return a.Doc
}

// RangeDocIdSetIterator matches documents in the range [minDoc, maxDoc).
type RangeDocIdSetIterator struct {
	AbstractDocIdSetIterator
	minDoc int
	maxDoc int
}

func NewRangeDocIdSetIterator(minDoc, maxDoc int) *RangeDocIdSetIterator {
	return &RangeDocIdSetIterator{
		AbstractDocIdSetIterator: AbstractDocIdSetIterator{Doc: -1},
		minDoc:                   minDoc,
		maxDoc:                   maxDoc,
	}
}

func (r *RangeDocIdSetIterator) NextDoc() (int, error) {
	return r.Advance(r.Doc + 1), nil
}

func (r *RangeDocIdSetIterator) Advance(target int) (int, error) {
	if target >= r.maxDoc {
		r.Doc = NO_MORE_DOCS
	} else if target < r.minDoc {
		r.Doc = r.minDoc
	} else {
		r.Doc = target
	}
	return r.Doc, nil
}

func (r *RangeDocIdSetIterator) Cost() int64 {
	return int64(r.maxDoc - r.minDoc)
}

func (r *RangeDocIdSetIterator) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	if upTo > r.Doc {
		limit := upTo
		if r.maxDoc < limit {
			limit = r.maxDoc
		}
		if limit > r.Doc {
			bitSet.Set(r.Doc-offset, limit-offset)
			r.Advance(limit)
		}
	}
	return nil
}

func (r *RangeDocIdSetIterator) DocIDRunEnd() (int, error) {
	return r.maxDoc, nil
}

// Empty returns an empty DocIdSetIterator.
func Empty() DocIdSetIterator {
	return NewRangeDocIdSetIterator(0, 0)
}

// All returns a DocIdSetIterator that matches all documents up to maxDoc-1.
func All(maxDoc int) DocIdSetIterator {
	if maxDoc < 0 {
		panic(fmt.Sprintf("maxDoc must be >= 0, but got maxDoc=%d", maxDoc))
	}
	return NewRangeDocIdSetIterator(0, maxDoc)
}

// Range returns a DocIdSetIterator that matches documents in [minDoc, maxDoc).
func Range(minDoc, maxDoc int) DocIdSetIterator {
	if minDoc >= maxDoc {
		panic(fmt.Sprintf("minDoc must be < maxDoc but got minDoc=%d maxDoc=%d", minDoc, maxDoc))
	}
	if minDoc < 0 {
		panic(fmt.Sprintf("minDoc must be >= 0 but got minDoc=%d", minDoc))
	}
	return NewRangeDocIdSetIterator(minDoc, maxDoc)
}
