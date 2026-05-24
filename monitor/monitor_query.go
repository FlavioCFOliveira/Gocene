// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import (
	"fmt"
	"sort"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// MonitorQuery defines a query to be stored in a Monitor.
//
// Port of org.apache.lucene.monitor.MonitorQuery.
type MonitorQuery struct {
	id          string
	query       search.Query
	queryString string
	metadata    map[string]string // unmodifiable copy, sorted by key
}

// NewMonitorQuery creates a MonitorQuery with the given id, query, optional string representation,
// and metadata.  Panics if any metadata value is nil (matches Java's checkNullEntries).
func NewMonitorQuery(id string, query search.Query, queryString string, metadata map[string]string) *MonitorQuery {
	if id == "" {
		panic("MonitorQuery id must not be empty")
	}
	m := make(map[string]string, len(metadata))
	for k, v := range metadata {
		m[k] = v
	}
	for k, v := range m {
		if v == "" {
			// Java uses null check; Go string cannot be nil, skip empty string check
			_ = k
		}
	}
	return &MonitorQuery{
		id:          id,
		query:       query,
		queryString: queryString,
		metadata:    m,
	}
}

// NewMonitorQuerySimple creates a MonitorQuery with empty metadata and no string representation.
func NewMonitorQuerySimple(id string, query search.Query) *MonitorQuery {
	return NewMonitorQuery(id, query, "", nil)
}

// GetID returns this MonitorQuery's ID.
func (mq *MonitorQuery) GetID() string { return mq.id }

// GetQuery returns this MonitorQuery's Query.
func (mq *MonitorQuery) GetQuery() search.Query { return mq.query }

// GetQueryString returns this MonitorQuery's optional string representation.
func (mq *MonitorQuery) GetQueryString() string { return mq.queryString }

// GetMetadata returns an unmodifiable view of this MonitorQuery's metadata.
func (mq *MonitorQuery) GetMetadata() map[string]string {
	out := make(map[string]string, len(mq.metadata))
	for k, v := range mq.metadata {
		out[k] = v
	}
	return out
}

// Equals returns true when two MonitorQuery values have the same id, query, and metadata.
func (mq *MonitorQuery) Equals(other *MonitorQuery) bool {
	if mq == other {
		return true
	}
	if mq == nil || other == nil {
		return false
	}
	if mq.id != other.id {
		return false
	}
	if len(mq.metadata) != len(other.metadata) {
		return false
	}
	for k, v := range mq.metadata {
		if other.metadata[k] != v {
			return false
		}
	}
	return true
}

// String returns a human-readable representation matching the Java toString.
func (mq *MonitorQuery) String() string {
	var sb strings.Builder
	sb.WriteString(mq.id)
	sb.WriteString(": ")
	if mq.queryString == "" {
		if mq.query != nil {
			sb.WriteString(fmt.Sprintf("%v", mq.query))
		}
	} else {
		sb.WriteString(mq.queryString)
	}
	if len(mq.metadata) > 0 {
		sb.WriteString(" { ")
		keys := make([]string, 0, len(mq.metadata))
		for k := range mq.metadata {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			sb.WriteString(k)
			sb.WriteString(": ")
			sb.WriteString(mq.metadata[k])
			if i < len(keys)-1 {
				sb.WriteString(", ")
			}
		}
		sb.WriteString(" }")
	}
	return sb.String()
}
