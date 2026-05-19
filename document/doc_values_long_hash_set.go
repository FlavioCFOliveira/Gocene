// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License. You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

package document

import (
	"fmt"
	"math"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// DocValuesLongHashSet is a set of int64 values optimised for doc-values
// membership tests. It is the Go port of Lucene 10.4.0's
// org.apache.lucene.document.DocValuesLongHashSet (a package-private
// utility in Java).
//
// # Why exported in Gocene
//
// The Java class is package-private because its only consumer lives in
// the same package (SortedNumericDocValuesSetQuery). Gocene keeps query
// types in the search package to avoid the search<->document import
// cycle that the Java original sidesteps via package-private access;
// the hash set therefore has to cross a package boundary and must be
// exported. The Lucene class location is preserved (document/) so the
// porting trail stays one-to-one with the Java tree.
//
// # Layout and contract
//
// The set is built once from a pre-sorted []int64 and is logically
// immutable thereafter. Internally it is an open-addressed hash table
// sized to 1.5x the input length, rounded up to the next power of two,
// using [math.MinInt64] as the "missing slot" sentinel. Because the
// sentinel doubles as a legitimate user value, an explicit
// hasMissingValue flag is tracked alongside the table.
//
// Callers should consult [DocValuesLongHashSet.Min] and
// [DocValuesLongHashSet.Max] to guide or short-circuit iteration before
// calling [DocValuesLongHashSet.Contains], matching the Java reference
// guidance.
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/document/DocValuesLongHashSet.java
type DocValuesLongHashSet struct {
	// table is the open-addressed hash table. Empty slots hold the
	// MISSING sentinel; every other slot holds a user value.
	table []int64
	// mask is tableSize - 1 (table size is always a power of two), so
	// slot = Long.hashCode(v) & mask.
	mask int
	// hasMissingValue records whether the sentinel itself was passed in
	// as a user value. Without this flag the sentinel collision would
	// shrink the effective set.
	hasMissingValue bool
	// size is the number of distinct values stored (including the
	// sentinel when hasMissingValue is true).
	size int
	// minValue is the smallest stored value, or [math.MaxInt64] for an
	// empty set. Mirrors the Java field of the same name.
	minValue int64
	// maxValue is the largest stored value, or [math.MinInt64] for an
	// empty set. Mirrors the Java field of the same name.
	maxValue int64
}

// docValuesLongHashSetMissing is the "empty slot" sentinel. Matches the
// Java constant DocValuesLongHashSet.MISSING == Long.MIN_VALUE.
const docValuesLongHashSetMissing = math.MinInt64

// docValuesLongHashSetBaseRAM approximates the JVM
// RamUsageEstimator.shallowSizeOfInstance(DocValuesLongHashSet.class)
// figure used by the Java reference. The literal is the observed
// unsafe.Sizeof(DocValuesLongHashSet{}) on amd64/arm64 (slice header +
// three int64 + int + bool, padded to 8-byte alignment).
const docValuesLongHashSetBaseRAM int64 = 64

// NewDocValuesLongHashSet builds a hash set from values, which must be
// passed in ascending sorted order (duplicates are tolerated and
// collapsed). The slice is not retained: callers may mutate or reuse it
// after the constructor returns.
//
// Mirrors the Java constructor DocValuesLongHashSet(long[]). The Java
// assert "values must be provided in sorted order" is intentionally not
// reproduced: it would only fire in test builds in Java, and Gocene's
// only producer (NewSortedNumericDocValuesSetQuery) sorts before
// calling, matching the upstream factory in
// SortedNumericDocValuesField.newSlowSetQuery.
func NewDocValuesLongHashSet(values []int64) *DocValuesLongHashSet {
	// Java: int tableSize = Math.toIntExact(values.length * 3L / 2);
	//       tableSize = 1 << PackedInts.bitsRequired(tableSize);
	// Use int64 in the multiplication to mirror the Java widening, then
	// round up to the next power of two via the packed helper (which
	// returns 1 for input 0, matching Java's unsignedBitsRequired(0)).
	tableSizeHint := int64(len(values)) * 3 / 2
	tableSize := 1 << packed.BitsRequired(tableSizeHint)

	s := &DocValuesLongHashSet{
		table: make([]int64, tableSize),
		mask:  tableSize - 1,
	}
	for i := range s.table {
		s.table[i] = docValuesLongHashSetMissing
	}

	for _, v := range values {
		if v == docValuesLongHashSetMissing {
			// The sentinel is stored out-of-band via hasMissingValue.
			// Increment size only on first occurrence to match the
			// Java "size += hasMissingValue ? 0 : 1" branch.
			if !s.hasMissingValue {
				s.size++
				s.hasMissingValue = true
			}
			continue
		}
		if s.add(v) {
			s.size++
		}
	}

	if len(values) == 0 {
		// Java sentinels for empty sets: Long.MAX_VALUE / Long.MIN_VALUE.
		// They make every range comparison in the scorer loop reject
		// the doc without requiring an extra "is empty" branch.
		s.minValue = math.MaxInt64
		s.maxValue = math.MinInt64
	} else {
		s.minValue = values[0]
		s.maxValue = values[len(values)-1]
	}
	return s
}

// add inserts v into the table. Returns true when v was not already
// present. v must not equal the MISSING sentinel; the caller routes
// the sentinel through hasMissingValue.
func (s *DocValuesLongHashSet) add(v int64) bool {
	// Java's Long.hashCode(v) is `(int) (value ^ (value >>> 32))`.
	// Reproduce the unsigned right shift on the int64 by going through
	// uint64 before the cast back to a signed 32-bit value. The mask is
	// always positive so the indexing remains in bounds.
	slot := int(longHashCode(v)) & s.mask
	for i := slot; ; i = (i + 1) & s.mask {
		if s.table[i] == docValuesLongHashSetMissing {
			s.table[i] = v
			return true
		}
		if s.table[i] == v {
			return false
		}
	}
}

// Contains reports whether v is in the set.
//
// Callers in tight loops should guard the call with the
// [DocValuesLongHashSet.Min] / [DocValuesLongHashSet.Max] bounds, as
// the Java reference recommends.
func (s *DocValuesLongHashSet) Contains(v int64) bool {
	if v == docValuesLongHashSetMissing {
		return s.hasMissingValue
	}
	slot := int(longHashCode(v)) & s.mask
	for i := slot; ; i = (i + 1) & s.mask {
		if s.table[i] == docValuesLongHashSetMissing {
			return false
		}
		if s.table[i] == v {
			return true
		}
	}
}

// Size returns the number of distinct values in the set, including the
// MISSING sentinel when it was explicitly inserted.
func (s *DocValuesLongHashSet) Size() int { return s.size }

// Min returns the smallest value in the set, or [math.MaxInt64] for an
// empty set (matching Java's Long.MAX_VALUE sentinel).
func (s *DocValuesLongHashSet) Min() int64 { return s.minValue }

// Max returns the largest value in the set, or [math.MinInt64] for an
// empty set (matching Java's Long.MIN_VALUE sentinel).
func (s *DocValuesLongHashSet) Max() int64 { return s.maxValue }

// Values returns every member of the set as a freshly allocated slice.
// When the MISSING sentinel was inserted it appears first, followed by
// the table contents in slot order. The relative order of the non-
// sentinel values mirrors the Java [DocValuesLongHashSet.stream] output
// (Arrays.stream over the table, filtering MISSING out).
//
// This is the Go counterpart of the Java stream() helper; it is used by
// tests and by callers that need a materialised view of the set.
func (s *DocValuesLongHashSet) Values() []int64 {
	out := make([]int64, 0, s.size)
	if s.hasMissingValue {
		out = append(out, docValuesLongHashSetMissing)
	}
	for _, v := range s.table {
		if v != docValuesLongHashSetMissing {
			out = append(out, v)
		}
	}
	return out
}

// Equals mirrors DocValuesLongHashSet.equals: same size, same
// min/max/mask, same hasMissingValue flag and same backing table
// (element-wise).
//
// The comparison is intentionally strict on the table contents and not
// only on the logical set membership. The Java reference does the same
// because two sets with identical members are guaranteed to land on
// identical table layouts: the sizing rule and the linear-probing order
// are both deterministic functions of the input length and values.
func (s *DocValuesLongHashSet) Equals(other *DocValuesLongHashSet) bool {
	if s == other {
		return true
	}
	if other == nil {
		return false
	}
	if s.size != other.size ||
		s.minValue != other.minValue ||
		s.maxValue != other.maxValue ||
		s.mask != other.mask ||
		s.hasMissingValue != other.hasMissingValue {
		return false
	}
	if len(s.table) != len(other.table) {
		return false
	}
	for i, v := range s.table {
		if v != other.table[i] {
			return false
		}
	}
	return true
}

// HashCode mirrors DocValuesLongHashSet.hashCode: an Objects.hash of
// size, minValue, maxValue, mask, hasMissingValue and the table's
// java.util.Arrays.hashCode.
//
// Java's Objects.hash boxes its arguments into an Object[] and delegates
// to Arrays.hashCode(Object[]); the per-element fold is
// 31*result + element.hashCode(). The boxed hashCode for primitives is:
//   - Integer.hashCode(int x)       == x
//   - Long.hashCode(long x)         == (int) (x ^ (x >>> 32))
//   - Boolean.hashCode(boolean b)   == b ? 1231 : 1237
//   - Arrays.hashCode(long[] table) == 31-folded Long.hashCode of each
//
// We reproduce the same arithmetic so the hash matches the Java value
// element-for-element.
func (s *DocValuesLongHashSet) HashCode() int {
	// Objects.hash starts at result = 1 then folds: result = 31*result + (e==null ? 0 : e.hashCode()).
	h := int32(1)
	h = 31*h + int32(s.size)
	h = 31*h + longHashCode(s.minValue)
	h = 31*h + longHashCode(s.maxValue)
	h = 31*h + int32(s.mask)
	if s.hasMissingValue {
		h = 31*h + 1231
	} else {
		h = 31*h + 1237
	}
	h = 31*h + arrayHashCodeInt64(s.table)
	return int(h)
}

// String mirrors DocValuesLongHashSet.toString: a comma-separated list
// of the values produced by [DocValuesLongHashSet.Values], wrapped in
// square brackets. The Java reference uses Collectors.joining(", ", "[",
// "]"); the Go port produces a byte-identical rendering for the same
// values (signed-decimal formatting matches Long.toString).
func (s *DocValuesLongHashSet) String() string {
	vs := s.Values()
	if len(vs) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, v := range vs {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("%d", v))
	}
	sb.WriteByte(']')
	return sb.String()
}

// RamBytesUsed reports the in-memory footprint of the set: the shallow
// header plus the backing table. Mirrors the Java
// ramBytesUsed = BASE_RAM_BYTES + RamUsageEstimator.sizeOfObject(table).
func (s *DocValuesLongHashSet) RamBytesUsed() int64 {
	return docValuesLongHashSetBaseRAM + util.SizeOfInt64Slice(s.table)
}

// longHashCode reproduces java.lang.Long.hashCode(long): the upper and
// lower 32-bit halves XOR-ed together, narrowed to int32. The unsigned
// right shift is done on uint64 so the upper-bit copy in Java's >>>
// operator is preserved.
func longHashCode(v int64) int32 {
	u := uint64(v)
	return int32(uint32(u ^ (u >> 32)))
}

// arrayHashCodeInt64 reproduces java.util.Arrays.hashCode(long[]):
// start at 1, fold each element via 31*result + Long.hashCode(e). Nil
// arrays hash to 0 (Java returns 0 for a null array; an empty array
// returns 1). Since the constructor always allocates the table, the
// "len == 0" branch only fires defensively.
func arrayHashCodeInt64(arr []int64) int32 {
	if arr == nil {
		return 0
	}
	h := int32(1)
	for _, v := range arr {
		h = 31*h + longHashCode(v)
	}
	return h
}

// Ensure DocValuesLongHashSet implements util.Accountable.
var _ util.Accountable = (*DocValuesLongHashSet)(nil)
