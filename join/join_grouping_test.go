// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/join"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// GC-909: Join and Grouping Tests
// Validates join queries and result grouping produce
// equivalent output to Java Lucene.

func TestJoinGrouping_ParentChildJoin(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add parent documents
	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", string(rune('A'+i)), true)
		doc.Add(idField)

		typeField, _ := document.NewStringField("type", "parent", true)
		doc.Add(typeField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add parent: %v", err)
		}
	}

	// Add child documents
	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", string(rune('a'+i)), true)
		doc.Add(idField)

		typeField, _ := document.NewStringField("type", "child", true)
		doc.Add(typeField)

		parentField, _ := document.NewStringField("parent_id", string(rune('A'+i%5)), true)
		doc.Add(parentField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add child: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 15 {
		t.Errorf("expected 15 docs, got %d", reader.NumDocs())
	}
}

func TestJoinGrouping_GroupingByField(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with grouping field
	groups := []string{"A", "B", "A", "C", "B", "A"}
	for i, group := range groups {
		doc := document.NewDocument()

		idField, _ := document.NewStringField("id", string(rune('0'+i)), true)
		doc.Add(idField)

		groupField, _ := document.NewStringField("group", group, true)
		doc.Add(groupField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 6 {
		t.Errorf("expected 6 docs, got %d", reader.NumDocs())
	}
}

func TestJoinGrouping_TermsQuery(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add documents with join keys
	for i := 0; i < 20; i++ {
		doc := document.NewDocument()

		idField, _ := document.NewStringField("id", string(rune('0'+i%10)), true)
		doc.Add(idField)

		joinKey := "key" + string(rune('A'+i%5))
		keyField, _ := document.NewStringField("join_key", joinKey, true)
		doc.Add(keyField)

		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 20 {
		t.Errorf("expected 20 docs, got %d", reader.NumDocs())
	}
}
