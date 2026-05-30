// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestParentField_FnmRoundTrip is acceptance criterion (2) of rmp #4789: when a
// configured parentField is materialised as a real document field, the parent
// bit is written into the per-segment .fnm (Lucene94FieldInfosFormat) and is
// recovered from .fnm — not from a _gocene_parent userData key — on reopen.
func TestParentField_FnmRoundTrip(t *testing.T) {
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	defer dir.Close()

	const parentName = "blockparent"
	cfg := index.NewIndexWriterConfig(createTestAnalyzer())
	cfg.SetParentField(parentName)
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	// Add documents that include the configured parent field as a real field so
	// the parent bit has a FieldInfo to attach to in the .fnm.
	for i := 0; i < 3; i++ {
		doc := document.NewDocument()
		pf, ferr := document.NewStringField(parentName, "p", true)
		if ferr != nil {
			t.Fatalf("NewStringField(parent): %v", ferr)
		}
		doc.Add(pf)
		bf, ferr := document.NewTextField("body", "alpha beta gamma", true)
		if ferr != nil {
			t.Fatalf("NewTextField(body): %v", ferr)
		}
		doc.Add(bf)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument %d: %v", i, err)
		}
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Reopen and assert the parent bit was recovered from the .fnm.
	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer r.Close()
	fis := r.GetFieldInfos()
	pfi := fis.GetByName(parentName)
	if pfi == nil {
		t.Fatalf("parent field %q missing after reopen", parentName)
	}
	if !pfi.IsParentField() {
		t.Errorf("parent bit not recovered from .fnm for field %q", parentName)
	}
	if bfi := fis.GetByName("body"); bfi == nil || bfi.IsParentField() {
		t.Errorf("non-parent field 'body' must not carry the parent bit")
	}
}

// TestParentField_AddIndexesValidatesFromFnm verifies that AddIndexes parent
// compatibility is decided from the .fnm parent bit: a source whose parent field
// is materialised in .fnm and differs from the destination's parent field is
// rejected without relying on the _gocene_parent userData key.
func TestParentField_AddIndexesValidatesFromFnm(t *testing.T) {
	src, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory(src): %v", err)
	}
	defer src.Close()
	dst, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory(dst): %v", err)
	}
	defer dst.Close()

	// Source index: parent field "srcparent", materialised as a real field.
	srcCfg := index.NewIndexWriterConfig(createTestAnalyzer())
	srcCfg.SetParentField("srcparent")
	sw, err := index.NewIndexWriter(src, srcCfg)
	if err != nil {
		t.Fatalf("NewIndexWriter(src): %v", err)
	}
	doc := document.NewDocument()
	pf, _ := document.NewStringField("srcparent", "p", true)
	doc.Add(pf)
	if err := sw.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument(src): %v", err)
	}
	if err := sw.Close(); err != nil {
		t.Fatalf("Close(src): %v", err)
	}

	// Destination index: a different parent field "dstparent".
	dstCfg := index.NewIndexWriterConfig(createTestAnalyzer())
	dstCfg.SetParentField("dstparent")
	dw, err := index.NewIndexWriter(dst, dstCfg)
	if err != nil {
		t.Fatalf("NewIndexWriter(dst): %v", err)
	}
	defer dw.Close()

	// AddIndexes must reject the incompatible parent fields, derived from the
	// source's .fnm parent bit (srcparent) vs the destination config (dstparent).
	if err := dw.AddIndexes(src); err == nil {
		t.Error("AddIndexes accepted incompatible parent fields (srcparent vs dstparent)")
	}
}
