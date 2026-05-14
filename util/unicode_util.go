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
	"math"
	"strings"
)

// Unicode surrogate and replacement constants used by Lucene's UnicodeUtil.
// Values are part of the public API and must remain byte-stable.
const (
	UniSurHighStart     = 0xD800
	UniSurHighEnd       = 0xDBFF
	UniSurLowStart      = 0xDC00
	UniSurLowEnd        = 0xDFFF
	UniReplacementChar  = 0xFFFD
	uniMaxBMP           = 0x0000FFFF
	halfShift           = 10
	halfMask            = 0x3FF
	minSupplementary    = 0x10000
	surrogateOffset     = minSupplementary - (UniSurHighStart << halfShift) - UniSurLowStart
	leadSurrogateShift  = 10
	trailSurrogateMask  = 0x3FF
	trailSurrogateMin   = 0xDC00
	leadSurrogateMin    = 0xD800
	leadSurrogateOffset = leadSurrogateMin - (minSupplementary >> leadSurrogateShift)
)

// MaxUTF8BytesPerChar is the maximum number of UTF-8 bytes per UTF-16 char.
// Mirrors UnicodeUtil.MAX_UTF8_BYTES_PER_CHAR.
const MaxUTF8BytesPerChar = 3

// BigTerm is a binary term made of 0xFF bytes intentionally larger than any
// legal UTF-8 term. Mirrors UnicodeUtil.BIG_TERM exactly.
var BigTerm = NewBytesRef([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF})

// MaxUTF8Length returns the maximum number of UTF-8 bytes required to encode
// a UTF-16 sequence of utf16Length code units. Panics on multiplication
// overflow, matching Lucene's Math.multiplyExact.
func MaxUTF8Length(utf16Length int) int {
	if utf16Length < 0 {
		panic(fmt.Sprintf("MaxUTF8Length: negative length %d", utf16Length))
	}
	hi, lo := mulOverflowCheck(utf16Length, MaxUTF8BytesPerChar)
	if hi {
		panic("MaxUTF8Length: overflow")
	}
	return lo
}

// UTF16ToUTF8Chars encodes the UTF-16 code units in source[offset:offset+length]
// into out as UTF-8 bytes. Returns the number of bytes written. Lone or
// out-of-order surrogates are replaced with the Unicode replacement character
// (U+FFFD), exactly as in Lucene's UnicodeUtil.UTF16toUTF8(char[], int, int,
// byte[]). The caller must size out to MaxUTF8Length(length) at minimum.
func UTF16ToUTF8Chars(source []uint16, offset, length int, out []byte) int {
	upto := 0
	i := offset
	end := offset + length
	for i < end {
		code := int(source[i])
		i++
		switch {
		case code < 0x80:
			out[upto] = byte(code)
			upto++
		case code < 0x800:
			out[upto] = byte(0xC0 | (code >> 6))
			out[upto+1] = byte(0x80 | (code & 0x3F))
			upto += 2
		case code < 0xD800 || code > 0xDFFF:
			out[upto] = byte(0xE0 | (code >> 12))
			out[upto+1] = byte(0x80 | ((code >> 6) & 0x3F))
			out[upto+2] = byte(0x80 | (code & 0x3F))
			upto += 3
		default:
			// Surrogate pair handling: only emit a 4-byte sequence if the high
			// surrogate is followed by a valid low surrogate. Otherwise emit
			// the three-byte replacement character EF BF BD.
			if code < 0xDC00 && i < end {
				utf32 := int(source[i])
				if utf32 >= 0xDC00 && utf32 <= 0xDFFF {
					utf32 = (code << 10) + utf32 + surrogateOffset
					i++
					out[upto] = byte(0xF0 | (utf32 >> 18))
					out[upto+1] = byte(0x80 | ((utf32 >> 12) & 0x3F))
					out[upto+2] = byte(0x80 | ((utf32 >> 6) & 0x3F))
					out[upto+3] = byte(0x80 | (utf32 & 0x3F))
					upto += 4
					continue
				}
			}
			out[upto] = 0xEF
			out[upto+1] = 0xBF
			out[upto+2] = 0xBD
			upto += 3
		}
	}
	return upto
}

// UTF16ToUTF8 encodes a Go string (treated as UTF-8 to be re-decoded as
// UTF-16 code units) into UTF-8 bytes, starting at outOffset and using the
// substring [offset, offset+length) of the string measured in UTF-16 code
// units. Returns the final output offset (outOffset + bytes written).
//
// This is the Go analogue of Lucene's UTF16toUTF8(CharSequence, int, int,
// byte[], int). For pure-ASCII input this is a 1:1 copy; for higher
// code points the function transcodes to UTF-16 first (so the surrogate
// handling above is reused) and emits UTF-8 with Lucene's replacement
// rule for lone surrogates.
func UTF16ToUTF8(s string, offset, length int, out []byte, outOffset int) int {
	units := stringToUTF16(s)
	return outOffset + UTF16ToUTF8Chars(units, offset, length, out[outOffset:])
}

// CalcUTF16toUTF8Length computes the number of UTF-8 bytes needed to encode
// s[offset:offset+length] without performing the encoding. Mirrors Lucene's
// UnicodeUtil.calcUTF16toUTF8Length.
func CalcUTF16toUTF8Length(s string, offset, length int) int {
	units := stringToUTF16(s)
	end := offset + length
	res := 0
	for i := offset; i < end; i++ {
		code := int(units[i])
		switch {
		case code < 0x80:
			res++
		case code < 0x800:
			res += 2
		case code < 0xD800 || code > 0xDFFF:
			res += 3
		default:
			if code < 0xDC00 && i < end-1 {
				next := int(units[i+1])
				if next >= 0xDC00 && next <= 0xDFFF {
					i++
					res += 4
					continue
				}
			}
			res += 3
		}
	}
	return res
}

// ValidUTF16String reports whether the string contains a well-formed sequence
// of UTF-16 code units (no unmatched surrogates). Mirrors
// UnicodeUtil.validUTF16String.
func ValidUTF16String(s string) bool {
	units := stringToUTF16(s)
	return ValidUTF16Chars(units, len(units))
}

// ValidUTF16Chars is the char[] overload of ValidUTF16String. The size
// parameter mirrors Lucene's API: it is the number of leading code units to
// validate, not the slice length.
func ValidUTF16Chars(s []uint16, size int) bool {
	for i := 0; i < size; i++ {
		ch := s[i]
		if ch >= UniSurHighStart && ch <= UniSurHighEnd {
			if i < size-1 {
				i++
				next := s[i]
				if !(next >= UniSurLowStart && next <= UniSurLowEnd) {
					return false
				}
			} else {
				return false
			}
		} else if ch >= UniSurLowStart && ch <= UniSurLowEnd {
			return false
		}
	}
	return true
}

// utf8CodeLength maps a UTF-8 lead byte to its sequence length, mirroring
// the static table in UnicodeUtil. Indices 0x80..0xBF, 0xF8..0xFF and 0xC0..
// 0xC1 are marked invalid using math.MinInt32 (the same sentinel Lucene
// uses; we keep the bit pattern for parity).
var utf8CodeLength = func() [256]int {
	var t [256]int
	for i := 0; i < 0x80; i++ {
		t[i] = 1
	}
	const inv = math.MinInt32
	for i := 0x80; i < 0xC0; i++ {
		t[i] = inv
	}
	for i := 0xC0; i < 0xE0; i++ {
		t[i] = 2
	}
	for i := 0xE0; i < 0xF0; i++ {
		t[i] = 3
	}
	for i := 0xF0; i < 0xF8; i++ {
		t[i] = 4
	}
	for i := 0xF8; i < 0x100; i++ {
		t[i] = inv
	}
	return t
}()

// CodePointCount returns the number of code points in a UTF-8 BytesRef.
// Panics with an IllegalArgumentException-equivalent on invalid header bytes
// or premature truncation. Matches Lucene's UnicodeUtil.codePointCount; only
// the first byte of each sequence is inspected (the trailing bytes are
// assumed valid).
func CodePointCount(utf8 *BytesRef) int {
	pos := utf8.Offset
	limit := pos + utf8.Length
	bytes := utf8.Bytes
	count := 0
	for pos < limit {
		v := int(bytes[pos]) & 0xFF
		switch {
		case v < 0x80:
			pos++
		case v >= 0xC0 && v < 0xE0:
			pos += 2
		case v >= 0xE0 && v < 0xF0:
			pos += 3
		case v >= 0xF0 && v < 0xF8:
			pos += 4
		default:
			panic(fmt.Sprintf("invalid UTF-8 lead byte: 0x%X at offset %d", v, pos))
		}
		count++
	}
	if pos > limit {
		panic(fmt.Sprintf("UTF-8 ran past limit by %d", pos-limit))
	}
	return count
}

// UTF8CodePoint holds the result of CodePointAt: the decoded code point and
// the number of bytes consumed. Mirrors UnicodeUtil.UTF8CodePoint.
type UTF8CodePoint struct {
	CodePoint int
	NumBytes  int
}

// CodePointAt decodes the UTF-8 code point starting at utf8[pos]. The reuse
// parameter, when non-nil, is filled in and returned. A nil reuse triggers a
// fresh allocation, exactly matching Lucene's contract.
func CodePointAt(utf8 []byte, pos int, reuse *UTF8CodePoint) *UTF8CodePoint {
	if reuse == nil {
		reuse = &UTF8CodePoint{}
	}
	leadByte := int(utf8[pos]) & 0xFF
	numBytes := utf8CodeLength[leadByte]
	reuse.NumBytes = numBytes
	var v int
	switch numBytes {
	case 1:
		reuse.CodePoint = leadByte
		return reuse
	case 2:
		v = leadByte & 31
	case 3:
		v = leadByte & 15
	case 4:
		v = leadByte & 7
	default:
		panic(fmt.Sprintf("Invalid UTF8 header byte: 0x%X", leadByte))
	}
	limit := pos + numBytes
	pos++
	for pos < limit {
		v = v<<6 | int(utf8[pos])&63
		pos++
	}
	reuse.CodePoint = v
	return reuse
}

// UTF8ToUTF32 decodes a UTF-8 BytesRef into the provided UTF-32 buffer.
// Returns the number of code points written. The caller must ensure ints is
// large enough; Lucene grows it externally before invoking this method.
func UTF8ToUTF32(utf8 *BytesRef, ints []int) int {
	count := 0
	upto := utf8.Offset
	limit := utf8.Offset + utf8.Length
	bytes := utf8.Bytes
	var reuse *UTF8CodePoint
	for upto < limit {
		reuse = CodePointAt(bytes, upto, reuse)
		ints[count] = reuse.CodePoint
		count++
		upto += reuse.NumBytes
	}
	return count
}

// UTF8ToUTF16 decodes utf8[offset:offset+length] into out as UTF-16 code
// units. Returns the number of code units written. The caller is responsible
// for sizing out. Matches Lucene's UnicodeUtil.UTF8toUTF16(byte[], int, int,
// char[]) byte-for-byte; NOTE that invalid UTF-8 may cause an out-of-range
// access (Lucene's note: "Full characters are read, even if this reads past
// the length passed").
func UTF8ToUTF16(utf8 []byte, offset, length int, out []uint16) int {
	outOff := 0
	limit := offset + length
	for offset < limit {
		b := int(utf8[offset]) & 0xFF
		offset++
		switch {
		case b < 0xC0:
			// Note: Java asserts b < 0x80 here; we trust the caller.
			out[outOff] = uint16(b)
			outOff++
		case b < 0xE0:
			out[outOff] = uint16(((b & 0x1F) << 6) + int(utf8[offset]&0x3F))
			offset++
			outOff++
		case b < 0xF0:
			out[outOff] = uint16(((b & 0x0F) << 12) + (int(utf8[offset]&0x3F) << 6) + int(utf8[offset+1]&0x3F))
			offset += 2
			outOff++
		default:
			ch := ((b & 0x07) << 18) +
				(int(utf8[offset]&0x3F) << 12) +
				(int(utf8[offset+1]&0x3F) << 6) +
				int(utf8[offset+2]&0x3F)
			offset += 3
			if ch < uniMaxBMP {
				out[outOff] = uint16(ch)
				outOff++
			} else {
				chHalf := ch - minSupplementary
				out[outOff] = uint16((chHalf >> 10) + 0xD800)
				out[outOff+1] = uint16((chHalf & halfMask) + 0xDC00)
				outOff += 2
			}
		}
	}
	return outOff
}

// UTF8ToUTF16Ref is the BytesRef overload of UTF8ToUTF16.
func UTF8ToUTF16Ref(ref *BytesRef, chars []uint16) int {
	return UTF8ToUTF16(ref.Bytes, ref.Offset, ref.Length, chars)
}

// NewStringFromCodePoints constructs a Go string from the slice of Unicode
// code points in codePoints[offset:offset+count]. Mirrors Lucene's
// UnicodeUtil.newString. Panics with an IllegalArgumentException-equivalent
// for negative count or for any code point outside [0, 0x10FFFF]; mirrors
// IndexOutOfBoundsException by panicking on out-of-range indices.
func NewStringFromCodePoints(codePoints []int, offset, count int) string {
	if count < 0 {
		panic("NewStringFromCodePoints: negative count")
	}
	if offset < 0 || offset+count > len(codePoints) {
		panic("NewStringFromCodePoints: index out of range")
	}
	chars := make([]uint16, count)
	w := 0
	for r, e := offset, offset+count; r < e; r++ {
		cp := codePoints[r]
		if cp < 0 || cp > 0x10FFFF {
			panic(fmt.Sprintf("invalid code point 0x%X", cp))
		}
		if cp < 0x10000 {
			ensureCharsCap(&chars, w+1)
			chars[w] = uint16(cp)
			w++
		} else {
			ensureCharsCap(&chars, w+2)
			chars[w] = uint16(leadSurrogateOffset + (cp >> leadSurrogateShift))
			chars[w+1] = uint16(trailSurrogateMin + (cp & trailSurrogateMask))
			w += 2
		}
	}
	return utf16ToString(chars[:w])
}

// ToHexStringCodePoints renders a string into a debug-friendly hex form
// matching UnicodeUtil.toHexString — ASCII printable characters are kept
// verbatim, surrogates are tagged H:/L:/E:/F:, and all non-ASCII code units
// are printed as 0x<hex>.
func ToHexStringCodePoints(s string) string {
	units := stringToUTF16(s)
	var b strings.Builder
	for i, ch := range units {
		if i > 0 {
			b.WriteByte(' ')
		}
		if ch < 128 {
			b.WriteByte(byte(ch))
			continue
		}
		switch {
		case ch >= UniSurHighStart && ch <= UniSurHighEnd:
			b.WriteString("H:")
		case ch >= UniSurLowStart && ch <= UniSurLowEnd:
			b.WriteString("L:")
		case ch > UniSurLowEnd:
			if ch == 0xFFFF {
				b.WriteString("F:")
			} else {
				b.WriteString("E:")
			}
		}
		fmt.Fprintf(&b, "0x%x", ch)
	}
	return b.String()
}

// stringToUTF16 encodes a Go string into UTF-16 code units. Behaviour
// matches Java's String.toCharArray for any valid UTF-8 Go string. Lone
// surrogates cannot appear in a valid Go string, so they round-trip through
// the replacement character path inside UTF16ToUTF8.
func stringToUTF16(s string) []uint16 {
	// Pre-size optimistically: ASCII strings need len(s) code units exactly.
	out := make([]uint16, 0, len(s))
	for _, r := range s {
		if r < 0x10000 {
			out = append(out, uint16(r))
			continue
		}
		// Supplementary: encode as surrogate pair.
		r -= 0x10000
		out = append(out, uint16(0xD800+(r>>10)), uint16(0xDC00+(r&0x3FF)))
	}
	return out
}

// utf16ToString decodes a slice of UTF-16 code units into a Go string. Lone
// or out-of-order surrogates are replaced with the Unicode replacement
// character to match Java's String(char[]) behaviour for our purposes
// (constructed strings only carry valid pairs anyway).
func utf16ToString(units []uint16) string {
	var b strings.Builder
	b.Grow(len(units))
	for i := 0; i < len(units); i++ {
		u := units[i]
		switch {
		case u >= UniSurHighStart && u <= UniSurHighEnd:
			if i+1 < len(units) {
				low := units[i+1]
				if low >= UniSurLowStart && low <= UniSurLowEnd {
					r := rune(0x10000 + (int(u-UniSurHighStart) << 10) + int(low-UniSurLowStart))
					b.WriteRune(r)
					i++
					continue
				}
			}
			b.WriteRune(UniReplacementChar)
		case u >= UniSurLowStart && u <= UniSurLowEnd:
			b.WriteRune(UniReplacementChar)
		default:
			b.WriteRune(rune(u))
		}
	}
	return b.String()
}

// ensureCharsCap grows the chars slice if needed to accommodate at least
// minLen units. Used by NewStringFromCodePoints to mirror Java's
// catch-and-grow loop.
func ensureCharsCap(chars *[]uint16, minLen int) {
	if cap(*chars) >= minLen {
		if len(*chars) < minLen {
			*chars = (*chars)[:minLen]
		}
		return
	}
	next := minLen
	if next < cap(*chars)*2 {
		next = cap(*chars) * 2
	}
	grown := make([]uint16, next)
	copy(grown, *chars)
	*chars = grown[:minLen]
}

// mulOverflowCheck multiplies x and y as ints and reports overflow.
// Returns (overflow, product). Implemented in terms of int64 to keep the
// helper portable; the int domain matches Java's int multiplyExact.
func mulOverflowCheck(x, y int) (bool, int) {
	if x == 0 || y == 0 {
		return false, 0
	}
	p := int64(x) * int64(y)
	if int64(int(p)) != p {
		return true, 0
	}
	// Detect 32-bit-int overflow on 64-bit platforms — int is at least 32 bits
	// per Go spec but Lucene's contract is fixed to 32 bits.
	if p > math.MaxInt32 || p < math.MinInt32 {
		return true, 0
	}
	return false, int(p)
}
