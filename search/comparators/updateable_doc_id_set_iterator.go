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

// Package comparators hosts the Sprint 51 ports for
// org.apache.lucene.search.comparators.
package comparators

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/comparators/UpdateableDocIdSetIterator.java

import "github.com/FlavioCFOliveira/Gocene/search"

// UpdateableDocIdSetIterator is a DocIdSetIterator that wraps a mutable inner
// iterator. Calling Update replaces the inner iterator without repositioning
// this iterator's current doc. The next Advance or NextDoc call drives the new
// inner iterator forward from the current position.
//
// Mirrors org.apache.lucene.search.comparators.UpdateableDocIdSetIterator
// (Lucene 10.4.0).
//
// Deviations from Java:
//   - Java extends AbstractDocIdSetIterator which tracks protected field doc.
//     Go tracks doc explicitly.
//   - Java's intoBitSet(upTo, FixedBitSet, offset) is not on Gocene's
//     DocIdSetIterator interface; that method is omitted. The optional
//     IntoBitSet method may be added when the interface is extended.
//   - Java's docIDRunEnd() override delegates to in.docIDRunEnd() when in is
//     synced to the current doc; Go replicates this via DocIDRunEnd().
type UpdateableDocIdSetIterator struct {
	in  search.DocIdSetIterator
	doc int
}

// NewUpdateableDocIdSetIterator creates an UpdateableDocIdSetIterator
// starting at doc = -1 with an empty inner iterator.
func NewUpdateableDocIdSetIterator() *UpdateableDocIdSetIterator {
	return &UpdateableDocIdSetIterator{
		in:  search.NewEmptyDocIdSetIterator(),
		doc: -1,
	}
}

// Update replaces the inner DocIdSetIterator. The new iterator does not need
// to be positioned on the same doc as this iterator; the next Advance or
// NextDoc call will synchronise it.
//
// Mirrors UpdateableDocIdSetIterator.update(DocIdSetIterator).
func (it *UpdateableDocIdSetIterator) Update(iterator search.DocIdSetIterator) {
	if iterator == nil {
		panic("UpdateableDocIdSetIterator.Update: iterator must not be nil")
	}
	it.in = iterator
}

// DocID returns the current document ID.
func (it *UpdateableDocIdSetIterator) DocID() int {
	return it.doc
}

// NextDoc advances to the next document (doc+1).
//
// Mirrors AbstractDocIdSetIterator.nextDoc() which calls advance(doc+1).
func (it *UpdateableDocIdSetIterator) NextDoc() (int, error) {
	return it.Advance(it.doc + 1)
}

// Advance advances the iterator to the first document ≥ target.
//
// Mirrors UpdateableDocIdSetIterator.advance(int).
func (it *UpdateableDocIdSetIterator) Advance(target int) (int, error) {
	curDoc := it.in.DocID()
	if curDoc < target {
		var err error
		curDoc, err = it.in.Advance(target)
		if err != nil {
			return search.NO_MORE_DOCS, err
		}
	}
	it.doc = curDoc
	return it.doc, nil
}

// Cost returns the cost of the inner iterator.
func (it *UpdateableDocIdSetIterator) Cost() int64 {
	return it.in.Cost()
}

// DocIDRunEnd returns the end of the current consecutive doc-ID run.
//
// Mirrors UpdateableDocIdSetIterator.docIDRunEnd():
//   - If the inner iterator's docID is behind the current position, advance it.
//   - If after advancement the inner iterator is at the current doc, return
//     its docIDRunEnd (potentially a larger run).
//   - Otherwise fall back to doc+1 (mirrors AbstractDocIdSetIterator.docIDRunEnd()).
func (it *UpdateableDocIdSetIterator) DocIDRunEnd() int {
	// Re-sync inner iterator in case Update was called.
	if it.in.DocID() < it.doc {
		_, err := it.in.Advance(it.doc)
		if err != nil {
			return it.doc + 1
		}
	}
	if it.in.DocID() == it.doc {
		return it.in.DocIDRunEnd()
	}
	// Inner iterator has moved past doc (or doc is NO_MORE_DOCS).
	return it.doc + 1
}

var _ search.DocIdSetIterator = (*UpdateableDocIdSetIterator)(nil)
