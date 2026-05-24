// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// Monitor holds a set of queries and efficiently matches them against documents.
//
// Port of org.apache.lucene.monitor.Monitor.
//
// Deviation: Match execution against real documents (IndexSearcher integration,
// DocumentBatch, LeafReader) is deferred to backlog #2693.  The present port
// exposes the registration/deletion/query-management API backed by
// WritableQueryIndex / ReadonlyQueryIndex.
type Monitor struct {
	presearcher     Presearcher
	analyzer        analysis.Analyzer
	queryIndex      QueryIndex
	commitBatchSize int
}

// NewMonitor creates a non-persistent Monitor with the default TermFilteredPresearcher.
func NewMonitor(analyzer analysis.Analyzer) (*Monitor, error) {
	return NewMonitorWithPresearcher(analyzer, NewTermFilteredPresearcher())
}

// NewMonitorWithPresearcher creates a Monitor with a custom Presearcher.
func NewMonitorWithPresearcher(analyzer analysis.Analyzer, presearcher Presearcher) (*Monitor, error) {
	return NewMonitorFull(analyzer, presearcher, NewMonitorConfiguration())
}

// NewMonitorWithConfig creates a Monitor with a custom configuration and the default presearcher.
func NewMonitorWithConfig(analyzer analysis.Analyzer, cfg *MonitorConfiguration) (*Monitor, error) {
	return NewMonitorFull(analyzer, NewTermFilteredPresearcher(), cfg)
}

// NewMonitorFull creates a Monitor with all options.
func NewMonitorFull(analyzer analysis.Analyzer, presearcher Presearcher, cfg *MonitorConfiguration) (*Monitor, error) {
	if cfg == nil {
		cfg = NewMonitorConfiguration()
	}
	var qi QueryIndex
	if cfg.ReadOnly {
		inner := NewWritableQueryIndex(cfg, presearcher)
		qi = NewReadonlyQueryIndex(inner)
	} else {
		qi = NewWritableQueryIndex(cfg, presearcher)
	}
	return &Monitor{
		presearcher:     presearcher,
		analyzer:        analyzer,
		queryIndex:      qi,
		commitBatchSize: cfg.QueryUpdateBufferSize,
	}, nil
}

// Register stores the given queries in the Monitor's query index.
func (m *Monitor) Register(queries []*MonitorQuery) error {
	return m.queryIndex.Commit(queries)
}

// Delete removes queries with the given IDs.
func (m *Monitor) Delete(queryIDs ...string) error {
	return m.queryIndex.DeleteQueries(queryIDs)
}

// Clear removes all queries from the Monitor.
func (m *Monitor) Clear() error { return m.queryIndex.Clear() }

// GetQuery returns the stored MonitorQuery for the given ID, or nil.
func (m *Monitor) GetQuery(queryID string) (*MonitorQuery, error) {
	return m.queryIndex.GetQuery(queryID)
}

// GetQueryCount returns the number of registered queries.
func (m *Monitor) GetQueryCount() (int, error) { return m.queryIndex.NumDocs() }

// GetPresearcher returns the Presearcher used by this Monitor.
func (m *Monitor) GetPresearcher() Presearcher { return m.presearcher }

// AddUpdateListener registers a MonitorUpdateListener.
func (m *Monitor) AddUpdateListener(listener MonitorUpdateListener) {
	m.queryIndex.AddListener(listener)
}

// Close releases resources held by the Monitor.
func (m *Monitor) Close() error { return m.queryIndex.Close() }
