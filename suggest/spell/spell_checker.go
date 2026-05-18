package spell

// SpellChecker is the index-backed spell checker that suggests corrections by
// looking up n-grams of the input. Mirrors
// org.apache.lucene.search.spell.SpellChecker.
//
// The Go port operates in-memory on a Dictionary; the n-gram index that
// Lucene builds with an IndexWriter is replaced by an in-memory map keyed
// by short n-grams. Output ranking uses the supplied StringDistance.
type SpellChecker struct {
	Accuracy float32
	Distance StringDistance
	NGram    int
	terms    map[string]int64        // term -> frequency
	gramIdx  map[string]map[string]bool // ngram -> set of terms containing it
}

// NewSpellChecker builds an empty SpellChecker with the supplied distance.
func NewSpellChecker(distance StringDistance) *SpellChecker {
	if distance == nil {
		distance = LevenshteinDistance{}
	}
	return &SpellChecker{
		Accuracy: 0.5,
		Distance: distance,
		NGram:    3,
		terms:    make(map[string]int64),
		gramIdx:  make(map[string]map[string]bool),
	}
}

// IndexDictionary builds the in-memory index from the supplied dictionary.
func (s *SpellChecker) IndexDictionary(d Dictionary) error {
	it := d.GetEntryIterator()
	for {
		term, w, ok, err := it.Next()
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		s.terms[term] += w
		for _, g := range ngramKeys(term, s.NGram) {
			set, ok := s.gramIdx[g]
			if !ok {
				set = make(map[string]bool)
				s.gramIdx[g] = set
			}
			set[term] = true
		}
	}
}

// SuggestSimilar returns up to numSug suggestions ordered by distance.
func (s *SpellChecker) SuggestSimilar(word string, numSug int) []*SuggestWord {
	candidates := make(map[string]bool)
	for _, g := range ngramKeys(word, s.NGram) {
		if set, ok := s.gramIdx[g]; ok {
			for t := range set {
				candidates[t] = true
			}
		}
	}
	q := NewSuggestWordQueue(numSug, SuggestWordScoreComparator)
	for cand := range candidates {
		if cand == word {
			continue
		}
		dist := s.Distance.GetDistance(word, cand)
		if dist < s.Accuracy {
			continue
		}
		q.Insert(NewSuggestWord(cand, dist, int(s.terms[cand])))
	}
	out := make([]*SuggestWord, 0, q.Size())
	for q.Size() > 0 {
		out = append([]*SuggestWord{q.Pop()}, out...)
	}
	return out
}

func ngramKeys(s string, n int) []string {
	r := []rune(s)
	if len(r) < n {
		return []string{string(r)}
	}
	out := make([]string, 0, len(r)-n+1)
	for i := 0; i <= len(r)-n; i++ {
		out = append(out, string(r[i:i+n]))
	}
	return out
}
