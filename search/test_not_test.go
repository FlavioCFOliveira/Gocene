// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestNot.java
// (Apache Lucene 10.5.0).

package search_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// Similarity unit test.
func TestNotNot(t *testing.T) {
	store := newDirectory()
	writer := newRandomIndexWriter(t, store)

	d1 := document.NewDocument()
	d1.Add(newTextField(t, "field", "a b", true))

	mustAddDocument(t, writer, d1)
	reader := mustGetReader(t, writer)

	searcher := newSearcher(t, reader)

	query := search.NewBooleanQueryBuilder()
	query.Add(search.NewTermQuery(index.NewTerm("field", "a")), search.SHOULD)
	query.Add(search.NewTermQuery(index.NewTerm("field", "b")), search.MUST_NOT)

	hits := mustSearch(t, searcher, query.Build(), 1000).ScoreDocs
	assertIntEquals(t, 0, len(hits))
	mustClose(t, writer, reader, store)
}
