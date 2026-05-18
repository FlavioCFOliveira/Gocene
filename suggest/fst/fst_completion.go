package fst

import (
	"sort"
	"strings"
)

// FSTCompletion is the runtime side of the FST-based suggester. Mirrors
// org.apache.lucene.search.suggest.fst.FSTCompletion.
//
// The Go port uses a sorted slice rather than an FST since util/fst is in a
// separate module. The public contract (DoLookup) matches the Java original.
type FSTCompletion struct {
	terms []completionEntry
	exactFirst bool
}

type completionEntry struct {
	key    string
	bucket int
}

// NewFSTCompletion builds an empty completion engine.
func NewFSTCompletion(exactFirst bool) *FSTCompletion {
	return &FSTCompletion{exactFirst: exactFirst}
}

// AddEntry registers (key, bucket); higher bucket values rank earlier.
func (c *FSTCompletion) AddEntry(key string, bucket int) {
	c.terms = append(c.terms, completionEntry{key: key, bucket: bucket})
}

// Finalize sorts the entries.
func (c *FSTCompletion) Finalize() {
	sort.Slice(c.terms, func(i, j int) bool { return c.terms[i].key < c.terms[j].key })
}

// CompletionMatch carries one completion.
type CompletionMatch struct {
	Key    string
	Bucket int
}

// DoLookup returns up to num completions sorted by descending bucket, with
// optional exact-match priority.
func (c *FSTCompletion) DoLookup(prefix string, num int) []CompletionMatch {
	if num < 1 {
		num = 10
	}
	idx := sort.Search(len(c.terms), func(i int) bool { return c.terms[i].key >= prefix })
	var matches []CompletionMatch
	exactMatchIdx := -1
	for i := idx; i < len(c.terms); i++ {
		if !strings.HasPrefix(c.terms[i].key, prefix) {
			break
		}
		matches = append(matches, CompletionMatch{Key: c.terms[i].key, Bucket: c.terms[i].bucket})
		if c.exactFirst && c.terms[i].key == prefix {
			exactMatchIdx = len(matches) - 1
		}
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].Bucket > matches[j].Bucket })
	if c.exactFirst && exactMatchIdx > 0 {
		// move exact match to top
		exact := matches[exactMatchIdx]
		matches = append(matches[:exactMatchIdx], matches[exactMatchIdx+1:]...)
		matches = append([]CompletionMatch{exact}, matches...)
	}
	if len(matches) > num {
		matches = matches[:num]
	}
	return matches
}
