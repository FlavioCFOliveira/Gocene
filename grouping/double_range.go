// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import "fmt"

// DoubleRange represents a contiguous range of double values, with an
// inclusive minimum and exclusive maximum.
//
// Mirrors org.apache.lucene.search.grouping.DoubleRange.
type DoubleRange struct {
	// Min is the inclusive minimum value of this range.
	Min float64

	// Max is the exclusive maximum value of this range.
	Max float64
}

// NewDoubleRange creates a new double range, running from min inclusive to
// max exclusive.
//
// Mirrors DoubleRange(double min, double max).
func NewDoubleRange(min, max float64) *DoubleRange {
	return &DoubleRange{Min: min, Max: max}
}

// String mirrors DoubleRange.toString().
func (r *DoubleRange) String() string {
	return fmt.Sprintf("DoubleRange(%v, %v)", r.Min, r.Max)
}

// Equals mirrors DoubleRange.equals(Object), which compares both bounds with
// Double.compare.
func (r *DoubleRange) Equals(o *DoubleRange) bool {
	if r == o {
		return true
	}
	if o == nil {
		return false
	}
	return canonicalFloat64Bits(r.Min) == canonicalFloat64Bits(o.Min) &&
		canonicalFloat64Bits(r.Max) == canonicalFloat64Bits(o.Max)
}

// HashCode mirrors DoubleRange.hashCode(), i.e. Objects.hash(min, max).
func (r *DoubleRange) HashCode() int {
	result := 1
	result = 31*result + javaDoubleHashCode(r.Min)
	result = 31*result + javaDoubleHashCode(r.Max)
	return result
}

// javaDoubleHashCode mirrors Double.hashCode(double): the exclusive or of the
// two halves of doubleToLongBits.
func javaDoubleHashCode(v float64) int {
	bits := canonicalFloat64Bits(v)
	return int(int32(uint32(bits ^ (bits >> 32))))
}
