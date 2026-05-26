// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestIndexWriter_UseCompoundFile verifies that IndexWriter with
// UseCompoundFile=true packs each flushed segment into a .cfs/.cfe pair and
// leaves no loose per-format files (AC#2 for T4643).
//
// The expected on-disk layout for a single segment is:
//
//	_0.cfs, _0.cfe, _0.si, segments_N, write.lock
//
// No loose codec artefacts (.fdt, .fdx, .fnm, .tim, .tip, .doc, .pos, …)
// should remain after Commit.
func TestIndexWriter_UseCompoundFile(t *testing.T) {
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	cfg.SetUseCompoundFile(true)

	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	doc := document.NewDocument()
	tf, err := document.NewTextField("body", "hello world lucene compound", false)
	if err != nil {
		t.Fatalf("NewTextField: %v", err)
	}
	doc.Add(tf)
	if err := w.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	all, err := dir.ListAll()
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}

	// Classify each file.
	var cfsFiles, cfeFiles, siFiles, otherFiles []string
	for _, f := range all {
		switch {
		case strings.HasSuffix(f, ".cfs"):
			cfsFiles = append(cfsFiles, f)
		case strings.HasSuffix(f, ".cfe"):
			cfeFiles = append(cfeFiles, f)
		case strings.HasSuffix(f, ".si"):
			siFiles = append(siFiles, f)
		case f == "write.lock" || strings.HasPrefix(f, "segments_"):
			// Expected overhead files — ignore.
		default:
			otherFiles = append(otherFiles, f)
		}
	}

	if len(cfsFiles) == 0 {
		t.Errorf("no .cfs file produced; useCompoundFile=true must produce a .cfs")
	}
	if len(cfeFiles) == 0 {
		t.Errorf("no .cfe file produced; useCompoundFile=true must produce a .cfe")
	}
	if len(siFiles) == 0 {
		t.Errorf("no .si file produced")
	}
	if len(otherFiles) > 0 {
		t.Errorf("loose per-format files remain after CFS pack: %v", otherFiles)
	}
}
