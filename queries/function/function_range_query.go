// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package function

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// FunctionRangeQuery wraps a [ValueSource] and matches documents whose
// values fall in the configured (lower, upper) range. The score is the
// raw FloatVal. It is the Go port of
// org.apache.lucene.queries.function.FunctionRangeQuery.
type FunctionRangeQuery struct {
	valueSource  ValueSource
	lowerVal     string // empty string mirrors Java null (==> -Inf)
	upperVal     string // empty string mirrors Java null (==> +Inf)
	includeLower bool
	includeUpper bool
	lowerSet     bool // distinguishes explicit "" bound vs missing bound
	upperSet     bool
}

// NewFunctionRangeQuery returns a FunctionRangeQuery with explicit string
// bounds. Use the empty string + lowerSet=false / upperSet=false (via
// [NewFunctionRangeQueryUnbounded]) to express open ends.
func NewFunctionRangeQuery(valueSource ValueSource, lowerVal, upperVal string, includeLower, includeUpper bool) *FunctionRangeQuery {
	return &FunctionRangeQuery{
		valueSource:  valueSource,
		lowerVal:     lowerVal,
		upperVal:     upperVal,
		includeLower: includeLower,
		includeUpper: includeUpper,
		lowerSet:     true,
		upperSet:     true,
	}
}

// NewFunctionRangeQueryUnbounded returns a FunctionRangeQuery whose
// bound semantics follow the explicit flags: a "set" bound uses the
// provided string; an "unset" bound is treated as ±Inf, matching the
// Java constructor where a null Number expands to a null String.
func NewFunctionRangeQueryUnbounded(valueSource ValueSource, lowerVal string, lowerSet bool, upperVal string, upperSet bool, includeLower, includeUpper bool) *FunctionRangeQuery {
	return &FunctionRangeQuery{
		valueSource:  valueSource,
		lowerVal:     lowerVal,
		upperVal:     upperVal,
		includeLower: includeLower,
		includeUpper: includeUpper,
		lowerSet:     lowerSet,
		upperSet:     upperSet,
	}
}

// GetValueSource returns the wrapped ValueSource.
func (q *FunctionRangeQuery) GetValueSource() ValueSource { return q.valueSource }

// GetLowerVal returns the lower bound as text.
func (q *FunctionRangeQuery) GetLowerVal() string { return q.lowerVal }

// GetUpperVal returns the upper bound as text.
func (q *FunctionRangeQuery) GetUpperVal() string { return q.upperVal }

// IsIncludeLower reports whether the lower bound is inclusive.
func (q *FunctionRangeQuery) IsIncludeLower() bool { return q.includeLower }

// IsIncludeUpper reports whether the upper bound is inclusive.
func (q *FunctionRangeQuery) IsIncludeUpper() bool { return q.includeUpper }

// String renders the canonical frange(...):[lo TO hi] / {lo TO hi} form.
func (q *FunctionRangeQuery) String() string {
	var b strings.Builder
	b.WriteString("frange(")
	b.WriteString(q.valueSource.Description())
	b.WriteString("):")
	if q.includeLower {
		b.WriteByte('[')
	} else {
		b.WriteByte('{')
	}
	if !q.lowerSet {
		b.WriteByte('*')
	} else {
		b.WriteString(q.lowerVal)
	}
	b.WriteString(" TO ")
	if !q.upperSet {
		b.WriteByte('*')
	} else {
		b.WriteString(q.upperVal)
	}
	if q.includeUpper {
		b.WriteByte(']')
	} else {
		b.WriteByte('}')
	}
	return b.String()
}

// Equals checks value-equality with another query.
func (q *FunctionRangeQuery) Equals(other search.Query) bool {
	o, ok := other.(*FunctionRangeQuery)
	if !ok || o == nil {
		return false
	}
	return q.includeLower == o.includeLower &&
		q.includeUpper == o.includeUpper &&
		q.lowerSet == o.lowerSet && q.upperSet == o.upperSet &&
		q.lowerVal == o.lowerVal && q.upperVal == o.upperVal &&
		q.valueSource.Equals(o.valueSource)
}

// HashCode mirrors the Java classHash() ^ Objects.hash(...) layout.
func (q *FunctionRangeQuery) HashCode() int {
	h := int(q.valueSource.HashCode())
	h = int(combineHash(int32(h), hashString(q.lowerVal)))
	h = int(combineHash(int32(h), hashString(q.upperVal)))
	h = int(combineHash(int32(h), hashBool(q.includeLower)))
	h = int(combineHash(int32(h), hashBool(q.includeUpper)))
	h = int(combineHash(int32(h), hashBool(q.lowerSet)))
	h = int(combineHash(int32(h), hashBool(q.upperSet)))
	return h ^ 0x46_72_61_6e // "Fran" magic
}

// Clone returns a defensive copy.
func (q *FunctionRangeQuery) Clone() search.Query {
	cp := *q
	return &cp
}

// Rewrite returns the query itself.
func (q *FunctionRangeQuery) Rewrite(_ search.IndexReader) (search.Query, error) { return q, nil }

// Visit invokes the leaf hook on the visitor.
func (q *FunctionRangeQuery) Visit(visitor search.QueryVisitor) { visitor.VisitLeaf(q) }

// CreateWeight returns a Weight that scores documents via the wrapped
// ValueSource and gates them through the configured range.
func (q *FunctionRangeQuery) CreateWeight(searcher *search.IndexSearcher, _ bool, _ float32) (search.Weight, error) {
	w := &functionRangeWeight{
		BaseWeight: search.NewBaseWeight(q),
		query:      q,
		searcher:   searcher,
		ctx:        NewContext(),
	}
	w.ctx.Put(SearcherKey, searcher)
	if err := q.valueSource.CreateWeight(w.ctx, searcher); err != nil {
		return nil, err
	}
	return w, nil
}

// functionRangeWeight is the search.Weight implementation for FunctionRangeQuery.
type functionRangeWeight struct {
	*search.BaseWeight
	query    *FunctionRangeQuery
	searcher *search.IndexSearcher
	ctx      Context
}

// IsCacheable mirrors Lucene's false default.
func (w *functionRangeWeight) IsCacheable(_ *index.LeafReaderContext) bool { return false }

// Explain wires the underlying FunctionValues explanation into a match /
// no-match envelope.
func (w *functionRangeWeight) Explain(ctx *index.LeafReaderContext, doc int) (search.Explanation, error) {
	vals, err := w.query.valueSource.GetValues(w.ctx, ctx)
	if err != nil {
		return nil, err
	}
	scorer, err := w.makeRangeScorer(vals, ctx)
	if err != nil {
		return nil, err
	}
	matched, err := scorer.Matches(doc)
	if err != nil {
		return nil, err
	}
	leafExpl, expErr := vals.Explain(doc)
	if expErr != nil {
		return nil, expErr
	}
	if !matched {
		root := search.NoMatchExplanation(w.query.String())
		root.AddDetail(search.NewExplanation(true, 0, leafExpl))
		return root, nil
	}
	score, err := scorer.Score(doc)
	if err != nil {
		return nil, err
	}
	root := search.MatchExplanation(score, w.query.String())
	root.AddDetail(search.NewExplanation(true, score, leafExpl))
	return root, nil
}

// Scorer returns the per-leaf range scorer adapted to search.Scorer.
func (w *functionRangeWeight) Scorer(ctx *index.LeafReaderContext) (search.Scorer, error) {
	vals, err := w.query.valueSource.GetValues(w.ctx, ctx)
	if err != nil {
		return nil, err
	}
	scorer, err := w.makeRangeScorer(vals, ctx)
	if err != nil {
		return nil, err
	}
	return newRangeScorerAdapter(scorer), nil
}

func (w *functionRangeWeight) ScorerSupplier(ctx *index.LeafReaderContext) (search.ScorerSupplier, error) {
	scorer, err := w.Scorer(ctx)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return search.NewScorerSupplierAdapter(scorer), nil
}

func (w *functionRangeWeight) BulkScorer(ctx *index.LeafReaderContext) (search.BulkScorer, error) {
	scorer, err := w.Scorer(ctx)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return search.NewDefaultBulkScorer(scorer), nil
}

func (w *functionRangeWeight) Matches(_ *index.LeafReaderContext, _ int) (search.Matches, error) {
	return nil, nil
}

func (w *functionRangeWeight) Count(_ *index.LeafReaderContext) (int, error) { return -1, nil }

// makeRangeScorer resolves textual bounds + range flags to a typed scorer.
func (w *functionRangeWeight) makeRangeScorer(vals FunctionValues, leaf *index.LeafReaderContext) (ValueSourceScorer, error) {
	lo := w.query.lowerVal
	hi := w.query.upperVal
	if !w.query.lowerSet {
		lo = ""
	}
	if !w.query.upperSet {
		hi = ""
	}
	scorer, err := vals.GetRangeScorer(leaf, lo, hi, w.query.includeLower, w.query.includeUpper)
	if err != nil {
		return nil, fmt.Errorf("function range scorer: %w", err)
	}
	return scorer, nil
}

// rangeScorerAdapter exposes ValueSourceScorer through the search.Scorer
// contract. It iterates docs in order, invoking Matches per doc.
type rangeScorerAdapter struct {
	scorer ValueSourceScorer
	iter   *search.RangeDocIdSetIterator
	doc    int
	cached float32
}

func newRangeScorerAdapter(scorer ValueSourceScorer) *rangeScorerAdapter {
	return &rangeScorerAdapter{
		scorer: scorer,
		iter:   search.NewRangeDocIdSetIterator(0, scorer.MaxDoc()),
		doc:    -1,
	}
}

func (a *rangeScorerAdapter) DocID() int { return a.doc }

func (a *rangeScorerAdapter) NextDoc() (int, error) {
	for {
		doc, err := a.iter.NextDoc()
		if err != nil {
			return search.NO_MORE_DOCS, err
		}
		if doc == search.NO_MORE_DOCS {
			a.doc = doc
			return doc, nil
		}
		ok, err := a.scorer.Matches(doc)
		if err != nil {
			return search.NO_MORE_DOCS, err
		}
		if ok {
			a.doc = doc
			return doc, nil
		}
	}
}

func (a *rangeScorerAdapter) Advance(target int) (int, error) {
	doc, err := a.iter.Advance(target)
	if err != nil {
		return search.NO_MORE_DOCS, err
	}
	if doc == search.NO_MORE_DOCS {
		a.doc = doc
		return doc, nil
	}
	ok, err := a.scorer.Matches(doc)
	if err != nil {
		return search.NO_MORE_DOCS, err
	}
	if ok {
		a.doc = doc
		return doc, nil
	}
	return a.NextDoc()
}

func (a *rangeScorerAdapter) Cost() int64      { return a.iter.Cost() }
func (a *rangeScorerAdapter) DocIDRunEnd() int { return a.doc + 1 }
func (a *rangeScorerAdapter) GetMaxScore(_ int) float32 {
	return a.scorer.MaxScore(0)
}

func (a *rangeScorerAdapter) Score() float32 {
	score, err := a.scorer.Score(a.doc)
	if err != nil {
		return 0
	}
	a.cached = score
	return score
}

var _ search.Query = (*FunctionRangeQuery)(nil)
var _ search.Weight = (*functionRangeWeight)(nil)
var _ search.Scorer = (*rangeScorerAdapter)(nil)
