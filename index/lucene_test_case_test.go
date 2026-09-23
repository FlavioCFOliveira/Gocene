// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
)

// This file renders, for the tests of package index, the members of
// org.apache.lucene.tests.util.LuceneTestCase that the ported Lucene index
// tests call. The test-framework package (tests/util) imports index, so the
// tests of package index cannot use it without an import cycle.

// newDirectory renders LuceneTestCase.newDirectory(): a MockDirectoryWrapper
// over an in-memory directory.
func newDirectory() *store.MockDirectoryWrapper {
	return store.NewMockDirectoryWrapper(store.NewByteBuffersDirectory())
}

// newIndexWriterConfig renders LuceneTestCase.newIndexWriterConfig(), which
// configures a MockAnalyzer(random()) (MockTokenizer.WHITESPACE, lower-case).
func newIndexWriterConfig() *IndexWriterConfig {
	return NewIndexWriterConfigWithAnalyzer(testanalysis.NewMockAnalyzer(testanalysis.WHITESPACE, true, 0, nil, true))
}

// atLeast renders LuceneTestCase.atLeast(int) with RANDOM_MULTIPLIER = 1 and
// TEST_NIGHTLY = false: a random value in [i, i + i/2].
func atLeast(i int) int {
	return i + rand.Intn(i/2+1)
}

// rarely renders LuceneTestCase.rarely() with RANDOM_MULTIPLIER = 1 and
// TEST_NIGHTLY = false: true with probability 1%.
func rarely() bool {
	return rand.Intn(100) >= 99
}

// iwDocStats renders IndexWriter.getDocStats(), failing the test on error.
func iwDocStats(t testing.TB, w *IndexWriter) DocStats {
	t.Helper()
	stats, err := w.GetDocStats()
	if err != nil {
		t.Fatalf("getDocStats: %v", err)
	}
	return stats
}
