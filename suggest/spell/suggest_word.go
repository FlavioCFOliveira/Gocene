package spell

// SuggestWord is a single suggestion result: the candidate word together with
// the distance to the input and the candidate's collection frequency. Mirrors
// org.apache.lucene.search.spell.SuggestWord.
type SuggestWord struct {
	String string
	Score  float32
	Freq   int
}

// NewSuggestWord builds a SuggestWord.
func NewSuggestWord(word string, score float32, freq int) *SuggestWord {
	return &SuggestWord{String: word, Score: score, Freq: freq}
}
