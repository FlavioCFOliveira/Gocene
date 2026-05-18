package tst

import (
	"sort"

	"github.com/FlavioCFOliveira/Gocene/suggest"
)

// TSTLookup is the suggest.Lookup-compliant front-end for the TST tree.
// Mirrors org.apache.lucene.search.suggest.tst.TSTLookup.
type TSTLookup struct {
	tree  *TSTAutocomplete
	count int64
}

// NewTSTLookup builds an empty lookup.
func NewTSTLookup() *TSTLookup { return &TSTLookup{tree: NewTSTAutocomplete()} }

// Build ingests an InputIterator.
func (l *TSTLookup) Build(it suggest.InputIterator) error {
	for {
		t, w, _, _, ok, err := it.Next()
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		l.tree.Insert(string(t), w)
		l.count++
	}
}

// LookupResults returns up to num completions for key sorted by descending
// weight.
func (l *TSTLookup) LookupResults(key string, _ [][]byte, _ bool, num int) ([]*suggest.LookupResult, error) {
	if num < 1 {
		num = 10
	}
	entries := l.tree.PrefixCompletion(key)
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Val > entries[j].Val })
	if len(entries) > num {
		entries = entries[:num]
	}
	out := make([]*suggest.LookupResult, len(entries))
	for i, e := range entries {
		out[i] = suggest.NewLookupResult(e.Token, e.Val)
	}
	return out, nil
}

// GetCount returns the number of tokens stored.
func (l *TSTLookup) GetCount() int64 { return l.count }

var _ suggest.Lookup = (*TSTLookup)(nil)
