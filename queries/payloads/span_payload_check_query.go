// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/payloads/SpanPayloadCheckQuery.java

package payloads

import (
	"fmt"
	"github.com/FlavioCFOliveira/Gocene/spi"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/spans"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SpanPayloadCheckQuery only returns those matches that have a specific payload
// at the given position.
//
// Mirrors org.apache.lucene.queries.payloads.SpanPayloadCheckQuery.
type SpanPayloadCheckQuery struct {
	search.BaseQuery
	match          spans.SpanQuery
	payloadToMatch []*util.BytesRef
	payloadType    PayloadType
	operation      MatchOperation
}

// NewSpanPayloadCheckQuery creates a SpanPayloadCheckQuery with STRING type
// and EQ operation (default).
func NewSpanPayloadCheckQuery(match spans.SpanQuery, payloadToMatch []*util.BytesRef) *SpanPayloadCheckQuery {
	return NewSpanPayloadCheckQueryWithType(match, payloadToMatch, PayloadTypeSTRING, MatchOperationEQ)
}

// NewSpanPayloadCheckQueryWithType creates a SpanPayloadCheckQuery with the
// given payload type and match operation.
func NewSpanPayloadCheckQueryWithType(match spans.SpanQuery, payloadToMatch []*util.BytesRef,
	payloadType PayloadType, operation MatchOperation) *SpanPayloadCheckQuery {
	return &SpanPayloadCheckQuery{
		match:          match,
		payloadToMatch: payloadToMatch,
		payloadType:    payloadType,
		operation:      operation,
	}
}

// GetField returns the field of the wrapped query.
func (q *SpanPayloadCheckQuery) GetField() string { return q.match.GetField() }

// Rewrite rewrites the inner query and returns a new SpanPayloadCheckQuery if
// the inner query changed.
func (q *SpanPayloadCheckQuery) Rewrite(searcher *search.IndexSearcher) (search.Query, error) {
	rewritten, err := q.match.Rewrite(searcher)
	if err != nil {
		return nil, err
	}
	if rewritten != q.match {
		sp, ok := rewritten.(spans.SpanQuery)
		if !ok {
			return nil, fmt.Errorf("SpanPayloadCheckQuery.Rewrite: inner rewrite returned non-SpanQuery %T", rewritten)
		}
		return NewSpanPayloadCheckQueryWithType(sp, q.payloadToMatch, q.payloadType, q.operation), nil
	}
	return q, nil
}

// Visit visits the query tree. This method is accessed via duck-type assertion
// (interface{ Visit(search.QueryVisitor) }) by callers such as IndexSearcher.
func (q *SpanPayloadCheckQuery) Visit(visitor search.QueryVisitor) {
	if visitor.AcceptField(q.match.GetField()) {
		if v, ok := q.match.(interface{ Visit(search.QueryVisitor) }); ok {
			v.Visit(visitor.GetSubVisitor(search.MUST, q))
		}
	}
}

// CreateWeight creates a Weight for this query.
func (q *SpanPayloadCheckQuery) CreateWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (search.Weight, error) {
	matchWeight, err := q.match.CreateWeight(searcher, scoreMode, boost)
	if err != nil {
		return nil, err
	}
	if !scoreMode.NeedsScores() {
		return matchWeight, nil
	}
	return &spanPayloadCheckWeight{
		BaseWeight:     search.NewBaseWeight(q),
		matchWeight:    matchWeight,
		field:          q.GetField(),
		payloadToMatch: q.payloadToMatch,
		payloadType:    q.payloadType,
		operation:      q.operation,
	}, nil
}

// CreateSpanWeight delegates to the inner span query, which is what the
// payload-check weight filters.
func (q *SpanPayloadCheckQuery) CreateSpanWeight(searcher *search.IndexSearcher, scoreMode search.ScoreMode, boost float32) (*spans.SpanWeight, error) {
	return q.match.CreateSpanWeight(searcher, scoreMode, boost)
}

// Equals returns true if other is equal to this.
func (q *SpanPayloadCheckQuery) Equals(other spi.Query) bool {
	o, ok := other.(*SpanPayloadCheckQuery)
	if !ok {
		return false
	}
	if q.operation != o.operation || q.payloadType != o.payloadType {
		return false
	}
	if !q.match.Equals(o.match) {
		return false
	}
	if len(q.payloadToMatch) != len(o.payloadToMatch) {
		return false
	}
	for i := range q.payloadToMatch {
		if !util.BytesRefEquals(q.payloadToMatch[i], o.payloadToMatch[i]) {
			return false
		}
	}
	return true
}

// HashCode returns a hash code for this query.
func (q *SpanPayloadCheckQuery) HashCode() int {
	h := 17
	for _, b := range q.GetField() {
		h = 31*h + int(b)
	}
	h = 31*h + q.match.HashCode()
	for _, br := range q.payloadToMatch {
		h = 31*h + br.HashCode()
	}
	h = 31*h + int(q.operation)
	h = 31*h + int(q.payloadType)
	return h
}

// String returns a string representation.
// ToString prints this query, with field assumed to be the default field and
// omitted.
func (q *SpanPayloadCheckQuery) ToString(field string) string { return q.String(field) }

func (q *SpanPayloadCheckQuery) String(field string) string {
	buf := "SpanPayloadCheckQuery("
	buf += q.match.ToString(field)
	buf += ", payloadRef: "
	for _, br := range q.payloadToMatch {
		buf += util.ToStringBytesRef(br)
		buf += ";"
	}
	buf += fmt.Sprintf(", payloadType:%d;", q.payloadType)
	buf += fmt.Sprintf(", operation:%d;", q.operation)
	buf += ")"
	return buf
}

// Ensure SpanPayloadCheckQuery implements spans.SpanQuery.
var _ spans.SpanQuery = (*SpanPayloadCheckQuery)(nil)

// --- Weight implementation ---

// spanPayloadCheckWeight is the Weight for SpanPayloadCheckQuery.
type spanPayloadCheckWeight struct {
	*search.BaseWeight
	matchWeight    search.Weight
	field          string
	payloadToMatch []*util.BytesRef
	payloadType    PayloadType
	operation      MatchOperation
}

// spansProvider is an interface for weights that can provide span iterators.
type spansProvider interface {
	GetSpans(ctx *index.LeafReaderContext, postings spans.Postings) (spans.Spans, error)
}

func (w *spanPayloadCheckWeight) getSpans(ctx *index.LeafReaderContext) (spans.Spans, error) {
	sp, ok := w.matchWeight.(spansProvider)
	if !ok {
		return nil, nil
	}
	return sp.GetSpans(ctx, spans.PostingsPayloads)
}

// ScorerSupplier returns a ScorerSupplier for the given context.
func (w *spanPayloadCheckWeight) ScorerSupplier(ctx *index.LeafReaderContext) (search.ScorerSupplier, error) {
	if w.field == "" {
		return nil, nil
	}

	// Get inner spans at PAYLOADS level.
	innerSpans, err := w.getSpans(ctx)
	if err != nil {
		return nil, err
	}
	if innerSpans == nil {
		return nil, nil
	}

	// Wrap in payload-checking filter.
	checker := &payloadChecker{
		payloadToMatch: w.payloadToMatch,
		payloadMatcher: CreateMatcherForOpAndType(w.payloadType, w.operation),
	}
	filtered := &payloadCheckFilterSpans{
		inner:   innerSpans,
		checker: checker,
	}

	// Get norms.
	var norms index.NumericDocValues
	if ctx != nil {
		if lr := ctx.LeafReader(); lr != nil {
			norms, _ = lr.GetNormValues(w.field)
		}
	}

	sc := newPayloadCheckScorer(filtered, nil, norms)
	return search.NewDefaultScorerSupplier(sc), nil
}

// Explain returns an explanation for the given document.
func (w *spanPayloadCheckWeight) Explain(ctx *index.LeafReaderContext, doc int) (search.Explanation, error) {
	sc, err := w.Scorer(ctx)
	if err != nil {
		return nil, err
	}
	if sc == nil {
		return search.NoMatchExplanation("no matching spans"), nil
	}
	advanced, err := sc.Iterator().Advance(doc)
	if err != nil {
		return nil, err
	}
	if advanced != doc {
		return search.NoMatchExplanation("no matching spans"), nil
	}
	score, err := sc.Score()
	if err != nil {
		return nil, err
	}
	return search.MatchExplanation(score, "SpanPayloadCheckQuery match"), nil
}

// IsCacheable returns true if the inner weight is cacheable.
func (w *spanPayloadCheckWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return w.matchWeight.IsCacheable(ctx)
}

// Count returns -1 (no sub-linear count available).
func (w *spanPayloadCheckWeight) Count(_ *index.LeafReaderContext) (int, error) { return -1, nil }

// Matches returns nil (simplified).
func (w *spanPayloadCheckWeight) Matches(_ *index.LeafReaderContext, _ int) (search.Matches, error) {
	return nil, nil
}

// Scorer returns a scorer for the given context.
func (w *spanPayloadCheckWeight) Scorer(ctx *index.LeafReaderContext) (search.Scorer, error) {
	supplier, err := w.ScorerSupplier(ctx)
	if err != nil {
		return nil, err
	}
	if supplier == nil {
		return nil, nil
	}
	return supplier.Get(0)
}

var _ search.Weight = (*spanPayloadCheckWeight)(nil)

// --- Filter spans for payload checking ---

// payloadCheckFilterSpans wraps inner spans and filters positions based on
// payload matching. It mirrors the FilterSpans+PayloadChecker pattern in Java.
type payloadCheckFilterSpans struct {
	inner   spans.Spans
	checker *payloadChecker
}

func (fs *payloadCheckFilterSpans) DocID() int                { return fs.inner.DocID() }
func (fs *payloadCheckFilterSpans) Cost() int64               { return fs.inner.Cost() }
func (fs *payloadCheckFilterSpans) DocIDRunEnd() (int, error) { return fs.inner.DocIDRunEnd() }
func (fs *payloadCheckFilterSpans) StartPosition() int        { return fs.inner.StartPosition() }
func (fs *payloadCheckFilterSpans) EndPosition() int          { return fs.inner.EndPosition() }
func (fs *payloadCheckFilterSpans) Width() int                { return fs.inner.Width() }
func (fs *payloadCheckFilterSpans) PositionsCost() float32    { return fs.inner.PositionsCost() }
func (fs *payloadCheckFilterSpans) AsTwoPhaseIterator() *search.TwoPhaseIterator {
	return fs.inner.AsTwoPhaseIterator()
}
func (fs *payloadCheckFilterSpans) DoStartCurrentDoc() error { return fs.inner.DoStartCurrentDoc() }
func (fs *payloadCheckFilterSpans) DoCurrentSpans() error    { return fs.inner.DoCurrentSpans() }
func (fs *payloadCheckFilterSpans) Collect(collector spans.SpanCollector) error {
	return fs.inner.Collect(collector)
}
func (fs *payloadCheckFilterSpans) NextDoc() (int, error) {
	return fs.inner.NextDoc()
}
func (fs *payloadCheckFilterSpans) Advance(target int) (int, error) {
	return fs.inner.Advance(target)
}
func (fs *payloadCheckFilterSpans) NextStartPosition() (int, error) {
	for {
		pos, err := fs.inner.NextStartPosition()
		if err != nil {
			return spans.NoMorePositions, err
		}
		if pos == spans.NoMorePositions {
			return spans.NoMorePositions, nil
		}
		// Check if the payloads at this position match.
		fs.checker.Reset()
		if err := fs.inner.Collect(fs.checker); err != nil {
			return spans.NoMorePositions, err
		}
		if fs.checker.Match() {
			return pos, nil
		}
		// Position rejected — try next.
	}
}

// --- Payload checker (SpanCollector) ---

// payloadChecker implements SpanCollector to check payloads against expected values.
type payloadChecker struct {
	payloadToMatch []*util.BytesRef
	payloadMatcher PayloadMatcher
	upto           int
	matches        bool
}

func (pc *payloadChecker) Reset() {
	pc.upto = 0
	pc.matches = true
}

func (pc *payloadChecker) Match() bool {
	return pc.matches && pc.upto == len(pc.payloadToMatch)
}

func (pc *payloadChecker) CollectLeaf(postings index.PostingsEnum, position int, term index.Term) error {
	if !pc.matches {
		return nil
	}
	if pc.upto >= len(pc.payloadToMatch) {
		pc.matches = false
		return nil
	}
	payloadBytes, err := postings.GetPayload()
	if err != nil {
		return err
	}
	var payload *util.BytesRef
	if payloadBytes != nil {
		payload = util.NewBytesRef(payloadBytes)
	}
	expected := pc.payloadToMatch[pc.upto]
	if expected == nil {
		pc.matches = (payload == nil)
		pc.upto++
		return nil
	}
	if payload == nil {
		pc.matches = false
		pc.upto++
		return nil
	}
	pc.matches = pc.payloadMatcher.ComparePayload(expected, payload)
	pc.upto++
	return nil
}

var _ spans.SpanCollector = (*payloadChecker)(nil)

// --- Scorer for the filtered spans ---

// payloadCheckScorer implements search.Scorer for payload-checked spans.
// It replicates the relevant scoring logic from queries/spans.SpanScorer.
type payloadCheckScorer struct {
	spans     *payloadCheckFilterSpans
	simScorer search.SimScorer
	norms     index.NumericDocValues
	freq      float32
	lastDoc   int
}

func newPayloadCheckScorer(spans *payloadCheckFilterSpans, simScorer search.SimScorer,
	norms index.NumericDocValues) *payloadCheckScorer {
	return &payloadCheckScorer{
		spans:     spans,
		simScorer: simScorer,
		norms:     norms,
		lastDoc:   -1,
	}
}

func (s *payloadCheckScorer) DocID() int { return s.spans.DocID() }

func (s *payloadCheckScorer) NextDoc() (int, error) {
	doc, err := s.spans.NextDoc()
	if err != nil {
		return search.NO_MORE_DOCS, err
	}
	s.lastDoc = -1
	return doc, nil
}

func (s *payloadCheckScorer) Advance(target int) (int, error) {
	doc, err := s.spans.Advance(target)
	if err != nil {
		return search.NO_MORE_DOCS, err
	}
	s.lastDoc = -1
	return doc, nil
}

func (s *payloadCheckScorer) Cost() int64               { return s.spans.Cost() }
func (s *payloadCheckScorer) DocIDRunEnd() (int, error) { return s.spans.DocIDRunEnd() }

func (s *payloadCheckScorer) setFreqCurrentDoc() error {
	s.freq = 0
	if err := s.spans.DoStartCurrentDoc(); err != nil {
		return err
	}
	pos, err := s.spans.NextStartPosition()
	if err != nil {
		return err
	}
	if pos == spans.NoMorePositions {
		return nil
	}
	for {
		if s.simScorer != nil {
			s.freq += 1.0 / (1.0 + float32(s.spans.Width()))
		}
		if err := s.spans.DoCurrentSpans(); err != nil {
			return err
		}
		next, err := s.spans.NextStartPosition()
		if err != nil {
			return err
		}
		if next == spans.NoMorePositions {
			break
		}
	}
	return nil
}

func (s *payloadCheckScorer) ensureFreq() error {
	cur := s.DocID()
	if s.lastDoc != cur {
		if err := s.setFreqCurrentDoc(); err != nil {
			return err
		}
		s.lastDoc = cur
	}
	return nil
}

func (s *payloadCheckScorer) Score() (float32, error) {
	if err := s.ensureFreq(); err != nil {
		return 0, err
	}
	if s.simScorer == nil {
		return 0, nil
	}
	return s.simScorer.Score104(s.freq, 1), nil
}

func (s *payloadCheckScorer) GetMaxScore(_ int) (float32, error) { return 1<<24 - 1, nil }

// Iterator returns the DocIdSetIterator view of this scorer.
func (s *payloadCheckScorer) Iterator() search.DocIdSetIterator {
	return &payloadCheckIterator{s: s}
}

// TwoPhaseIterator carries Scorer#twoPhaseIterator()'s default body (null).
func (s *payloadCheckScorer) TwoPhaseIterator() *search.TwoPhaseIterator { return nil }

// GetChildren carries Scorable.getChildren()'s default body (empty list).
func (s *payloadCheckScorer) GetChildren() ([]search.ChildScorable, error) {
	return []search.ChildScorable{}, nil
}

// SmoothingScore carries Scorable.smoothingScore(int)'s default body (0f).
func (s *payloadCheckScorer) SmoothingScore(docID int) (float32, error) { return 0, nil }

// SetMinCompetitiveScore carries Scorable.setMinCompetitiveScore's empty default.
func (s *payloadCheckScorer) SetMinCompetitiveScore(minScore float32) error { return nil }

// NextDocsAndScores carries Scorer#nextDocsAndScores's default body.
func (s *payloadCheckScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *search.DocAndFloatFeatureBuffer) error {
	return search.DefaultNextDocsAndScores(s, upTo, liveDocs, buffer)
}

// payloadCheckIterator is the DocIdSetIterator view of payloadCheckScorer.
type payloadCheckIterator struct {
	s *payloadCheckScorer
}

func (it *payloadCheckIterator) DocID() int                 { return it.s.DocID() }
func (it *payloadCheckIterator) Cost() int64                { return it.s.Cost() }
func (it *payloadCheckIterator) NextDoc() (int, error)      { return it.s.NextDoc() }
func (it *payloadCheckIterator) Advance(t int) (int, error) { return it.s.Advance(t) }
func (it *payloadCheckIterator) DocIDRunEnd() (int, error)  { return it.s.DocIDRunEnd() }
func (it *payloadCheckIterator) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(it, upTo, bitSet, offset)
}

func (s *payloadCheckScorer) AdvanceShallow(target int) (int, error) {
	return search.NO_MORE_DOCS, nil
}

var _ search.Scorer = (*payloadCheckScorer)(nil)

// --- Helpers ---

func cloneBytesRefSlice(src []*util.BytesRef) []*util.BytesRef {
	if src == nil {
		return nil
	}
	dst := make([]*util.BytesRef, len(src))
	for i, br := range src {
		if br != nil {
			dst[i] = br.Clone()
		}
	}
	return dst
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene
// 10.5.0, which every subclass inherits unless it overrides it.
func (fs *payloadCheckFilterSpans) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(fs, upTo, bitSet, offset)
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene
// 10.5.0, which every subclass inherits unless it overrides it.
func (s *payloadCheckScorer) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(s, upTo, bitSet, offset)
}

// BulkScorer renders the final Weight.bulkScorer(LeafReaderContext):
// scorerSupplier(context), marked as the top-level scoring clause, supplies
// the bulk scorer; nil when no document matches. It is restated because the
// embedded BaseWeight.BulkScorer would call BaseWeight.ScorerSupplier, not
// this type's override.
func (w *spanPayloadCheckWeight) BulkScorer(context *index.LeafReaderContext) (search.BulkScorer, error) {
	scorerSupplier, err := w.ScorerSupplier(context)
	if err != nil || scorerSupplier == nil {
		// No docs match
		return nil, err
	}
	if err := scorerSupplier.SetTopLevelScoringClause(); err != nil {
		return nil, err
	}
	return scorerSupplier.BulkScorer()
}
