package search

import (
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Scorer exposes an iterator over documents matching a query in increasing order of doc id.
//
// Mirrors org.apache.lucene.search.Scorer (Lucene 10.5.0).
type Scorer interface {
	Scorable

	// DocID returns the doc ID that is currently being scored.
	DocID() int

	// Iterator returns a DocIdSetIterator over matching documents.
	//
	// Mirrors the abstract Scorer.iterator(), which returns
	// org.apache.lucene.search.DocIdSetIterator.
	//
	// The returned iterator will either be positioned on -1 if no documents have been
	// scored yet, NO_MORE_DOCS if all documents have been scored already, or
	// the last document id that has been scored otherwise.
	//
	// The returned iterator is a view: calling this method several times will return
	// iterators that have the same state.
	Iterator() DocIdSetIterator

	// TwoPhaseIterator returns a TwoPhaseIterator view of this Scorer.
	// A return value of nil indicates that two-phase iteration is not supported.
	//
	// Note that the returned TwoPhaseIterator's Approximation() must advance synchronously
	// with the Iterator(): advancing the approximation must advance the iterator and vice-versa.
	TwoPhaseIterator() *TwoPhaseIterator

	// AdvanceShallow advances to the block of documents that contains target in order
	// to get scoring information about this block. This method is implicitly called by
	// util.DocIdSetIterator.Advance() and util.DocIdSetIterator.NextDoc() on the returned doc ID.
	// Calling this method doesn't modify the current DocID(). It returns a
	// number that is greater than or equal to all documents contained in the current block,
	// but less than any doc IDs of the next block. target must be >= DocID() as well as
	// all targets that have been passed to AdvanceShallow(int) so far.
	AdvanceShallow(target int) (int, error)

	// GetMaxScore returns the maximum score that documents between the last target
	// that this iterator was AdvanceShallow() to included and upTo included.
	GetMaxScore(upTo int) (float32, error)

	// NextDocsAndScores returns a new batch of doc IDs and scores, starting at the
	// current doc ID, and ending before upTo. Because it starts on the current doc ID,
	// it is illegal to call this method if the current doc ID is -1.
	//
	// An empty return value indicates that there are no postings left between the
	// current doc ID and upTo.
	//
	// Implementations should ideally fill the buffer with a number of entries comprised
	// between 8 and a couple hundreds, to keep heap requirements contained, while still
	// being large enough to enable operations on the buffer to auto-vectorize efficiently.
	NextDocsAndScores(upTo int, liveDocs util.Bits, buffer *DocAndFloatFeatureBuffer) error
}

// ScoreErrorReporter is the optional Scorer extension for scorers that can
// detect an error condition while computing a score. Gocene's Scorer.Score
// returns only a float32 (no error), unlike Lucene where Scorer.score() throws
// IOException/IllegalStateException; this interface lets such scorers surface a
// deferred error that the search loop consults after the score is consumed.
//
// It is used to faithfully reproduce the block-join "Child query must not match
// same docs with parent filter" IllegalStateException that Lucene raises from
// ToParentBlockJoinQuery.BlockJoinScorer.scoreChildDocs.
type ScoreErrorReporter interface {
	// ScoreError returns a non-nil error if the most recent Score call detected
	// an invariant violation, or nil otherwise.
	ScoreError() error
}

// MinCompetitiveScorer is the optional Scorer extension that lets a collector
// (or a parent scorer) hint at the minimum score a hit must reach to be
// competitive, enabling non-competitive documents to be skipped. It mirrors
// org.apache.lucene.search.Scorer#setMinCompetitiveScore.
//
// It is modelled as an optional interface rather than a method on Scorer so
// that the many existing Scorer implementations keep compiling unchanged: only
// scorers that participate in TOP_SCORES early termination implement it, and
// callers type-assert before forwarding the hint.
type MinCompetitiveScorer interface {
	// SetMinCompetitiveScore informs the scorer that hits scoring below
	// minScore are not competitive and may be skipped. Implementations that
	// cannot skip should leave it a no-op.
	SetMinCompetitiveScore(minScore float32) error
}

// DefaultTwoPhaseIterator returns nil, mirroring the default implementation in Lucene.
func DefaultTwoPhaseIterator() *TwoPhaseIterator {
	return nil
}

// DefaultAdvanceShallow returns util.NO_MORE_DOCS, mirroring the default implementation in Lucene.
func DefaultAdvanceShallow(target int) (int, error) {
	return util.NO_MORE_DOCS, nil
}

// DefaultNextDocsAndScores provides the default implementation of Scorer.NextDocsAndScores.
func DefaultNextDocsAndScores(s Scorer, upTo int, liveDocs util.Bits, buffer *DocAndFloatFeatureBuffer) error {
	batchSize := 64
	buffer.GrowNoCopy(batchSize)
	size := 0
	iterator := s.Iterator()
	doc := s.DocID()
	for doc < upTo && size < batchSize {
		if doc >= 0 && (liveDocs == nil || liveDocs.Get(doc)) {
			score, err := s.Score()
			if err != nil {
				return err
			}
			buffer.Docs[size] = doc
			buffer.Features[size] = score
			size++
		}
		nextDoc, err := iterator.NextDoc()
		if err != nil {
			return err
		}
		doc = nextDoc
	}
	buffer.Size = size
	return nil
}

// BaseScorer carries the concrete members of the abstract class
// org.apache.lucene.search.Scorer (Lucene 10.5.0): the default bodies of
// twoPhaseIterator() and advanceShallow(int). It embeds BaseScorable because
// Java's Scorer extends Scorable, so a Scorer also inherits that class's
// defaults.
//
// Java's docID(), iterator() and getMaxScore(int) are abstract and are
// therefore not provided here: the embedder must supply them. Java's
// nextDocsAndScores(int, Bits, DocAndFloatFeatureBuffer) is concrete, but its
// body dispatches back to the abstract iterator(), docID() and score(); Go
// embedding cannot do that, so it is rendered as the free function
// DefaultNextDocsAndScores above, which takes the concrete Scorer explicitly.
type BaseScorer struct {
	BaseScorable
}

// TwoPhaseIterator mirrors Scorer.twoPhaseIterator(), whose default body in
// Java returns null.
func (s *BaseScorer) TwoPhaseIterator() *TwoPhaseIterator {
	return nil
}

// AdvanceShallow mirrors Scorer.advanceShallow(int), whose default body in
// Java returns DocIdSetIterator.NO_MORE_DOCS.
func (s *BaseScorer) AdvanceShallow(target int) (int, error) {
	return util.NO_MORE_DOCS, nil
}
