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

package util

import (
	"math"
	"math/rand"
)

// IntroSelectorInterface defines the operations a concrete IntroSelector
// implementation must provide. It mirrors the abstract methods of Lucene's
// IntroSelector. setPivot/comparePivot must be a pair: comparePivot
// returns the result of comparing the saved pivot against slot j as if
// compare(pivotSlot, j) had been called.
type IntroSelectorInterface interface {
	SelectorInterface
	// Compare returns negative/zero/positive when slot i is less than/equal
	// to/greater than slot j respectively.
	Compare(i, j int) int
	// SetPivot saves the value at slot i so it can later be used as a pivot.
	SetPivot(i int)
	// ComparePivot compares the saved pivot with slot j.
	ComparePivot(j int) int
}

// IntroSelector is an adaptive selection algorithm based on the
// introspective quickselect algorithm. The quickselect uses an
// interpolation variant of Tukey's ninther median-of-medians for pivot,
// and Bentley-McIlroy 3-way partitioning. When the maximum recursion
// depth is exceeded, the sub-range is shuffled (Durstenfeld) to avoid
// pathological quadratic behaviour.
//
// This is a port of Apache Lucene's org.apache.lucene.util.IntroSelector.
type IntroSelector struct {
	Selector
	impl   IntroSelectorInterface
	random *rand.Rand
}

// NewIntroSelector creates a new IntroSelector backed by the given impl.
func NewIntroSelector(impl IntroSelectorInterface) *IntroSelector {
	return &IntroSelector{impl: impl}
}

// Select reorders the range [from, to) so that slot k holds the value it
// would have if the range had been sorted.
func (s *IntroSelector) Select(from, to, k int) {
	s.CheckArgs(from, to, k)
	maxDepth := 2 * int(math.Log2(float64(to-from)))
	s.selectInternal(from, to, k, maxDepth)
}

// SelectWithMaxDepth exposes the depth-bounded selection method used by
// Lucene's package-private overload for testing.
func (s *IntroSelector) SelectWithMaxDepth(from, to, k, maxDepth int) {
	s.CheckArgs(from, to, k)
	s.selectInternal(from, to, k, maxDepth)
}

// selectInternal is the depth-bounded core of the algorithm. The loop
// shrinks the candidate window [from, to) until it contains 3 or fewer
// elements, at which point a specialised tiny sort finishes the job.
func (s *IntroSelector) selectInternal(from, to, k, maxDepth int) {
	for {
		size := to - from
		if size <= 3 {
			switch size {
			case 2:
				if s.compare(from, from+1) > 0 {
					s.impl.Swap(from, from+1)
				}
			case 3:
				s.sort3(from)
			}
			return
		}

		maxDepth--
		if maxDepth == -1 {
			// Max recursion depth exceeded: shuffle (only once) and continue.
			s.shuffle(from, to)
		}

		// Pivot selection based on medians.
		last := to - 1
		mid := (from + last) >> 1
		var pivot int
		if size <= SingleMedianThreshold {
			rng := size >> 2
			pivot = s.median(mid-rng, mid, mid+rng)
		} else {
			rng := size >> 3
			doubleRange := rng << 1
			medianFirst := s.median(from, from+rng, from+doubleRange)
			medianMiddle := s.median(mid-rng, mid, mid+rng)
			medianLast := s.median(last-doubleRange, last-rng, last)
			switch {
			case k-from < rng:
				pivot = s.minOf3(medianFirst, medianMiddle, medianLast)
			case to-k <= rng:
				pivot = s.maxOf3(medianFirst, medianMiddle, medianLast)
			default:
				pivot = s.median(medianFirst, medianMiddle, medianLast)
			}
		}

		// Bentley-McIlroy 3-way partitioning.
		s.impl.SetPivot(pivot)
		s.impl.Swap(from, pivot)
		i := from
		j := to
		p := from + 1
		q := last

		for {
			var leftCmp, rightCmp int
			for {
				i++
				leftCmp = s.impl.ComparePivot(i)
				if leftCmp <= 0 {
					break
				}
			}
			for {
				j--
				rightCmp = s.impl.ComparePivot(j)
				if rightCmp >= 0 {
					break
				}
			}
			if i >= j {
				if i == j && rightCmp == 0 {
					s.impl.Swap(i, p)
				}
				break
			}
			s.impl.Swap(i, j)
			if rightCmp == 0 {
				s.impl.Swap(i, p)
				p++
			}
			if leftCmp == 0 {
				s.impl.Swap(j, q)
				q--
			}
		}
		i = j + 1
		for l := from; l < p; {
			s.impl.Swap(l, j)
			l++
			j--
		}
		for l := last; l > q; {
			s.impl.Swap(l, i)
			l--
			i++
		}

		// Select the partition containing the k-th element.
		switch {
		case k <= j:
			to = j + 1
		case k >= i:
			from = i
		default:
			return
		}
	}
}

// compare wraps the dynamic-dispatch Compare for cleaner call sites.
func (s *IntroSelector) compare(i, j int) int { return s.impl.Compare(i, j) }

// minOf3 returns the index of the minimum value among the three slots.
func (s *IntroSelector) minOf3(i, j, k int) int {
	if s.compare(i, j) <= 0 {
		if s.compare(i, k) <= 0 {
			return i
		}
		return k
	}
	if s.compare(j, k) <= 0 {
		return j
	}
	return k
}

// maxOf3 returns the index of the maximum value among the three slots.
func (s *IntroSelector) maxOf3(i, j, k int) int {
	if s.compare(i, j) <= 0 {
		if s.compare(j, k) < 0 {
			return k
		}
		return j
	}
	if s.compare(i, k) < 0 {
		return k
	}
	return i
}

// median returns the index of the median value among the three slots.
func (s *IntroSelector) median(i, j, k int) int {
	if s.compare(i, j) < 0 {
		if s.compare(j, k) <= 0 {
			return j
		}
		if s.compare(i, k) < 0 {
			return k
		}
		return i
	}
	if s.compare(j, k) >= 0 {
		return j
	}
	if s.compare(i, k) < 0 {
		return i
	}
	return k
}

// sort3 sorts the three slots starting at from.
func (s *IntroSelector) sort3(from int) {
	mid := from + 1
	last := from + 2
	switch {
	case s.compare(from, mid) <= 0:
		if s.compare(mid, last) > 0 {
			s.impl.Swap(mid, last)
			if s.compare(from, mid) > 0 {
				s.impl.Swap(from, mid)
			}
		}
	case s.compare(mid, last) >= 0:
		s.impl.Swap(from, last)
	default:
		s.impl.Swap(from, mid)
		if s.compare(mid, last) > 0 {
			s.impl.Swap(mid, last)
		}
	}
}

// shuffle implements Durstenfeld's algorithm on the range [from, to).
// Uses a lazily-initialised math/rand.Rand seeded from time, mirroring
// the lazy SplittableRandom semantics of the Lucene reference.
func (s *IntroSelector) shuffle(from, to int) {
	if s.random == nil {
		s.random = rand.New(rand.NewSource(rand.Int63()))
	}
	for i := to - 1; i > from; i-- {
		// nextInt(from, i+1) in Java is uniform over [from, i+1), i.e. (i+1-from) values.
		s.impl.Swap(i, from+s.random.Intn(i+1-from))
	}
}

// SetRandomSeed allows tests to make the introspection shuffle deterministic.
func (s *IntroSelector) SetRandomSeed(seed int64) {
	s.random = rand.New(rand.NewSource(seed))
}
