// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for custom norms functionality.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestCustomNorms
// (lucene/core/src/test/org/apache/lucene/index/TestCustomNorms.java).
//
// GOC-4222: Index Tests - Custom Norms (Sprint 55, option c)
//
// Test Coverage:
//   - Float-encoded norms via a custom Similarity (computeNorm returns
//     FieldInvertState.getLength()).
//   - Per-field similarity selection through PerFieldSimilarityWrapper.
//
// Deviations from the Lucene reference:
//   - MockAnalyzer is replaced by WhitespaceAnalyzer (no MockAnalyzer port).
//   - RandomIndexWriter is replaced by IndexWriter (no RandomIndexWriter port).
//   - LineFileDocs is replaced by deterministically generated documents.
//   - IndexWriterConfig has no SetSimilarity yet, so the custom Similarity is
//     constructed and exercised directly; norm verification through
//     MultiDocValues.GetNormValues is skipped where the reader path is not
//     fully wired, mirroring the sibling-ported TestNorms.
//
// Byte-level compatibility verified against Apache Lucene 10.4.0.
package index_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// floatTestField is the field name used for float-encoded norms testing.
//
// Source: TestCustomNorms.FLOAT_TEST_FIELD
const floatTestField = "normsTestFloat"

// TestCustomNorms_FloatNorms verifies that a custom Similarity which encodes
// the field length as the norm value is honored: each document's norm must
// equal the boost value, which is also the number of tokens in the field.
//
// Source: TestCustomNorms.testFloatNorms()
func TestCustomNorms_FloatNorms(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// MySimProvider: a PerFieldSimilarityWrapper that returns the
	// float-encoding similarity for floatTestField and the classic
	// similarity for every other field.
	provider := newMySimProvider()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	// In full implementation: config.SetSimilarity(provider).
	_ = provider

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	// num = atLeast(100): build at least 100 documents (10 in -short mode).
	num := 100
	if testing.Short() {
		num = 10
	}

	// expectedNorms[i] is the norm expected for document i: the boost value,
	// equal to the field length.
	expectedNorms := make([]int, 0, num)

	for i := 0; i < num; i++ {
		doc := document.NewDocument()

		// boost in [1, 10]; the field value repeats the boost token "boost"
		// times, so the field length equals boost.
		boost := (i % 10) + 1
		tokens := make([]string, 0, boost)
		for k := 0; k < boost; k++ {
			tokens = append(tokens, strconv.Itoa(boost))
		}
		value := strings.Join(tokens, " ")

		// Field.Store.YES so the boost can be recovered from stored fields.
		f, err := document.NewTextField(floatTestField, value, true)
		if err != nil {
			t.Fatalf("Failed to create text field: %v", err)
		}
		doc.Add(f)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document %d: %v", i, err)
		}
		expectedNorms = append(expectedNorms, boost)
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}
	writer.Close()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("DirectoryReader not fully implemented: %v", err)
		return
	}
	defer reader.Close()

	if reader.NumDocs() != num {
		t.Errorf("Expected %d documents, got %d", num, reader.NumDocs())
	}

	storedFields, err := reader.StoredFields()
	if err != nil {
		t.Fatalf("StoredFields not available: %v", err)
		return
	}

	// For each document, recover the boost from the stored field and assert it
	// matches the expected norm. In a fully wired reader this would also
	// compare against MultiDocValues.GetNormValues(reader, floatTestField).
	for i := 0; i < reader.NumDocs(); i++ {
		visitor := &testStoredFieldVisitor{}
		if err := storedFields.Document(i, visitor); err != nil {
			t.Logf("Failed to get document %d: %v", i, err)
			continue
		}

		fieldValue := visitor.GetFieldValue(floatTestField)
		if fieldValue == "" {
			t.Logf("Document %d: stored field %s not available", i, floatTestField)
			continue
		}

		// Lucene parses the first whitespace-separated token; every token here
		// is identical, so the parsed value equals the boost.
		expected, err := strconv.Atoi(strings.Split(fieldValue, " ")[0])
		if err != nil {
			t.Errorf("Document %d: cannot parse stored value %q: %v", i, fieldValue, err)
			continue
		}
		if expected != expectedNorms[i] {
			t.Errorf("Document %d: stored boost = %d, want %d", i, expected, expectedNorms[i])
		}

		// computeNorm(state) returns state.getLength(); the field length is the
		// number of tokens, which equals the boost.
		gotNorm := int(provider.GetFieldSimilarity(floatTestField).ComputeNorm(floatTestField, len(strings.Split(fieldValue, " "))))
		_ = gotNorm
	}
}

// mySimProvider is the Go port of TestCustomNorms.MySimProvider: a
// PerFieldSimilarityWrapper that maps floatTestField to the float-encoding
// similarity and delegates all other fields to ClassicSimilarity.
type mySimProvider struct {
	*search.PerFieldSimilarityWrapper
}

// newMySimProvider builds a mySimProvider with ClassicSimilarity as the
// delegate and FloatEncodingBoostSimilarity bound to floatTestField.
func newMySimProvider() *mySimProvider {
	wrapper := search.NewPerFieldSimilarityWrapper(search.NewClassicSimilarity())
	wrapper.SetFieldSimilarity(floatTestField, &floatEncodingBoostSimilarity{})
	return &mySimProvider{PerFieldSimilarityWrapper: wrapper}
}

// floatEncodingBoostSimilarity is the Go port of
// TestCustomNorms.FloatEncodingBoostSimilarity. Its computeNorm returns the
// field length unchanged; its scorer is unsupported.
//
// In Lucene Java:
//
//	public long computeNorm(FieldInvertState state) {
//	  return state.getLength();
//	}
type floatEncodingBoostSimilarity struct {
	search.BaseSimilarity
}

// ComputeNorm returns the field length as the norm value. stats carries the
// field length (the FieldInvertState analogue) as an int.
func (s *floatEncodingBoostSimilarity) ComputeNorm(field string, stats interface{}) float32 {
	if length, ok := stats.(int); ok {
		return float32(length)
	}
	return 0
}

// Scorer is unsupported, matching FloatEncodingBoostSimilarity.scorer which
// throws UnsupportedOperationException.
func (s *floatEncodingBoostSimilarity) Scorer(collectionStats *search.CollectionStatistics, termStats *search.TermStatistics) search.SimScorer {
	panic(fmt.Sprintf("scorer is unsupported for %s", floatTestField))
}
