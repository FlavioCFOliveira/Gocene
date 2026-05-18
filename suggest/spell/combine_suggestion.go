package spell

// CombineSuggestion ties a SuggestWord to the indices of the input tokens
// that produced it. Mirrors
// org.apache.lucene.search.spell.CombineSuggestion.
type CombineSuggestion struct {
	Suggestion *SuggestWord
	OrigTermIndexes []int
}

// NewCombineSuggestion builds a CombineSuggestion.
func NewCombineSuggestion(s *SuggestWord, origIndexes ...int) *CombineSuggestion {
	clone := make([]int, len(origIndexes))
	copy(clone, origIndexes)
	return &CombineSuggestion{Suggestion: s, OrigTermIndexes: clone}
}
