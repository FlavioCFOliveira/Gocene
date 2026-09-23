// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
	testindex "github.com/FlavioCFOliveira/Gocene/tests/index"
)

// This file is the port of
// lucene/grouping/src/test/org/apache/lucene/search/grouping/AbstractGroupingTestCase.java
// (Apache Lucene 10.5.0): the base class for grouping related tests. Its
// protected members are rendered as package-level helpers of the test
// package.

// generateRandomNonEmptyString renders
// AbstractGroupingTestCase.generateRandomNonEmptyString().
func generateRandomNonEmptyString() string {
	var randomValue string
	for {
		// B/c of DV based impl we can't see the difference between an empty string and a null
		// value. For that reason we don't generate empty string groups.
		randomValue = randomRealisticUnicodeString(random())
		// randomValue = _TestUtil.randomSimpleString(random());
		if randomValue != "" {
			return randomValue
		}
	}
}

// assertScoreDocsEquals renders the static
// AbstractGroupingTestCase.assertScoreDocsEquals(ScoreDoc[], ScoreDoc[]).
func assertScoreDocsEquals(t testing.TB, expected, actual []*search.ScoreDoc) {
	t.Helper()
	if len(expected) != len(actual) {
		t.Fatalf("scoreDocs length: expected %d, got %d", len(expected), len(actual))
	}
	for i := range expected {
		if expected[i].Doc != actual[i].Doc {
			t.Fatalf("scoreDocs[%d].doc: expected %d, got %d", i, expected[i].Doc, actual[i].Doc)
		}
		if expected[i].Score != actual[i].Score {
			t.Fatalf("scoreDocs[%d].score: expected %v, got %v", i, expected[i].Score, actual[i].Score)
		}
	}
}

// shard renders the protected static class AbstractGroupingTestCase.Shard.
type shard struct {
	directory store.Directory
	writer    *testindex.RandomIndexWriter
	searcher  *search.IndexSearcher
}

// newShard renders Shard().
func newShard(t testing.TB) *shard {
	t.Helper()
	directory := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMergePolicy(newLogMergePolicy())
	return &shard{directory: directory, writer: newRandomIndexWriterWithConfig(t, directory, iwc)}
}

// getIndexSearcher renders Shard.getIndexSearcher().
func (s *shard) getIndexSearcher(t testing.TB) *search.IndexSearcher {
	t.Helper()
	if s.searcher == nil {
		// BlockGroupingCollectorManager does not support INTRA_SEGMENT concurrency (requires more
		// work).
		s.searcher = newSearcherWithConcurrency(t, mustGetReader(t, s.writer),
			random().Intn(2) == 0, random().Intn(2) == 0)
	}
	return s.searcher
}

// close renders Shard.close().
func (s *shard) close(t testing.TB) {
	t.Helper()
	if s.searcher != nil {
		mustClose(t, s.searcher.GetIndexReader())
	}
	mustClose(t, s.writer, s.directory)
}
