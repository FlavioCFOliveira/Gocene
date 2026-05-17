// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package function

import (
	"math"
)

// hashString returns a 32-bit FNV-1a hash of s, suitable for stable
// HashCode methods that need to combine string identity with other
// fields. Matches Java's Objects.hash(String) shape closely enough for
// equivalence-class purposes (we do not promise byte-for-byte parity).
func hashString(s string) int32 {
	const (
		offset32 uint32 = 2166136261
		prime32  uint32 = 16777619
	)
	h := offset32
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= prime32
	}
	return int32(h)
}

// hashFloat64 hashes the IEEE-754 bit pattern of v. Mirrors Java's
// Double.hashCode shape used by Objects.hash(Double).
func hashFloat64(v float64) int32 {
	bits := math.Float64bits(v)
	return int32(bits ^ (bits >> 32))
}

// hashFloat32 hashes the IEEE-754 bit pattern of v.
func hashFloat32(v float32) int32 {
	return int32(math.Float32bits(v))
}

// hashInt32 mixes v into a 32-bit hash via Murmur-inspired finaliser.
// Used wherever we need an int field to participate stably in HashCode.
func hashInt32(v int32) int32 {
	x := uint32(v)
	x ^= x >> 16
	x *= 0x85ebca6b
	x ^= x >> 13
	x *= 0xc2b2ae35
	x ^= x >> 16
	return int32(x)
}

// hashBool returns 1 for true and 0 for false.
func hashBool(b bool) int32 {
	if b {
		return 1
	}
	return 0
}

// combineHash mixes two hash values, mirroring the loop body of
// Objects.hash(Object...) (result = 31*result + h).
func combineHash(a, b int32) int32 {
	return 31*a + b
}
