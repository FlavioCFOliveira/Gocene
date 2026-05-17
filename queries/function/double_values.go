// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package function

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// DoubleValues is the Go mirror of org.apache.lucene.search.DoubleValues —
// a per-doc iterator that exposes a float64 value and an AdvanceExact
// predicate to gate access. Implementations are not safe for concurrent
// use; callers obtain a fresh DoubleValues per leaf via
// [DoubleValuesSource.GetValues].
//
// Gocene deviation: the canonical search.DoubleValuesSource type currently
// ships a struct-shaped stub that returns []float64. Rather than rewrite
// it and break downstream callers (see feedback-gocene-store-bc), the
// Lucene-faithful iterator contract lives in this package and queries-
// layer code consumes it directly.
type DoubleValues interface {
	// DoubleValue returns the value for the current doc. Only valid after
	// AdvanceExact returned true for the same doc.
	DoubleValue() (float64, error)
	// AdvanceExact positions the iterator on doc, returning true if a
	// value exists.
	AdvanceExact(doc int) (bool, error)
}

// EmptyDoubleValues yields no values for any document.
var EmptyDoubleValues DoubleValues = emptyDoubleValues{}

type emptyDoubleValues struct{}

func (emptyDoubleValues) DoubleValue() (float64, error)    { return 0, nil }
func (emptyDoubleValues) AdvanceExact(_ int) (bool, error) { return false, nil }

// DoubleValuesWithDefault wraps in so that AdvanceExact always returns
// true; documents without a real value get the supplied default.
//
// Mirrors DoubleValues.withDefault.
func DoubleValuesWithDefault(in DoubleValues, defaultValue float64) DoubleValues {
	return &defaultDoubleValues{in: in, def: defaultValue}
}

type defaultDoubleValues struct {
	in       DoubleValues
	def      float64
	hasValue bool
}

func (d *defaultDoubleValues) DoubleValue() (float64, error) {
	if d.hasValue {
		return d.in.DoubleValue()
	}
	return d.def, nil
}

func (d *defaultDoubleValues) AdvanceExact(doc int) (bool, error) {
	ok, err := d.in.AdvanceExact(doc)
	if err != nil {
		return false, err
	}
	d.hasValue = ok
	return true, nil
}

// LongValues is the Go mirror of org.apache.lucene.search.LongValues.
// Same shape as [DoubleValues] but yields int64 values.
type LongValues interface {
	LongValue() (int64, error)
	AdvanceExact(doc int) (bool, error)
}

// EmptyLongValues yields no values for any document.
var EmptyLongValues LongValues = emptyLongValues{}

type emptyLongValues struct{}

func (emptyLongValues) LongValue() (int64, error)        { return 0, nil }
func (emptyLongValues) AdvanceExact(_ int) (bool, error) { return false, nil }

// DoubleValuesSource is the Go mirror of
// org.apache.lucene.search.DoubleValuesSource. Implementations are
// MT-safe and lightweight enough to act as cache keys.
type DoubleValuesSource interface {
	// GetValues returns the per-doc DoubleValues for ctx, optionally
	// providing an upstream scorer view (for sources that surface the
	// query score, e.g. via DoubleValuesSource.fromScorer).
	GetValues(ctx *index.LeafReaderContext, scores DoubleValues) (DoubleValues, error)
	// NeedsScores reports whether this source consumes the query score.
	NeedsScores() bool
	// IsCacheable reports whether results can be cached for ctx.
	IsCacheable(ctx *index.LeafReaderContext) bool
	// Rewrite returns a (possibly equivalent) source bound to searcher.
	Rewrite(searcher *search.IndexSearcher) (DoubleValuesSource, error)
	// Equals reports value equality.
	Equals(other DoubleValuesSource) bool
	// HashCode returns a stable hash agreeing with Equals.
	HashCode() int32
	// Description renders a human-readable representation.
	Description() string
	// Explain returns a leaf-level explanation for doc, given an upstream
	// score explanation.
	Explain(ctx *index.LeafReaderContext, doc int, scoreExplanation search.Explanation) (search.Explanation, error)
}

// LongValuesSource is the Go mirror of
// org.apache.lucene.search.LongValuesSource. Same shape as
// [DoubleValuesSource] but yields LongValues per leaf.
type LongValuesSource interface {
	GetValues(ctx *index.LeafReaderContext, scores DoubleValues) (LongValues, error)
	NeedsScores() bool
	IsCacheable(ctx *index.LeafReaderContext) bool
	Rewrite(searcher *search.IndexSearcher) (LongValuesSource, error)
	Equals(other LongValuesSource) bool
	HashCode() int32
	Description() string
}

// ConstantDoubleValuesSource returns a source whose every doc yields value.
// Mirrors DoubleValuesSource.constant in spirit; the result is cacheable
// and does not consume scores.
func ConstantDoubleValuesSource(value float64, description string) DoubleValuesSource {
	return &constantDoubleValuesSource{value: value, description: description}
}

type constantDoubleValuesSource struct {
	value       float64
	description string
}

func (s *constantDoubleValuesSource) GetValues(_ *index.LeafReaderContext, _ DoubleValues) (DoubleValues, error) {
	return &constantDoubleValues{value: s.value}, nil
}

func (s *constantDoubleValuesSource) NeedsScores() bool                           { return false }
func (s *constantDoubleValuesSource) IsCacheable(_ *index.LeafReaderContext) bool { return true }
func (s *constantDoubleValuesSource) Rewrite(_ *search.IndexSearcher) (DoubleValuesSource, error) {
	return s, nil
}
func (s *constantDoubleValuesSource) Equals(other DoubleValuesSource) bool {
	o, ok := other.(*constantDoubleValuesSource)
	if !ok || o == nil {
		return false
	}
	return s.value == o.value && s.description == o.description
}
func (s *constantDoubleValuesSource) HashCode() int32 {
	return hashFloat64(s.value) ^ hashString(s.description)
}
func (s *constantDoubleValuesSource) Description() string { return s.description }
func (s *constantDoubleValuesSource) Explain(_ *index.LeafReaderContext, _ int, _ search.Explanation) (search.Explanation, error) {
	return search.NewExplanation(true, float32(s.value), s.description), nil
}

type constantDoubleValues struct {
	value float64
}

func (c *constantDoubleValues) DoubleValue() (float64, error)    { return c.value, nil }
func (c *constantDoubleValues) AdvanceExact(_ int) (bool, error) { return true, nil }
