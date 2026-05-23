// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/intervals/NoMatchIntervalsSource.java

package intervals

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// NoMatchIntervalsSource is an IntervalsSource that always returns no matches.
//
// Mirrors org.apache.lucene.queries.intervals.NoMatchIntervalsSource (Lucene 10.4.0).
type NoMatchIntervalsSource struct {
	reason string
}

// NewNoMatchIntervalsSource creates a NoMatchIntervalsSource with the given reason.
func NewNoMatchIntervalsSource(reason string) *NoMatchIntervalsSource {
	return &NoMatchIntervalsSource{reason: reason}
}

func (s *NoMatchIntervalsSource) Intervals(_ string, _ *index.LeafReaderContext) (IntervalIterator, error) {
	return nil, nil
}

func (s *NoMatchIntervalsSource) Matches(_ string, _ *index.LeafReaderContext, _ int) (IntervalMatchesIterator, error) {
	return nil, nil
}

func (s *NoMatchIntervalsSource) Visit(_ string, _ search.QueryVisitor) {}

func (s *NoMatchIntervalsSource) MinExtent() int { return 0 }

func (s *NoMatchIntervalsSource) PullUpDisjunctions() []IntervalsSource {
	return []IntervalsSource{s}
}

func (s *NoMatchIntervalsSource) Equals(other IntervalsSource) bool {
	o, ok := other.(*NoMatchIntervalsSource)
	return ok && s.reason == o.reason
}

func (s *NoMatchIntervalsSource) HashCode() int { return hashString(s.reason) }

func (s *NoMatchIntervalsSource) String() string { return "NOMATCH(" + s.reason + ")" }

func hashString(str string) int {
	h := 0
	for i := 0; i < len(str); i++ {
		h = h*31 + int(str[i])
	}
	return h
}
