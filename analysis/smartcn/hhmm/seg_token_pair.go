// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hhmm

import (
	"math"
	"slices"
)

// SegTokenPair is a pair of tokens in a SegGraph, used for bigram scoring.
//
// Go port of org.apache.lucene.analysis.cn.smart.hhmm.SegTokenPair.
type SegTokenPair struct {
	// CharArray is the concatenated character representation of the pair.
	CharArray []rune

	// From is the index of the first token in SegGraph.
	From int

	// To is the index of the second token in SegGraph.
	To int

	// Weight is the smoothed bigram cost (−log probability).
	Weight float64
}

// NewSegTokenPair creates a new SegTokenPair.
func NewSegTokenPair(charArray []rune, from, to int, weight float64) *SegTokenPair {
	return &SegTokenPair{
		CharArray: charArray,
		From:      from,
		To:        to,
		Weight:    weight,
	}
}

// Equal reports whether p equals other by value (mirrors Java equals).
func (p *SegTokenPair) Equal(other *SegTokenPair) bool {
	if p == other {
		return true
	}
	if other == nil {
		return false
	}
	return slices.Equal(p.CharArray, other.CharArray) &&
		p.From == other.From &&
		p.To == other.To &&
		math.Float64bits(p.Weight) == math.Float64bits(other.Weight)
}
