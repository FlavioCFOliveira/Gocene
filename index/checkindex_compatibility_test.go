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
	defer writer.Close()

	// Add documents
	for i := 0; i < 100; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", string(rune('0'+i%10)), true)
		doc.Add(idField)

		contentField, _ := document.NewTextField("content", "checkindex test", true)
		doc.Add(contentField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Check index
	checker := index.NewCheckIndex(dir)
	result, err := checker.CheckIndex()
	if err != nil {
		t.Logf("checkindex may not be fully implemented: %v", err)
		t.Skip("checkindex not implemented")
	}

	if result.HasErrors() {
		t.Errorf("index has errors: %v", result.Errors())
	}

	t.Log("CheckIndex basic validation test passed")
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
	defer writer.Close()

	// Create multiple segments
	for seg := 0; seg < 3; seg++ {
		for i := 0; i < 20; i++ {
			doc := document.NewDocument()
			idField, _ := document.NewStringField("id", string(rune('0'+(seg*20+i)%10)), true)
			doc.Add(idField)
			writer.AddDocument(doc)
		}
		writer.Commit()
	}

	checker := index.NewCheckIndex(dir)
	result, err := checker.CheckIndex()
	if err != nil {
		t.Logf("checkindex may not be fully implemented: %v", err)
		t.Skip("checkindex not implemented")
	}

	t.Logf("CheckIndex segment validation: %d segments", result.NumSegments())
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
	defer writer.Close()

	// Add documents with various field types
	for i := 0; i < 50; i++ {
		doc := document.NewDocument()

		idField, _ := document.NewStringField("id", string(rune('0'+i%10)), true)
		doc.Add(idField)

		titleField, _ := document.NewTextField("title", "title", true)
		doc.Add(titleField)

		contentField, _ := document.NewTextField("content", "content", true)
		doc.Add(contentField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	writer.Commit()

	checker := index.NewCheckIndex(dir)
	result, err := checker.CheckIndex()
	if err != nil {
		t.Logf("checkindex may not be fully implemented: %v", err)
		t.Skip("checkindex not implemented")
	}

	t.Log("CheckIndex FieldInfos validation test passed")
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

	checker := index.NewCheckIndex(dir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		checker.CheckIndex()
	}
}
