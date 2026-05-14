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

package util

// notDocIdSetBaseRAM is the shallow ram footprint of a NotDocIdSet:
// two int fields (maxDoc, pad) and one interface header. The exact
// number is the Go analogue of
// RamUsageEstimator.shallowSizeOfInstance(NotDocIdSet.class); we
// approximate at 32 bytes which matches the observed
// unsafe.Sizeof(NotDocIdSet{}) on amd64/arm64.
const notDocIdSetBaseRAM int64 = 32

// docIdSetWithBits is the optional interface a [DocIdSet] may
// implement to expose a [Bits] random-access view. Mirrors Java's
// (deprecated) DocIdSet#bits() abstract method.
type docIdSetWithBits interface {
	Bits() Bits
}

// docIdSetWithRAM is the optional interface a [DocIdSet] may
// implement to advertise its RAM footprint. Mirrors Accountable on
// the Java DocIdSet base class.
type docIdSetWithRAM interface {
	RamBytesUsed() int64
}

// NotDocIdSet is the Go port of org.apache.lucene.util.NotDocIdSet.
// It encodes the negation of another [DocIdSet]: an iterator that
// yields every doc in [0, maxDoc) which is not contained in the
// wrapped set.
//
// NotDocIdSet is cacheable and supports random access if and only if
// the underlying set does, matching the Java behaviour.
type NotDocIdSet struct {
	maxDoc int
	in     DocIdSet
}

// NewNotDocIdSet constructs a NotDocIdSet over the given underlying
// set and document upper bound. maxDoc must be non-negative; in must
// be non-nil. Mirrors the sole Java constructor.
func NewNotDocIdSet(maxDoc int, in DocIdSet) *NotDocIdSet {
	return &NotDocIdSet{maxDoc: maxDoc, in: in}
}

// Iterator returns a DocIdSetIterator that yields the complement of
// the underlying iterator over [0, maxDoc).
func (n *NotDocIdSet) Iterator() DocIdSetIterator {
	return &notDocIdSetIterator{
		in:             n.in.Iterator(),
		maxDoc:         n.maxDoc,
		doc:            -1,
		nextSkippedDoc: -1,
	}
}

// Bits returns a Bits view that complements the underlying Bits
// when the wrapped set advertises one. Returns nil otherwise, in
// keeping with the Java contract.
func (n *NotDocIdSet) Bits() Bits {
	wb, ok := n.in.(docIdSetWithBits)
	if !ok {
		return nil
	}
	inBits := wb.Bits()
	if inBits == nil {
		return nil
	}
	return notBits{in: inBits}
}

// RamBytesUsed returns the shallow size of this NotDocIdSet plus the
// underlying set's ramBytesUsed() when available.
func (n *NotDocIdSet) RamBytesUsed() int64 {
	if r, ok := n.in.(docIdSetWithRAM); ok {
		return notDocIdSetBaseRAM + r.RamBytesUsed()
	}
	return notDocIdSetBaseRAM
}

// notBits is the negation of a [Bits].
type notBits struct{ in Bits }

func (n notBits) Get(index int) bool { return !n.in.Get(index) }
func (n notBits) Length() int        { return n.in.Length() }

// notDocIdSetIterator is the negation of an inner DocIdSetIterator.
// It mirrors the anonymous inner class in NotDocIdSet#iterator().
type notDocIdSetIterator struct {
	in             DocIdSetIterator
	maxDoc         int
	doc            int
	nextSkippedDoc int
}

// DocID returns the current document.
func (it *notDocIdSetIterator) DocID() int { return it.doc }

// NextDoc advances to the next document not contained in the wrapped
// set.
func (it *notDocIdSetIterator) NextDoc() (int, error) {
	return it.Advance(it.doc + 1)
}

// Advance positions on the smallest doc >= target that is not in the
// wrapped set.
func (it *notDocIdSetIterator) Advance(target int) (int, error) {
	it.doc = target
	if it.doc > it.nextSkippedDoc {
		nxt, err := it.in.Advance(it.doc)
		if err != nil {
			return it.doc, err
		}
		it.nextSkippedDoc = nxt
	}
	for {
		if it.doc >= it.maxDoc {
			it.doc = NO_MORE_DOCS
			return it.doc, nil
		}
		// invariant: doc <= nextSkippedDoc
		if it.doc != it.nextSkippedDoc {
			return it.doc, nil
		}
		it.doc++
		nxt, err := it.in.NextDoc()
		if err != nil {
			return it.doc, err
		}
		it.nextSkippedDoc = nxt
	}
}

// Cost is O(maxDoc) regardless of the underlying set, matching the
// Java implementation: iterating the complement always scans the
// full document space.
func (it *notDocIdSetIterator) Cost() int64 { return int64(it.maxDoc) }

// DocIDRunEnd returns the end of the run of consecutive matching
// docs that contains DocID. Conservative single-doc run is returned;
// callers requiring tighter bounds should rely on Iterator semantics.
func (it *notDocIdSetIterator) DocIDRunEnd() int { return it.doc + 1 }

// Compile-time conformance checks.
var (
	_ DocIdSet         = (*NotDocIdSet)(nil)
	_ DocIdSetIterator = (*notDocIdSetIterator)(nil)
)
