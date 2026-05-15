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

package fst

import (
	"fmt"
	"unicode/utf16"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// Util.java port — conversion helpers between [BytesRef], [IntsRef]
// and Unicode strings.
//
// Lucene 10.4.0 reference:
//
//	lucene/core/src/java/org/apache/lucene/util/fst/Util.java

// ToIntsRef populates scratch with the unsigned byte values of input
// and returns scratch.Get(). Mirrors
// {@code Util.toIntsRef(BytesRef, IntsRefBuilder)}.
func ToIntsRef(input *util.BytesRef, scratch *util.IntsRefBuilder) *util.IntsRef {
	scratch.GrowNoCopy(input.Length)
	end := input.Offset + input.Length
	for i, j := input.Offset, 0; i < end; i, j = i+1, j+1 {
		scratch.SetIntAt(j, int(input.Bytes[i])&0xFF)
	}
	scratch.SetLength(input.Length)
	return scratch.Get()
}

// ToBytesRef populates scratch with the values of input truncated to
// byte and returns scratch.Get(). Each input value must fit in
// [Byte.MIN_VALUE, 255]; values outside that range cause a panic to
// mirror the Java {@code assert} in the reference.
//
// Mirrors {@code Util.toBytesRef(IntsRef, BytesRefBuilder)}.
func ToBytesRef(input *util.IntsRef, scratch *util.BytesRefBuilder) *util.BytesRef {
	scratch.GrowNoCopy(input.Length)
	end := input.Offset + input.Length
	for i, j := input.Offset, 0; i < end; i, j = i+1, j+1 {
		value := input.Ints[i]
		// Lucene allows -128..255 (signed byte plus unsigned byte range).
		if value < -128 || value > 255 {
			panic(fmt.Sprintf("fst.ToBytesRef: value %d does not fit in a byte", value))
		}
		scratch.SetByteAt(j, byte(value))
	}
	scratch.SetLength(input.Length)
	return scratch.Get()
}

// ToUTF16 stores each UTF-16 code unit of s as an int in scratch and
// returns scratch.Get(). Mirrors
// {@code Util.toUTF16(CharSequence, IntsRefBuilder)}.
//
// The Java reference iterates with {@code String.charAt(idx)} which
// yields raw UTF-16 code units (including surrogate halves). To keep
// byte-for-byte parity with that semantics, we convert s to a UTF-16
// slice via {@code unicode/utf16.Encode} first; for input that contains
// non-BMP code points the resulting IntsRef has one entry per UTF-16
// code unit, matching the Java behaviour exactly.
func ToUTF16(s string, scratch *util.IntsRefBuilder) *util.IntsRef {
	units := utf16.Encode([]rune(s))
	charLimit := len(units)
	scratch.SetLength(charLimit)
	scratch.GrowNoCopy(charLimit)
	for idx, u := range units {
		scratch.SetIntAt(idx, int(u))
	}
	return scratch.Get()
}

// ToUTF32 decodes the Unicode code points of s and places them in
// scratch, returning scratch.Get(). Mirrors
// {@code Util.toUTF32(CharSequence, IntsRefBuilder)}.
func ToUTF32(s string, scratch *util.IntsRefBuilder) *util.IntsRef {
	intIdx := 0
	for _, r := range s {
		scratch.Grow(intIdx + 1)
		scratch.SetIntAt(intIdx, int(r))
		intIdx++
	}
	scratch.SetLength(intIdx)
	return scratch.Get()
}

// ToUTF32Runes decodes the supplied runes (Unicode code points,
// already-decoded) into scratch and returns scratch.Get(). This is the
// Go counterpart to Lucene's {@code Util.toUTF32(char[], int, int, IntsRefBuilder)}
// overload — since Go runes already are code points there is no UTF-16
// surrogate-pair recomposition step.
func ToUTF32Runes(runes []rune, offset, length int, scratch *util.IntsRefBuilder) *util.IntsRef {
	end := offset + length
	scratch.GrowNoCopy(length)
	for i, j := offset, 0; i < end; i, j = i+1, j+1 {
		scratch.SetIntAt(j, int(runes[i]))
	}
	scratch.SetLength(length)
	return scratch.Get()
}
