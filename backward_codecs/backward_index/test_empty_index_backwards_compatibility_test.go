// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package backward_index

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"

	_ "github.com/FlavioCFOliveira/Gocene/codecs"
	_ "github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
)

// TestEmptyIndexBackwardsCompatibility verifies that an empty index created
// with a backward-registered codec name can be opened and queried correctly.
// An empty index has no documents but carries valid segment metadata.
//
// Port of org.apache.lucene.backward_index.TestEmptyIndexBackwardsCompatibility
// (Lucene 10.4.0, backward-codecs/src/test).
func TestEmptyIndexBackwardsCompatibility(t *testing.T) {
	base := newBwcTestBase(t)

	const codecName = "Lucene90"
	backwardCodec := base.registerBackwardCodec(codecName)

	dir := base.createDir()
	defer dir.Close()

	// Create empty index (no documents, just a commit).
	config := base.createConfig(backwardCodec)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Read segment infos — an empty index with only a commit and no documents
	// may produce zero or one segment depending on the flush policy. Both
	// outcomes are valid; we assert only that the directory is readable.
	if _, err := index.ReadSegmentInfos(dir); err != nil {
		t.Fatalf("ReadSegmentInfos: %v", err)
	}

	// Open reader - must succeed.
	reader := base.mustOpenReader(dir)
	defer base.mustClose(reader)

	if reader.NumDocs() != 0 {
		t.Fatalf("NumDocs: expected 0 for empty index, got %d", reader.NumDocs())
	}
}

// TestEmptyIndex_DefaultCodec verifies an empty index with the default codec.
func TestEmptyIndex_DefaultCodec(t *testing.T) {
	base := newBwcTestBase(t)
	dir := base.createDir()
	defer dir.Close()

	config := base.createConfig(nil)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	writer.Commit()
	writer.Close()

	reader := base.mustOpenReader(dir)
	defer base.mustClose(reader)

	if reader.NumDocs() != 0 {
		t.Fatalf("expected 0 docs, got %d", reader.NumDocs())
	}
}

// TestEmptyIndexRoundtrip verifies a round-trip where no documents are added
// but a commit is performed, using the default codec. The resulting index
// should be readable and report 0 docs.
func TestEmptyIndexRoundtrip(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	base := newBwcTestBase(t)
	config := base.createConfig(nil)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	// Commit without adding any documents (empty index).
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reader := base.mustOpenReader(dir)
	defer base.mustClose(reader)

	if reader.NumDocs() != 0 {
		t.Fatalf("expected 0 docs for empty index, got %d", reader.NumDocs())
	}
}
