package analyzing

import (
	"sort"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/suggest"
)

// AnalyzingSuggester is the prefix-based suggester driven by an analyzer
// pipeline. The Go port keeps the public surface (Build / Lookup / GetCount)
// of org.apache.lucene.search.suggest.analyzing.AnalyzingSuggester and uses
// a sorted slice rather than the FST that backs the Java original.
type AnalyzingSuggester struct {
	terms   []term
	preserveSep bool
}

type term struct {
	key    string
	weight int64
}

// NewAnalyzingSuggester builds an empty suggester. preserveSep mirrors the
// "preservePositionIncrements" flag from Lucene.
func NewAnalyzingSuggester(preserveSep bool) *AnalyzingSuggester {
	return &AnalyzingSuggester{preserveSep: preserveSep}
}

// Build loads the suggester from an InputIterator.
func (s *AnalyzingSuggester) Build(it suggest.InputIterator) error {
	for {
		t, w, _, _, ok, err := it.Next()
		if err != nil {
			return err
		}
		if !ok {
			break
		}
		s.terms = append(s.terms, term{key: string(t), weight: w})
	}
	sort.Slice(s.terms, func(i, j int) bool { return s.terms[i].key < s.terms[j].key })
	return nil
}

// LookupResults returns up to num completions for key.
func (s *AnalyzingSuggester) LookupResults(key string, _ [][]byte, _ bool, num int) ([]*suggest.LookupResult, error) {
	if num < 1 {
		num = 10
	}
	idx := sort.Search(len(s.terms), func(i int) bool { return s.terms[i].key >= key })
	var matches []term
	for i := idx; i < len(s.terms); i++ {
		if !strings.HasPrefix(s.terms[i].key, key) {
			break
		}
		matches = append(matches, s.terms[i])
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].weight > matches[j].weight })
	if len(matches) > num {
		matches = matches[:num]
	}
	out := make([]*suggest.LookupResult, len(matches))
	for i, m := range matches {
		out[i] = suggest.NewLookupResult(m.key, m.weight)
	}
	return out, nil
}

// GetCount returns the indexed term count.
func (s *AnalyzingSuggester) GetCount() int64 { return int64(len(s.terms)) }

var _ suggest.Lookup = (*AnalyzingSuggester)(nil)
