// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "errors"

// OrdinalMap maps per-segment ordinals to a single global ordinal space so
// that callers can compare or aggregate SortedDocValues / SortedSetDocValues
// across the leaves of a composite reader. Mirrors
// org.apache.lucene.index.OrdinalMap from Apache Lucene 10.4.0.
//
// Gocene skeleton: only the public surface and constructor stub are in
// place. The build implementations (which require LongValues, PackedInts
// composition and TermsEnum merge logic) are deferred to backlog #2703.
type OrdinalMap struct {
	// Owner is the cache key owner this map is associated with.
	Owner *CacheKey

	// valueCount is the number of distinct global ordinals.
	valueCount int64

	// ramBytesUsed is the cached estimate of memory usage.
	ramBytesUsed int64
}

// ErrOrdinalMapNotImplemented is returned by the OrdinalMap build helpers
// until the full Lucene parity port lands.
var ErrOrdinalMapNotImplemented = errors.New("OrdinalMap.Build helpers are not implemented yet (Sprint 22 follow-up #2703)")

// NewOrdinalMap returns an OrdinalMap stub owned by owner. The map is empty
// until build helpers are implemented.
func NewOrdinalMap(owner *CacheKey) *OrdinalMap {
	return &OrdinalMap{Owner: owner}
}

// BuildFromSortedValues builds an OrdinalMap from per-segment SortedDocValues.
// Currently returns ErrOrdinalMapNotImplemented.
func BuildOrdinalMapFromSortedValues(_ *CacheKey, _ []SortedDocValues, _ float32) (*OrdinalMap, error) {
	return nil, ErrOrdinalMapNotImplemented
}

// BuildOrdinalMapFromSortedSetValues builds an OrdinalMap from per-segment
// SortedSetDocValues. Currently returns ErrOrdinalMapNotImplemented.
func BuildOrdinalMapFromSortedSetValues(_ *CacheKey, _ []SortedSetDocValues, _ float32) (*OrdinalMap, error) {
	return nil, ErrOrdinalMapNotImplemented
}

// GetValueCount returns the number of distinct global ordinals.
func (m *OrdinalMap) GetValueCount() int64 { return m.valueCount }

// RAMBytesUsed returns the estimated RAM usage in bytes.
func (m *OrdinalMap) RAMBytesUsed() int64 { return m.ramBytesUsed }

// GetGlobalOrds returns the per-segment to global ord map for the given
// segment index. Returns nil until the build helpers are implemented.
func (m *OrdinalMap) GetGlobalOrds(_ int) []int64 { return nil }

// GetFirstSegmentOrd returns the first segment ord for a global ord. Returns
// -1 until the build helpers are implemented.
func (m *OrdinalMap) GetFirstSegmentOrd(_ int64) int64 { return -1 }

// GetFirstSegmentNumber returns the first segment number for a global ord.
// Returns -1 until the build helpers are implemented.
func (m *OrdinalMap) GetFirstSegmentNumber(_ int64) int { return -1 }
