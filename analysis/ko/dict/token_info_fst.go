// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

// TokenInfoFST wraps an FST that maps character sequences to integer word IDs,
// with root-arc caching for Hangul syllables (11,172 arcs, U+AC00..U+D7A3).
//
// This is the Go port of org.apache.lucene.analysis.ko.dict.TokenInfoFST from
// Apache Lucene 10.4.0.
//
// Deviation: the Lucene reference wraps org.apache.lucene.util.fst.FST<Long>
// with root-arc caching. This Go port provides a map-backed placeholder;
// full FST wrapping is deferred to the nori codec sprint.
type TokenInfoFST struct {
	// outputs maps UTF-16 character sequences to output ordinals.
	outputs map[string]int64
}

// NewTokenInfoFST creates an empty TokenInfoFST.
func NewTokenInfoFST() *TokenInfoFST {
	return &TokenInfoFST{outputs: make(map[string]int64)}
}

// Put stores an output value for a character sequence.
func (f *TokenInfoFST) Put(seq []rune, value int64) {
	f.outputs[string(seq)] = value
}

// Lookup returns the output value for seq, or -1 if not found.
func (f *TokenInfoFST) Lookup(seq []rune) int64 {
	if v, ok := f.outputs[string(seq)]; ok {
		return v
	}
	return -1
}
