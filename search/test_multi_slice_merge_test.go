// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestMultiSliceMerge.java
// (Apache Lucene 10.5.0).

package search_test

import (
	"math"
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	testsearch "github.com/FlavioCFOliveira/Gocene/tests/search"
)

// multiSliceMergeTestCase renders the fields of TestMultiSliceMerge.
type multiSliceMergeTestCase struct {
	reader1 *index.DirectoryReader
	reader2 *index.DirectoryReader
}

// msmSetUp renders setUp(); the returned function renders tearDown().
func msmSetUp(t *testing.T) (*multiSliceMergeTestCase, func()) {
	t.Helper()
	tc := &multiSliceMergeTestCase{}
	dir1 := newDirectory()
	dir2 := newDirectory()
	rnd := random()
	iwc1 := newIndexWriterConfig()
	iwc1.SetMergePolicy(newLogMergePolicy())
	iw1 := newRandomIndexWriterWithConfig(t, dir1, iwc1)
	for i := 0; i < 100; i++ {
		mustAddDocument(t, iw1, msmDoc(t, i))
		if rnd.Intn(2) == 0 {
			mustClose(t, mustGetReader(t, iw1))
		}
	}
	tc.reader1 = mustGetReader(t, iw1)
	mustClose(t, iw1)

	iwc2 := newIndexWriterConfig()
	iwc2.SetMergePolicy(newLogMergePolicy())
	iw2 := newRandomIndexWriterWithConfig(t, dir2, iwc2)
	for i := 0; i < 100; i++ {
		mustAddDocument(t, iw2, msmDoc(t, i))
		if rnd.Intn(2) == 0 {
			mustCommit(t, iw2)
		}
	}
	tc.reader2 = mustGetReader(t, iw2)
	mustClose(t, iw2)
	return tc, func() {
		mustClose(t, tc.reader1, tc.reader2, dir1, dir2)
	}
}

// msmDoc builds the document the two setUp loops index.
func msmDoc(t *testing.T, i int) *document.Document {
	t.Helper()
	doc := document.NewDocument()
	doc.Add(newStringField(t, "field", strconv.Itoa(i), false))
	doc.Add(newStringField(t, "field2", strconv.FormatBool(i%2 == 0), false))
	dv, err := document.NewSortedDocValuesField("field2", []byte(strconv.FormatBool(i%2 == 0)))
	if err != nil {
		t.Fatalf("new SortedDocValuesField: %v", err)
	}
	doc.Add(dv)
	return doc
}

// msmDirectExecutor renders the Executor `runnable -> runnable.run()`.
type msmDirectExecutor struct{}

func (msmDirectExecutor) Execute(runnable func()) { runnable() }

func TestMultiSliceMergeMultipleSlicesOfSameIndexSearcher(t *testing.T) {
	tc, tearDown := msmSetUp(t)
	defer tearDown()
	executor1 := msmDirectExecutor{}
	executor2 := msmDirectExecutor{}
	searchers := []*search.IndexSearcher{
		search.NewIndexSearcherWithExecutor(tc.reader1, executor1),
		search.NewIndexSearcherWithExecutor(tc.reader2, executor2),
	}

	query := search.NewMatchAllDocsQuery()

	topDocs1 := mustSearch(t, searchers[0], query, math.MaxInt32)
	topDocs2 := mustSearch(t, searchers[1], query, math.MaxInt32)

	testsearch.CheckEqual(t, query, topDocs1.ScoreDocs, topDocs2.ScoreDocs)
}

func TestMultiSliceMergeMultipleSlicesOfMultipleIndexSearchers(t *testing.T) {
	tc, tearDown := msmSetUp(t)
	defer tearDown()
	executor1 := msmDirectExecutor{}
	executor2 := msmDirectExecutor{}
	searchers := []*search.IndexSearcher{
		search.NewIndexSearcherWithExecutor(tc.reader1, executor1),
		search.NewIndexSearcherWithExecutor(tc.reader2, executor2),
	}

	query := search.NewMatchAllDocsQuery()

	topDocs1 := mustSearch(t, searchers[0], query, math.MaxInt32)
	topDocs2 := mustSearch(t, searchers[1], query, math.MaxInt32)

	assertIntEquals(t, len(topDocs1.ScoreDocs), len(topDocs2.ScoreDocs))

	for i := 0; i < len(topDocs1.ScoreDocs); i++ {
		topDocs1.ScoreDocs[i].ShardIndex = 0
		topDocs2.ScoreDocs[i].ShardIndex = 1
	}

	shardHits := []*search.TopDocs{topDocs1, topDocs2}

	mergedHits1 := search.Merge(0, len(topDocs1.ScoreDocs), shardHits, nil)
	mergedHits2 := search.Merge(0, len(topDocs1.ScoreDocs), shardHits, nil)

	testsearch.CheckEqual(t, query, mergedHits1.ScoreDocs, mergedHits2.ScoreDocs)
}
