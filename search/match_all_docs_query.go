// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// MatchAllDocsQuery matches all documents in the index.
type MatchAllDocsQuery struct {
	*BaseQuery
}

// Instance is a singleton instance of MatchAllDocsQuery.
var Instance = NewMatchAllDocsQuery()

// NewMatchAllDocsQuery creates a new MatchAllDocsQuery.
func NewMatchAllDocsQuery() *MatchAllDocsQuery {
	return &MatchAllDocsQuery{
		BaseQuery: &BaseQuery{},
	}
}

// Equals checks if this query equals another.
func (q *MatchAllDocsQuery) Equals(other spi.Query) bool {
	_, ok := other.(*MatchAllDocsQuery)
	return ok
}

// HashCode returns a hash code for this query.
func (q *MatchAllDocsQuery) HashCode() int {
	return 0
}

// Rewrite rewrites the query to a simpler form.
func (q *MatchAllDocsQuery) Rewrite(searcher *IndexSearcher) (Query, error) {
	return q, nil
}

// CreateWeight creates a Weight for this query.
func (q *MatchAllDocsQuery) CreateWeight(searcher *IndexSearcher, scoreMode ScoreMode, boost float32) (Weight, error) {
	return NewMatchAllDocsWeight(q, boost), nil
}

// ToString returns the string representation of the query.
func (q *MatchAllDocsQuery) ToString(field string) string {
	return "*:*"
}

// Visit visits the query with the given visitor.
func (q *MatchAllDocsQuery) Visit(visitor QueryVisitor) {
	visitor.VisitLeaf(q)
}

// MatchAllDocsWeight is the Weight implementation for MatchAllDocsQuery.
type MatchAllDocsWeight struct {
	*BaseWeight
	boost float32
}

// NewMatchAllDocsWeight creates a new MatchAllDocsWeight.
func NewMatchAllDocsWeight(query Query, boost float32) *MatchAllDocsWeight {
	return &MatchAllDocsWeight{
		BaseWeight: NewBaseWeight(query),
		boost:      boost,
	}
}

// String returns the string representation of the weight.
func (w *MatchAllDocsWeight) String() string {
	return "weight(MatchAllDocsQuery)"
}

// Scorer creates a scorer for this weight.
func (w *MatchAllDocsWeight) Scorer(context *index.LeafReaderContext) (Scorer, error) {
	reader := context.Reader()
	if reader == nil {
		return nil, nil
	}
	return NewMatchAllDocsScorer(w, reader.MaxDoc(), w.boost), nil
}

// ScorerSupplier creates a scorer supplier for this weight.
func (w *MatchAllDocsWeight) ScorerSupplier(context *index.LeafReaderContext) (ScorerSupplier, error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return NewDefaultScorerSupplier(scorer), nil
}

// Explain returns an explanation of the score for the given document.
func (w *MatchAllDocsWeight) Explain(context *index.LeafReaderContext, doc int) (Explanation, error) {
	if doc >= 0 && doc < context.Reader().MaxDoc() {
		return NewExplanation(true, w.boost, "MatchAllDocsQuery, product of:"), nil
	}
	return NewExplanation(false, 0, "MatchAllDocsQuery, no document"), nil
}

// BulkScorer creates a bulk scorer for efficient bulk scoring.
func (w *MatchAllDocsWeight) BulkScorer(context *index.LeafReaderContext) (BulkScorer, error) {
	scorer, err := w.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		return nil, nil
	}
	return NewDefaultBulkScorer(scorer), nil
}

// IsCacheable returns true if this weight can be cached for the given leaf.
func (w *MatchAllDocsWeight) IsCacheable(ctx *index.LeafReaderContext) bool {
	return true
}

// Count returns the count of matching documents in sub-linear time.
func (w *MatchAllDocsWeight) Count(context *index.LeafReaderContext) (int, error) {
	return context.Reader().NumDocs(), nil
}

// Matches returns the matches for a specific document.
func (w *MatchAllDocsWeight) Matches(context *index.LeafReaderContext, doc int) (Matches, error) {
	return nil, nil
}

// Ensure MatchAllDocsWeight implements Weight
var _ Weight = (*MatchAllDocsWeight)(nil)

// MatchAllDocsScorer is the Scorer implementation for MatchAllDocsQuery.
type MatchAllDocsScorer struct {
	BaseScorer
	maxDoc int
	score  float32
	// disi mirrors the DocIdSetIterator that
	// ConstantScoreScorerSupplier.matchAll(score, scoreMode, maxDoc) hands to
	// the ConstantScoreScorer constructor: DocIdSetIterator.all(maxDoc).
	disi DocIdSetIterator
}

// NewMatchAllDocsScorer creates a new MatchAllDocsScorer.
func NewMatchAllDocsScorer(weight Weight, maxDoc int, score float32) *MatchAllDocsScorer {
	return &MatchAllDocsScorer{
		maxDoc: maxDoc,
		score:  score,
		disi:   All(maxDoc),
	}
}

// DocID mirrors ConstantScoreScorer.docID(), whose body is
// `return disi.docID();`.
func (s *MatchAllDocsScorer) DocID() int {
	return s.disi.DocID()
}

// NextDoc advances the shared iterator; Java reaches it through iterator().
func (s *MatchAllDocsScorer) NextDoc() (int, error) {
	return s.disi.NextDoc()
}

// Advance advances the shared iterator; Java reaches it through iterator().
func (s *MatchAllDocsScorer) Advance(target int) (int, error) {
	return s.disi.Advance(target)
}

// Iterator mirrors ConstantScoreScorer.iterator(), whose body is
// `return disi;`.
func (s *MatchAllDocsScorer) Iterator() DocIdSetIterator {
	return s.disi
}

// Score mirrors ConstantScoreScorer.score(), whose body is `return score;`.
func (s *MatchAllDocsScorer) Score() (float32, error) {
	return s.score, nil
}

// GetMaxScore mirrors ConstantScoreScorer.getMaxScore(int), whose body is
// `return score;`.
func (s *MatchAllDocsScorer) GetMaxScore(_ int) (float32, error) {
	return s.score, nil
}

// Cost returns the cost of the shared iterator.
func (s *MatchAllDocsScorer) Cost() int64 {
	return s.disi.Cost()
}

// DocIDRunEnd returns the end of the current run of consecutive doc IDs.
func (s *MatchAllDocsScorer) DocIDRunEnd() (int, error) {
	return s.disi.DocIDRunEnd()
}

// NextDocsAndScores mirrors ConstantScoreScorer.nextDocsAndScores(int, Bits,
// DocAndFloatFeatureBuffer) (Lucene 10.5.0):
//
//	int batchSize = 64;
//	buffer.growNoCopy(batchSize);
//	int size = 0;
//	DocIdSetIterator iterator = iterator();
//	for (int doc = iterator.docID(); doc < upTo && size < batchSize; doc = iterator.nextDoc()) {
//	  if (liveDocs == null || liveDocs.get(doc)) {
//	    buffer.docs[size] = doc;
//	    ++size;
//	  }
//	}
//	Arrays.fill(buffer.features, 0, size, score);
//	buffer.size = size;
func (s *MatchAllDocsScorer) NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *DocAndFloatFeatureBuffer) error {
	batchSize := 64
	buffer.GrowNoCopy(batchSize)
	size := 0
	iterator := s.Iterator()
	for doc := iterator.DocID(); doc < upTo && size < batchSize; {
		if liveDocs == nil || liveDocs.Get(doc) {
			buffer.Docs[size] = doc
			size++
		}
		next, err := iterator.NextDoc()
		if err != nil {
			return err
		}
		doc = next
	}
	for i := 0; i < size; i++ {
		buffer.Features[i] = s.score
	}
	buffer.Size = size
	return nil
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int) in Apache Lucene
// 10.5.0, which every subclass inherits unless it overrides it.
func (s *MatchAllDocsScorer) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(s, upTo, bitSet, offset)
}
