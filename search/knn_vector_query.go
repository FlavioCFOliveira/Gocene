// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "fmt"

// KnnFloatVectorQuery searches for the k documents whose float vector for the
// given field is most similar to the target vector. An optional pre-filter and
// search-strategy hook mirror Lucene's KnnFloatVectorQuery.
//
// Mirrors org.apache.lucene.search.KnnFloatVectorQuery. The full HNSW-driven
// scorer pipeline lives in codecs/hnsw and util/hnsw (Sprint 19/25); this
// query carries the public surface and dispatches to the codec-side scorer at
// CreateWeight time.
type KnnFloatVectorQuery struct {
	BaseQuery
	field    string
	target   []float32
	k        int
	filter   Query
	strategy KnnSearchStrategy
}

// NewKnnFloatVectorQuery builds the simplest form (no filter, default strategy).
func NewKnnFloatVectorQuery(field string, target []float32, k int) *KnnFloatVectorQuery {
	return &KnnFloatVectorQuery{field: field, target: target, k: k}
}

// NewKnnFloatVectorQueryWithFilter is the variant that pre-filters the
// candidate set with filter.
func NewKnnFloatVectorQueryWithFilter(field string, target []float32, k int, filter Query) *KnnFloatVectorQuery {
	return &KnnFloatVectorQuery{field: field, target: target, k: k, filter: filter}
}

// NewKnnFloatVectorQueryWithStrategy is the variant accepting a custom
// KnnSearchStrategy.
func NewKnnFloatVectorQueryWithStrategy(field string, target []float32, k int, filter Query, strategy KnnSearchStrategy) *KnnFloatVectorQuery {
	return &KnnFloatVectorQuery{field: field, target: target, k: k, filter: filter, strategy: strategy}
}

// GetField returns the vector field name.
func (q *KnnFloatVectorQuery) GetField() string { return q.field }

// GetTargetCopy returns a defensive copy of the target vector.
func (q *KnnFloatVectorQuery) GetTargetCopy() []float32 {
	return append([]float32(nil), q.target...)
}

// K returns the requested number of neighbours.
func (q *KnnFloatVectorQuery) K() int { return q.k }

// Filter returns the optional pre-filter query (may be nil).
func (q *KnnFloatVectorQuery) Filter() Query { return q.filter }

// Strategy returns the optional KnnSearchStrategy.
func (q *KnnFloatVectorQuery) Strategy() KnnSearchStrategy { return q.strategy }

// String returns a debug representation.
func (q *KnnFloatVectorQuery) String() string {
	return fmt.Sprintf("KnnFloatVectorQuery(field=%s, k=%d, dim=%d)", q.field, q.k, len(q.target))
}

// Equals checks structural equality.
func (q *KnnFloatVectorQuery) Equals(other Query) bool {
	o, ok := other.(*KnnFloatVectorQuery)
	if !ok || q.field != o.field || q.k != o.k || len(q.target) != len(o.target) {
		return false
	}
	for i := range q.target {
		if q.target[i] != o.target[i] {
			return false
		}
	}
	if (q.filter == nil) != (o.filter == nil) {
		return false
	}
	if q.filter != nil && !q.filter.Equals(o.filter) {
		return false
	}
	return true
}

// HashCode hashes the field, k, and vector contents.
func (q *KnnFloatVectorQuery) HashCode() int {
	h := 17
	for i := 0; i < len(q.field); i++ {
		h = 31*h + int(q.field[i])
	}
	h = 31*h + q.k
	for _, v := range q.target {
		h = 31*h + int(v)
	}
	if q.filter != nil {
		h = 31*h + q.filter.HashCode()
	}
	return h
}

// Clone returns an independent copy.
func (q *KnnFloatVectorQuery) Clone() Query {
	cp := *q
	cp.target = append([]float32(nil), q.target...)
	if q.filter != nil {
		cp.filter = q.filter.Clone()
	}
	return &cp
}

// KnnByteVectorQuery is the byte-vector analogue of KnnFloatVectorQuery.
//
// Mirrors org.apache.lucene.search.KnnByteVectorQuery.
type KnnByteVectorQuery struct {
	BaseQuery
	field    string
	target   []byte
	k        int
	filter   Query
	strategy KnnSearchStrategy
}

// NewKnnByteVectorQuery is the simplest form.
func NewKnnByteVectorQuery(field string, target []byte, k int) *KnnByteVectorQuery {
	return &KnnByteVectorQuery{field: field, target: target, k: k}
}

// NewKnnByteVectorQueryWithFilter is the filter variant.
func NewKnnByteVectorQueryWithFilter(field string, target []byte, k int, filter Query) *KnnByteVectorQuery {
	return &KnnByteVectorQuery{field: field, target: target, k: k, filter: filter}
}

// NewKnnByteVectorQueryWithStrategy is the variant taking a custom strategy.
func NewKnnByteVectorQueryWithStrategy(field string, target []byte, k int, filter Query, strategy KnnSearchStrategy) *KnnByteVectorQuery {
	return &KnnByteVectorQuery{field: field, target: target, k: k, filter: filter, strategy: strategy}
}

// GetField returns the field name.
func (q *KnnByteVectorQuery) GetField() string { return q.field }

// GetTargetCopy returns a defensive copy.
func (q *KnnByteVectorQuery) GetTargetCopy() []byte { return append([]byte(nil), q.target...) }

// K returns the requested number of neighbours.
func (q *KnnByteVectorQuery) K() int { return q.k }

// Filter returns the pre-filter query (may be nil).
func (q *KnnByteVectorQuery) Filter() Query { return q.filter }

// Strategy returns the configured KnnSearchStrategy.
func (q *KnnByteVectorQuery) Strategy() KnnSearchStrategy { return q.strategy }

// String returns a debug representation.
func (q *KnnByteVectorQuery) String() string {
	return fmt.Sprintf("KnnByteVectorQuery(field=%s, k=%d, dim=%d)", q.field, q.k, len(q.target))
}

// Equals checks structural equality.
func (q *KnnByteVectorQuery) Equals(other Query) bool {
	o, ok := other.(*KnnByteVectorQuery)
	if !ok || q.field != o.field || q.k != o.k || !bytesEqual(q.target, o.target) {
		return false
	}
	if (q.filter == nil) != (o.filter == nil) {
		return false
	}
	if q.filter != nil && !q.filter.Equals(o.filter) {
		return false
	}
	return true
}

// HashCode hashes the field, k, and vector contents.
func (q *KnnByteVectorQuery) HashCode() int {
	h := 17
	for i := 0; i < len(q.field); i++ {
		h = 31*h + int(q.field[i])
	}
	h = 31*h + q.k
	for _, b := range q.target {
		h = 31*h + int(b)
	}
	if q.filter != nil {
		h = 31*h + q.filter.HashCode()
	}
	return h
}

// Clone returns an independent copy.
func (q *KnnByteVectorQuery) Clone() Query {
	cp := *q
	cp.target = append([]byte(nil), q.target...)
	if q.filter != nil {
		cp.filter = q.filter.Clone()
	}
	return &cp
}

// KnnSearchStrategy is the search-strategy hook surfaced by the
// KnnFloat/Byte vector queries. The concrete strategies live in
// search/knn (Sprint 25).
type KnnSearchStrategy interface {
	// StrategyName returns the canonical strategy name used in toString.
	StrategyName() string
}
