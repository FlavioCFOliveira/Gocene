package spell

// DirectSpellChecker scans the index's terms dictionary directly for
// candidates within a Damerau-Levenshtein distance. Mirrors
// org.apache.lucene.search.spell.DirectSpellChecker.
//
// The Go port operates over the same in-memory term frequency table the
// SpellChecker uses but with the Lucene-style fuzzy semantics.
type DirectSpellChecker struct {
	Accuracy        float32
	Distance        StringDistance
	MinPrefix       int
	MaxEdits        int
	terms           map[string]int64
	SuggestMode     SuggestMode
}

// NewDirectSpellChecker builds the checker.
func NewDirectSpellChecker() *DirectSpellChecker {
	return &DirectSpellChecker{
		Accuracy:    0.5,
		Distance:    LuceneLevenshteinDistance{},
		MinPrefix:   1,
		MaxEdits:    2,
		terms:       make(map[string]int64),
		SuggestMode: SuggestWhenNotInIndex,
	}
}

// SetTerms loads the term-frequency table.
func (c *DirectSpellChecker) SetTerms(terms map[string]int64) { c.terms = terms }

// SuggestSimilar returns up to numSug suggestions for word.
func (c *DirectSpellChecker) SuggestSimilar(word string, numSug int) []*SuggestWord {
	prefix := ""
	r := []rune(word)
	if len(r) > c.MinPrefix {
		prefix = string(r[:c.MinPrefix])
	}
	q := NewSuggestWordQueue(numSug, SuggestWordScoreComparator)
	for cand, freq := range c.terms {
		if c.SuggestMode == SuggestMoreFrequentlyThanExisting {
			if existing, ok := c.terms[word]; ok && freq <= existing {
				continue
			}
		}
		if cand == word && c.SuggestMode != SuggestAlways {
			continue
		}
		if prefix != "" && !startsWith(cand, prefix) {
			continue
		}
		score := c.Distance.GetDistance(word, cand)
		if score < c.Accuracy {
			continue
		}
		q.Insert(NewSuggestWord(cand, score, int(freq)))
	}
	out := make([]*SuggestWord, 0, q.Size())
	for q.Size() > 0 {
		out = append([]*SuggestWord{q.Pop()}, out...)
	}
	return out
}

func startsWith(s, prefix string) bool {
	if len(prefix) == 0 {
		return true
	}
	if len(s) < len(prefix) {
		return false
	}
	return s[:len(prefix)] == prefix
}
