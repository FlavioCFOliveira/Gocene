// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// DoubleValues provides double values for use in queries and sorting.
type DoubleValues interface {
	// DoubleValue returns the double value for the current document.
	DoubleValue() (float64, error)
	// AdvanceExact positions the reader on doc and reports whether the document is present.
	AdvanceExact(doc int) (bool, error)
}

// DoubleValuesSource provides double values for use in queries and sorting.
// Mirrors org.apache.lucene.search.DoubleValuesSource.
type DoubleValuesSource interface {
	// GetValues returns the double values from the given context.
	GetValues(ctx *index.LeafReaderContext, scores DoubleValues) (DoubleValues, error)
	// Field returns the underlying field name.
	Field() string
	// NeedsScores reports whether the source consumes the underlying query's scores.
	NeedsScores() bool
	// IsCacheable reports whether results are safe to cache for this leaf.
	IsCacheable(ctx *index.LeafReaderContext) bool
	// Rewrite returns the rewritten source.
	Rewrite(searcher *IndexSearcher) DoubleValuesSource
}

// numericProvider is the narrow capability required from a context argument:
// the ability to obtain a NumericDocValues iterator for a field.
//
// PORT NOTE: Gocene-only. Apache Lucene 10.5.0 calls
// DocValues.getNumeric(ctx.reader(), field) directly; this indirection exists
// only because basicDoubleValuesSource accepts context shapes that do not
// implement spi.LeafReader.
type numericProvider interface {
	GetNumericDocValues(field string) (index.NumericDocValues, error)
}

// numericProviderFromContext extracts a NumericDocValues iterator from a
// context argument. Accepted shapes:
//
//   - *index.LeafReaderContext (unwraps via LeafReader())
//   - any type exposing GetNumericDocValues(field string)
//
// Returns a nil iterator and no error when no provider can be located.
func numericProviderFromContext(ctx interface{}, field string) (index.NumericDocValues, error) {
	if ctx == nil {
		return nil, nil
	}
	switch v := ctx.(type) {
	case *index.LeafReaderContext:
		if v == nil {
			return nil, nil
		}
		reader := v.LeafReader()
		if reader == nil {
			return nil, nil
		}
		if np, ok := interface{}(reader).(numericProvider); ok {
			return np.GetNumericDocValues(field)
		}
		return nil, nil
	case numericProvider:
		return v.GetNumericDocValues(field)
	default:
		return nil, nil
	}
}

// basicDoubleValuesSource is the default implementation of DoubleValuesSource.
type basicDoubleValuesSource struct {
	field string
}

// NewDoubleValuesSource creates a new DoubleValuesSource.
func NewDoubleValuesSource(field string) DoubleValuesSource {
	return &basicDoubleValuesSource{field: field}
}

func (s *basicDoubleValuesSource) Field() string { return s.field }

func (s *basicDoubleValuesSource) GetValues(ctx *index.LeafReaderContext, scores DoubleValues) (DoubleValues, error) {
	_ = scores
	if ctx == nil {
		return nil, fmt.Errorf("leaf reader context must not be nil")
	}
	reader := ctx.LeafReader()
	if reader == nil {
		return nil, fmt.Errorf("leaf reader must not be nil")
	}

	dv, err := numericProviderFromContext(ctx, s.field)
	if err != nil {
		return nil, err
	}
	if dv == nil {
		return nil, nil
	}

	return &basicDoubleValues{dv: dv}, nil
}

func (s *basicDoubleValuesSource) NeedsScores() bool { return false }

func (s *basicDoubleValuesSource) IsCacheable(ctx *index.LeafReaderContext) bool { return true }

func (s *basicDoubleValuesSource) Rewrite(searcher *IndexSearcher) DoubleValuesSource {
	return s
}

type basicDoubleValues struct {
	dv index.NumericDocValues
}

func (v *basicDoubleValues) DoubleValue() (float64, error) {
	val, err := v.dv.LongValue()
	if err != nil {
		return 0, err
	}
	return math.Float64frombits(uint64(val)), nil
}

func (v *basicDoubleValues) AdvanceExact(doc int) (bool, error) {
	currentDoc := v.dv.DocID()
	if doc < currentDoc {
		return false, nil
	}
	if currentDoc == doc {
		return true, nil
	}
	next, err := v.dv.Advance(doc)
	if err != nil {
		return false, err
	}
	return next == doc, nil
}

// DoubleValuesSourceFromScorer returns a DoubleValues instance that wraps
// scores returned by a Scorer.
//
// Mirrors the static org.apache.lucene.search.DoubleValuesSource#fromScorer
// (Apache Lucene 10.5.0). It is a free function because Go has no static
// methods on an interface type.
func DoubleValuesSourceFromScorer(scorer Scorable) DoubleValues {
	return &fromScorerDoubleValues{scorer: scorer}
}

// fromScorerDoubleValues renders the anonymous DoubleValues returned by
// DoubleValuesSource.fromScorer(Scorable), whose advanceExact always returns
// true.
//
// PORT NOTE: search/rescore_top_n_query.go carries an unrelated local
// scorerDoubleValues whose AdvanceExact instead reports scorer.DocID() == doc;
// it is not this Lucene member and is left untouched.
type fromScorerDoubleValues struct {
	scorer Scorable
}

func (v *fromScorerDoubleValues) DoubleValue() (float64, error) {
	score, err := v.scorer.Score()
	if err != nil {
		return 0, err
	}
	return float64(score), nil
}

func (v *fromScorerDoubleValues) AdvanceExact(doc int) (bool, error) {
	return true, nil
}
