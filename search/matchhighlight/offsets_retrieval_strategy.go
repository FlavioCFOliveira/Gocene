package matchhighlight

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// OffsetsRetrievalStrategy determines how match offset regions are computed
// from a search.MatchesIterator. Several possibilities exist, ranging from
// retrieving offsets directly from a match instance to re-evaluating the
// document's field and recomputing offsets from there.
//
// Mirrors org.apache.lucene.search.matchhighlight.OffsetsRetrievalStrategy of
// Apache Lucene 10.5.0.
type OffsetsRetrievalStrategy interface {
	// Get returns value offsets (match ranges) acquired from the given
	// search.MatchesIterator.
	Get(matchesIterator search.MatchesIterator, doc FieldValueProvider) ([]OffsetRange, error)
}
