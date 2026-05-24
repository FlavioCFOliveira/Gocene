// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import (
	"sync"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// WritableQueryIndex is an in-memory, writable implementation of QueryIndex.
//
// Port of org.apache.lucene.monitor.WritableQueryIndex.
//
// Deviation: The Java implementation uses Lucene's IndexWriter + SearcherManager.
// Gocene's WritableQueryIndex stores queries in a Go map and delegates read-only
// access to ReadonlyQueryIndex.  Full on-disk persistence is deferred to
// backlog #2693.
type WritableQueryIndex struct {
	mu          sync.RWMutex
	queries     map[string]*MonitorQuery // queryID → MonitorQuery
	cache       map[string]*QueryCacheEntry
	decomposer  *QueryDecomposer
	serializer  MonitorQuerySerializer
	listeners   []MonitorUpdateListener
	lastPurged  int64
	presearcher Presearcher
}

// NewWritableQueryIndex creates an in-memory WritableQueryIndex.
func NewWritableQueryIndex(cfg *MonitorConfiguration, presearcher Presearcher) *WritableQueryIndex {
	if cfg == nil {
		cfg = NewMonitorConfiguration()
	}
	return &WritableQueryIndex{
		queries:     make(map[string]*MonitorQuery),
		cache:       make(map[string]*QueryCacheEntry),
		decomposer:  cfg.QueryDecomposer,
		serializer:  cfg.Serializer,
		presearcher: presearcher,
	}
}

// Commit adds or replaces queries in the index.
func (w *WritableQueryIndex) Commit(updates []*MonitorQuery) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, mq := range updates {
		w.queries[mq.GetID()] = mq
		for _, entry := range DecomposeMonitorQuery(mq, w.decomposer) {
			w.cache[entry.CacheID] = entry
		}
	}
	for _, l := range w.listeners {
		l.AfterUpdate(updates)
	}
	return nil
}

// GetQuery returns the stored MonitorQuery for the given ID, or nil.
func (w *WritableQueryIndex) GetQuery(queryID string) (*MonitorQuery, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.queries[queryID], nil
}

// Scan calls the collector for every entry in the cache.
func (w *WritableQueryIndex) Scan(collector QueryCollector) error {
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, entry := range w.cache {
		dv := &QueryIndexDataValues{QueryID: entry.QueryID, CacheID: entry.CacheID}
		if err := collector.MatchQuery(entry.CacheID, entry, dv); err != nil {
			return err
		}
	}
	return nil
}

// Search runs the query against the index (no-op stub).
// Full implementation deferred to backlog #2693.
func (w *WritableQueryIndex) Search(_ search.Query, _ QueryCollector) (int64, error) {
	return 0, nil
}

// PurgeCache is a no-op stub.
func (w *WritableQueryIndex) PurgeCache() error { return nil }

// NumDocs returns the number of stored queries.
func (w *WritableQueryIndex) NumDocs() (int, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.queries), nil
}

// CacheSize returns the number of cache entries.
func (w *WritableQueryIndex) CacheSize() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.cache)
}

// DeleteQueries removes queries with the given IDs.
func (w *WritableQueryIndex) DeleteQueries(ids []string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, id := range ids {
		delete(w.queries, id)
	}
	for id, entry := range w.cache {
		if _, gone := w.queries[entry.QueryID]; !gone {
			delete(w.cache, id)
		}
	}
	for _, l := range w.listeners {
		l.AfterDelete(ids)
	}
	return nil
}

// Clear removes all queries and cache entries.
func (w *WritableQueryIndex) Clear() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.queries = make(map[string]*MonitorQuery)
	w.cache = make(map[string]*QueryCacheEntry)
	for _, l := range w.listeners {
		l.AfterClear()
	}
	return nil
}

// GetLastPurged returns the timestamp of the last purge (always 0 for this stub).
func (w *WritableQueryIndex) GetLastPurged() int64 { return w.lastPurged }

// AddListener registers a MonitorUpdateListener.
func (w *WritableQueryIndex) AddListener(listener MonitorUpdateListener) {
	w.listeners = append(w.listeners, listener)
}

// Close is a no-op for the in-memory index.
func (w *WritableQueryIndex) Close() error { return nil }
