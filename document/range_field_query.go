// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import "fmt"

// RangeFieldQueryType enumerates the four spatial relations supported by
// RangeFieldQuery, mirroring Lucene 10.4.0's
// org.apache.lucene.document.RangeFieldQuery.QueryType.
type RangeFieldQueryType int

const (
	// RangeFieldQueryTypeIntersects matches ranges overlapping the query.
	RangeFieldQueryTypeIntersects RangeFieldQueryType = iota
	// RangeFieldQueryTypeWithin matches ranges fully contained in the query.
	RangeFieldQueryTypeWithin
	// RangeFieldQueryTypeContains matches ranges containing the query.
	RangeFieldQueryTypeContains
	// RangeFieldQueryTypeCrosses matches ranges partially overlapping
	// (intersect but not contain or within).
	RangeFieldQueryTypeCrosses
)

// String returns the Lucene-canonical name for the query type.
func (t RangeFieldQueryType) String() string {
	switch t {
	case RangeFieldQueryTypeIntersects:
		return "INTERSECTS"
	case RangeFieldQueryTypeWithin:
		return "WITHIN"
	case RangeFieldQueryTypeContains:
		return "CONTAINS"
	case RangeFieldQueryTypeCrosses:
		return "CROSSES"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", int(t))
	}
}

// RangeFieldQuery is the abstract Query produced by Int/Long/Float/Double/
// InetAddress Range static factories. The concrete weight/scorer
// implementation depends on search.ConstantScoreWeight and a points-tree
// visitor; both belong to a later sprint.
//
// Sprint 21 ships the type surface (field metadata, equality, String)
// so callers (and future Range field static factories) can hold and pass
// query instances. CreateWeight remains stubbed and panics if invoked.
//
// Backlog #2696 tracks the full implementation.
type RangeFieldQuery struct {
	field   string
	ranges  []byte
	numDims int
	qType   RangeFieldQueryType
}

// NewRangeFieldQuery constructs a RangeFieldQuery for the given field and
// encoded range bytes.
func NewRangeFieldQuery(field string, ranges []byte, numDims int, qType RangeFieldQueryType) (*RangeFieldQuery, error) {
	if field == "" {
		return nil, fmt.Errorf("field name cannot be empty")
	}
	if numDims <= 0 || numDims > 4 {
		return nil, fmt.Errorf("numDims must be in [1, 4]; got %d", numDims)
	}
	if len(ranges)%(numDims*2) != 0 {
		return nil, fmt.Errorf("ranges length %d not a multiple of 2 * numDims (%d)", len(ranges), 2*numDims)
	}
	dup := make([]byte, len(ranges))
	copy(dup, ranges)
	return &RangeFieldQuery{
		field:   field,
		ranges:  dup,
		numDims: numDims,
		qType:   qType,
	}, nil
}

// Field returns the target field name.
func (q *RangeFieldQuery) Field() string { return q.field }

// QueryType returns the spatial relation type.
func (q *RangeFieldQuery) QueryType() RangeFieldQueryType { return q.qType }

// NumDims returns the number of dimensions per range.
func (q *RangeFieldQuery) NumDims() int { return q.numDims }

// Ranges returns a defensive copy of the encoded ranges payload.
func (q *RangeFieldQuery) Ranges() []byte {
	out := make([]byte, len(q.ranges))
	copy(out, q.ranges)
	return out
}

// String returns a human-readable representation matching Lucene's
// toString shape.
func (q *RangeFieldQuery) String() string {
	return fmt.Sprintf("RangeFieldQuery(field=%s, type=%s, numDims=%d)", q.field, q.qType, q.numDims)
}

// Equals reports whether two RangeFieldQuery instances target the same
// field, type, dimensionality and byte payload.
func (q *RangeFieldQuery) Equals(other *RangeFieldQuery) bool {
	if q == other {
		return true
	}
	if q == nil || other == nil {
		return false
	}
	if q.field != other.field || q.qType != other.qType || q.numDims != other.numDims {
		return false
	}
	if len(q.ranges) != len(other.ranges) {
		return false
	}
	for i := range q.ranges {
		if q.ranges[i] != other.ranges[i] {
			return false
		}
	}
	return true
}
