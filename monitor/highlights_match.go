// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import (
	"fmt"
	"sort"
)

// Hit represents a single highlighted region within a field.
//
// Port of org.apache.lucene.monitor.HighlightsMatch.Hit.
type Hit struct {
	// StartPosition is the start token position.
	StartPosition int
	// StartOffset is the start character offset.
	StartOffset int
	// EndPosition is the end token position.
	EndPosition int
	// EndOffset is the end character offset.
	EndOffset int
}

// String returns a human-readable representation.
func (h Hit) String() string {
	return fmt.Sprintf("%d(%d)->%d(%d)", h.StartPosition, h.StartOffset, h.EndPosition, h.EndOffset)
}

// HighlightsMatch extends QueryMatch with per-field hit positions.
//
// Port of org.apache.lucene.monitor.HighlightsMatch.
type HighlightsMatch struct {
	QueryMatch
	hits map[string][]Hit // field → sorted hits
}

// NewHighlightsMatch creates a HighlightsMatch for the given query ID.
func NewHighlightsMatch(queryID string) *HighlightsMatch {
	return &HighlightsMatch{
		QueryMatch: QueryMatch{queryID: queryID},
		hits:       make(map[string][]Hit),
	}
}

// AddHit records a single highlight for the given field.
func (m *HighlightsMatch) AddHit(field string, startPos, endPos, startOffset, endOffset int) {
	m.hits[field] = append(m.hits[field], Hit{startPos, startOffset, endPos, endOffset})
}

// GetHits returns all hits per field (read-only).
func (m *HighlightsMatch) GetHits() map[string][]Hit {
	out := make(map[string][]Hit, len(m.hits))
	for k, v := range m.hits {
		cp := make([]Hit, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}

// GetFields returns the fields in which matches were found.
func (m *HighlightsMatch) GetFields() []string {
	fields := make([]string, 0, len(m.hits))
	for f := range m.hits {
		fields = append(fields, f)
	}
	sort.Strings(fields)
	return fields
}

// GetFieldHits returns the hits for a specific field.
func (m *HighlightsMatch) GetFieldHits(field string) []Hit {
	hits := m.hits[field]
	cp := make([]Hit, len(hits))
	copy(cp, hits)
	return cp
}

// GetHitCount returns the total number of hits across all fields.
func (m *HighlightsMatch) GetHitCount() int {
	c := 0
	for _, h := range m.hits {
		c += len(h)
	}
	return c
}

// MergeHighlights merges multiple HighlightsMatch values for the same query ID.
func MergeHighlights(queryID string, matches ...*HighlightsMatch) *HighlightsMatch {
	out := NewHighlightsMatch(queryID)
	for _, m := range matches {
		for field, hits := range m.hits {
			out.hits[field] = append(out.hits[field], hits...)
		}
	}
	// Sort hits per field for deterministic output.
	for f := range out.hits {
		h := out.hits[f]
		sort.Slice(h, func(i, j int) bool {
			if h[i].StartPosition != h[j].StartPosition {
				return h[i].StartPosition < h[j].StartPosition
			}
			return h[i].EndPosition < h[j].EndPosition
		})
	}
	return out
}

// Equals returns true when two HighlightsMatch values are equal.
func (m *HighlightsMatch) Equals(other *HighlightsMatch) bool {
	if m == other {
		return true
	}
	if m == nil || other == nil {
		return false
	}
	if m.queryID != other.queryID {
		return false
	}
	if len(m.hits) != len(other.hits) {
		return false
	}
	for f, h := range m.hits {
		oh, ok := other.hits[f]
		if !ok || len(h) != len(oh) {
			return false
		}
		for i := range h {
			if h[i] != oh[i] {
				return false
			}
		}
	}
	return true
}

// String returns a human-readable representation.
func (m *HighlightsMatch) String() string {
	return m.QueryMatch.String() + fmt.Sprintf("{hits=%v}", m.hits)
}
