// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"sync"
	"sync/atomic"
)

// CacheKey represents a unique key for caching purposes.
// This is the Go port of Lucene's org.apache.lucene.index.IndexReader.CacheKey.
//
// CacheKey is used to identify a specific state of an IndexReader for caching.
// Two readers with the same CacheKey are guaranteed to have identical content.
type CacheKey struct {
	// id is a unique identifier for this cache key
	id uint64
}

// cacheKeyCounter is used to generate unique cache keys.
var cacheKeyCounter atomic.Uint64

// NewCacheKey creates a new unique CacheKey.
func NewCacheKey() *CacheKey {
	return &CacheKey{
		id: cacheKeyCounter.Add(1),
	}
}

// String returns a string representation of the CacheKey.
func (ck *CacheKey) String() string {
	return string(rune(ck.id))
}

// ID returns the unique ID of the CacheKey.
func (ck *CacheKey) ID() uint64 {
	return ck.id
}

// Equals returns true if this CacheKey equals another.
func (ck *CacheKey) Equals(other *CacheKey) bool {
	if other == nil {
		return false
	}
	return ck.id == other.id
}

// CacheHelper provides caching support for IndexReaders.
// This is the Go port of Lucene's org.apache.lucene.index.IndexReader.CacheHelper.
//
// CacheHelper provides a unique CacheKey that remains constant for the lifetime
// of the reader. This allows caches to use the key as a stable identifier.
type CacheHelper interface {
	// CacheKey returns a CacheKey that uniquely identifies this reader state.
	// The same CacheKey is returned as long as the reader state has not changed.
	CacheKey() *CacheKey

	// AddClosedListener adds a listener that will be called when the reader is closed.
	AddClosedListener(listener func())
}

// ReaderCacheHelper is a CacheHelper implementation for IndexReaders.
// This is the Go port of Lucene's org.apache.lucene.index.IndexReader.CacheHelper.
type ReaderCacheHelper struct {
	// cacheKey is the unique cache key for this reader
	cacheKey *CacheKey

	// closedListeners are listeners to notify when the reader is closed
	closedListeners []func()

	// mu protects closedListeners
	mu sync.Mutex
}

// NewReaderCacheHelper creates a new ReaderCacheHelper.
func NewReaderCacheHelper() *ReaderCacheHelper {
	return &ReaderCacheHelper{
		cacheKey: NewCacheKey(),
	}
}

// CacheKey returns the unique CacheKey for this reader.
func (h *ReaderCacheHelper) CacheKey() *CacheKey {
	return h.cacheKey
}

// AddClosedListener adds a listener to be called when the reader is closed.
func (h *ReaderCacheHelper) AddClosedListener(listener func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closedListeners = append(h.closedListeners, listener)
}

// NotifyClosedListeners notifies all registered listeners that the reader is closed.
func (h *ReaderCacheHelper) NotifyClosedListeners() {
	h.mu.Lock()
	listeners := h.closedListeners
	h.closedListeners = nil
	h.mu.Unlock()

	for _, listener := range listeners {
		listener()
	}
}

// CoreCacheKey returns a CacheKey for a core reader (SegmentCoreReaders).
// This is used for caching per-segment data that doesn't change when the
// segment is reopened.
type CoreCacheKey struct {
	// segmentName is the name of the segment
	segmentName string
	// id is a unique identifier
	id uint64
}

// NewCoreCacheKey creates a new CoreCacheKey for a segment.
func NewCoreCacheKey(segmentName string) *CoreCacheKey {
	return &CoreCacheKey{
		segmentName: segmentName,
		id:          cacheKeyCounter.Add(1),
	}
}

// String returns a string representation of the CoreCacheKey.
func (ck *CoreCacheKey) String() string {
	return ck.segmentName
}

// ID returns the unique ID of the CoreCacheKey.
func (ck *CoreCacheKey) ID() uint64 {
	return ck.id
}

// Equals returns true if this CoreCacheKey equals another.
func (ck *CoreCacheKey) Equals(other *CoreCacheKey) bool {
	if other == nil {
		return false
	}
	return ck.id == other.id && ck.segmentName == other.segmentName
}

// CombinedCacheKey combines multiple cache keys into one.
// This is useful for caches that depend on multiple sources.
type CombinedCacheKey struct {
	keys []*CacheKey
}

// NewCombinedCacheKey creates a new CombinedCacheKey from multiple keys.
func NewCombinedCacheKey(keys ...*CacheKey) *CombinedCacheKey {
	return &CombinedCacheKey{
		keys: keys,
	}
}

// String returns a string representation of the CombinedCacheKey.
func (ck *CombinedCacheKey) String() string {
	result := ""
	for _, key := range ck.keys {
		result += key.String()
	}
	return result
}

// Equals returns true if this CombinedCacheKey equals another.
func (ck *CombinedCacheKey) Equals(other *CombinedCacheKey) bool {
	if other == nil {
		return false
	}
	if len(ck.keys) != len(other.keys) {
		return false
	}
	for i, key := range ck.keys {
		if !key.Equals(other.keys[i]) {
			return false
		}
	}
	return true
}

// CacheHelpers provides utility functions for working with cache helpers.
var CacheHelpers = &cacheHelpers{}

type cacheHelpers struct{}

// GetCacheKey returns a CacheKey for the given reader, or nil if not available.
func (h *cacheHelpers) GetCacheKey(reader IndexReaderInterface) *CacheKey {
	if withHelper, ok := reader.(interface{ GetCacheHelper() CacheHelper }); ok {
		helper := withHelper.GetCacheHelper()
		if helper != nil {
			return helper.CacheKey()
		}
	}
	return nil
}

// AddClosedListener adds a closed listener to a reader, if supported.
func (h *cacheHelpers) AddClosedListener(reader IndexReaderInterface, listener func()) bool {
	if withHelper, ok := reader.(interface{ GetCacheHelper() CacheHelper }); ok {
		helper := withHelper.GetCacheHelper()
		if helper != nil {
			helper.AddClosedListener(listener)
			return true
		}
	}
	return false
}

// HasDeletions checks if a reader has deletions.
func (h *cacheHelpers) HasDeletions(reader IndexReaderInterface) bool {
	if withDeletions, ok := reader.(interface{ HasDeletions() bool }); ok {
		return withDeletions.HasDeletions()
	}
	return false
}

// GetLiveDocs returns the live docs Bits for a reader, or nil if not available.
func (h *cacheHelpers) GetLiveDocs(reader IndexReaderInterface) interface {
	Get(index int) bool
	Length() int
} {
	if withLiveDocs, ok := reader.(interface{ GetLiveDocs() interface {
		Get(index int) bool
		Length() int
	} }); ok {
		return withLiveDocs.GetLiveDocs()
	}
	return nil
}