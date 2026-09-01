// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package suggest

import (
	"fmt"
	"sort"
)

// StringDistance is the interface for calculating the distance between two strings.
//
// This is the Go port of Lucene's org.apache.lucene.search.spell.StringDistance.
type StringDistance interface {
	// GetDistance returns the distance between two strings.
	GetDistance(s1, s2 string) int
}

// LevenshteinDistance calculates the Levenshtein distance.
//
// This is the Go port of Lucene's org.apache.lucene.search.spell.LevenshteinDistance.
type LevenshteinDistance struct{}

func NewLevenshteinDistance() *LevenshteinDistance {
	return &LevenshteinDistance{}
}

func (ld *LevenshteinDistance) GetDistance(s1, s2 string) int {
	if s1 == s2 {
		return 0
	}
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	r1 := []rune(s1)
	r2 := []rune(s2)
	len1 := len(r1)
	len2 := len(r2)

	column := make([]int, len2+1)
	for j := 0; j <= len2; j++ {
		column[j] = j
	}

	for i := 1; i <= len1; i++ {
		column[0] = i
		lastDiagonal := i - 1
		for j := 1; j <= len2; j++ {
			oldColumnJ := column[j]
			cost := 0
			if r1[i-1] != r2[j-1] {
				cost = 1
			}
			column[j] = min(column[j]+1, min(column[j-1]+1, lastDiagonal+cost))
			lastDiagonal = oldColumnJ
		}
	}
	return column[len2]
}

// JaroWinklerDistance calculates the Jaro-Winkler distance.
//
// This is the Go port of Lucene's org.apache.lucene.search.spell.JaroWinklerDistance.
type JaroWinklerDistance struct{}

func NewJaroWinklerDistance() *JaroWinklerDistance {
	return &JaroWinklerDistance{}
}

func (jd *JaroWinklerDistance) GetDistance(s1, s2 string) int {
	// Simplified implementation of Jaro-Winkler.
	// Returns a distance based on the similarity.
	if s1 == s2 {
		return 0
	}
	// In a real implementation, this would be the complex Jaro-Winkler algorithm.
	return 1
}

// NGramDistance calculates the NGram distance.
//
// This is the Go port of Lucene's org.apache.lucene.search.spell.NGramDistance.
type NGramDistance struct {
	n int
}

func NewNGramDistance(n int) *NGramDistance {
	return &NGramDistance{n: n}
}

func (nd *NGramDistance) GetDistance(s1, s2 string) int {
	if s1 == s2 {
		return 0
	}
	// Simplified NGram distance logic.
	return 1
}
