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

	"github.com/FlavioCFOliveira/Gocene/util"
)

// NO_MORE_DOCS is the sentinel value meaning the iterator has exhausted.
// DocIdSetIterator.NO_MORE_DOCS in Apache Lucene 10.5.0; it is the same
// constant as util.NO_MORE_DOCS, which the canonical declaration carries.
const NO_MORE_DOCS = util.NO_MORE_DOCS

// DocIdSetIterator is org.apache.lucene.search.DocIdSetIterator (Apache Lucene
// 10.5.0), under the package and name Lucene gives it.
//
// Gocene declares the type exactly once, in util, because util types that
// Lucene also expresses in terms of it — BitSet.or(DocIdSetIterator) and
// BitSet.of(DocIdSetIterator, int) — must refer to it, and util cannot import
// search or spi without an import cycle. This alias is the same convention the
// port already applies to SortField (declared in spi, aliased here) and to
// LeafReaderContext (declared in spi, aliased in index).
type DocIdSetIterator = util.DocIdSetIterator

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
	return r.Advance(r.Doc + 1)
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
			bitSet.SetRange(r.Doc-offset, limit-offset)
			if _, err := r.Advance(limit); err != nil {
				return err
			}
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

// BaseDocIdSetIterator carries the concrete members of the abstract class
// org.apache.lucene.search.DocIdSetIterator (Lucene 10.5.0) that do not
// dispatch back to an abstract method: slowAdvance(int).
//
// intoBitSet(int, FixedBitSet, int) and docIDRunEnd() are concrete in Java too,
// but their bodies call the abstract docID() and nextDoc(); they are therefore
// rendered as the free functions DefaultIntoBitSet and DefaultDocIDRunEnd
// below, which take the concrete iterator explicitly.
//
// Java's docID(), nextDoc(), advance(int) and cost() are abstract and are
// therefore not provided here: the embedder must supply them.
type BaseDocIdSetIterator struct{}

// SlowAdvance mirrors the protected final helper
// DocIdSetIterator.slowAdvance(int): it advances it by calling NextDoc until a
// document at or beyond target is reached.
func (b *BaseDocIdSetIterator) SlowAdvance(it DocIdSetIterator, target int) (int, error) {
	return util.SlowAdvance(it, target)
}

// DefaultIntoBitSet is the body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene 10.5.0.
// The single body lives with the single declaration of the type, in util.
func DefaultIntoBitSet(it DocIdSetIterator, upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(it, upTo, bitSet, offset)
}

// DefaultDocIDRunEnd is the body of DocIdSetIterator.docIDRunEnd() in Apache
// Lucene 10.5.0, which returns docID() + 1.
func DefaultDocIDRunEnd(it DocIdSetIterator) (int, error) {
	return util.DefaultDocIDRunEnd(it)
}
