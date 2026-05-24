// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// QueryCacheEntry holds a (possibly partial, due to decomposition) query ready
// to be run during a match.
//
// Port of org.apache.lucene.monitor.QueryCacheEntry.
type QueryCacheEntry struct {
	// MatchQuery is the query to run against documents.
	MatchQuery search.Query

	// CacheID is the unique ID of this cache entry (may differ from QueryID due to decomposition).
	CacheID string

	// QueryID is the ID of the parent MonitorQuery.
	QueryID string

	// Metadata is the parent MonitorQuery's metadata.
	Metadata map[string]string
}

// DecomposeMonitorQuery decomposes a MonitorQuery into QueryCacheEntry values
// using the given decomposer.
func DecomposeMonitorQuery(mq *MonitorQuery, decomposer *QueryDecomposer) []*QueryCacheEntry {
	subqueries := decomposer.Decompose(mq.GetQuery())
	entries := make([]*QueryCacheEntry, len(subqueries))
	for i, q := range subqueries {
		entries[i] = &QueryCacheEntry{
			CacheID:    fmt.Sprintf("%s_%d", mq.GetID(), i),
			QueryID:    mq.GetID(),
			MatchQuery: q,
			Metadata:   mq.GetMetadata(),
		}
	}
	return entries
}

// String returns a human-readable representation.
func (e *QueryCacheEntry) String() string {
	return fmt.Sprintf("%s/%s/%v", e.QueryID, e.CacheID, e.MatchQuery)
}
