// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import "github.com/FlavioCFOliveira/Gocene/search"

// QueryIndex is the abstract backing store for a Monitor's registered queries.
//
// Port of org.apache.lucene.monitor.QueryIndex (abstract class).
//
// Deviation: SearcherManager, IndexWriter and related Lucene IO types are not
// yet available in Gocene.  QueryIndex is provided as an interface that captures
// the observable contract; WritableQueryIndex and ReadonlyQueryIndex implement it.
// Full integration is deferred to backlog #2693.
type QueryIndex interface {
	// Commit stores a new batch of queries.
	Commit(updates []*MonitorQuery) error

	// GetQuery retrieves a stored MonitorQuery by its ID.
	GetQuery(queryID string) (*MonitorQuery, error)

	// Scan calls the collector for every entry in the index.
	Scan(collector QueryCollector) error

	// Search runs the given query against the index and calls the collector for
	// each matching entry.
	Search(query search.Query, collector QueryCollector) (int64, error)

	// PurgeCache removes stale entries from the query cache.
	PurgeCache() error

	// NumDocs returns the number of documents (queries) in the index.
	NumDocs() (int, error)

	// CacheSize returns the current size of the query cache.
	CacheSize() int

	// DeleteQueries removes queries with the given IDs.
	DeleteQueries(ids []string) error

	// Clear removes all queries from the index.
	Clear() error

	// GetLastPurged returns the timestamp of the last purge run.
	GetLastPurged() int64

	// AddListener registers a MonitorUpdateListener.
	AddListener(listener MonitorUpdateListener)

	// Close releases resources.
	Close() error
}

// QueryCollector is called for each matching query during a scan or search.
type QueryCollector interface {
	// MatchQuery is called for each matching cache entry.
	MatchQuery(id string, query *QueryCacheEntry, dataValues *QueryIndexDataValues) error
}

// QueryIndexDataValues carries the per-document values exposed during a scan.
// These are placeholders until the Lucene DocValues/BinaryDocValues port lands.
type QueryIndexDataValues struct {
	// QueryID is the stored query ID.
	QueryID string
	// CacheID is the stored cache entry ID.
	CacheID string
	// SerializedQuery is the raw bytes of the serialized MonitorQuery, if present.
	SerializedQuery []byte
}
