package uhighlight

// UHComponents bundles every reusable piece the unified highlighter needs to
// render a single field: the field name, the OffsetsRetrievalStrategy, the
// PhraseHelper, and the break iterator. Mirrors
// org.apache.lucene.search.uhighlight.UHComponents.
type UHComponents struct {
	Field        string
	OffsetStrat  FieldOffsetStrategy
	PhraseHelper *PhraseHelper
	BreakIter    BreakIterator
	Matchers     []CharArrayMatcher
}

// NewUHComponents builds the bundle.
func NewUHComponents(field string, strat FieldOffsetStrategy, phrase *PhraseHelper, breakIter BreakIterator, matchers []CharArrayMatcher) *UHComponents {
	clone := make([]CharArrayMatcher, len(matchers))
	copy(clone, matchers)
	return &UHComponents{
		Field:        field,
		OffsetStrat:  strat,
		PhraseHelper: phrase,
		BreakIter:    breakIter,
		Matchers:     clone,
	}
}
