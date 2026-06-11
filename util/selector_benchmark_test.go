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
	"math/rand"
	"testing"
)

// Port of Apache Lucene's SelectorBenchmark (test util, Lucene 10.4.0,
// org.apache.lucene.util.SelectorBenchmark).
//
// The original is a hand-rolled main() that measures wall-clock time of
// IntroSelector across BaseSortTestCase.Strategy presets (RANDOM,
// RANDOM_LOW_CARDINALITY, ASCENDING, DESCENDING, ...). Gocene has not yet
// ported BaseSortTestCase nor its Strategy enum, so a strict 1:1 port would
// require a sibling stub of unported test infrastructure.
//
// Per Sprint 55 option c, the port is split:
//
//   - TestSelectorBenchmarkPlaceholder records the dependency gap with t.Skip
//     so the missing port stays visible in `go test -v`.
//   - BenchmarkSelectorIntro_* exposes the same micro-benchmark shape (build
//     array, clone per loop, select with rolling k) as Go testing.B benches,
//     restricted to the RANDOM strategy that does not require Strategy.set.
//
// Both pieces re-use the package-private IntroSelector entry point already
// covered by intro_selector_test.go, so no production behaviour is exercised
// beyond what existing tests already validate.
const (
	benchSelectorArrayLength = 20000
	benchSelectorLoops       = 800
)

// TestSelectorBenchmarkPlaceholder is replaced by a light-weight
// IntroSelector unit test that validates the select logic over a
// small random array, exercising the same code path that the Java
// benchmark targets without requiring the BaseSortTestCase strategy
// enum.
func TestSelectorBenchmarkPlaceholder(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	arr := make([]int, 256)
	for i := range arr {
		arr[i] = rng.Int()
	}
	impl := &benchIntroSelectorImpl{arr: arr}
	selector := NewIntroSelector(impl)
	// Select the median element (k=128). After selection, the
	// array must be partitioned: all elements before k are <=
	// arr[k], and all elements after k are >= arr[k].
	selector.Select(0, len(arr), len(arr)/2)
	pivot := arr[len(arr)/2]
	for i := 0; i < len(arr)/2; i++ {
		if arr[i] > pivot {
			t.Fatalf("left side: arr[%d]=%d > pivot=%d", i, arr[i], pivot)
		}
	}
	for i := len(arr)/2 + 1; i < len(arr); i++ {
		if arr[i] < pivot {
			t.Fatalf("right side: arr[%d]=%d < pivot=%d", i, arr[i], pivot)
		}
	}
}

// benchIntroSelectorImpl is the testing.B counterpart of the anonymous
// IntroSelector defined in SelectorBenchmark.java, specialised to int
// entries so the benchmark does not depend on Entry.
type benchIntroSelectorImpl struct {
	arr   []int
	pivot int
}

func (s *benchIntroSelectorImpl) Swap(i, j int) { s.arr[i], s.arr[j] = s.arr[j], s.arr[i] }
func (s *benchIntroSelectorImpl) Select(from, to, k int) {
	// Required by SelectorInterface; the driving Select is on IntroSelector.
}
func (s *benchIntroSelectorImpl) Compare(i, j int) int {
	switch {
	case s.arr[i] < s.arr[j]:
		return -1
	case s.arr[i] > s.arr[j]:
		return 1
	}
	return 0
}
func (s *benchIntroSelectorImpl) SetPivot(i int) { s.pivot = s.arr[i] }
func (s *benchIntroSelectorImpl) ComparePivot(j int) int {
	switch {
	case s.pivot < s.arr[j]:
		return -1
	case s.pivot > s.arr[j]:
		return 1
	}
	return 0
}

// BenchmarkSelectorIntro_Random mirrors benchmarkSelector(...) in
// SelectorBenchmark.java for the RANDOM strategy. Each iteration restores
// the working buffer from a pristine source then runs Select with a
// rolling k, so the measurement reflects partitioning cost rather than
// allocation.
func BenchmarkSelectorIntro_Random(b *testing.B) {
	rng := rand.New(rand.NewSource(1))
	original := make([]int, benchSelectorArrayLength)
	for i := range original {
		original[i] = rng.Int()
	}
	clone := make([]int, len(original))
	impl := &benchIntroSelectorImpl{arr: clone}
	selector := NewIntroSelector(impl)
	k := rng.Intn(len(clone))
	kIncrement := rng.Intn(len(clone)/14)*2 + 1

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		for j := 0; j < benchSelectorLoops; j++ {
			copy(clone, original)
			selector.Select(0, len(clone), k)
			k += kIncrement
			if k >= len(clone) {
				k -= len(clone)
			}
		}
	}
}
