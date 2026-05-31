// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Coverage for CheckIndex.ExorciseIndex (rmp #16): a corrupt segment is
// detected by CheckIndex and removed, after which the repaired index reopens
// cleanly and re-checks clean, retaining the undamaged segment's documents.

package index_test

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"

	_ "github.com/FlavioCFOliveira/Gocene/codecs"
)

func TestCheckIndex_ExorciseRemovesCorruptSegment(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	add := func(id string) {
		doc := document.NewDocument()
		f, _ := document.NewStringField("id", id, true)
		doc.Add(f)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument: %v", err)
		}
	}
	// Segment 0 (keep) and segment 1 (corrupt).
	add("1")
	add("2")
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit seg0: %v", err)
	}
	add("3")
	add("4")
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit seg1: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Corrupt the second segment by deleting one of its (non-.si) files.
	si, err := index.ReadSegmentInfos(dir)
	if err != nil {
		t.Fatalf("ReadSegmentInfos: %v", err)
	}
	if si.Size() != 2 {
		t.Fatalf("expected 2 segments, got %d", si.Size())
	}
	corruptSeg := si.Get(1)
	corruptName := corruptSeg.SegmentInfo().Name()
	var deleted string
	for _, f := range corruptSeg.GetFiles() {
		if !strings.HasSuffix(f, ".si") {
			if err := dir.DeleteFile(f); err != nil {
				t.Fatalf("DeleteFile(%s): %v", f, err)
			}
			deleted = f
			break
		}
	}
	if deleted == "" {
		t.Fatalf("no deletable data file found for segment %s", corruptName)
	}

	// CheckIndex must now report the index unclean with the corrupt segment.
	checker, err := index.NewCheckIndex(dir)
	if err != nil {
		t.Fatalf("NewCheckIndex: %v", err)
	}
	checker.SetLevel(index.CheckIndexLevelMinIntegrityChecks)
	status, err := checker.CheckIndex()
	if err != nil {
		t.Fatalf("CheckIndex: %v", err)
	}
	if status.Clean {
		t.Fatalf("expected unclean index after deleting %s", deleted)
	}
	foundCorrupt := false
	for _, ss := range status.SegmentInfos {
		if ss.Name == corruptName && ss.Error != nil {
			foundCorrupt = true
		}
	}
	if !foundCorrupt {
		t.Fatalf("CheckIndex did not flag segment %s as corrupt", corruptName)
	}

	// Exorcise: remove the corrupt segment.
	if err := checker.ExorciseIndex(status); err != nil {
		t.Fatalf("ExorciseIndex: %v", err)
	}
	checker.Close()

	// The repaired index re-checks clean.
	checker2, err := index.NewCheckIndex(dir)
	if err != nil {
		t.Fatalf("NewCheckIndex (post-exorcise): %v", err)
	}
	defer checker2.Close()
	checker2.SetLevel(index.CheckIndexLevelMinIntegrityChecks)
	status2, err := checker2.CheckIndex()
	if err != nil {
		t.Fatalf("CheckIndex (post-exorcise): %v", err)
	}
	if !status2.Clean {
		t.Fatalf("index still unclean after exorcise (errors: %v)", status2.Errors)
	}

	// The surviving segment's documents are still readable.
	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader (post-exorcise): %v", err)
	}
	defer reader.Close()
	if got := reader.NumDocs(); got != 2 {
		t.Fatalf("NumDocs after exorcise = %d, want 2 (the clean segment)", got)
	}
}
