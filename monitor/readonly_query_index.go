// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import "github.com/FlavioCFOliveira/Gocene/search"

// ReadonlyQueryIndex wraps a QueryIndex and rejects all mutation operations.
//
// Port of org.apache.lucene.monitor.ReadonlyQueryIndex.
//
// Deviation: The Java implementation extends WritableQueryIndex and overrides
// the mutation methods.  In Gocene it wraps an arbitrary QueryIndex.
type ReadonlyQueryIndex struct {
	inner QueryIndex
}

// NewReadonlyQueryIndex wraps inner in a read-only view.
func NewReadonlyQueryIndex(inner QueryIndex) *ReadonlyQueryIndex {
	return &ReadonlyQueryIndex{inner: inner}
}

// Commit panics because the index is read-only.
func (r *ReadonlyQueryIndex) Commit(_ []*MonitorQuery) error {
	panic("cannot modify a ReadonlyQueryIndex")
}

// GetQuery delegates to the underlying index.
func (r *ReadonlyQueryIndex) GetQuery(queryID string) (*MonitorQuery, error) {
	return r.inner.GetQuery(queryID)
}

// Scan delegates to the underlying index.
func (r *ReadonlyQueryIndex) Scan(collector QueryCollector) error {
	return r.inner.Scan(collector)
}

// Search delegates to the underlying index.
func (r *ReadonlyQueryIndex) Search(q search.Query, collector QueryCollector) (int64, error) {
	return r.inner.Search(q, collector)
}

// PurgeCache delegates to the underlying index.
func (r *ReadonlyQueryIndex) PurgeCache() error { return r.inner.PurgeCache() }

// NumDocs delegates to the underlying index.
func (r *ReadonlyQueryIndex) NumDocs() (int, error) { return r.inner.NumDocs() }

// CacheSize delegates to the underlying index.
func (r *ReadonlyQueryIndex) CacheSize() int { return r.inner.CacheSize() }

// DeleteQueries panics because the index is read-only.
func (r *ReadonlyQueryIndex) DeleteQueries(_ []string) error {
	panic("cannot modify a ReadonlyQueryIndex")
}

// Clear panics because the index is read-only.
func (r *ReadonlyQueryIndex) Clear() error {
	panic("cannot modify a ReadonlyQueryIndex")
}

// GetLastPurged delegates to the underlying index.
func (r *ReadonlyQueryIndex) GetLastPurged() int64 { return r.inner.GetLastPurged() }

// AddListener delegates to the underlying index.
func (r *ReadonlyQueryIndex) AddListener(listener MonitorUpdateListener) {
	r.inner.AddListener(listener)
}

// Close delegates to the underlying index.
func (r *ReadonlyQueryIndex) Close() error { return r.inner.Close() }
