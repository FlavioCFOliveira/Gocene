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
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package search

// IntersectScorers creates a conjunction over the provided Scorers.
// The returned DocIdSetIterator may leverage two-phase iteration; use
// [AsTwoPhaseIterator] to retrieve the TwoPhaseIterator if available.
//
// Panics when len(scorers) < 2, mirroring Java's IllegalArgumentException.
//
// Mirrors ConjunctionUtils.intersectScorers.
func IntersectScorers(scorers []Scorer) DocIdSetIterator {
	if len(scorers) < 2 {
		panic("search: cannot make a ConjunctionDISI of fewer than 2 iterators")
	}
	var allIters []DocIdSetIterator
	var twoPhaseIters []*TwoPhaseIterator
	for _, s := range scorers {
		addScorer(s, &allIters, &twoPhaseIters)
	}
	return createConjunction(allIters, twoPhaseIters)
}

// IntersectIterators creates a conjunction over the provided
// DocIdSetIterators. The returned iterator may leverage two-phase
// iteration; use [AsTwoPhaseIterator] to retrieve the TwoPhaseIterator
// if available.
//
// Panics when len(iterators) < 2, mirroring Java's
// IllegalArgumentException.
//
// Mirrors ConjunctionUtils.intersectIterators.
func IntersectIterators(iterators []DocIdSetIterator) DocIdSetIterator {
	if len(iterators) < 2 {
		panic("search: cannot make a ConjunctionDISI of fewer than 2 iterators")
	}
	var allIters []DocIdSetIterator
	var twoPhaseIters []*TwoPhaseIterator
	for _, it := range iterators {
		addIteratorToConjunction(it, &allIters, &twoPhaseIters)
	}
	return createConjunction(allIters, twoPhaseIters)
}

// CreateConjunction builds a conjunction from already-separated
// DISI and TwoPhaseIterator lists. Useful when the caller has already
// split scorers into approximations and confirmations.
//
// Mirrors ConjunctionUtils.createConjunction.
func CreateConjunction(allIterators []DocIdSetIterator, twoPhaseIterators []*TwoPhaseIterator) DocIdSetIterator {
	return createConjunction(allIterators, twoPhaseIterators)
}

// AddTwoPhaseIterator decomposes a TwoPhaseIterator
// and appends to the accumulator slices.
//
// Mirrors ConjunctionUtils.addTwoPhaseIterator.
func AddTwoPhaseIterator(
	twoPhaseIter *TwoPhaseIterator,
	allIterators *[]DocIdSetIterator,
	twoPhaseIterators *[]*TwoPhaseIterator,
) {
	addTwoPhaseIteratorToConjunction(twoPhaseIter, allIterators, twoPhaseIterators)
}

// AddIterator decomposes a DocIdSetIterator and
// appends to the accumulator slices.
//
// Mirrors ConjunctionUtils.addIterator.
func AddIterator(
	disi DocIdSetIterator,
	allIterators *[]DocIdSetIterator,
	twoPhaseIterators *[]*TwoPhaseIterator,
) {
	addIteratorToConjunction(disi, allIterators, twoPhaseIterators)
}
