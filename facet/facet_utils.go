// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facet

import (
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// FacetUtils provides utility functions for the facet module.
//
// This is the Go port of Lucene's org.apache.lucene.facet.FacetUtils.
type FacetUtils struct{}

// LiveDocsDISI wraps the given DocIdSetIterator and liveDocs into another DocIdSetIterator
// that returns non-deleted documents during iteration.
func LiveDocsDISI(it search.DocIdSetIterator, liveDocs util.Bits) search.DocIdSetIterator {
	return &liveDocsDISI{
		it:       it,
		liveDocs: liveDocs,
	}
}

type liveDocsDISI struct {
	it       search.DocIdSetIterator
	liveDocs util.Bits
}

func (l *liveDocsDISI) doNext(doc int) int {
	// Find next document that is not deleted until we exhaust all documents
	for doc != search.NoMoreDocs && !l.liveDocs.Get(doc) {
		doc = l.it.NextDoc()
	}
	return doc
}

func (l *liveDocsDISI) DocID() int {
	return l.it.DocID()
}

func (l *liveDocsDISI) NextDoc() int {
	return l.doNext(l.it.NextDoc())
}

func (l *liveDocsDISI) Advance(target int) int {
	return l.doNext(l.it.Advance(target))
}

func (l *liveDocsDISI) NextDocWithFilter(filter search.DocIdSetIterator) int {
	// Lucene's FilterDocIdSetIterator doesn't override this in a way that's needed here,
	// but we should maintain the interface.
	return l.it.NextDocWithFilter(filter)
}

func (l *liveDocsDISI) HasNext() bool {
	return l.it.HasNext()
}
