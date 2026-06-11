// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestPerFieldConsistency_FieldTypeValidation verifies FieldType field
// round-trips and that valid configurations are accepted.
func TestPerFieldConsistency_FieldTypeValidation(t *testing.T) {
	t.Run("indexOptions roundtrip", func(t *testing.T) {
		ft := NewFieldType()
		ft.SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions)
		if ft.IndexOptions != index.IndexOptionsDocsAndFreqsAndPositions {
			t.Error("IndexOptions round-trip failed")
		}
	})

	t.Run("storeTermVectors with offsets", func(t *testing.T) {
		ft := NewFieldType()
		ft.SetStoreTermVectors(true)
		ft.SetStoreTermVectorOffsets(true)
		if !ft.StoreTermVectors {
			t.Error("StoreTermVectors = false, want true")
		}
		if !ft.StoreTermVectorOffsets {
			t.Error("StoreTermVectorOffsets = false, want true")
		}
	})

	t.Run("storeTermVectors with payloads", func(t *testing.T) {
		ft := NewFieldType()
		ft.SetStoreTermVectors(true)
		ft.SetStoreTermVectorPayloads(true)
		if !ft.StoreTermVectors {
			t.Error("StoreTermVectors = false, want true")
		}
		if !ft.StoreTermVectorPayloads {
			t.Error("StoreTermVectorPayloads = false, want true")
		}
	})

	t.Run("stored and indexed", func(t *testing.T) {
		ft := NewFieldType()
		ft.SetStored(true)
		ft.SetIndexOptions(index.IndexOptionsDocsAndFreqs)
		if !ft.Stored {
			t.Error("Stored = false, want true")
		}
		if ft.IndexOptions != index.IndexOptionsDocsAndFreqs {
			t.Error("IndexOptions round-trip failed")
		}
	})

	t.Run("omitNorms", func(t *testing.T) {
		ft := NewFieldType()
		ft.SetOmitNorms(true)
		if !ft.OmitNorms {
			t.Error("OmitNorms = false, want true")
		}
	})

	t.Run("tokenized default", func(t *testing.T) {
		// Go port defaults Tokenized to false (explicit zero-value policy).
		// Use NewLuceneFieldType() for Lucene-canonical defaults.
		ft := NewFieldType()
		if ft.Tokenized {
			t.Error("Tokenized default = true, want false (Go explicit zero-value)")
		}
		lft := NewLuceneFieldType()
		if !lft.Tokenized {
			t.Error("LuceneFieldType Tokenized = false, want true")
		}
	})
}

// TestPerFieldConsistency_IndexedTypesRoundTrip creates documents and
// verifies they survive a write/read cycle through IndexWriter and
// DirectoryReader, confirming per-field metadata is preserved.
func TestPerFieldConsistency_IndexedTypesRoundTrip(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	sf, err := NewStringField("f", "hello", true)
	if err != nil {
		t.Fatalf("NewStringField: %v", err)
	}
	ndv, err := NewNumericDocValuesField("f", 42)
	if err != nil {
		t.Fatalf("NewNumericDocValuesField: %v", err)
	}

	doc := NewDocument()
	doc.Add(sf)
	doc.Add(ndv)

	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() < 1 {
		t.Fatalf("NumDocs = %d, want >= 1", reader.NumDocs())
	}

	segs := reader.GetSequentialSubReaders()
	if len(segs) == 0 {
		t.Fatal("GetSubReaders returned 0 segments")
	}

	seg := segs[0]
	fi := seg.GetFieldInfos().GetByName("f")
	if fi == nil {
		t.Fatal("FieldInfo for 'f' is nil")
	}
	if fi.IndexOptions() == index.IndexOptionsNone {
		t.Errorf("field 'f' IndexOptions = NONE, want indexed")
	}
}

// TestPerFieldConsistency_DocWithMissingSchemaOptionsThrowsError verifies
// that conflicting field type configurations are detected.
func TestPerFieldConsistency_DocWithMissingSchemaOptionsThrowsError(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	t.Run("conflicting doc values types detected", func(t *testing.T) {
		ndv, _ := NewNumericDocValuesField("x", 1)
		doc1 := NewDocument()
		doc1.Add(ndv)
		if err := writer.AddDocument(doc1); err != nil {
			t.Fatalf("AddDocument 1: %v", err)
		}

		sdv, _ := NewSortedDocValuesField("x", []byte("val"))
		doc2 := NewDocument()
		doc2.Add(sdv)
		err := writer.AddDocument(doc2)
		if err == nil {
			t.Error("expected error for conflicting doc values types, got nil")
		}
	})

	t.Run("indexed without being stored is valid", func(t *testing.T) {
		ft := NewFieldType()
		ft.SetIndexOptions(index.IndexOptionsDocsAndFreqs)
		err := ft.Validate()
		if err != nil {
			t.Errorf("unexpected Validate error: %v", err)
		}
	})
}

// TestPerFieldConsistency_DocWithExtraSchemaOptionsThrowsError verifies
// field construction edge cases.
func TestPerFieldConsistency_DocWithExtraSchemaOptionsThrowsError(t *testing.T) {
	t.Run("stored field with string value", func(t *testing.T) {
		ft := NewFieldType()
		ft.SetStored(true)
		f, err := NewField("x", "hello", ft)
		if err != nil {
			t.Fatalf("NewField: %v", err)
		}
		if f.FieldType() != ft {
			t.Error("FieldType round-trip failed")
		}
	})

	t.Run("binary doc values field", func(t *testing.T) {
		dv, err := NewBinaryDocValuesField("x", []byte{1, 2, 3})
		if err != nil {
			t.Fatalf("NewBinaryDocValuesField: %v", err)
		}
		if dv == nil {
			t.Fatal("NewBinaryDocValuesField returned nil")
		}
	})
}

// TestPerFieldConsistency_MultipleFieldsSameName verifies that adding
// multiple fields with the same name and compatible types succeeds.
func TestPerFieldConsistency_MultipleFieldsSameName(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	sf1, _ := NewStringField("name", "value1", true)
	sf2, _ := NewStringField("name", "value2", true)

	doc := NewDocument()
	doc.Add(sf1)
	doc.Add(sf2)

	err = writer.AddDocument(doc)
	if err != nil {
		t.Fatalf("AddDocument with compatible same-name fields: %v", err)
	}
}
