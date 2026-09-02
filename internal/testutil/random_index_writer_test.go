// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package testutil

import (
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

func TestRandomIndexWriter_BasicOperations(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	dir := store.NewByteBuffersDirectory()

	riw, err := New(rng, dir)
	if err != nil {
		t.Fatalf("failed to create RandomIndexWriter: %v", err)
	}
	defer riw.Close()

	// Test AddDocument
	doc1 := []index.IndexableField{
		index.NewStringField("text", "hello world", index.Indexed),
	}
	seqNo, err := riw.AddDocument(doc1)
	if err != nil {
		t.Errorf("AddDocument failed: %v", err)
	}
	if seqNo <= 0 {
		t.Errorf("expected positive seqNo, got %d", seqNo)
	}

	// Test AddDocuments
	docs := [][]index.IndexableField{
		{index.NewStringField("text", "doc 2", index.Indexed)},
		{index.NewStringField("text", "doc 3", index.Indexed)},
	}
	seqNo, err = riw.AddDocuments(docs)
	if err != nil {
		t.Errorf("AddDocuments failed: %v", err)
	}
	if seqNo <= 0 {
		t.Errorf("expected positive seqNo, got %d", seqNo)
	}

	// Test Commit
	_, err = riw.Commit()
	if err != nil {
		t.Errorf("Commit failed: %v", err)
	}

	// Test GetReader
	reader, err := riw.GetReader()
	if err != nil {
		t.Errorf("GetReader failed: %v", err)
	}
	if reader == nil {
		t.Error("GetReader returned nil reader")
	}

	// Test ForceMerge
	err = riw.ForceMerge(1)
	if err != nil {
		t.Errorf("ForceMerge failed: %v", err)
	}

	// Test Flush
	err = riw.Flush()
	if err != nil {
		t.Errorf("Flush failed: %v", err)
	}
}

func TestRandomIndexWriter_RandomConfigs(t *testing.T) {
	for i := 0; i < 10; i++ {
		rng := rand.New(rand.NewSource(int64(i)))
		dir := store.NewByteBuffersDirectory()

		riw, err := New(rng, dir)
		if err != nil {
			t.Fatalf("iteration %d: failed to create RandomIndexWriter: %v", i, err)
		}

		// Add some docs to trigger maybeFlushOrCommit
		for j := 0; j < 200; j++ {
			doc := []index.IndexableField{
				index.NewStringField("text", "random doc", index.Indexed),
			}
			if _, err := riw.AddDocument(doc); err != nil {
				t.Fatalf("iteration %d, doc %d: AddDocument failed: %v", i, j, err)
			}
		}

		if err := riw.Close(); err != nil {
			t.Errorf("iteration %d: Close failed: %v", i, err)
		}
	}
}

func TestRandomIndexWriter_UpdateDocument(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	dir := store.NewByteBuffersDirectory()
	riw, err := New(rng, dir)
	if err != nil {
		t.Fatalf("failed to create RandomIndexWriter: %v", err)
	}
	defer riw.Close()

	term := &index.Term{Field: "id", Term: []byte("1")}
	doc := []index.IndexableField{
		index.NewStringField("text", "initial", index.Indexed),
	}
	riw.AddDocument(doc)

	_, err = riw.UpdateDocument(term, doc)
	if err != nil {
		t.Errorf("UpdateDocument failed: %v", err)
	}
}

func TestRandomIndexWriter_MockIndexWriter(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	dir := store.NewByteBuffersDirectory()
	conf := index.NewIndexWriterConfig(nil) // Use default analyzer

	iw, err := MockIndexWriter(dir, conf, rng)
	if err != nil {
		t.Fatalf("MockIndexWriter failed: %v", err)
	}
	if iw == nil {
		t.Fatal("MockIndexWriter returned nil")
	}
	iw.Close()
}
