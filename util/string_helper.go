// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"math/rand"
	"sync"
	"time"
)

// BytesDifference compares two BytesRef, element by element, and returns the number
// of elements common to both arrays (from the start of each).
// This method assumes currentTerm comes after priorTerm.
//
// Returns the number of common elements (from the start of each).
// Returns an error if the terms are equal (duplicates indicate out of order terms).
func BytesDifference(priorTerm, currentTerm *BytesRef) (int, error) {
	if priorTerm == nil || currentTerm == nil {
		return 0, nil
	}

	leftBytes := priorTerm.ValidBytes()
	rightBytes := currentTerm.ValidBytes()

	// Find the first position where they differ
	minLen := len(leftBytes)
	if len(rightBytes) < minLen {
		minLen = len(rightBytes)
	}

	for i := 0; i < minLen; i++ {
		if leftBytes[i] != rightBytes[i] {
			return i, nil
		}
	}

	// If we get here, one is a prefix of the other or they are equal
	if len(leftBytes) == len(rightBytes) {
		// They are equal - this indicates out of order terms
		return 0, errors.New("terms out of order: priorTerm=" + priorTerm.String() + ",currentTerm=" + currentTerm.String())
	}

	// Return the length of the shorter one
	return minLen, nil
}

// SortKeyLength returns the length of currentTerm needed for use as a sort key
// so that BytesRefCompare still returns the same result.
// This method assumes currentTerm comes after priorTerm.
func SortKeyLength(priorTerm, currentTerm *BytesRef) (int, error) {
	diff, err := BytesDifference(priorTerm, currentTerm)
	if err != nil {
		return 0, err
	}
	return diff + 1, nil
}

// StartsWith returns true iff the ref starts with the given prefix.
//
// ref is the BytesRef to test
// prefix is the expected prefix
// Returns true iff the ref starts with the given prefix, otherwise false.
func StartsWith(ref, prefix *BytesRef) bool {
	if ref == nil || prefix == nil {
		return false
	}

	// not long enough to start with the prefix
	if ref.Length < prefix.Length {
		return false
	}

	refBytes := ref.ValidBytes()
	prefixBytes := prefix.ValidBytes()

	return bytes.Equal(refBytes[:prefix.Length], prefixBytes)
}

// EndsWith returns true iff the ref ends with the given suffix.
//
// ref is the BytesRef to test
// suffix is the expected suffix
// Returns true iff the ref ends with the given suffix, otherwise false.
func EndsWith(ref, suffix *BytesRef) bool {
	if ref == nil || suffix == nil {
		return false
	}

	startAt := ref.Length - suffix.Length
	// not long enough to end with the suffix
	if startAt < 0 {
		return false
	}

	refBytes := ref.ValidBytes()
	suffixBytes := suffix.ValidBytes()

	return bytes.Equal(refBytes[startAt:], suffixBytes)
}

// MurmurHash3_x86_32 returns the MurmurHash3_x86_32 hash.
// Original source/tests at https://github.com/yonik/java_util/
//
// data is the byte array to hash
// offset is the starting position in the array
// length is the number of bytes to hash
// seed is the initial seed value
func MurmurHash3_x86_32(data []byte, offset, length, seed int) int {
	const c1 uint32 = 0xcc9e2d51
	const c2 uint32 = 0x1b873593

	h1 := uint32(seed)
	roundedEnd := offset + (length & 0xfffffffc) // round down to 4 byte block

	// body
	for i := offset; i < roundedEnd; i += 4 {
		// little endian load order
		k1 := uint32(data[i]) | uint32(data[i+1])<<8 | uint32(data[i+2])<<16 | uint32(data[i+3])<<24

		k1 *= c1
		k1 = (k1 << 15) | (k1 >> 17) // rotate left by 15
		k1 *= c2

		h1 ^= k1
		h1 = (h1 << 13) | (h1 >> 19) // rotate left by 13
		h1 = h1*5 + 0xe6546b64
	}

	// tail
	var k1 uint32 = 0

	switch length & 0x03 {
	case 3:
		k1 = uint32(data[roundedEnd+2]) << 16
		fallthrough
	case 2:
		k1 |= uint32(data[roundedEnd+1]) << 8
		fallthrough
	case 1:
		k1 |= uint32(data[roundedEnd])
		k1 *= c1
		k1 = (k1 << 15) | (k1 >> 17) // rotate left by 15
		k1 *= c2
		h1 ^= k1
	}

	// finalization
	h1 ^= uint32(length)

	// fmix(h1)
	h1 ^= h1 >> 16
	h1 *= 0x85ebca6b
	h1 ^= h1 >> 13
	h1 *= 0xc2b2ae35
	h1 ^= h1 >> 16

	return int(int32(h1)) // Convert back to signed int to match Java's int behavior
}

// MurmurHash3_x86_32_BytesRef returns the MurmurHash3_x86_32 hash for a BytesRef.
//
// bytes is the BytesRef to hash
// seed is the initial seed value
func MurmurHash3_x86_32_BytesRef(bytes *BytesRef, seed int) int {
	if bytes == nil || bytes.Length == 0 {
		return MurmurHash3_x86_32(nil, 0, 0, seed)
	}
	return MurmurHash3_x86_32(bytes.Bytes, bytes.Offset, bytes.Length, seed)
}

// (GoodFastHashSeed lives in bytes_ref_hash.go to avoid duplicate
// declarations; it is the per-process randomized salt initialised
// from time.Now().UnixNano() at package init time.)

// IDLength is the byte length of an id returned by [RandomId].
const IDLength = 16

// xorshift128 state used by RandomId. Mirrors Java's nextId BigInteger
// counter combined with an xorshift128 seed scatter step.
var (
	idMu     sync.Mutex
	idState  [2]uint64
	idActive *big.Int
	idMask   *big.Int
	idInit   sync.Once
)

func initIDState() {
	// Seed from time.Now().UnixNano() — the Go-side equivalent of
	// Lucene's /dev/urandom + nanoTime fallback. The xorshift step
	// below scatters the seed so successive calls don't share the
	// high-order bits of the system clock.
	idState[0] = uint64(time.Now().UnixNano())
	idState[1] = idState[0] ^ 0x9E3779B97F4A7C15
	for i := 0; i < 10; i++ {
		s1 := idState[0]
		s0 := idState[1]
		idState[0] = s0
		s1 ^= s1 << 23
		idState[1] = s1 ^ s0 ^ (s1 >> 17) ^ (s0 >> 26)
	}
	// 128-bit mask of all ones.
	maskBytes := make([]byte, 16)
	for i := range maskBytes {
		maskBytes[i] = 0xFF
	}
	idMask = new(big.Int).SetBytes(maskBytes)
	hi := new(big.Int).SetUint64(idState[0])
	lo := new(big.Int).SetUint64(idState[1])
	idActive = new(big.Int).Or(new(big.Int).Lsh(hi, 64), lo)
}

// RandomId returns a non-cryptographic globally unique 16-byte id.
// Successive calls increment the internal counter (mod 2^128) so that
// every id in a single process is unique.
func RandomId() []byte {
	idInit.Do(initIDState)
	idMu.Lock()
	defer idMu.Unlock()
	raw := idActive.Bytes()
	idActive.Add(idActive, big.NewInt(1))
	idActive.And(idActive, idMask)
	out := make([]byte, IDLength)
	if len(raw) > IDLength {
		// big.Int.Bytes never includes a leading sign byte, but defensively
		// drop any overflow byte and copy the trailing IDLength bytes.
		copy(out, raw[len(raw)-IDLength:])
	} else {
		copy(out[IDLength-len(raw):], raw)
	}
	return out
}

// IdToString renders an id (typically produced by [RandomId]) as a
// human-readable string for debugging. Returns "(null)" for nil and
// flags ids with an unexpected length as "(INVALID FORMAT)".
func IdToString(id []byte) string {
	if id == nil {
		return "(null)"
	}
	bi := new(big.Int).SetBytes(id)
	// Character.MAX_RADIX in Java is 36; strconv supports the same.
	s := bi.Text(36)
	if len(id) != IDLength {
		s += " (INVALID FORMAT)"
	}
	return s
}

// RandomBytes returns n bytes from a deterministic PRNG seeded by
// seed. The output is not cryptographically secure; it exists for
// test fixtures that mirror Java's seeded RandomNumber generators.
func RandomBytes(seed int64, n int) []byte {
	rng := rand.New(rand.NewSource(seed))
	buf := make([]byte, n)
	_, _ = rng.Read(buf)
	return buf
}

// StartsWithBytes is the byte-slice variant of [StartsWith].
func StartsWithBytes(ref []byte, prefix *BytesRef) bool {
	if prefix == nil {
		return false
	}
	if len(ref) < prefix.Length {
		return false
	}
	return bytes.Equal(ref[:prefix.Length], prefix.ValidBytes())
}

// IntsRefToBytesRef converts each int in ints to a byte, returning an
// error when any value is outside [0, 255]. Mirrors
// org.apache.lucene.util.StringHelper.intsRefToBytesRef.
func IntsRefToBytesRef(ints *IntsRef) (*BytesRef, error) {
	if ints == nil {
		return nil, errors.New("ints must not be nil")
	}
	out := make([]byte, ints.Length)
	for i := 0; i < ints.Length; i++ {
		x := ints.Ints[ints.Offset+i]
		if x < 0 || x > 255 {
			return nil, fmt.Errorf("int at pos=%d with value=%d is out-of-bounds for byte", i, x)
		}
		out[i] = byte(x)
	}
	return NewBytesRef(out), nil
}
