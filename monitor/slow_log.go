// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import (
	"fmt"
	"strings"
)

// SlowLog records queries that exceeded a time threshold during a match run.
//
// Port of org.apache.lucene.monitor.SlowLog.
type SlowLog struct {
	entries []SlowLogEntry
}

// SlowLogEntry is a single record in the slow log.
type SlowLogEntry struct {
	// QueryID is the ID of the slow query.
	QueryID string
	// Time is the duration of the query in nanoseconds.
	Time int64
}

// NewSlowLog returns an empty SlowLog.
func NewSlowLog() *SlowLog { return &SlowLog{} }

// AddQuery appends a query and its execution time to the log.
func (s *SlowLog) AddQuery(queryID string, timeNs int64) {
	s.entries = append(s.entries, SlowLogEntry{QueryID: queryID, Time: timeNs})
}

// AddAll copies all entries from another iterable source into this log.
func (s *SlowLog) AddAll(entries []SlowLogEntry) {
	s.entries = append(s.entries, entries...)
}

// Entries returns the recorded entries (read-only view — callers must not modify the slice).
func (s *SlowLog) Entries() []SlowLogEntry { return s.entries }

// String returns a human-readable representation of all slow log entries.
func (s *SlowLog) String() string {
	var sb strings.Builder
	for _, e := range s.entries {
		fmt.Fprintf(&sb, "%s [%dns]\n", e.QueryID, e.Time)
	}
	return sb.String()
}
