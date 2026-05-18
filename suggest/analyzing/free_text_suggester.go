package analyzing

import (
	"sort"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/suggest"
)

// FreeTextSuggester is the n-gram-based suggester that synthesises
// completions by chaining the top trailing words. Mirrors
// org.apache.lucene.search.suggest.analyzing.FreeTextSuggester.
//
// The Go port keeps the public Build/Lookup contract while using an
// in-memory bigram model rather than the FST used in Lucene.
type FreeTextSuggester struct {
	GramSize int
	// counts maps "prev_word current_word" to weight.
	counts map[string]int64
	totalCount int64
}

// NewFreeTextSuggester builds a suggester with the supplied gram size
// (defaults to 2 — bigrams).
func NewFreeTextSuggester(gramSize int) *FreeTextSuggester {
	if gramSize < 1 {
		gramSize = 2
	}
	return &FreeTextSuggester{GramSize: gramSize, counts: make(map[string]int64)}
}

// Build trains the model with every term that flows through the iterator.
// Whitespace inside the term is treated as a token boundary.
func (s *FreeTextSuggester) Build(it suggest.InputIterator) error {
	for {
		t, w, _, _, ok, err := it.Next()
		if err != nil {
			return err
		}
		if !ok {
			break
		}
		toks := strings.Fields(string(t))
		for i := 0; i+s.GramSize-1 < len(toks); i++ {
			key := strings.Join(toks[i:i+s.GramSize], " ")
			s.counts[key] += w
			s.totalCount += w
		}
	}
	return nil
}

// LookupResults predicts the top-N continuations of key.
func (s *FreeTextSuggester) LookupResults(key string, _ [][]byte, _ bool, num int) ([]*suggest.LookupResult, error) {
	if num < 1 {
		num = 10
	}
	type cand struct {
		text string
		score int64
	}
	var pool []cand
	for k, w := range s.counts {
		if strings.HasPrefix(k, key) {
			pool = append(pool, cand{text: k, score: w})
		}
	}
	sort.Slice(pool, func(i, j int) bool { return pool[i].score > pool[j].score })
	if len(pool) > num {
		pool = pool[:num]
	}
	out := make([]*suggest.LookupResult, len(pool))
	for i, c := range pool {
		out[i] = suggest.NewLookupResult(c.text, c.score)
	}
	return out, nil
}

// GetCount returns the total weight observed during Build.
func (s *FreeTextSuggester) GetCount() int64 { return s.totalCount }

var _ suggest.Lookup = (*FreeTextSuggester)(nil)
