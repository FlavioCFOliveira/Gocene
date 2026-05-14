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

import (
	"strconv"
	"strings"
)

// EmptyBytes is the Go equivalent of {@code BytesRef.EMPTY_BYTES}, a
// shared zero-length slice used as the canonical "no bytes" payload.
// Mutations are forbidden (the slice is treated as immutable).
var EmptyBytes = []byte{}

// NewBytesRefRange constructs a BytesRef that directly references
// bytes[offset:offset+length] without copying. Mirrors the Java
// constructor {@code BytesRef(byte[], int, int)}. The caller retains
// ownership of bytes and must not mutate it once the BytesRef is in
// use elsewhere.
func NewBytesRefRange(bytes []byte, offset, length int) *BytesRef {
	return &BytesRef{Bytes: bytes, Offset: offset, Length: length}
}

// WrapBytes constructs a BytesRef that directly references bytes
// without copying. Mirrors the Java constructor {@code BytesRef(byte[])}
// when no copy is desired. This is the zero-allocation counterpart of
// [NewBytesRef].
func WrapBytes(bytes []byte) *BytesRef {
	return NewBytesRefRange(bytes, 0, len(bytes))
}

// BytesRefFromString constructs a BytesRef holding the UTF-8 encoding
// of text. Mirrors the Java {@code BytesRef(CharSequence)} constructor.
// The encoding is performed through Go's native UTF-8 representation
// (strings are already UTF-8) so the bytes are byte-for-byte identical
// to the Java output for well-formed input.
func BytesRefFromString(text string) *BytesRef {
	return NewBytesRefRange([]byte(text), 0, len(text))
}

// ShallowClone returns a new BytesRef sharing the same underlying byte
// slice. Mirrors the Java {@code BytesRef#clone()} contract:
// "The underlying bytes are not copied and will be shared by both the
// returned object and this object."
//
// The existing [BytesRef.Clone] performs a deep copy and is kept for
// source compatibility; new code should prefer ShallowClone for
// Lucene-matching semantics and [DeepCopyOf] for the deep variant.
func (br *BytesRef) ShallowClone() *BytesRef {
	if br == nil {
		return nil
	}
	return &BytesRef{Bytes: br.Bytes, Offset: br.Offset, Length: br.Length}
}

// BytesRefDeepCopyOf returns a new BytesRef containing a freshly
// allocated copy of other's valid bytes, with offset 0 and length
// other.Length. Mirrors the static {@code BytesRef.deepCopyOf(BytesRef)}.
//
// The function is named with the BytesRef prefix to avoid collision
// with the pre-existing [DeepCopyOf] for CharsRef.
func BytesRefDeepCopyOf(other *BytesRef) *BytesRef {
	if other == nil {
		return nil
	}
	if other.Length == 0 {
		return &BytesRef{Bytes: EmptyBytes, Offset: 0, Length: 0}
	}
	out := make([]byte, other.Length)
	copy(out, other.ValidBytes())
	return &BytesRef{Bytes: out, Offset: 0, Length: other.Length}
}

// Utf8ToString interprets the valid bytes as UTF-8 and returns the
// resulting string. Mirrors {@code BytesRef#utf8ToString()}. For
// well-formed UTF-8 the output is identical to the Lucene reference;
// malformed bytes are replaced by U+FFFD (Go's stdlib default), where
// Lucene throws an AssertionError or RuntimeException.
func (br *BytesRef) Utf8ToString() string {
	if br == nil || br.Length == 0 {
		return ""
	}
	return string(br.Bytes[br.Offset : br.Offset+br.Length])
}

// ToHexString returns the bytes in hex encoding, formatted as
// "[6c 75 63 65 6e 65]", mirroring {@code BytesRef#toString()} in
// Lucene. The Go [BytesRef.String] method returns the UTF-8 string for
// Go-friendliness; use ToHexString when byte-for-byte parity with the
// Java toString output is required.
func (br *BytesRef) ToHexString() string {
	if br == nil {
		return "[]"
	}
	var sb strings.Builder
	sb.Grow(2 + 3*br.Length)
	sb.WriteByte('[')
	for i := br.Offset; i < br.Offset+br.Length; i++ {
		if i > br.Offset {
			sb.WriteByte(' ')
		}
		sb.WriteString(strconv.FormatInt(int64(br.Bytes[i]&0xff), 16))
	}
	sb.WriteByte(']')
	return sb.String()
}

// BytesEqualsRange is the Go port of {@code BytesRef#bytesEquals},
// comparing the valid bytes of two BytesRefs for equality. It is
// equivalent to [BytesRefEquals]; provided to mirror the Lucene method
// name explicitly. Implementations of BytesRefComparator (#937) may
// prefer this when forwarding to the package helper.
func (br *BytesRef) BytesEqualsRange(other *BytesRef) bool {
	return BytesRefEquals(br, other)
}
