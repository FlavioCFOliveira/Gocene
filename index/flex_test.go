// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestFlex_NonFlex ports org.apache.lucene.index.TestFlex#testNonFlex.
//
// It indexes 177 identical documents in two passes (the second pass
// force-merges to a single segment) and, after each pass, asserts that
// seeking past the only term in "field3" reports SeekStatus.END.
//
// Divergences from Lucene:
//   - MockAnalyzer is unavailable; WhitespaceAnalyzer is substituted.
//   - RandomIndexWriter / newLogMergePolicy randomization is unavailable; the
//     plain IndexWriter is used with an explicit LogMergePolicy, matching the
//     established pattern in TestBinaryTerms.
//   - Lucene's IndexReader.getReader (near-real-time) is unavailable; the index
//     is reopened from the directory after Commit.
//   - TermsEnum.SeekCeil returns *Term, not a SeekStatus; END is therefore
//     expressed as a nil return (see index/terms_enum_index.go).
func TestFlex_NonFlex(t *testing.T) {
	const docCount = 177

	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to open directory: %v", err)
	}
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	config.SetMaxBufferedDocs(7)
	config.SetMergePolicy(index.NewLogMergePolicy())

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}
	defer writer.Close()

	for iter := 0; iter < 2; iter++ {
		if iter == 0 {
			doc := document.NewDocument()
			for name, value := range map[string]string{
				"field1": "this is field1",
				"field2": "this is field2",
				"field3": "aaa",
				"field4": "bbb",
			} {
				f, err := document.NewTextField(name, value, false)
				if err != nil {
					t.Fatalf("Failed to create %s: %v", name, err)
				}
				doc.Add(f)
			}
			for i := 0; i < docCount; i++ {
				if err := writer.AddDocument(doc); err != nil {
					t.Fatalf("Failed to add document %d: %v", i, err)
				}
			}
		} else {
			if err := writer.ForceMerge(1); err != nil {
				t.Fatalf("ForceMerge failed: %v", err)
			}
		}

		if err := writer.Commit(); err != nil {
			t.Fatalf("Commit failed on iter %d: %v", iter, err)
		}

		reader, err := index.OpenDirectoryReader(dir)
		if err != nil {
			t.Fatalf("Failed to open reader on iter %d: %v", iter, err)
		}

		leaves, err := reader.Leaves()
		if err != nil {
			t.Fatalf("Failed to obtain leaves on iter %d: %v", iter, err)
		}

		// Emulate Lucene's MultiTerms.getTerms(r, "field3"): seeking past the
		// only term ("aaa") must report END on every leaf carrying the field.
		for _, leaf := range leaves {
			terms, err := leaf.LeafReader().Terms("field3")
			if err != nil {
				t.Fatalf("Failed to read terms on iter %d: %v", iter, err)
			}
			if terms == nil {
				continue
			}
			it, err := terms.GetIterator()
			if err != nil {
				t.Fatalf("Failed to obtain iterator on iter %d: %v", iter, err)
			}
			got, err := it.SeekCeil(index.NewTermFromBytes("field3", []byte("abc")))
			if err != nil {
				t.Fatalf("SeekCeil failed on iter %d: %v", iter, err)
			}
			if got != nil {
				t.Errorf("iter %d: expected END (nil) seeking past last term, got %q", iter, got.Text())
			}
		}

		if err := reader.Close(); err != nil {
			t.Fatalf("Failed to close reader on iter %d: %v", iter, err)
		}
	}
}

// TestFlex_TermOrd ports org.apache.lucene.index.TestFlex#testTermOrd.
//
// It indexes one document with field "f" holding three terms, force-merges to
// a single segment, and asserts the first enumerated term is non-nil.
//
// Divergences from Lucene:
//   - MockAnalyzer is unavailable; WhitespaceAnalyzer is substituted.
//   - TestUtil.alwaysPostingsFormat codec pinning is unavailable; the default
//     codec is used.
//   - Gocene's TermsEnum exposes no ord() operation, which Lucene itself treats
//     as optional; the ord() assertion is therefore omitted.
func TestFlex_TermOrd(t *testing.T) {
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to open directory: %v", err)
	}
	defer dir.Close()

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}
	defer writer.Close()

	doc := document.NewDocument()
	f, err := document.NewTextField("f", "a b c", false)
	if err != nil {
		t.Fatalf("Failed to create field f: %v", err)
	}
	doc.Add(f)

	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("Failed to add document: %v", err)
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge failed: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("Failed to open reader: %v", err)
	}
	defer reader.Close()

	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("Failed to obtain leaves: %v", err)
	}
	if len(leaves) != 1 {
		t.Fatalf("Expected exactly one leaf after ForceMerge(1), got %d", len(leaves))
	}

	terms, err := leaves[0].LeafReader().Terms("f")
	if err != nil {
		t.Fatalf("Failed to read terms: %v", err)
	}
	if terms == nil {
		t.Fatal("Expected terms for field f, got nil")
	}
	it, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("Failed to obtain iterator: %v", err)
	}
	got, err := it.Next()
	if err != nil {
		t.Fatalf("Next failed: %v", err)
	}
	if got == nil {
		t.Fatal("Expected a first term, got nil")
	}
}

var _ = util.NewBytesRef
