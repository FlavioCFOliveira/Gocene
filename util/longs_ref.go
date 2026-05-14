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

// LongsRef represents a long[] as a slice (offset + length) into an
// existing []int64. The Longs field must never be nil; callers needing
// an empty ref should use the package-level EmptyLongs slice or
// NewLongsRefEmpty. This is a direct port of
// org.apache.lucene.util.LongsRef.
type LongsRef struct {
	// Longs is the underlying int64 backing array.
	Longs []int64
	// Offset is the start of the valid region.
	Offset int
	// Length is the number of valid entries starting at Offset.
	Length int
}

// EmptyLongs is the shared zero-length int64 slice used by empty
// LongsRef instances. Mirrors LongsRef.EMPTY_LONGS in Lucene.
//
//nolint:gochecknoglobals // mirrors LongsRef.EMPTY_LONGS in Lucene.
var EmptyLongs = []int64{}

// NewLongsRefEmpty returns a new LongsRef pointing at EmptyLongs.
func NewLongsRefEmpty() *LongsRef { return &LongsRef{Longs: EmptyLongs} }

// NewLongsRefWithCapacity returns a new LongsRef whose Longs slice is a
// freshly-allocated array of the given capacity. Offset and Length are
// both zero.
func NewLongsRefWithCapacity(capacity int) *LongsRef {
	return &LongsRef{Longs: make([]int64, capacity)}
}

// NewLongsRefFromSlice constructs a LongsRef that directly references
// longs[offset:offset+length] without copying. The slice must outlive
// the returned LongsRef. Mirrors Lucene's three-arg constructor with
// the same `assert isValid()` guard; we panic to mirror the Java
// IllegalStateException for the same kinds of contract violations.
func NewLongsRefFromSlice(longs []int64, offset, length int) *LongsRef {
	r := &LongsRef{Longs: longs, Offset: offset, Length: length}
	if err := r.IsValid(); err != nil {
		panic(err)
	}
	return r
}

// Clone returns a shallow clone — the underlying Longs slice is shared
// with the receiver.
func (r *LongsRef) Clone() *LongsRef {
	if r == nil {
		return nil
	}
	return &LongsRef{Longs: r.Longs, Offset: r.Offset, Length: r.Length}
}

// HashCode returns the Java LongsRef.hashCode() value: a 31-prime rolling
// hash that folds each int64 by `(int)(v ^ (v >>> 32))`. Java's int
// arithmetic wraps modulo 2^32; we mimic that by computing in int32.
func (r *LongsRef) HashCode() int {
	if r == nil {
		return 0
	}
	const prime = 31
	var result int32
	end := r.Offset + r.Length
	for i := r.Offset; i < end; i++ {
		v := r.Longs[i]
		// (int) (v ^ (v >>> 32)) — unsigned shift in Java; in Go use uint64.
		folded := int32(int64(v) ^ int64(uint64(v)>>32))
		result = prime*result + folded
	}
	return int(result)
}

// LongsEquals reports whether other has the same valid slice contents.
func (r *LongsRef) LongsEquals(other *LongsRef) bool { return LongsRefEquals(r, other) }

// CompareTo returns the lexicographic signed-int64 comparison between
// the valid slices of r and other.
func (r *LongsRef) CompareTo(other *LongsRef) int { return LongsRefCompare(r, other) }

// HexString returns the Lucene-style hex representation of the valid
// region: `[h1 h2 ... hn]`, each h being the unsigned-64-bit hex of the
// value. Mirrors LongsRef.toString() (Long.toHexString).
func (r *LongsRef) HexString() string {
	if r == nil {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	end := r.Offset + r.Length
	for i := r.Offset; i < end; i++ {
		if i > r.Offset {
			sb.WriteByte(' ')
		}
		sb.WriteString(fmt.Sprintf("%x", uint64(r.Longs[i])))
	}
	sb.WriteByte(']')
	return sb.String()
}

// DeepCopyOfLongsRef returns a new LongsRef whose Longs slice is a copy
// of the valid region of other. Mirrors LongsRef.deepCopyOf in Lucene.
func DeepCopyOfLongsRef(other *LongsRef) *LongsRef {
	if other == nil {
		return nil
	}
	cp := make([]int64, other.Length)
	copy(cp, other.Longs[other.Offset:other.Offset+other.Length])
	return &LongsRef{Longs: cp, Offset: 0, Length: other.Length}
}

// IsValid runs the same consistency checks as Lucene's isValid(),
// returning an error instead of throwing IllegalStateException.
func (r *LongsRef) IsValid() error {
	if r.Longs == nil {
		// Allow nil only for the canonical empty zero-value LongsRef.
		if r.Offset == 0 && r.Length == 0 {
			return nil
		}
		return fmt.Errorf("longs is nil but offset=%d length=%d", r.Offset, r.Length)
	}
	if r.Length < 0 {
		return fmt.Errorf("length is negative: %d", r.Length)
	}
	if r.Length > len(r.Longs) {
		return fmt.Errorf("length is out of bounds: %d, longs.length=%d", r.Length, len(r.Longs))
	}
	if r.Offset < 0 {
		return fmt.Errorf("offset is negative: %d", r.Offset)
	}
	if r.Offset > len(r.Longs) {
		return fmt.Errorf("offset out of bounds: %d, longs.length=%d", r.Offset, len(r.Longs))
	}
	if r.Offset+r.Length < 0 {
		return fmt.Errorf("offset+length is negative: offset=%d length=%d", r.Offset, r.Length)
	}
	if r.Offset+r.Length > len(r.Longs) {
		return fmt.Errorf("offset+length out of bounds: offset=%d length=%d longs.length=%d",
			r.Offset, r.Length, len(r.Longs))
	}
	return nil
}

// LongsRefEquals returns true when both refs point at identical valid
// regions. Either side may be nil; two nil pointers compare equal.
func LongsRefEquals(a, b *LongsRef) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.Length != b.Length {
		return false
	}
	for i := 0; i < a.Length; i++ {
		if a.Longs[a.Offset+i] != b.Longs[b.Offset+i] {
			return false
		}
	}
	return true
}

// LongsRefCompare returns the lexicographic signed-int64 order between
// the valid regions of a and b. A nil ref sorts before a non-nil ref.
func LongsRefCompare(a, b *LongsRef) int {
	if a == b {
		return 0
	}
	if a == nil {
		return -1
	}
	if b == nil {
		return 1
	}
	minLen := a.Length
	if b.Length < minLen {
		minLen = b.Length
	}
	for i := 0; i < minLen; i++ {
		av := a.Longs[a.Offset+i]
		bv := b.Longs[b.Offset+i]
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	switch {
	case a.Length < b.Length:
		return -1
	case a.Length > b.Length:
		return 1
	}
	return 0
}
