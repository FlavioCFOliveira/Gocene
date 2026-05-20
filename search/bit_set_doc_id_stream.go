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
//   lucene/core/src/java/org/apache/lucene/search/BitSetDocIdStream.java

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// BitSetDocIdStream is a DocIdStream backed by a util.FixedBitSet.
// It shifts doc IDs by a base offset so that the caller sees global doc
// IDs while the bitset stores segment-local bit positions.
//
// Mirrors org.apache.lucene.search.BitSetDocIdStream (Lucene 10.4.0).
//
// Deviations from Java:
//   - Java uses MathUtil.unsignedMin(Integer.MAX_VALUE, offset+bitSet.length())
//     to clamp max. Go mirrors this by clamping to math.MaxInt32.
//   - Java's FixedBitSet.forEach / cardinality(from,to) / intoArray are
//     not present in Gocene's FixedBitSet port; this implementation
//     performs equivalent iteration via FixedBitSet.NextSetBit.
//   - ForEachUpTo, CountUpTo and IntoArrayUpTo map to Java's
//     forEach(int, CheckedIntConsumer), count(int) and intoArray(int, int[]).
type BitSetDocIdStream struct {
	bitSet *util.FixedBitSet
	offset int
	upTo   int // global position of the next-to-consume boundary
	max    int // global upper bound (inclusive cap on upTo)
}

// NewBitSetDocIdStream creates a BitSetDocIdStream over bitSet with the
// given global base offset.
//
// Mirrors BitSetDocIdStream(FixedBitSet, int).
func NewBitSetDocIdStream(bitSet *util.FixedBitSet, offset int) *BitSetDocIdStream {
	rawMax := int64(offset) + int64(bitSet.Length())
	var max int
	if rawMax > math.MaxInt32 {
		max = math.MaxInt32
	} else {
		max = int(rawMax)
	}
	return &BitSetDocIdStream{
		bitSet: bitSet,
		offset: offset,
		upTo:   offset,
		max:    max,
	}
}

// MayHaveRemaining reports whether any doc IDs remain in the stream.
func (s *BitSetDocIdStream) MayHaveRemaining() bool {
	return s.upTo < s.max
}

// ForEachUpTo iterates over doc IDs in [s.upTo, upTo) calling consumer
// for each. Advances s.upTo to min(upTo, s.max).
//
// Mirrors BitSetDocIdStream.forEach(int, CheckedIntConsumer).
func (s *BitSetDocIdStream) ForEachUpTo(upTo int, consumer IntConsumer) error {
	if upTo <= s.upTo {
		return nil
	}
	if upTo > s.max {
		upTo = s.max
	}
	// Iterate local bits in [s.upTo-offset, upTo-offset).
	fromLocal := s.upTo - s.offset
	toLocal := upTo - s.offset
	bit := s.bitSet.NextSetBit(fromLocal)
	for bit >= 0 && bit < toLocal {
		if err := consumer(bit + s.offset); err != nil {
			return err
		}
		bit = s.bitSet.NextSetBit(bit + 1)
	}
	s.upTo = upTo
	return nil
}

// CountUpTo counts doc IDs in [s.upTo, upTo) and advances s.upTo.
//
// Mirrors BitSetDocIdStream.count(int).
func (s *BitSetDocIdStream) CountUpTo(upTo int) (int, error) {
	if upTo <= s.upTo {
		return 0, nil
	}
	if upTo > s.max {
		upTo = s.max
	}
	fromLocal := s.upTo - s.offset
	toLocal := upTo - s.offset
	count := 0
	bit := s.bitSet.NextSetBit(fromLocal)
	for bit >= 0 && bit < toLocal {
		count++
		bit = s.bitSet.NextSetBit(bit + 1)
	}
	s.upTo = upTo
	return count, nil
}

// IntoArrayUpTo copies doc IDs in [s.upTo, upTo) into array, returning
// the number of elements written. If the array fills before upTo is
// reached, s.upTo is advanced to the position after the last copied doc.
//
// Mirrors BitSetDocIdStream.intoArray(int, int[]).
func (s *BitSetDocIdStream) IntoArrayUpTo(upTo int, array []int) int {
	if upTo <= s.upTo || len(array) == 0 {
		return 0
	}
	if upTo > s.max {
		upTo = s.max
	}
	fromLocal := s.upTo - s.offset
	toLocal := upTo - s.offset
	n := 0
	bit := s.bitSet.NextSetBit(fromLocal)
	for bit >= 0 && bit < toLocal && n < len(array) {
		array[n] = bit + s.offset
		n++
		bit = s.bitSet.NextSetBit(bit + 1)
	}
	if n == len(array) && n > 0 {
		// Array may be full before upTo — advance to just after last copied doc.
		s.upTo = array[n-1] + 1
	} else {
		s.upTo = upTo
	}
	return n
}

// Compile-time check: BitSetDocIdStream satisfies DocIdStream.
var _ DocIdStream = (*BitSetDocIdStream)(nil)
