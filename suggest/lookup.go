// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package suggest

// LookupResult is a single autocomplete result. Mirrors
// org.apache.lucene.search.suggest.Lookup.LookupResult.
type LookupResult struct {
	Key      string
	Value    int64
	Payload  []byte
	Contexts [][]byte
}

// NewLookupResult builds a LookupResult.
func NewLookupResult(key string, value int64) *LookupResult {
	return &LookupResult{Key: key, Value: value}
}

// Lookup is the abstract base every suggester satisfies. Mirrors
// org.apache.lucene.search.suggest.Lookup.
type Lookup interface {
	// Build loads the suggester from an InputIterator.
	Build(iter InputIterator) error

	// LookupResults returns up to num completions for key, optionally
	// requiring an onlyMorePopular ordering and a contexts filter.
	LookupResults(key string, contexts [][]byte, onlyMorePopular bool, num int) ([]*LookupResult, error)

	// GetCount returns the number of indexed entries.
	GetCount() int64
}
