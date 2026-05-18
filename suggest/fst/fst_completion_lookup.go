package fst

import "github.com/FlavioCFOliveira/Gocene/suggest"

// FSTCompletionLookup is the Lookup-compliant wrapper around FSTCompletion.
// Mirrors org.apache.lucene.search.suggest.fst.FSTCompletionLookup.
type FSTCompletionLookup struct {
	completion *FSTCompletion
	count      int64
	exactFirst bool
	buckets    int
}

// NewFSTCompletionLookup builds an empty lookup.
func NewFSTCompletionLookup(buckets int, exactFirst bool) *FSTCompletionLookup {
	if buckets < 1 {
		buckets = 10
	}
	return &FSTCompletionLookup{buckets: buckets, exactFirst: exactFirst}
}

// Build loads the lookup from an InputIterator.
func (l *FSTCompletionLookup) Build(it suggest.InputIterator) error {
	builder := NewFSTCompletionBuilder(l.buckets, NewInMemoryBytesRefSorter(), l.exactFirst)
	for {
		t, w, _, _, ok, err := it.Next()
		if err != nil {
			return err
		}
		if !ok {
			break
		}
		if err := builder.Add(t, w); err != nil {
			return err
		}
		l.count++
	}
	c, err := builder.Build()
	if err != nil {
		return err
	}
	l.completion = c
	return nil
}

// LookupResults returns up to num completions for key.
func (l *FSTCompletionLookup) LookupResults(key string, _ [][]byte, _ bool, num int) ([]*suggest.LookupResult, error) {
	if l.completion == nil {
		return nil, nil
	}
	matches := l.completion.DoLookup(key, num)
	out := make([]*suggest.LookupResult, len(matches))
	for i, m := range matches {
		out[i] = suggest.NewLookupResult(m.Key, int64(m.Bucket))
	}
	return out, nil
}

// GetCount returns the number of entries ingested at Build time.
func (l *FSTCompletionLookup) GetCount() int64 { return l.count }

var _ suggest.Lookup = (*FSTCompletionLookup)(nil)
