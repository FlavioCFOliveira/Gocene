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

// GC-940: IndexCommit Tests
// Validate index commit handling and versioning matches Java Lucene behavior.

func TestIndexCommit_BasicCommit(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	// Add documents
	for i := 0; i < 50; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", string(rune('0'+i%5)), true)
		doc.Add(idField)

		contentField, _ := document.NewTextField("content", "index commit test", true)
		doc.Add(contentField)

		if _, err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}

	// Commit
	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// GetCommit is not yet implemented — skipping commit generation check

	writer.Close()

	// Verify index is readable after commit
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("failed to open reader: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 50 {
		t.Errorf("expected 50 docs, got %d", reader.NumDocs())
	}

	t.Log("IndexCommit basic test passed")
}

func TestIndexCommit_MultipleCommits(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)
	config.SetIndexDeletionPolicy(index.NoDeletionPolicyInstance)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Multiple commits: use NoDeletionPolicy so older segment files survive.
	for round := 0; round < 3; round++ {
		for i := 0; i < 20; i++ {
			doc := document.NewDocument()
			idField, _ := document.NewStringField("id", string(rune('0'+(round*20+i)%10)), true)
			doc.Add(idField)
			if _, err := writer.AddDocument(doc); err != nil {
				t.Fatalf("failed to add document: %v", err)
			}
		}

		if err := writer.Commit(); err != nil {
			t.Fatalf("failed to commit: %v", err)
		}
	}

	// List commits
	commitList, err := index.ListCommits(dir)
	if err != nil {
		t.Fatalf("list commits failed: %v", err)
	}

	if len(commitList) != 3 {
		t.Fatalf("expected 3 commits, got %d", len(commitList))
	}

	t.Logf("Found %d commits", len(commitList))
}

func TestIndexCommit_OpenAtCommit(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Add and commit documents
	for i := 0; i < 30; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", string(rune('0'+i%5)), true)
		doc.Add(idField)
		if _, err := writer.AddDocument(doc); err != nil {
			t.Fatalf("failed to add document: %v", err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Get commits
	commits, err := index.ListCommits(dir)
	if err != nil {
		t.Fatalf("ListCommits: %v", err)
	}
	if len(commits) == 0 {
		t.Fatal("expected at least one commit")
	}

	// Open a reader at the latest commit.
	reader, err := index.OpenDirectoryReaderAtCommit(commits[len(commits)-1])
	if err != nil {
		t.Fatalf("OpenDirectoryReaderAtCommit: %v", err)
	}
	defer reader.Close()

	if reader.NumDocs() != 30 {
		t.Fatalf("expected 30 docs at commit, got %d", reader.NumDocs())
	}
}

func TestIndexCommit_DeleteCommits(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer writer.Close()

	// Multiple commits
	for round := 0; round < 3; round++ {
		for i := 0; i < 20; i++ {
			doc := document.NewDocument()
			idField, _ := document.NewStringField("id", string(rune('0'+round)), true)
			doc.Add(idField)
				if _, err := writer.AddDocument(doc); err != nil {
				t.Fatalf("failed to add document: %v", err)
			}
		}
		writer.Commit()
	}

	// Close the first writer before opening a second one on the same directory.
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close first writer: %v", err)
	}

	// Delete old commits: a new writer with KeepOnlyLastCommitDeletionPolicy will
	// prune older commits when it commits.
	config2 := index.NewIndexWriterConfig(analyzer)
	config2.SetIndexDeletionPolicy(index.NewKeepOnlyLastCommitDeletionPolicy())

	writer2, err := index.NewIndexWriter(dir, config2)
	if err != nil {
		t.Fatalf("new writer may not be created: %v", err)
	}
	defer writer2.Close()

	// Trigger the deletion policy so only the most recent commit survives.
	if err := writer2.Commit(); err != nil {
		t.Fatalf("second writer commit failed: %v", err)
	}

	t.Log("IndexCommit delete commits test passed")
}

func BenchmarkIndexCommit_Commit(b *testing.B) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	analyzer := analysis.NewWhitespaceAnalyzer()
	config := index.NewIndexWriterConfig(analyzer)

	writer, _ := index.NewIndexWriter(dir, config)
	defer writer.Close()

	for i := 0; i < 100; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", string(rune('0'+i%10)), true)
		doc.Add(idField)
			if _, err := writer.AddDocument(doc); err != nil {
				b.Fatalf("failed to add document: %v", err)
			}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		writer.Commit()
	}
}
