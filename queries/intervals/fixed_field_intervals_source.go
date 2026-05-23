// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/intervals/FixedFieldIntervalsSource.java

package intervals

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// FixedFieldIntervalsSource wraps another IntervalsSource but always uses
// a fixed field name regardless of the field passed by the caller.
//
// Mirrors org.apache.lucene.queries.intervals.FixedFieldIntervalsSource (Lucene 10.4.0).
type FixedFieldIntervalsSource struct {
	field  string
	source IntervalsSource
}

// NewFixedFieldIntervalsSource creates a FixedFieldIntervalsSource.
func NewFixedFieldIntervalsSource(field string, source IntervalsSource) *FixedFieldIntervalsSource {
	return &FixedFieldIntervalsSource{field: field, source: source}
}

func (s *FixedFieldIntervalsSource) Intervals(_ string, ctx *index.LeafReaderContext) (IntervalIterator, error) {
	return s.source.Intervals(s.field, ctx)
}

func (s *FixedFieldIntervalsSource) Matches(_ string, ctx *index.LeafReaderContext, doc int) (IntervalMatchesIterator, error) {
	return s.source.Matches(s.field, ctx, doc)
}

func (s *FixedFieldIntervalsSource) Visit(_ string, visitor search.QueryVisitor) {
	s.source.Visit(s.field, visitor)
}

func (s *FixedFieldIntervalsSource) MinExtent() int { return s.source.MinExtent() }

func (s *FixedFieldIntervalsSource) PullUpDisjunctions() []IntervalsSource {
	inner := s.source.PullUpDisjunctions()
	if len(inner) == 1 {
		return []IntervalsSource{s}
	}
	out := make([]IntervalsSource, len(inner))
	for i, src := range inner {
		out[i] = NewFixedFieldIntervalsSource(s.field, src)
	}
	return out
}

func (s *FixedFieldIntervalsSource) Equals(other IntervalsSource) bool {
	o, ok := other.(*FixedFieldIntervalsSource)
	return ok && s.field == o.field && s.source.Equals(o.source)
}

func (s *FixedFieldIntervalsSource) HashCode() int {
	return hashString(s.field)*31 + s.source.HashCode()
}

func (s *FixedFieldIntervalsSource) String() string {
	return "field(" + s.field + "):" + s.source.String()
}
