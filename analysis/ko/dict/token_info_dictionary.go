// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

// TokenInfoDictionary is the binary dictionary for known-word morphological
// analysis. Words are encoded into an FST mapping to a list of wordIDs.
//
// This is the Go port of
// org.apache.lucene.analysis.ko.dict.TokenInfoDictionary from Apache Lucene
// 10.4.0.
//
// Deviation: the Java original loads pre-built binary resources from the JAR
// classpath. The Go port provides the type with constructors; resource loading
// is deferred to the nori codec sprint.
type TokenInfoDictionary struct {
	// fst maps character sequences to word ID ordinals.
	fst *TokenInfoFST
	// morphAtts provides morphological attributes for dictionary entries.
	morphAtts *TokenInfoMorphData
	// targetMap maps word-ID ordinals to lists of byte-offsets in buffer.
	targetMap [][]int
}

// NewTokenInfoDictionary creates a TokenInfoDictionary with the given
// components.
func NewTokenInfoDictionary(
	fst *TokenInfoFST,
	morphAtts *TokenInfoMorphData,
	targetMap [][]int,
) *TokenInfoDictionary {
	return &TokenInfoDictionary{fst: fst, morphAtts: morphAtts, targetMap: targetMap}
}

// GetFST returns the FST for this dictionary.
func (d *TokenInfoDictionary) GetFST() *TokenInfoFST { return d.fst }

// GetMorphAttributes returns the TokenInfoMorphData for this dictionary.
func (d *TokenInfoDictionary) GetMorphAttributes() *TokenInfoMorphData { return d.morphAtts }

// LookupWordIDs returns the list of byte offsets (wordIDs) for the given FST
// ordinal. Returns nil if ordinal is out of range.
func (d *TokenInfoDictionary) LookupWordIDs(ordinal int) []int {
	if ordinal < 0 || ordinal >= len(d.targetMap) {
		return nil
	}
	return d.targetMap[ordinal]
}

