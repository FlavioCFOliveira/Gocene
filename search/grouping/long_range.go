// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import "fmt"

// LongRange represents a contiguous range of long values, with an inclusive
// minimum and exclusive maximum.
//
// Mirrors org.apache.lucene.search.grouping.LongRange.
type LongRange struct {
	// Min is the inclusive minimum value of this range.
	Min int64

	// Max is the exclusive maximum value of this range.
	Max int64
}

// NewLongRange creates a new double range, running from min inclusive to max
// exclusive.
//
// Mirrors LongRange(long min, long max).
func NewLongRange(min, max int64) *LongRange {
	return &LongRange{Min: min, Max: max}
}

// String mirrors LongRange.toString().
func (r *LongRange) String() string {
	return fmt.Sprintf("LongRange(%d, %d)", r.Min, r.Max)
}

// Equals mirrors LongRange.equals(Object).
func (r *LongRange) Equals(o *LongRange) bool {
	if r == o {
		return true
	}
	if o == nil {
		return false
	}
	return r.Min == o.Min && r.Max == o.Max
}

// HashCode mirrors LongRange.hashCode(), i.e. Objects.hash(min, max).
func (r *LongRange) HashCode() int {
	result := 1
	result = 31*result + javaLongHashCode(r.Min)
	result = 31*result + javaLongHashCode(r.Max)
	return result
}

// javaLongHashCode mirrors Long.hashCode(long): the exclusive or of the two
// halves of the value.
func javaLongHashCode(v int64) int {
	return int(int32(uint32(uint64(v) ^ (uint64(v) >> 32))))
}
