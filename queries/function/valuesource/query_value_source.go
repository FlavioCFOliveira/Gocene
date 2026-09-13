// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/queries/function/docvalues"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// QueryValueSource returns the relevance score of the query.
//
// Mirrors org.apache.lucene.queries.function.valuesource.QueryValueSource.
type QueryValueSource struct {
	function.BaseValueSource
	q      search.Query
	defVal float32
}

var _ function.ValueSource = (*QueryValueSource)(nil)

// NewQueryValueSource mirrors QueryValueSource(Query, float), which rejects a
// nil query.
func NewQueryValueSource(q search.Query, defVal float32) (*QueryValueSource, error) {
	if q == nil {
		return nil, fmt.Errorf("query cannot be null")
	}
	return &QueryValueSource{q: q, defVal: defVal}, nil
}

// GetQuery mirrors getQuery().
func (v *QueryValueSource) GetQuery() search.Query { return v.q }

// GetDefaultValue mirrors getDefaultValue().
func (v *QueryValueSource) GetDefaultValue() float32 { return v.defVal }

// Description mirrors description().
func (v *QueryValueSource) Description() string {
	return fmt.Sprintf("query(%v,def=%v)", v.q, v.defVal)
}

// GetValues mirrors getValues(Map, LeafReaderContext).
func (v *QueryValueSource) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	return newQueryDocValues(v, readerContext, ctx)
}

// HashCode mirrors q.hashCode() * 29.
func (v *QueryValueSource) HashCode() int32 { return int32(v.q.HashCode()) * 29 }

// Equals mirrors equals(Object).
func (v *QueryValueSource) Equals(other function.ValueSource) bool {
	o, ok := other.(*QueryValueSource)
	if !ok {
		return false
	}
	return v.q.Equals(o.q) && v.defVal == o.defVal
}

// CreateWeight mirrors createWeight(Map, IndexSearcher).
func (v *QueryValueSource) CreateWeight(ctx function.Context, searcher any) error {
	s, ok := searcher.(*search.IndexSearcher)
	if !ok || s == nil {
		return fmt.Errorf("valuesource: query() requires an *search.IndexSearcher")
	}
	rewritten, err := s.Rewrite(v.q)
	if err != nil {
		return err
	}
	w, err := s.CreateWeight(rewritten, search.COMPLETE, 1)
	if err != nil {
		return err
	}
	ctx[v] = w
	return nil
}

// queryDocValues renders the package-private class QueryDocValues that
// QueryValueSource.java declares alongside it.
type queryDocValues struct {
	*docvalues.FloatDocValues
	readerContext *index.LeafReaderContext
	weight        search.Weight
	defVal        float32
	fcontext      function.Context
	q             search.Query

	scorer         search.Scorer
	disi           search.DocIdSetIterator
	tpi            *search.TwoPhaseIterator
	thisDocMatches *bool

	// lastDocRequested is the last document requested.
	lastDocRequested int
}

func newQueryDocValues(vs *QueryValueSource, readerContext *index.LeafReaderContext, fcontext function.Context) (*queryDocValues, error) {
	q := &queryDocValues{
		readerContext:    readerContext,
		defVal:           vs.defVal,
		q:                vs.q,
		fcontext:         fcontext,
		lastDocRequested: -1,
	}

	var w search.Weight
	if fcontext != nil {
		w, _ = fcontext[vs].(search.Weight)
	}
	if w == nil {
		var weightSearcher *search.IndexSearcher
		if fcontext != nil {
			weightSearcher, _ = fcontext["searcher"].(*search.IndexSearcher)
		}
		if weightSearcher == nil {
			return nil, fmt.Errorf("valuesource: query() requires a searcher in the context")
		}
		if fcontext == nil {
			return nil, fmt.Errorf("valuesource: query() requires a non-nil context")
		}
		if err := vs.CreateWeight(fcontext, weightSearcher); err != nil {
			return nil, err
		}
		w, _ = fcontext[vs].(search.Weight)
	}
	q.weight = w
	q.FloatDocValues = docvalues.NewFloatDocValues(vs, q.floatVal)
	q.SetSelf(q)
	return q, nil
}

// floatVal mirrors floatVal(int).
func (v *queryDocValues) floatVal(doc int) (float32, error) {
	ok, err := v.Exists(doc)
	if err != nil {
		return 0, err
	}
	if !ok {
		return v.defVal, nil
	}
	return v.scorer.Score()
}

// Exists mirrors exists(int).
func (v *queryDocValues) Exists(doc int) (bool, error) {
	if doc < v.lastDocRequested {
		return false, fmt.Errorf("docs were sent out-of-order: lastDocID=%d vs docID=%d", v.lastDocRequested, doc)
	}
	v.lastDocRequested = doc

	if v.disi == nil {
		scorer, err := v.weight.Scorer(v.readerContext)
		if err != nil {
			return false, err
		}
		v.scorer = scorer
		if scorer == nil {
			v.disi = search.Empty()
		} else {
			v.tpi = scorer.TwoPhaseIterator()
			if v.tpi == nil {
				v.disi = scorer.Iterator()
			} else {
				v.disi = v.tpi.Approximation()
			}
		}
		v.thisDocMatches = nil
	}

	if v.disi.DocID() < doc {
		if _, err := v.disi.Advance(doc); err != nil {
			return false, err
		}
		v.thisDocMatches = nil
	}
	if v.disi.DocID() == doc {
		if v.thisDocMatches == nil {
			matches := true
			if v.tpi != nil {
				var err error
				matches, err = v.tpi.Matches()
				if err != nil {
					return false, err
				}
			}
			v.thisDocMatches = &matches
		}
		return *v.thisDocMatches, nil
	}
	return false, nil
}

// ObjectVal mirrors objectVal(int).
func (v *queryDocValues) ObjectVal(doc int) (any, error) { return v.floatVal(doc) }

// ToString mirrors toString(int).
func (v *queryDocValues) ToString(doc int) (string, error) {
	val, err := v.floatVal(doc)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("query(%v,def=%v)=%v", v.q, v.defVal, val), nil
}
