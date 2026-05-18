package fst

import (
	"sort"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/suggest"
)

// WFSTCompletionLookup is the weighted variant that returns completions
// sorted by descending weight rather than bucket. Mirrors
// org.apache.lucene.search.suggest.fst.WFSTCompletionLookup.
type WFSTCompletionLookup struct {
	terms []wfstEntry
	count int64
}

type wfstEntry struct {
	key    string
	weight int64
}

// NewWFSTCompletionLookup builds an empty lookup.
func NewWFSTCompletionLookup() *WFSTCompletionLookup { return &WFSTCompletionLookup{} }

// Build ingests an InputIterator.
func (l *WFSTCompletionLookup) Build(it suggest.InputIterator) error {
	for {
		t, w, _, _, ok, err := it.Next()
		if err != nil {
			return err
		}
		if !ok {
			break
		}
		l.terms = append(l.terms, wfstEntry{key: string(t), weight: w})
		l.count++
	}
	sort.Slice(l.terms, func(i, j int) bool { return l.terms[i].key < l.terms[j].key })
	return nil
}

// LookupResults returns up to num completions for key.
func (l *WFSTCompletionLookup) LookupResults(key string, _ [][]byte, _ bool, num int) ([]*suggest.LookupResult, error) {
	if num < 1 {
		num = 10
	}
	idx := sort.Search(len(l.terms), func(i int) bool { return l.terms[i].key >= key })
	var matches []wfstEntry
	for i := idx; i < len(l.terms); i++ {
		if !strings.HasPrefix(l.terms[i].key, key) {
			break
		}
		matches = append(matches, l.terms[i])
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
func (l *WFSTCompletionLookup) GetCount() int64 { return l.count }

var _ suggest.Lookup = (*WFSTCompletionLookup)(nil)
