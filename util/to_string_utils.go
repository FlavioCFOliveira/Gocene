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
	"strconv"
	"strings"
)

// ToStringByteArray appends a textual representation of bytes to b in the
// exact format used by Lucene's ToStringUtils.byteArray:
//
//	b[0]=<v0>,b[1]=<v1>,...,b[N-1]=<v_{N-1}>
//
// Each value is rendered as a signed decimal int8 (Java's byte) so that the
// output matches the JVM byte-by-byte. Empty input yields no output.
//
// This is a port of org.apache.lucene.util.ToStringUtils.byteArray.
func ToStringByteArray(b *strings.Builder, bytes []byte) {
	n := len(bytes)
	for i := 0; i < n; i++ {
		b.WriteString("b[")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("]=")
		// Java byte is signed: 0xFF prints as -1, not 255.
		b.WriteString(strconv.Itoa(int(int8(bytes[i]))))
		if i < n-1 {
			b.WriteByte(',')
		}
	}
}

// hexChars mirrors Lucene's HEX table used by longHex.
const hexChars = "0123456789abcdef"

// ToStringLongHex returns a 16-digit lower-case hex representation of x with
// a "0x" prefix and all leading zeros preserved. Unlike strconv.FormatUint
// with base 16, this never strips leading zeros, matching Lucene's
// ToStringUtils.longHex contract.
func ToStringLongHex(x int64) string {
	var buf [18]byte
	buf[0] = '0'
	buf[1] = 'x'
	u := uint64(x)
	for i := 17; i >= 2; i-- {
		buf[i] = hexChars[u&0x0F]
		u >>= 4
	}
	return string(buf[:])
}

// ToStringBytesRef returns a textual representation of a BytesRef that
// combines its UTF-8 decoding and its hex byte dump, separated by a space:
//
//	"hello [68 65 6c 6c 6f]"
//
// If the contents are not valid UTF-8 (or otherwise trigger the UTF-8
// decoder's recovery path) only the hex dump is returned, matching Lucene's
// fallback in ToStringUtils.bytesRefToString.
//
// A nil BytesRef yields the literal "null", matching Lucene. The hex dump is
// produced by BytesRef.ToHexString which is Gocene's equivalent of Java's
// BytesRef.toString.
func ToStringBytesRef(ref *BytesRef) string {
	if ref == nil {
		return "null"
	}
	hex := ref.ToHexString()
	bytes := ref.ValidBytes()
	if !isValidUTF8(bytes) {
		return hex
	}
	return string(bytes) + " " + hex
}

// ToStringBytesRefBuilder mirrors the Java overload that accepts a
// BytesRefBuilder. The builder's Get() view is forwarded to ToStringBytesRef.
func ToStringBytesRefBuilder(b *BytesRefBuilder) string {
	if b == nil {
		return "null"
	}
	return ToStringBytesRef(b.Get())
}

// ToStringBytes mirrors the Java overload that accepts a raw byte slice by
// wrapping it into a BytesRef view and delegating.
func ToStringBytes(b []byte) string {
	return ToStringBytesRef(NewBytesRef(b))
}

// isValidUTF8 is the local equivalent of Go's utf8.Valid. Re-implemented here
// to avoid pulling in unicode/utf8 for a single call and to keep the helper
// allocation-free on the hot path.
func isValidUTF8(p []byte) bool {
	n := len(p)
	for i := 0; i < n; {
		b := p[i]
		if b < 0x80 {
			i++
			continue
		}
		switch {
		case b < 0xC2:
			return false
		case b < 0xE0:
			if i+1 >= n || p[i+1]&0xC0 != 0x80 {
				return false
			}
			i += 2
		case b < 0xF0:
			if i+2 >= n || p[i+1]&0xC0 != 0x80 || p[i+2]&0xC0 != 0x80 {
				return false
			}
			// Reject UTF-16 surrogate code points and overlong forms.
			if b == 0xE0 && p[i+1] < 0xA0 {
				return false
			}
			if b == 0xED && p[i+1] >= 0xA0 {
				return false
			}
			i += 3
		case b < 0xF5:
			if i+3 >= n || p[i+1]&0xC0 != 0x80 || p[i+2]&0xC0 != 0x80 || p[i+3]&0xC0 != 0x80 {
				return false
			}
			if b == 0xF0 && p[i+1] < 0x90 {
				return false
			}
			if b == 0xF4 && p[i+1] >= 0x90 {
				return false
			}
			i += 4
		default:
			return false
		}
	}
	return true
}
