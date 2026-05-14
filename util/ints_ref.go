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
	"fmt"
	"strings"
)

// IntsRef extension methods that round out the existing minimal IntsRef
// declaration in bytes_ref.go to match the Lucene 10.4.0 reference
// semantics. The underlying slice type stays []int (machine word) to
// preserve compatibility with existing callers in util and downstream
// packages; this is a documented divergence from Java's int[] (always
// 32-bit). See bytes_ref.go for the canonical struct definition.

// EmptyInts is the shared zero-length backing slice used by callers that
// need a non-nil empty IntsRef, mirroring Lucene's IntsRef.EMPTY_INTS.
var EmptyInts = []int{}

// NewIntsRefWithCapacity creates a new IntsRef pointing to a freshly
// allocated slice of the given capacity. Offset and Length are zero.
// Mirrors Lucene's `new IntsRef(int capacity)` constructor.
func NewIntsRefWithCapacity(capacity int) *IntsRef {
	return &IntsRef{Ints: make([]int, capacity)}
}

// NewIntsRefFromSlice creates a new IntsRef directly referencing the
// provided slice with the given offset and length. The slice is not
// copied and must outlive the returned IntsRef. Mirrors Lucene's
// `new IntsRef(int[] ints, int offset, int length)` constructor.
func NewIntsRefFromSlice(ints []int, offset, length int) *IntsRef {
	r := &IntsRef{Ints: ints, Offset: offset, Length: length}
	if err := r.IsValid(); err != nil {
		panic(err)
	}
	return r
}

// Clone returns a shallow clone of this IntsRef. The underlying ints
// slice is shared with the receiver, matching Lucene's Object.clone()
// override on IntsRef.
func (ir *IntsRef) Clone() *IntsRef {
	if ir == nil {
		return nil
	}
	return &IntsRef{Ints: ir.Ints, Offset: ir.Offset, Length: ir.Length}
}

// HashCode returns the Lucene IntsRef.hashCode() value: a 31-prime
// rolling hash over the valid slice region. The result is widened to
// the Go int range but the underlying integer pattern matches Java's
// signed-32-bit semantics by relying on Go's wraparound on overflow.
func (ir *IntsRef) HashCode() int {
	if ir == nil {
		return 0
	}
	const prime = 31
	var result int32
	end := ir.Offset + ir.Length
	for i := ir.Offset; i < end; i++ {
		// Truncate to int32 to mirror Java int arithmetic.
		result = prime*result + int32(ir.Ints[i])
	}
	return int(result)
}

// IntsEquals returns true if other has the same valid slice contents.
// Equivalent to Lucene's intsEquals(IntsRef other).
func (ir *IntsRef) IntsEquals(other *IntsRef) bool {
	return IntsRefEquals(ir, other)
}

// CompareTo returns negative/zero/positive when this IntsRef is less
// than/equal to/greater than other. The order is lexicographic over
// signed int values.
func (ir *IntsRef) CompareTo(other *IntsRef) int {
	return IntsRefCompare(ir, other)
}

// HexString returns the Lucene-style hexadecimal representation of the
// valid region: `[h1 h2 ... hn]`, each h being the unsigned 32-bit hex
// of the int value. Mirrors Lucene IntsRef.toString().
func (ir *IntsRef) HexString() string {
	if ir == nil {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	end := ir.Offset + ir.Length
	for i := ir.Offset; i < end; i++ {
		if i > ir.Offset {
			sb.WriteByte(' ')
		}
		// Java prints Integer.toHexString which prints unsigned 32-bit value.
		sb.WriteString(fmt.Sprintf("%x", uint32(int32(ir.Ints[i]))))
	}
	sb.WriteByte(']')
	return sb.String()
}

// DeepCopyOf creates a new IntsRef that points to a fresh copy of the
// valid slice region of other. The resulting IntsRef has Offset 0 and
// Length other.Length. Mirrors Lucene's static IntsRef.deepCopyOf.
func DeepCopyOfIntsRef(other *IntsRef) *IntsRef {
	if other == nil {
		return nil
	}
	cp := make([]int, other.Length)
	copy(cp, other.Ints[other.Offset:other.Offset+other.Length])
	return &IntsRef{Ints: cp, Offset: 0, Length: other.Length}
}

// IsValid performs the same self-consistency checks as Lucene's
// IntsRef.isValid(), returning an error rather than throwing
// IllegalStateException.
func (ir *IntsRef) IsValid() error {
	if ir.Ints == nil {
		// Lucene treats a null ints array as invalid; we allow nil only
		// when both Offset and Length are zero, matching how NewIntsRefEmpty
		// produces a usable zero value. This is a deliberate, documented
		// relaxation of the Java contract for ergonomic Go usage.
		if ir.Offset == 0 && ir.Length == 0 {
			return nil
		}
		return fmt.Errorf("ints is nil but offset=%d length=%d", ir.Offset, ir.Length)
	}
	if ir.Length < 0 {
		return fmt.Errorf("length is negative: %d", ir.Length)
	}
	if ir.Length > len(ir.Ints) {
		return fmt.Errorf("length is out of bounds: %d, ints.length=%d", ir.Length, len(ir.Ints))
	}
	if ir.Offset < 0 {
		return fmt.Errorf("offset is negative: %d", ir.Offset)
	}
	if ir.Offset > len(ir.Ints) {
		return fmt.Errorf("offset out of bounds: %d, ints.length=%d", ir.Offset, len(ir.Ints))
	}
	if ir.Offset+ir.Length < 0 {
		return fmt.Errorf("offset+length is negative: offset=%d length=%d", ir.Offset, ir.Length)
	}
	if ir.Offset+ir.Length > len(ir.Ints) {
		return fmt.Errorf("offset+length out of bounds: offset=%d length=%d ints.length=%d",
			ir.Offset, ir.Length, len(ir.Ints))
	}
	return nil
}
