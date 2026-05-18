package spell

// WordBreakSpellChecker proposes corrections that split or join input
// tokens. Mirrors org.apache.lucene.search.spell.WordBreakSpellChecker.
type WordBreakSpellChecker struct {
	Terms        map[string]int64
	MaxChanges   int
	MaxCombineWordLength int
}

// NewWordBreakSpellChecker builds a checker backed by terms (term -> freq).
func NewWordBreakSpellChecker(terms map[string]int64) *WordBreakSpellChecker {
	return &WordBreakSpellChecker{Terms: terms, MaxChanges: 1, MaxCombineWordLength: 20}
}

// SuggestWordBreaks returns possible splits of word that decompose into
// known terms. Each result is a CombineSuggestion whose Suggestion is the
// joined form and OrigTermIndexes is empty (the caller decides what to do
// with the parts when needed).
func (c *WordBreakSpellChecker) SuggestWordBreaks(word string) []*CombineSuggestion {
	var out []*CombineSuggestion
	r := []rune(word)
	for split := 1; split < len(r); split++ {
		left, right := string(r[:split]), string(r[split:])
		if _, lok := c.Terms[left]; !lok {
			continue
		}
		if _, rok := c.Terms[right]; !rok {
			continue
		}
		score := float32(c.Terms[left]+c.Terms[right]) / float32(len(r))
		out = append(out, NewCombineSuggestion(NewSuggestWord(left+" "+right, score, int(c.Terms[left]+c.Terms[right]))))
	}
	return out
}

// SuggestWordCombinations returns possible joinings of consecutive tokens
// whose concatenation is a known term.
func (c *WordBreakSpellChecker) SuggestWordCombinations(tokens []string) []*CombineSuggestion {
	var out []*CombineSuggestion
	for i := 0; i < len(tokens)-1; i++ {
		combined := tokens[i] + tokens[i+1]
		if len(combined) > c.MaxCombineWordLength {
			continue
		}
		if f, ok := c.Terms[combined]; ok {
			out = append(out, NewCombineSuggestion(NewSuggestWord(combined, 1, int(f)), i, i+1))
		}
	}
	return out
}
