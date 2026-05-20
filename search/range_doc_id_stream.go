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

package search

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/RangeDocIdStream.java

import "fmt"

// RangeDocIdStream is a DocIdStream over a contiguous range of doc IDs
// [min, max). It yields every integer in that half-open interval.
//
// Mirrors org.apache.lucene.search.RangeDocIdStream (Lucene 10.4.0).
//
// In Java this class is package-private (final); the Go port exports it
// so that test code in the search_test package and callers in sibling
// packages can construct instances.
//
// Deviations from Java:
//   - Java uses MathUtil.unsignedMin(int,int) in intoArray to cap upTo
//     at start+len(array). Go uses plain integer min since values are
//     bounded by max ≤ math.MaxInt32 and int is 64-bit on this platform.
type RangeDocIdStream struct {
	upTo int
	max  int
}

// NewRangeDocIdStream creates a RangeDocIdStream over [min, max).
// Panics if min >= max, mirroring the Java constructor's IllegalArgumentException.
func NewRangeDocIdStream(min, max int) *RangeDocIdStream {
	if min >= max {
		panic(fmt.Sprintf("RangeDocIdStream: min = %d >= max = %d", min, max))
	}
	return &RangeDocIdStream{upTo: min, max: max}
}

// MayHaveRemaining reports whether any doc IDs remain unconsumed.
func (s *RangeDocIdStream) MayHaveRemaining() bool {
	return s.upTo < s.max
}

// ForEachUpTo iterates over doc IDs in [s.upTo, min(upTo,s.max)) and
// calls consumer for each. Advances s.upTo afterward.
//
// Mirrors RangeDocIdStream.forEach(int, CheckedIntConsumer).
func (s *RangeDocIdStream) ForEachUpTo(upTo int, consumer IntConsumer) error {
	if upTo <= s.upTo {
		return nil
	}
	if upTo > s.max {
		upTo = s.max
	}
	for doc := s.upTo; doc < upTo; doc++ {
		if err := consumer(doc); err != nil {
			return err
		}
	}
	s.upTo = upTo
	return nil
}

// CountUpTo counts doc IDs in [s.upTo, min(upTo,s.max)) and advances
// s.upTo. Returns 0 when upTo ≤ current position.
//
// Mirrors RangeDocIdStream.count(int).
func (s *RangeDocIdStream) CountUpTo(upTo int) (int, error) {
	if upTo <= s.upTo {
		return 0, nil
	}
	if upTo > s.max {
		upTo = s.max
	}
	count := upTo - s.upTo
	s.upTo = upTo
	return count, nil
}

// IntoArrayUpTo copies doc IDs from [s.upTo, min(upTo,s.max)) into
// array, capped by len(array). Returns the number of elements written.
//
// Mirrors RangeDocIdStream.intoArray(int, int[]).
func (s *RangeDocIdStream) IntoArrayUpTo(upTo int, array []int) int {
	start := s.upTo
	if upTo > s.max {
		upTo = s.max
	}
	// Cap by array capacity (mirrors MathUtil.unsignedMin(upTo, start+len)).
	if cap := start + len(array); upTo > cap {
		upTo = cap
	}
	if upTo <= start {
		return 0
	}
	for doc := start; doc < upTo; doc++ {
		array[doc-start] = doc
	}
	s.upTo = upTo
	return upTo - start
}

// Compile-time check: RangeDocIdStream satisfies DocIdStream.
var _ DocIdStream = (*RangeDocIdStream)(nil)
