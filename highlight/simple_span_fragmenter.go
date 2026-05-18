package highlight

// SimpleSpanFragmenter splits text into fragments respecting span boundaries
// from the QueryScorer when possible. Mirrors
// org.apache.lucene.search.highlight.SimpleSpanFragmenter.
type SimpleSpanFragmenter struct {
	fragmentSize int
	currentTokenIndex int
	currentStart      int
	queryScorer       *QueryScorer
}

// NewSimpleSpanFragmenter builds a fragmenter with the supplied scorer.
func NewSimpleSpanFragmenter(scorer *QueryScorer, fragmentSize int) *SimpleSpanFragmenter {
	if fragmentSize <= 0 {
		fragmentSize = 100
	}
	return &SimpleSpanFragmenter{fragmentSize: fragmentSize, queryScorer: scorer}
}

// IsNewFragment reports whether the next token should start a new fragment
// — true when the cumulative token offset exceeds fragmentSize.
func (f *SimpleSpanFragmenter) IsNewFragment(currentOffset int) bool {
	if currentOffset-f.currentStart >= f.fragmentSize {
		f.currentStart = currentOffset
		return true
	}
	return false
}

// Start resets the fragmenter at the beginning of the text.
func (f *SimpleSpanFragmenter) Start(text string) {
	f.currentTokenIndex = 0
	f.currentStart = 0
}
