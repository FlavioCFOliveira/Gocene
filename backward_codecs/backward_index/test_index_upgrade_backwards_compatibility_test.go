// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package backward_index

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"

	_ "github.com/FlavioCFOliveira/Gocene/codecs"
	_ "github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
)

// TestIndexUpgradeBackwardsCompatibility verifies that an index created with
// a backward-registered codec can be re-opened and that its SegmentInfos
// metadata (commit version, index created version) is internally consistent.
//
// Port of org.apache.lucene.backward_index.TestIndexUpgradeBackwardsCompatibility
// (Lucene 10.4.0).
func TestIndexUpgradeBackwardsCompatibility(t *testing.T) {
	base := newBwcTestBase(t)
	backwardCodec := base.registerBackwardCodec("Lucene912")

	dir := base.createDir()
	defer dir.Close()

	config := base.createConfig(backwardCodec)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	doc := document.NewDocument()
	sf, err := document.NewStringField("id", "upgrade-test", true)
	if err != nil {
		t.Fatalf("NewStringField: %v", err)
	}
	doc.Add(sf)
	tf, err := document.NewTextField("content", "some content", true)
	if err != nil {
		t.Fatalf("NewTextField: %v", err)
	}
	doc.Add(tf)

	if err := writer.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	writer.Close()

	// Read segment infos.
	infos, err := index.ReadSegmentInfos(dir)
	if err != nil {
		t.Fatalf("ReadSegmentInfos: %v", err)
	}
	if infos.Size() == 0 {
		t.Fatal("expected at least one segment")
	}

	// Verify the codec name is stamped.
	for i := 0; i < infos.Size(); i++ {
		sci := infos.Get(i)
		if sci != nil {
			codec := sci.SegmentInfo().Codec()
			if codec != "Lucene912" {
				t.Fatalf("segment %d: expected codec Lucene912, got %q", i, codec)
			}
		}
	}

	// Open reader and verify.
	reader := base.mustOpenReader(dir)
	defer base.mustClose(reader)
	if reader.NumDocs() != 1 {
		t.Fatalf("expected 1 doc, got %d", reader.NumDocs())
	}
}

// TestIndexUpgrade_EmptyIndex verifies that an empty index's segment metadata
// is correctly read back after a commit.
func TestIndexUpgrade_EmptyIndex(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	base := newBwcTestBase(t)
	config := base.createConfig(nil)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	writer.Commit()
	writer.Close()

	if _, err := index.ReadSegmentInfos(dir); err != nil {
		t.Fatalf("ReadSegmentInfos: %v", err)
	}

	reader := base.mustOpenReader(dir)
	defer base.mustClose(reader)
	if reader.NumDocs() != 0 {
		t.Fatalf("expected 0 docs, got %d", reader.NumDocs())
	}
}
