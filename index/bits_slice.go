// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// BitsSlice exposes a contiguous slice of an existing [util.Bits] as a new
// [util.Bits]. Mirrors org.apache.lucene.index.BitsSlice (Apache Lucene
// 10.4.0).
//
// The slice is defined by a [ReaderSlice]'s Start (inclusive) and Length
// (exclusive upper bound is Start+Length).
//
// Marked @lucene.internal in the reference source; Gocene keeps it exported
// at the package boundary because Go has no equivalent of Java's
// package-private visibility, and downstream packages occasionally need to
// wrap a parent Bits.
type BitsSlice struct {
	parent util.Bits
	start  int
	length int
}

// NewBitsSlice builds a BitsSlice over parent using slice's Start and
// Length. Panics when slice.Length is negative, mirroring the Java assert.
func NewBitsSlice(parent util.Bits, slice ReaderSlice) *BitsSlice {
	if slice.Length < 0 {
		panic(fmt.Sprintf("BitsSlice: length=%d", slice.Length))
	}
	return &BitsSlice{
		parent: parent,
		start:  slice.Start,
		length: slice.Length,
	}
}

// Get returns true when the bit at doc (relative to the slice) is set in
// the parent Bits. Panics when doc is out of [0, Length()), matching
// Java's Objects.checkIndex contract.
func (b *BitsSlice) Get(doc int) bool {
	if doc < 0 || doc >= b.length {
		panic(fmt.Sprintf("BitsSlice: index %d out of bounds for length %d", doc, b.length))
	}
	return b.parent.Get(doc + b.start)
}

// Length returns the number of bits exposed by this slice.
func (b *BitsSlice) Length() int {
	return b.length
}
