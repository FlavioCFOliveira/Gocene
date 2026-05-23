// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hunspell

import (
	"fmt"
	"math/bits"
)

// NGramFragmentChecker is a FragmentChecker based on all character n-grams
// possible in a given language.  It uses a probabilistic bit-set to keep
// memory usage low.
//
// This is the Go port of
// org.apache.lucene.analysis.hunspell.NGramFragmentChecker from Apache Lucene 10.4.0.
type NGramFragmentChecker struct {
	n      int
	hashes []uint64 // bit-set
	size   int      // number of bits
}

// ngramBitSetSize computes the bit-set size for n.
func ngramBitSetSize(n int) int {
	return 1 << (7 + n*3) // empirical: matches Lucene's BitSet(1 << (7 + n*3))
}

// NGramFragmentCheckerFromWords builds an NGramFragmentChecker from the given
// word (or n-gram) strings.
func NGramFragmentCheckerFromWords(n int, words []string) (*NGramFragmentChecker, error) {
	if n < 2 || n > 4 {
		return nil, fmt.Errorf("hunspell: NGramFragmentChecker: n must be between 2 and 4, got %d", n)
	}
	size := nextPow2(len(words) * 4)
	if size < 64 {
		size = 64
	}
	bitWords := (size + 63) / 64
	hashes := make([]uint64, bitWords)

	consumer := func(word []rune, start, end int) {
		h := lowCollisionHash(word, start, end)
		if h < 0 {
			h = -h
		}
		idx := h % size
		hashes[idx>>6] |= 1 << (idx & 63)
	}

	for _, w := range words {
		runes := []rune(w)
		processNGrams(n, runes, consumer)
	}

	checker := &NGramFragmentChecker{n: n, hashes: hashes, size: size}
	if checker.cardinality() > size*2/3 {
		return nil, fmt.Errorf("hunspell: NGramFragmentChecker: too many hash collisions")
	}
	return checker, nil
}

func (c *NGramFragmentChecker) HasImpossibleFragmentAround(word []rune, start, end int) bool {
	if len(word) < c.n {
		return false
	}
	first := start - c.n + 1
	if first < 0 {
		first = 0
	}
	last := end - 1
	if last > len(word)-c.n {
		last = len(word) - c.n
	}
	for i := first; i <= last; i++ {
		h := lowCollisionHash(word, i, i+c.n)
		if h < 0 {
			h = -h
		}
		idx := h % c.size
		if c.hashes[idx>>6]>>(idx&63)&1 == 0 {
			return true
		}
	}
	return false
}

func (c *NGramFragmentChecker) cardinality() int {
	count := 0
	for _, w := range c.hashes {
		count += bits.OnesCount64(w)
	}
	return count
}

func lowCollisionHash(word []rune, offset, end int) int {
	result := 0
	for i := offset; i < end; i++ {
		result = 239*result + int(word[i])
	}
	return result
}

func processNGrams(n int, word []rune, consumer func([]rune, int, int)) {
	if len(word) >= n {
		for i := 0; i <= len(word)-n; i++ {
			consumer(word, i, i+n)
		}
	}
}

func nextPow2(n int) int {
	if n <= 0 {
		return 1
	}
	return 1 << (bits.Len(uint(n - 1)))
}
