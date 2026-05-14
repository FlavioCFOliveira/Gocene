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
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import (
	"bytes"
	"math"
)

// BytesRefComparator is a specialized BytesRef comparator that exposes a
// byte-level access hook so that radix-style string sorters can compare
// references one unsigned byte at a time.
//
// This is the Go port of org.apache.lucene.util.BytesRefComparator
// from Apache Lucene 10.4.0. The Java original is an abstract class that
// implements Comparator<BytesRef>; in Go the equivalent shape is an
// interface plus a concrete base struct.
//
// Implementations must provide ByteAt and may optionally override CompareK
// (the "skip first k known-equal bytes" variant). The default CompareK
// loops through ByteAt until a difference is found or both refs are
// exhausted.
type BytesRefComparator interface {
	// Compare returns a negative, zero, or positive value when o1 is
	// less than, equal to, or greater than o2 under this comparator's
	// ordering. Mirrors Java's Comparator<BytesRef>.compare.
	Compare(o1, o2 *BytesRef) int

	// CompareK compares two BytesRefs assuming the first k bytes are
	// already known to be equal. Mirrors Java's compare(BytesRef, BytesRef, int).
	CompareK(o1, o2 *BytesRef, k int) int

	// ByteAt returns the unsigned byte to use for comparison at index i,
	// or -1 if all bytes useful for comparison have been exhausted. May
	// only be called with i in [0, ComparedBytesCount()).
	ByteAt(ref *BytesRef, i int) int

	// ComparedBytesCount is the maximum number of bytes this comparator
	// will examine. The natural comparator returns math.MaxInt32.
	ComparedBytesCount() int
}

// BytesRefComparatorBase is the shared base for BytesRefComparator
// implementations. It mirrors Java's abstract class state: a single
// comparedBytesCount field and a default CompareK loop expressed in
// terms of ByteAt.
//
// Embed this struct in concrete comparators that need the default
// CompareK behavior. Concrete comparators MUST override ByteAt by
// implementing it on the embedding type (Go has no virtual dispatch),
// and MAY override CompareK for a faster path. NewBytesRefComparatorBase
// is the canonical constructor.
type BytesRefComparatorBase struct {
	comparedBytesCount int
}

// NewBytesRefComparatorBase constructs the embeddable base with the
// given maximum number of bytes to compare. Matches the protected Java
// constructor.
func NewBytesRefComparatorBase(comparedBytesCount int) BytesRefComparatorBase {
	return BytesRefComparatorBase{comparedBytesCount: comparedBytesCount}
}

// ComparedBytesCount returns the maximum number of bytes this
// comparator examines.
func (b BytesRefComparatorBase) ComparedBytesCount() int {
	return b.comparedBytesCount
}

// CompareKWith runs the default Lucene "skip first k bytes" loop using
// the caller-supplied byteAt hook. It exists so concrete comparators
// embedding BytesRefComparatorBase can reuse the loop while plugging in
// their own ByteAt method.
//
// The byteAt argument must implement the contract described on
// BytesRefComparator.ByteAt.
func (b BytesRefComparatorBase) CompareKWith(o1, o2 *BytesRef, k int, byteAt func(*BytesRef, int) int) int {
	for i := k; i < b.comparedBytesCount; i++ {
		b1 := byteAt(o1, i)
		b2 := byteAt(o2, i)
		if b1 != b2 {
			return b1 - b2
		}
		if b1 == -1 {
			break
		}
	}
	return 0
}

// naturalBytesRefComparator is the unsigned-lexicographic comparator
// exposed as NaturalBytesRefComparator. It mirrors the static NATURAL
// instance in Java, including the short-circuit CompareK that defers
// to bytes.Compare for unsigned ordering on the trailing range.
type naturalBytesRefComparator struct {
	BytesRefComparatorBase
}

// NaturalBytesRefComparator is the canonical unsigned, lexicographic
// BytesRef comparator. Equivalent to BytesRefComparator.NATURAL in
// Java.
//
//nolint:gochecknoglobals // mirrors the singleton in Lucene.
var NaturalBytesRefComparator BytesRefComparator = naturalBytesRefComparator{
	BytesRefComparatorBase: NewBytesRefComparatorBase(math.MaxInt32),
}

// ByteAt returns ref.Bytes[ref.Offset+i] as an unsigned byte, or -1 if
// i is past the valid range.
func (naturalBytesRefComparator) ByteAt(ref *BytesRef, i int) int {
	if ref == nil || ref.Length <= i {
		return -1
	}
	return int(ref.Bytes[ref.Offset+i]) & 0xFF
}

// Compare delegates to CompareK with k == 0, mirroring the final method
// in Java.
func (c naturalBytesRefComparator) Compare(o1, o2 *BytesRef) int {
	return c.CompareK(o1, o2, 0)
}

// CompareK compares the suffixes of o1 and o2 starting at offset k
// using unsigned byte ordering, equivalent to Java's
// Arrays.compareUnsigned over the trailing range.
func (naturalBytesRefComparator) CompareK(o1, o2 *BytesRef, k int) int {
	a := o1.Bytes[o1.Offset+k : o1.Offset+o1.Length]
	b := o2.Bytes[o2.Offset+k : o2.Offset+o2.Length]
	return bytes.Compare(a, b)
}
