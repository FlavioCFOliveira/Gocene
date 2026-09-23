// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs_test

// This file renders, for the external codecs_test package, the members of
// org.apache.lucene.tests.util.LuceneTestCase and TestUtil that the ported
// Lucene codec tests call, at the Lucene default scale (RANDOM_MULTIPLIER = 1,
// TEST_NIGHTLY = false).

import (
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
)

// luceneRandom renders LuceneTestCase.random(): a per-test source whose seed
// is logged.
func luceneRandom(t testing.TB) *rand.Rand {
	t.Helper()
	seed := time.Now().UnixNano()
	t.Logf("random seed: %d", seed)
	return rand.New(rand.NewSource(seed))
}

// luceneNewDirectory renders LuceneTestCase.newDirectory(): a
// MockDirectoryWrapper over an in-memory directory.
func luceneNewDirectory() *store.MockDirectoryWrapper {
	return store.NewMockDirectoryWrapper(store.NewByteBuffersDirectory())
}

// luceneNewMockAnalyzer renders new MockAnalyzer(random()).
func luceneNewMockAnalyzer() analysis.Analyzer {
	return testanalysis.NewMockAnalyzer(testanalysis.WHITESPACE, true, 0, nil, true)
}

// luceneNewIndexWriterConfig renders LuceneTestCase.newIndexWriterConfig(),
// which uses new MockAnalyzer(random()).
func luceneNewIndexWriterConfig() *index.IndexWriterConfig {
	return index.NewIndexWriterConfigWithAnalyzer(luceneNewMockAnalyzer())
}

// luceneAtLeast renders LuceneTestCase.atLeast(Random, int).
func luceneAtLeast(r *rand.Rand, i int) int {
	return luceneNextInt(r, i, i+i/2)
}

// luceneNextInt renders TestUtil.nextInt(Random, int, int): both bounds
// inclusive.
func luceneNextInt(r *rand.Rand, start, end int) int {
	return start + r.Intn(end-start+1)
}

// luceneStoredDocument renders StoredFields.document(int).
func luceneStoredDocument(t testing.TB, storedFields spi.StoredFields, docID int) *document.Document {
	t.Helper()
	visitor := document.NewDocumentStoredFieldVisitor()
	if err := storedFields.Document(docID, visitor); err != nil {
		t.Fatalf("document(%d): %v", docID, err)
	}
	return visitor.GetDocument()
}

// luceneDocGet renders Document.get(String): the string value of the first
// field with the given name, or "" with ok=false when absent.
func luceneDocGet(doc *document.Document, name string) (string, bool) {
	f := doc.Get(name)
	if f == nil {
		return "", false
	}
	return f.StringValue(), true
}

// luceneExpectPanic renders LuceneTestCase.expectThrows for the unchecked
// Java exceptions Gocene raises as panics.
func luceneExpectPanic(t testing.TB, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic")
		}
	}()
	fn()
}

// luceneLogMergePolicy is the method set of LogMergePolicy used by
// luceneNewLogMergePolicy.
type luceneLogMergePolicy interface {
	index.MergePolicy
	SetCalibrateSizeByDeletes(bool)
	SetTargetSearchConcurrency(int)
	SetMergeFactor(int)
	SetNoCFSRatio(float64)
	SetMaxCFSSegmentSizeMB(float64)
}

// luceneNewLogMergePolicy renders LuceneTestCase.newLogMergePolicy(Random).
func luceneNewLogMergePolicy(r *rand.Rand) luceneLogMergePolicy {
	var logmp luceneLogMergePolicy
	if r.Intn(2) == 0 {
		logmp = index.NewLogDocMergePolicy()
	} else {
		logmp = index.NewLogByteSizeMergePolicy()
	}
	logmp.SetCalibrateSizeByDeletes(r.Intn(2) == 0)
	logmp.SetTargetSearchConcurrency(luceneNextInt(r, 1, 16))
	if luceneRarely(r) {
		logmp.SetMergeFactor(luceneNextInt(r, 2, 9))
	} else {
		logmp.SetMergeFactor(luceneNextInt(r, 10, 50))
	}
	// LuceneTestCase.configureRandom(Random, MergePolicy)
	if r.Intn(2) == 0 {
		logmp.SetNoCFSRatio(0.1 + r.Float64()*0.8)
	} else if r.Intn(2) == 0 {
		logmp.SetNoCFSRatio(1.0)
	} else {
		logmp.SetNoCFSRatio(0.0)
	}
	if luceneRarely(r) {
		logmp.SetMaxCFSSegmentSizeMB(0.2 + r.Float64()*2.0)
	} else {
		logmp.SetMaxCFSSegmentSizeMB(math.Inf(1))
	}
	return logmp
}

// luceneRarely renders LuceneTestCase.rarely(Random) at the default scale.
func luceneRarely(r *rand.Rand) bool {
	return r.Intn(100) >= 99
}

// luceneRandomSimpleString renders TestUtil.randomSimpleString(Random, int
// minLength, int maxLength).
func luceneRandomSimpleString(r *rand.Rand, minLength, maxLength int) string {
	end := luceneNextInt(r, minLength, maxLength)
	if end == 0 {
		return ""
	}
	buffer := make([]byte, end)
	for i := range buffer {
		buffer[i] = byte(luceneNextInt(r, 'a', 'z'))
	}
	return string(buffer)
}

// luceneNumericValueInt64 renders IndexableField.numericValue().longValue().
func luceneNumericValueInt64(t testing.TB, v any) int64 {
	t.Helper()
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case int32:
		return int64(n)
	case float64:
		return int64(n)
	case float32:
		return int64(n)
	}
	t.Fatalf("numericValue %T is not a number", v)
	return 0
}
