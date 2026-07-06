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
)

// GC-929: CheckIndex Compatibility
// Test CheckIndex tool produces identical validation results on same index files.

func TestCheckIndexCompatibility_BasicValidation(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	for i := 0; i < 100; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", string(rune('0'+i%10)), true)
		doc.Add(idField)

		contentField, _ := document.NewTextField("content", "checkindex test", true)
		doc.Add(contentField)

		if err := writer.AddDocument(doc); err != nil {
			writer.Close()
			t.Fatalf("failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		writer.Close()
		t.Fatalf("failed to commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}

	checker, err := index.NewCheckIndex(dir)
	if err != nil {
		t.Fatalf("NewCheckIndex: %v", err)
	}
	defer checker.Close()

	result, err := checker.CheckIndex()
	if err != nil {
		t.Fatalf("CheckIndex: %v", err)
	}

	if !result.Clean || result.NumBadSegments > 0 {
		t.Errorf("index has errors: bad segments=%d, errors=%v", result.NumBadSegments, result.Errors)
	}
}

func TestCheckIndexCompatibility_SegmentValidation(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	for seg := 0; seg < 3; seg++ {
		for i := 0; i < 20; i++ {
			doc := document.NewDocument()
			idField, _ := document.NewStringField("id", string(rune('0'+(seg*20+i)%10)), true)
			doc.Add(idField)
			if err := writer.AddDocument(doc); err != nil {
				writer.Close()
				t.Fatalf("failed to add document: %v", err)
			}
		}
		if err := writer.Commit(); err != nil {
			writer.Close()
			t.Fatalf("failed to commit: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}

	checker, err := index.NewCheckIndex(dir)
	if err != nil {
		t.Fatalf("NewCheckIndex: %v", err)
	}
	defer checker.Close()

	result, err := checker.CheckIndex()
	if err != nil {
		t.Fatalf("CheckIndex: %v", err)
	}

	if result.NumSegments != 3 {
		t.Errorf("NumSegments = %d, want 3", result.NumSegments)
	}
	if !result.Clean || result.NumBadSegments > 0 {
		t.Errorf("index has errors: bad segments=%d, errors=%v", result.NumBadSegments, result.Errors)
	}
}

func TestCheckIndexCompatibility_FieldInfosValidation(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	for i := 0; i < 50; i++ {
		doc := document.NewDocument()

		idField, _ := document.NewStringField("id", string(rune('0'+i%10)), true)
		doc.Add(idField)

		titleField, _ := document.NewTextField("title", "title", true)
		doc.Add(titleField)

		contentField, _ := document.NewTextField("content", "content", true)
		doc.Add(contentField)

		if err := writer.AddDocument(doc); err != nil {
			writer.Close()
			t.Fatalf("failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		writer.Close()
		t.Fatalf("failed to commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}

	checker, err := index.NewCheckIndex(dir)
	if err != nil {
		t.Fatalf("NewCheckIndex: %v", err)
	}
	defer checker.Close()

	result, err := checker.CheckIndex()
	if err != nil {
		t.Fatalf("CheckIndex: %v", err)
	}

	if !result.Clean || result.NumBadSegments > 0 {
		t.Errorf("index has errors: bad segments=%d, errors=%v", result.NumBadSegments, result.Errors)
	}
}

func BenchmarkCheckIndexCompatibility_Validation(b *testing.B) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, _ := index.NewIndexWriter(dir, config)
	for i := 0; i < 1000; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", string(rune('0'+i%10)), true)
		doc.Add(idField)
		writer.AddDocument(doc)
	}
	writer.Commit()
	writer.Close()

	checker, err := index.NewCheckIndex(dir)
	if err != nil {
		b.Fatal("NewCheckIndex not implemented")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		checker.CheckIndex() //nolint:errcheck // benchmark ignores errors
	}
}
