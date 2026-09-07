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
	// The returned iterator will either be positioned on -1 if no documents have been
	// scored yet, util.NO_MORE_DOCS if all documents have been scored already, or
	// the last document id that has been scored otherwise.
	//
	// The returned iterator is a view: calling this method several times will return
	// iterators that have the same state.
	Iterator() util.DocIdSetIterator

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
