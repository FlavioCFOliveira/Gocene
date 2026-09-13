// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// TermsCollector collects terms from a set of documents.
//
// This is the Go port of Lucene's org.apache.lucene.search.join.TermsCollector.
type TermsCollector struct {
	field string
	terms map[string]int
}

func NewTermsCollector(field string) *TermsCollector {
	return &TermsCollector{
		field: field,
		terms: make(map[string]int),
	}
}

// Collect collects the term for the given document.
func (c *TermsCollector) Collect(doc int, context *index.LeafReaderContext) {
	val, err := context.Reader().GetFieldValue(doc, c.field)
	if err != nil {
		return
	}
	c.terms[val]++
}

func (c *TermsCollector) GetTerms() map[string]int {
	return c.terms
}
