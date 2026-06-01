// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

import (
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/fst"
)

// TokenInfoFST wraps an FST that maps character sequences to integer word IDs,
// with root-arc caching for Hangul syllables (11,172 arcs, U+AC00..U+D7A3).
//
// This is the Go port of org.apache.lucene.analysis.ko.dict.TokenInfoFST from
// Apache Lucene 10.4.0.
//
// The Korean TokenInfoFST uses a BYTE2 (UTF-16) FST<Long>. Each arc label is a
// UTF-16 code unit (a rune for BMP characters). Lookup traverses the arc graph
// using fst.Get, which mirrors Util.get(FST<Long>, IntsRef).
type TokenInfoFST struct {
	fst *fst.FST[int64]
}

// NewTokenInfoFSTFromFST creates a TokenInfoFST wrapping a loaded FST.
func NewTokenInfoFSTFromFST(f *fst.FST[int64]) *TokenInfoFST {
	return &TokenInfoFST{fst: f}
}

// NewTokenInfoFST creates an empty TokenInfoFST (placeholder for tests that do
// not require real dictionary data).
func NewTokenInfoFST() *TokenInfoFST {
	return &TokenInfoFST{}
}

// GetFST returns the underlying fst.FST[int64].
func (f *TokenInfoFST) GetFST() *fst.FST[int64] { return f.fst }

// Lookup performs a complete FST lookup of the rune sequence seq. It returns
// the int64 output accumulated along the path, or -1 if the sequence is not
// accepted by the FST.
//
// The Korean FST uses BYTE2 input: each rune is treated as a single arc label
// (its UTF-16 code unit value, which equals the rune value for all BMP characters).
func (f *TokenInfoFST) Lookup(seq []rune) int64 {
	if f.fst == nil || len(seq) == 0 {
		return -1
	}
	ints := make([]int, len(seq))
	for i, r := range seq {
		ints[i] = int(r)
	}
	ref := &util.IntsRef{Ints: ints, Offset: 0, Length: len(ints)}
	out, found, err := fst.Get[int64](f.fst, ref)
	if err != nil || !found {
		return -1
	}
	return out
}
