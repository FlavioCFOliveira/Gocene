package matchhighlight

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// FieldValueProvider provides access to field values of the highlighted document.
type FieldValueProvider interface {
	// GetValues returns a list of values for the provided field name or nil if the field is
	// not loaded or does not exist for the field.
	GetValues(field string) []string
}

// OffsetsRetrievalStrategy determines how match offset regions are computed from search.MatchesIterator.
// Several possibilities exist, ranging from retrieving offsets directly from a match instance to
// re-evaluating the document's field and recomputing offsets from there.
type OffsetsRetrievalStrategy interface {
	// Get returns value offsets (match ranges) acquired from the given search.MatchesIterator.
	Get(matchesIterator search.MatchesIterator, doc FieldValueProvider) ([]OffsetRange, error)
}
