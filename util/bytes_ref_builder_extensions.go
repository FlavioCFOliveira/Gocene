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

package util

// This file extends the pre-existing [BytesRefBuilder] in
// util/byte_block_pool.go with the methods required for full parity
// with org.apache.lucene.util.BytesRefBuilder. The base type already
// provides Grow, GrowNoCopy, Bytes, Get, CopyChars, and SetLength;
// this file adds Length, ByteAt, SetByteAt, AppendByte, AppendBytes,
// AppendBytesRef, AppendBuilder, Clear, CopyBytes, CopyBytesRef,
// CopyBuilder, CopyCharsRange, ToBytesRef, and a Lucene-format
// String() implementation.
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/util/BytesRefBuilder.java

// Length returns the number of valid bytes currently in the builder.
// Mirrors {@code BytesRefBuilder#length()}.
func (b *BytesRefBuilder) Length() int {
	return b.length
}

// ByteAt returns the byte at the given offset within the backing
// slice. offset must satisfy 0 <= offset < len(Bytes()); behaviour for
// out-of-range offsets is undefined, matching the Java reference.
func (b *BytesRefBuilder) ByteAt(offset int) byte {
	return b.bytes[offset]
}

// SetByteAt assigns v to the byte at offset. offset must satisfy
// 0 <= offset < len(Bytes()).
func (b *BytesRefBuilder) SetByteAt(offset int, v byte) {
	b.bytes[offset] = v
}

// AppendByte appends a single byte. Mirrors
// {@code BytesRefBuilder#append(byte)}.
func (b *BytesRefBuilder) AppendByte(v byte) {
	b.Grow(b.length + 1)
	if len(b.bytes) <= b.length {
		// Grow only enlarges capacity when the underlying array is too
		// small; ensure the visible slice covers the new write index.
		b.bytes = b.bytes[:b.length+1]
	}
	b.bytes[b.length] = v
	b.length++
}

// AppendBytes appends src[off:off+length] to the builder. Mirrors
// {@code BytesRefBuilder#append(byte[], int, int)}.
func (b *BytesRefBuilder) AppendBytes(src []byte, off, length int) {
	if length <= 0 {
		return
	}
	newLen := b.length + length
	b.Grow(newLen)
	if len(b.bytes) < newLen {
		b.bytes = b.bytes[:newLen]
	}
	copy(b.bytes[b.length:newLen], src[off:off+length])
	b.length = newLen
}

// AppendBytesRef appends the valid bytes of ref. Mirrors
// {@code BytesRefBuilder#append(BytesRef)}.
func (b *BytesRefBuilder) AppendBytesRef(ref *BytesRef) {
	if ref == nil || ref.Length == 0 {
		return
	}
	b.AppendBytes(ref.Bytes, ref.Offset, ref.Length)
}

// AppendBuilder appends the current valid bytes of other. Mirrors
// {@code BytesRefBuilder#append(BytesRefBuilder)}.
func (b *BytesRefBuilder) AppendBuilder(other *BytesRefBuilder) {
	if other == nil {
		return
	}
	b.AppendBytesRef(other.Get())
}

// Clear resets the builder to the empty state without releasing the
// underlying storage. Mirrors {@code BytesRefBuilder#clear()}.
func (b *BytesRefBuilder) Clear() {
	b.SetLength(0)
}

// CopyBytes replaces the builder's content with src[off:off+length].
// Mirrors {@code BytesRefBuilder#copyBytes(byte[], int, int)}.
func (b *BytesRefBuilder) CopyBytes(src []byte, off, length int) {
	b.length = length
	b.GrowNoCopy(length)
	copy(b.bytes, src[off:off+length])
}

// CopyBytesRef replaces the builder's content with the valid bytes of
// ref. Mirrors {@code BytesRefBuilder#copyBytes(BytesRef)}.
func (b *BytesRefBuilder) CopyBytesRef(ref *BytesRef) {
	if ref == nil {
		b.Clear()
		return
	}
	b.CopyBytes(ref.Bytes, ref.Offset, ref.Length)
}

// CopyBuilder replaces the builder's content with other's current
// valid bytes. Mirrors {@code BytesRefBuilder#copyBytes(BytesRefBuilder)}.
func (b *BytesRefBuilder) CopyBuilder(other *BytesRefBuilder) {
	if other == nil {
		b.Clear()
		return
	}
	b.CopyBytesRef(other.Get())
}

// CopyCharsRange replaces the builder's content with the bytes from
// text[off:off+length], in *byte* units. Mirrors
// {@code BytesRefBuilder#copyChars(CharSequence, int, int)} for
// well-formed UTF-8 input.
//
// Note: Lucene's char-array overloads count UTF-16 code units; Go
// strings are UTF-8 so off/length here are byte offsets. For ASCII
// input the two interpretations coincide.
func (b *BytesRefBuilder) CopyCharsRange(text string, off, length int) {
	src := []byte(text)
	b.GrowNoCopy(length)
	copy(b.bytes, src[off:off+length])
	b.length = length
}

// ToBytesRef returns a fresh [BytesRef] containing a copy of the
// builder's valid bytes. Mirrors {@code BytesRefBuilder#toBytesRef()}.
func (b *BytesRefBuilder) ToBytesRef() *BytesRef {
	out := make([]byte, b.length)
	copy(out, b.bytes[:b.length])
	return &BytesRef{Bytes: out, Offset: 0, Length: b.length}
}

// String returns the Lucene-format hex representation of the
// builder's current content. Mirrors {@code BytesRefBuilder#toString()},
// which delegates to the wrapped BytesRef's toString (hex).
func (b *BytesRefBuilder) String() string {
	return b.Get().ToHexString()
}
