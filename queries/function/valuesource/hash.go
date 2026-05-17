// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import "math"

// hashFloat32 returns the IEEE-754 bit pattern of v as int32, matching
// Java's Float.floatToIntBits(constant) idiom used by Lucene's hashCode
// implementations.
func hashFloat32(v float32) int32 {
	return int32(math.Float32bits(v))
}

// hashFloat64 mirrors Lucene's
//
//	long bits = Double.doubleToRawLongBits(constant);
//	return (int) (bits ^ (bits >>> 32));
func hashFloat64(v float64) int32 {
	bits := math.Float64bits(v)
	return int32(bits ^ (bits >> 32))
}

// hashString returns a 32-bit FNV-1a hash of s.
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
