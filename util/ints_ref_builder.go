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

import "unicode/utf8"

// IntsRefBuilder is a mutable builder for IntsRef instances. It accumulates
// int values in an internal buffer and exposes them as an IntsRef via Get.
// This is a port of org.apache.lucene.util.IntsRefBuilder.
type IntsRefBuilder struct {
	ref *IntsRef
}

// NewIntsRefBuilder returns a fresh, empty IntsRefBuilder. The backing
// IntsRef starts pointing at the shared empty slice; the builder will
// allocate on the first Append/Grow call.
func NewIntsRefBuilder() *IntsRefBuilder {
	return &IntsRefBuilder{ref: &IntsRef{Ints: EmptyInts}}
}

// Ints returns the underlying int slice backing this builder. Callers
// must not retain the slice across mutations because Grow may replace
// the storage.
func (b *IntsRefBuilder) Ints() []int { return b.ref.Ints }

// Length returns the number of valid ints currently in the builder.
func (b *IntsRefBuilder) Length() int { return b.ref.Length }

// SetLength sets the current logical length. The caller must guarantee
// that length <= len(Ints()); IntsRefBuilder does not grow on this call
// because Java's `setLength` does not grow either.
func (b *IntsRefBuilder) SetLength(length int) { b.ref.Length = length }

// Clear resets the logical length to zero. The underlying buffer is not
// freed, allowing subsequent appends to reuse the existing capacity.
func (b *IntsRefBuilder) Clear() { b.SetLength(0) }

// IntAt returns the int at the given absolute index. Mirrors Java's
// intAt(int offset). The offset is into the raw Ints() slice, NOT
// relative to the builder's logical view.
func (b *IntsRefBuilder) IntAt(offset int) int { return b.ref.Ints[offset] }

// SetIntAt writes v at the given absolute index. Mirrors Java's
// setIntAt(int offset, int b).
func (b *IntsRefBuilder) SetIntAt(offset, v int) { b.ref.Ints[offset] = v }

// Append appends an int to the builder, growing the buffer if needed.
func (b *IntsRefBuilder) Append(i int) {
	b.Grow(b.ref.Length + 1)
	b.ref.Ints[b.ref.Length] = i
	b.ref.Length++
}

// Grow ensures the backing slice can hold at least newLength ints,
// over-allocating exponentially via Oversize. Existing contents are
// preserved.
func (b *IntsRefBuilder) Grow(newLength int) {
	if len(b.ref.Ints) < newLength {
		newCap := Oversize(newLength, 4)
		next := make([]int, newCap)
		copy(next, b.ref.Ints)
		b.ref.Ints = next
	}
}

// GrowNoCopy ensures the backing slice can hold at least newLength
// ints, over-allocating via Oversize, but does NOT preserve the
// existing contents.
func (b *IntsRefBuilder) GrowNoCopy(newLength int) {
	if len(b.ref.Ints) < newLength {
		b.ref.Ints = make([]int, Oversize(newLength, 4))
	}
}

// CopyInts replaces the builder's contents with otherInts[otherOffset:otherOffset+otherLength].
// The builder's Offset always remains zero, matching Lucene semantics.
func (b *IntsRefBuilder) CopyInts(otherInts []int, otherOffset, otherLength int) {
	b.GrowNoCopy(otherLength)
	copy(b.ref.Ints[:otherLength], otherInts[otherOffset:otherOffset+otherLength])
	b.ref.Length = otherLength
}

// CopyIntsRef is the IntsRef-overload of CopyInts.
func (b *IntsRefBuilder) CopyIntsRef(other *IntsRef) {
	b.CopyInts(other.Ints, other.Offset, other.Length)
}

// CopyUTF8Bytes decodes the UTF-8 bytes of br into UTF-32 code points
// and stores them in the builder. The builder's Length is set to the
// number of decoded code points. Equivalent to Lucene's
// IntsRefBuilder.copyUTF8Bytes(BytesRef).
//
// Lucene calls UnicodeUtil.UTF8toUTF32 under the hood; here we lean on
// Go's `unicode/utf8` for the byte-to-rune decoding so the resulting
// code-point sequence matches the Java reference for any well-formed
// UTF-8 input.
func (b *IntsRefBuilder) CopyUTF8Bytes(br *BytesRef) {
	if br == nil || br.Length == 0 {
		b.ref.Length = 0
		return
	}
	src := br.Bytes[br.Offset : br.Offset+br.Length]
	// Upper bound: every byte becomes at most one code point.
	b.GrowNoCopy(br.Length)
	n := 0
	for i := 0; i < len(src); {
		r, size := utf8.DecodeRune(src[i:])
		b.ref.Ints[n] = int(r)
		n++
		i += size
	}
	b.ref.Length = n
}

// Get returns the underlying IntsRef. The returned pointer aliases the
// builder's internal state; subsequent mutations to the builder may
// invalidate the returned ref's contents.
func (b *IntsRefBuilder) Get() *IntsRef {
	if b.ref.Offset != 0 {
		panic("modifying the offset of the returned ref is illegal")
	}
	return b.ref
}

// ToIntsRef returns a freshly-allocated IntsRef that is a deep copy of
// the builder's current contents.
func (b *IntsRefBuilder) ToIntsRef() *IntsRef {
	return DeepCopyOfIntsRef(b.Get())
}
