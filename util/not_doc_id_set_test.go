// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"testing"
)

// docIdSetFromInts is a small DocIdSet wrapper backed by an int slice.
// It deliberately does NOT implement Bits()/RamBytesUsed() so we can
// exercise the "no random access" branch of NotDocIdSet.
type docIdSetFromInts struct{ docs []int }

func (d *docIdSetFromInts) Iterator() DocIdSetIterator {
	return &intsDocIdSetIter{docs: d.docs, idx: -1, doc: -1}
}

type intsDocIdSetIter struct {
	docs []int
	idx  int
	doc  int
}

func (it *intsDocIdSetIter) DocID() int { return it.doc }
func (it *intsDocIdSetIter) NextDoc() (int, error) {
	it.idx++
	if it.idx >= len(it.docs) {
		it.doc = NO_MORE_DOCS
		return NO_MORE_DOCS, nil
	}
	it.doc = it.docs[it.idx]
	return it.doc, nil
}
func (it *intsDocIdSetIter) Advance(target int) (int, error) {
	for {
		d, err := it.NextDoc()
		if err != nil {
			return d, err
		}
		if d >= target {
			return d, nil
		}
	}
}
func (it *intsDocIdSetIter) Cost() int64      { return int64(len(it.docs)) }
func (it *intsDocIdSetIter) DocIDRunEnd() int { return it.doc + 1 }

// TestNotDocIdSet_Complement walks the negated iterator and verifies
// it yields exactly the doc ids missing from the wrapped set.
// Mirrors the BaseDocIdSetTestCase comparison from TestNotDocIdSet.
func TestNotDocIdSet_Complement(t *testing.T) {
	maxDoc := 10
	in := &docIdSetFromInts{docs: []int{0, 2, 5, 9}}
	ns := NewNotDocIdSet(maxDoc, in)

	want := []int{1, 3, 4, 6, 7, 8}
	got := make([]int, 0, len(want))
	it := ns.Iterator()
	for {
		d, err := it.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if d == NO_MORE_DOCS {
			break
		}
		got = append(got, d)
	}
	if len(got) != len(want) {
		t.Fatalf("got=%v want=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pos %d: got=%d want=%d", i, got[i], want[i])
		}
	}
}

// TestNotDocIdSet_AdvanceMidway exercises Advance past several skips.
func TestNotDocIdSet_AdvanceMidway(t *testing.T) {
	maxDoc := 20
	in := &docIdSetFromInts{docs: []int{0, 1, 2, 5, 7, 11, 15}}
	ns := NewNotDocIdSet(maxDoc, in)
	it := ns.Iterator()

	d, err := it.Advance(8)
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if d != 8 {
		t.Fatalf("Advance(8)=%d want 8", d)
	}
	d, _ = it.NextDoc()
	if d != 9 {
		t.Fatalf("NextDoc=%d want 9", d)
	}
}

// TestNotDocIdSet_BitsNil verifies the Java testBits assertion that
// a set without Bits() (DocIdSet.EMPTY in Java) yields Bits()==nil.
func TestNotDocIdSet_BitsNil(t *testing.T) {
	in := &docIdSetFromInts{docs: nil} // intentionally no Bits() method
	if b := NewNotDocIdSet(3, in).Bits(); b != nil {
		t.Fatalf("expected nil Bits(), got %v", b)
	}
}

// TestNotDocIdSet_BitsFromBitDocIdSet verifies Bits() is non-nil when
// the wrapped set is a BitDocIdSet (which has random access), then
// checks every bit complements the input.
func TestNotDocIdSet_BitsFromBitDocIdSet(t *testing.T) {
	const length = 8
	fbs, err := NewFixedBitSet(length)
	if err != nil {
		t.Fatalf("NewFixedBitSet: %v", err)
	}
	// set every odd bit in the wrapped set
	for i := 1; i < length; i += 2 {
		fbs.Set(i)
	}
	bds, err := NewBitDocIdSet(fbs, int64(fbs.Cardinality()))
	if err != nil {
		t.Fatalf("NewBitDocIdSet: %v", err)
	}
	// We need BitDocIdSet to expose Bits() returning the underlying
	// FixedBitSet for the optional interface to pick it up. The
	// existing BitDocIdSet returns *FixedBitSet from Bits(); adapt
	// via a small shim that returns a Bits-typed value.
	adapted := bitDocIdSetWithBits{BitDocIdSet: bds}

	ns := NewNotDocIdSet(length, adapted)
	bits := ns.Bits()
	if bits == nil {
		t.Fatalf("expected non-nil Bits() for BitDocIdSet input")
	}
	if bits.Length() != length {
		t.Fatalf("Bits.Length=%d want %d", bits.Length(), length)
	}
	for i := 0; i < length; i++ {
		want := !fbs.Get(i)
		if got := bits.Get(i); got != want {
			t.Fatalf("Bits.Get(%d)=%v want %v", i, got, want)
		}
	}
}

// TestNotDocIdSet_RamBytesUsed exercises the RAM accounting path.
func TestNotDocIdSet_RamBytesUsed(t *testing.T) {
	// Wrapping a set that does not advertise RAM gives the base.
	ns := NewNotDocIdSet(3, &docIdSetFromInts{docs: []int{1}})
	if got := ns.RamBytesUsed(); got != notDocIdSetBaseRAM {
		t.Fatalf("RamBytesUsed=%d want %d", got, notDocIdSetBaseRAM)
	}
	// Wrapping a set that does advertises RAM adds it.
	rich := &accountableSet{base: ns.in, ram: 64}
	got := NewNotDocIdSet(3, rich).RamBytesUsed()
	if got != notDocIdSetBaseRAM+64 {
		t.Fatalf("RamBytesUsed=%d want %d", got, notDocIdSetBaseRAM+64)
	}
}

// TestNotDocIdSet_Cost asserts Cost == maxDoc regardless of inner set.
func TestNotDocIdSet_Cost(t *testing.T) {
	in := &docIdSetFromInts{docs: []int{0, 1}}
	it := NewNotDocIdSet(42, in).Iterator()
	if c := it.Cost(); c != 42 {
		t.Fatalf("Cost=%d want 42", c)
	}
}

// bitDocIdSetWithBits adapts BitDocIdSet to the docIdSetWithBits
// optional interface used by NotDocIdSet.
type bitDocIdSetWithBits struct{ *BitDocIdSet }

// Bits returns the underlying FixedBitSet as a Bits.
func (b bitDocIdSetWithBits) Bits() Bits {
	fbs := b.BitDocIdSet.Bits()
	if fbs == nil {
		return nil
	}
	return fbs
}

// accountableSet wraps an arbitrary DocIdSet and overrides
// RamBytesUsed() so we can deterministically test propagation.
type accountableSet struct {
	base DocIdSet
	ram  int64
}

func (a *accountableSet) Iterator() DocIdSetIterator { return a.base.Iterator() }
func (a *accountableSet) RamBytesUsed() int64        { return a.ram }
