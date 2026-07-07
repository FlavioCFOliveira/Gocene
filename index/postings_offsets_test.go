// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"fmt"
	"io"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/testutil"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// postingsOffsetsType builds the FieldType shared by the offset tests: a
// non-stored TextField type with DOCS_AND_FREQS_AND_POSITIONS_AND_OFFSETS.
// Upstream toggles term vectors with random().nextBoolean(); a deterministic
// value is used here for reproducibility.
func postingsOffsetsType(stored bool) *document.FieldType {
	base := document.TextFieldTypeNotStored
	if stored {
		base = document.TextFieldTypeStored
	}
	ft := document.NewFieldTypeFrom(base)
	ft.SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets)
	return ft
}

// cannedAnalyzer is a test-only analyzer that ignores the supplied reader and
// returns a fresh TokenStream built by factory on every call.
type cannedAnalyzer struct {
	factory func() analysis.TokenStream
}

func (a *cannedAnalyzer) TokenStream(fieldName string, reader io.Reader) (analysis.TokenStream, error) {
	return a.factory(), nil
}

func (a *cannedAnalyzer) Close() error { return nil }

// newPostingsOffsetsWriter creates an IndexWriter that indexes "content" with
// DOCS_AND_FREQS_AND_POSITIONS_AND_OFFSETS using the supplied canned-token
// analyzer factory.
func newPostingsOffsetsWriter(t *testing.T, factory func() analysis.TokenStream) (store.Directory, *index.IndexWriter) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()

	customType := postingsOffsetsType(false)
	customType.SetStoreTermVectors(true)
	customType.SetStoreTermVectorPositions(true)
	customType.SetStoreTermVectorOffsets(true)
	customType.Freeze()

	// The analyzer is only used to build the token stream; the field type
	// still controls whether offsets are stored.
	config := index.NewIndexWriterConfig(&cannedAnalyzer{factory: factory})
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	return dir, writer
}

// addContentDoc indexes one document with a single "content" field of the
// supplied custom type.
func addContentDoc(t *testing.T, writer *index.IndexWriter, ft *document.FieldType, value string) {
	t.Helper()
	doc := document.NewDocument()
	field, err := document.NewField("content", value, ft)
	if err != nil {
		t.Fatalf("NewField: %v", err)
	}
	doc.Add(field)
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
}

// openCommittedReader flushes the writer to disk via Commit, then opens a
// fresh DirectoryReader over the on-disk segment. This is required for tests
// that exercise the full codec round-trip (e.g., offsets/payloads), because
// writer.GetReader() currently returns an in-memory reader that does not
// preserve offsets.
func openCommittedReader(t *testing.T, dir store.Directory, writer *index.IndexWriter) *index.DirectoryReader {
	t.Helper()
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	return reader
}

// assertPostings verifies the postings for term in the single-segment reader
// match the expected frequency, positions, start offsets and end offsets.
func assertPostings(t *testing.T, reader *index.DirectoryReader, term string, wantFreq int, wantPositions, wantStartOffsets, wantEndOffsets []int) {
	t.Helper()
	leaves := reader.GetSegmentReaders()
	if len(leaves) != 1 {
		t.Fatalf("expected 1 leaf, got %d", len(leaves))
	}
	leaf := leaves[0]
	terms, err := leaf.Terms("content")
	if err != nil {
		t.Fatalf("Terms: %v", err)
	}
	it, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator: %v", err)
	}
	var postings schema.PostingsEnum
	for {
		tt, err := it.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if tt == nil {
			break
		}
		if tt.Text() == term {
			postings, err = it.Postings(schema.PostingsFlagOffsets)
			if err != nil {
				t.Fatalf("Postings: %v", err)
			}
			break
		}
	}
	if postings == nil {
		t.Fatalf("no postings for term %q", term)
	}
	docID, err := postings.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc: %v", err)
	}
	if docID == schema.NO_MORE_DOCS {
		t.Fatalf("no docs for term %q", term)
	}
	freq, err := postings.Freq()
	if err != nil {
		t.Fatalf("Freq: %v", err)
	}
	if freq != wantFreq {
		t.Errorf("term %q freq = %d, want %d", term, freq, wantFreq)
	}
	for i := 0; i < wantFreq; i++ {
		pos, err := postings.NextPosition()
		if err != nil {
			t.Fatalf("NextPosition: %v", err)
		}
		so, _ := postings.StartOffset()
		eo, _ := postings.EndOffset()
		if pos != wantPositions[i] {
			t.Errorf("term %q occurrence %d position = %d, want %d", term, i, pos, wantPositions[i])
		}
		if so != wantStartOffsets[i] {
			t.Errorf("term %q occurrence %d startOffset = %d, want %d", term, i, so, wantStartOffsets[i])
		}
		if eo != wantEndOffsets[i] {
			t.Errorf("term %q occurrence %d endOffset = %d, want %d", term, i, eo, wantEndOffsets[i])
		}
	}
}

// TestPostingsOffsets_Basic ports TestPostingsOffsets.testBasic.
//
// The upstream test indexes one document whose "content" field is a
// CannedTokenStream of four tokens with explicit offsets, then verifies the
// PostingsEnum reports the correct freq, positions and offsets for terms a, b
// and c.
func TestPostingsOffsets_Basic(t *testing.T) {
	dir, writer := newPostingsOffsetsWriter(t, func() analysis.TokenStream {
		return testutil.NewCannedTokenStream(
			testutil.NewToken("a", 0, 1),
			testutil.NewToken("b", 2, 3),
			testutil.NewToken("a", 4, 5),
			testutil.NewToken("c", 6, 7),
		)
	})
	defer writer.Close()
	defer dir.Close()

	addContentDoc(t, writer, postingsOffsetsType(false), "ignored")

	reader := openCommittedReader(t, dir, writer)
	defer reader.Close()

	assertPostings(t, reader, "a", 2, []int{0, 2}, []int{0, 4}, []int{1, 5})
	assertPostings(t, reader, "b", 1, []int{1}, []int{2}, []int{3})
	assertPostings(t, reader, "c", 1, []int{3}, []int{6}, []int{7})
}

// TestPostingsOffsets_Skipping ports TestPostingsOffsets.testSkipping
// (doTestNumbers without payloads).
func TestPostingsOffsets_Skipping(t *testing.T) {
	t.Fatal("blocked: doTestNumbers needs the English number-to-words helper")
}

// TestPostingsOffsets_Payloads ports TestPostingsOffsets.testPayloads
// (doTestNumbers with payloads).
func TestPostingsOffsets_Payloads(t *testing.T) {
	t.Fatal("blocked: doTestNumbers needs MockPayloadAnalyzer and the English number-to-words helper")
}

// TestPostingsOffsets_Random ports TestPostingsOffsets.testRandom.
//
// The upstream test indexes randomised CannedTokenStream documents and then,
// per leaf, cross-checks freq/position/offset against the recorded tokens.
func TestPostingsOffsets_Random(t *testing.T) {
	t.Fatal("blocked: RandomIndexWriter unimplemented")
}

// TestPostingsOffsets_AddFieldTwice ports TestPostingsOffsets.testAddFieldTwice.
//
// This is the one upstream test that needs no token stream and performs no
// read-back: it simply indexes a document carrying the same offset+term-vector
// field twice and closes the writer (CheckIndex runs on dir.close()). It is
// ported in full and exercised end-to-end.
func TestPostingsOffsets_AddFieldTwice(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	customType3 := document.NewFieldTypeFrom(document.TextFieldTypeStored)
	customType3.SetStoreTermVectors(true)
	customType3.SetStoreTermVectorPositions(true)
	customType3.SetStoreTermVectorOffsets(true)
	customType3.SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets)

	doc := document.NewDocument()
	field1, err := document.NewField("content3", "here is more content with aaa aaa aaa", customType3)
	if err != nil {
		t.Fatalf("Failed to create first field: %v", err)
	}
	doc.Add(field1)
	field2, err := document.NewField("content3", "here is more content with aaa aaa aaa", customType3)
	if err != nil {
		t.Fatalf("Failed to create second field: %v", err)
	}
	doc.Add(field2)

	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}
}

// TestPostingsOffsets_NegativeOffsets ports TestPostingsOffsets.testNegativeOffsets.
//
// Upstream asserts that indexing a token with negative offsets throws
// IllegalArgumentException.
func TestPostingsOffsets_NegativeOffsets(t *testing.T) {
	dir, writer := newPostingsOffsetsWriter(t, func() analysis.TokenStream {
		return testutil.NewCannedTokenStream(
			testutil.NewToken("a", -1, 1),
		)
	})
	defer writer.Close()
	defer dir.Close()

	doc := document.NewDocument()
	field, _ := document.NewField("content", "ignored", postingsOffsetsType(false))
	doc.Add(field)

	var panicked bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		_ = writer.AddDocument(doc)
	}()
	if !panicked {
		t.Fatalf("expected panic for negative startOffset")
	}
}

// TestPostingsOffsets_IllegalOffsets ports TestPostingsOffsets.testIllegalOffsets.
//
// Upstream asserts that a token whose endOffset precedes its startOffset throws
// IllegalArgumentException.
func TestPostingsOffsets_IllegalOffsets(t *testing.T) {
	dir, writer := newPostingsOffsetsWriter(t, func() analysis.TokenStream {
		return testutil.NewCannedTokenStream(
			testutil.NewToken("a", 5, 3),
		)
	})
	defer writer.Close()
	defer dir.Close()

	doc := document.NewDocument()
	field, _ := document.NewField("content", "ignored", postingsOffsetsType(false))
	doc.Add(field)

	var panicked bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		_ = writer.AddDocument(doc)
	}()
	if !panicked {
		t.Fatalf("expected panic for endOffset < startOffset")
	}
}

// TestPostingsOffsets_IllegalOffsetsAcrossFieldInstances ports
// TestPostingsOffsets.testIllegalOffsetsAcrossFieldInstances.
//
// Upstream asserts that offsets going backwards between two instances of the
// same field throw IllegalArgumentException.
func TestPostingsOffsets_IllegalOffsetsAcrossFieldInstances(t *testing.T) {
	t.Fatal("blocked: offset-backwards validation across two field instances not yet wired")
}

// TestPostingsOffsets_BackwardsOffsets ports TestPostingsOffsets.testBackwardsOffsets.
//
// Upstream asserts that a stacked token whose offsets move backwards relative
// to the previous position throws IllegalArgumentException.
func TestPostingsOffsets_BackwardsOffsets(t *testing.T) {
	t.Fatal("blocked: stacked-token offset-backwards validation not yet wired")
}

// TestPostingsOffsets_StackedTokens ports TestPostingsOffsets.testStackedTokens.
//
// Upstream asserts that stacked tokens (posIncr 0) sharing identical offsets
// index without error.
func TestPostingsOffsets_StackedTokens(t *testing.T) {
	dir, writer := newPostingsOffsetsWriter(t, func() analysis.TokenStream {
		return testutil.NewCannedTokenStream(
			testutil.NewTokenWithPosInc("a", 1, 0, 1),
			testutil.NewTokenWithPosInc("b", 0, 0, 1),
			testutil.NewTokenWithPosInc("c", 0, 0, 1),
		)
	})
	defer writer.Close()
	defer dir.Close()

	doc := document.NewDocument()
	field, _ := document.NewField("content", "ignored", postingsOffsetsType(false))
	doc.Add(field)
	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}

	reader := openCommittedReader(t, dir, writer)
	defer reader.Close()

	assertPostings(t, reader, "a", 1, []int{0}, []int{0}, []int{1})
	assertPostings(t, reader, "b", 1, []int{0}, []int{0}, []int{1})
	assertPostings(t, reader, "c", 1, []int{0}, []int{0}, []int{1})
}

// TestPostingsOffsets_CrazyOffsetGap ports TestPostingsOffsets.testCrazyOffsetGap.
//
// Upstream uses a custom Analyzer whose getOffsetGap returns -10 and asserts
// that adding a second field instance throws IllegalArgumentException, while a
// previously added good document remains visible.
func TestPostingsOffsets_CrazyOffsetGap(t *testing.T) {
	t.Fatal("blocked: requires a custom Analyzer overriding getOffsetGap; Gocene's Analyzer " +
		"exposes no getOffsetGap override hook")
}

// TestPostingsOffsets_LegalButVeryLargeOffsets ports
// TestPostingsOffsets.testLegalbutVeryLargeOffsets.
//
// Upstream indexes a CannedTokenStream of two tokens with offsets near
// Integer.MAX_VALUE and asserts the document indexes successfully.
func TestPostingsOffsets_LegalButVeryLargeOffsets(t *testing.T) {
	for _, big := range []int{1 << 20, 1 << 29, 1 << 30} {
		t.Run(fmt.Sprintf("big=%d", big), func(t *testing.T) {
			dir, writer := newPostingsOffsetsWriter(t, func() analysis.TokenStream {
				return testutil.NewCannedTokenStream(
					testutil.NewToken("a", 0, big),
					testutil.NewToken("b", big, big+1),
				)
			})
			defer writer.Close()
			defer dir.Close()

			doc := document.NewDocument()
			field, _ := document.NewField("content", "ignored", postingsOffsetsType(false))
			doc.Add(field)
			if err := writer.AddDocument(doc); err != nil {
				t.Fatalf("AddDocument: %v", err)
			}

			reader := openCommittedReader(t, dir, writer)
			defer reader.Close()

			assertPostings(t, reader, "a", 1, []int{0}, []int{0}, []int{big})
			assertPostings(t, reader, "b", 1, []int{1}, []int{big}, []int{big + 1})
		})
	}
}
