// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import "github.com/FlavioCFOliveira/Gocene/search"

// QueryDecomposer splits a disjunction query into its constituent parts so
// that they can be indexed and run separately in the Monitor.
//
// Port of org.apache.lucene.monitor.QueryDecomposer.
//
// Deviation: Gocene's search package does not yet expose BooleanQuery /
// DisjunctionMaxQuery / BoostQuery rewrite helpers.  This implementation
// therefore acts as a pass-through (no decomposition) and returns the query
// as a single-element slice, preserving the public contract.
// Full decomposition is deferred until BooleanQuery support lands (backlog #2693).
type QueryDecomposer struct{}

// NewQueryDecomposer returns a default QueryDecomposer.
func NewQueryDecomposer() *QueryDecomposer { return &QueryDecomposer{} }

// Decompose splits the query into indexable sub-queries.
// The default implementation returns the query unchanged (no-op decomposition).
func (d *QueryDecomposer) Decompose(q search.Query) []search.Query {
	if q == nil {
		return nil
	}
	return []search.Query{q}
}
