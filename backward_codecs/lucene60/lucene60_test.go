// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene60

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// ─────────────────────────────────────────────────────────────────────────────
// helpers
// ─────────────────────────────────────────────────────────────────────────────

func newDir(t *testing.T) *store.SimpleFSDirectory {
	t.Helper()
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	return dir
}

func newSegmentInfo(t *testing.T, name string, dir store.Directory) *index.SegmentInfo {
	t.Helper()
	id := make([]byte, 16)
	for i := range id {
		id[i] = byte(i + 1)
	}
	si := index.NewSegmentInfo(name, 0, dir)
	if err := si.SetID(id); err != nil {
		t.Fatalf("SetID: %v", err)
	}
	return si
}

// ─────────────────────────────────────────────────────────────────────────────
// Lucene60FieldInfosFormat round-trip
// ─────────────────────────────────────────────────────────────────────────────

func TestLucene60FieldInfosFormat_Name(t *testing.T) {
	f := NewLucene60FieldInfosFormat()
	if f.Name() != "Lucene60FieldInfosFormat" {
		t.Errorf("unexpected name: %q", f.Name())
	}
}

func TestLucene60FieldInfosFormat_RoundTrip_Empty(t *testing.T) {
	dir := newDir(t)
	defer dir.Close()

	f := NewLucene60FieldInfosFormat()
	si := newSegmentInfo(t, "_0", dir)
	ctx := store.IOContext{Context: store.ContextRead}

	empty := index.NewFieldInfos()
	empty.Freeze()

	if err := f.Write(dir, si, "", empty, ctx); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := f.Read(dir, si, "", ctx)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.Size() != 0 {
		t.Errorf("expected 0 fields, got %d", got.Size())
	}
}

func TestLucene60FieldInfosFormat_RoundTrip_OneField(t *testing.T) {
	dir := newDir(t)
	defer dir.Close()

	f := NewLucene60FieldInfosFormat()
	si := newSegmentInfo(t, "_1", dir)
	ctx := store.IOContext{Context: store.ContextRead}

	fi := index.NewFieldInfoBuilder("content", 0).
		SetIndexOptions(index.IndexOptionsDocsAndFreqsAndPositions).
		SetOmitNorms(false).
		SetStoreTermVectors(true).
		SetDocValuesType(index.DocValuesTypeNone).
		SetDocValuesGen(-1).
		Build()

	infos := index.NewFieldInfos()
	infos.Add(fi)
	infos.Freeze()

	if err := f.Write(dir, si, "", infos, ctx); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := f.Read(dir, si, "", ctx)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.Size() != 1 {
		t.Fatalf("expected 1 field, got %d", got.Size())
	}

	gotFI := got.GetByName("content")
	if gotFI == nil {
		t.Fatal("field 'content' not found")
	}
	if gotFI.Number() != 0 {
		t.Errorf("field number: got %d, want 0", gotFI.Number())
	}
	if gotFI.IndexOptions() != index.IndexOptionsDocsAndFreqsAndPositions {
		t.Errorf("index options: got %v, want DocsAndFreqsAndPositions", gotFI.IndexOptions())
	}
	if !gotFI.StoreTermVectors() {
		t.Error("expected StoreTermVectors=true")
	}
}

func TestLucene60FieldInfosFormat_RoundTrip_SoftDeletes(t *testing.T) {
	dir := newDir(t)
	defer dir.Close()

	f := NewLucene60FieldInfosFormat()
	si := newSegmentInfo(t, "_2", dir)
	ctx := store.IOContext{Context: store.ContextRead}

	fi := index.NewFieldInfoBuilder("_soft_deletes", 0).
		SetSoftDeletesField(true).
		Build()

	infos := index.NewFieldInfos()
	infos.Add(fi)
	infos.Freeze()

	if err := f.Write(dir, si, "", infos, ctx); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := f.Read(dir, si, "", ctx)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	gotFI := got.GetByName("_soft_deletes")
	if gotFI == nil {
		t.Fatal("soft-deletes field not found")
	}
	if !gotFI.IsSoftDeletesField() {
		t.Error("expected IsSoftDeletesField=true")
	}
}

func TestLucene60FieldInfosFormat_RoundTrip_PointFields(t *testing.T) {
	dir := newDir(t)
	defer dir.Close()

	f := NewLucene60FieldInfosFormat()
	si := newSegmentInfo(t, "_3", dir)
	ctx := store.IOContext{Context: store.ContextRead}

	fi := index.NewFieldInfoBuilder("lat_lon", 0).
		SetPointDimensions(2, 2, 8).
		Build()

	infos := index.NewFieldInfos()
	infos.Add(fi)
	infos.Freeze()

	if err := f.Write(dir, si, "", infos, ctx); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := f.Read(dir, si, "", ctx)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	gotFI := got.GetByName("lat_lon")
	if gotFI == nil {
		t.Fatal("point field not found")
	}
	if gotFI.PointDimensionCount() != 2 {
		t.Errorf("dim count: got %d, want 2", gotFI.PointDimensionCount())
	}
	if gotFI.PointNumBytes() != 8 {
		t.Errorf("point num bytes: got %d, want 8", gotFI.PointNumBytes())
	}
}

func TestLucene60FieldInfosFormat_RoundTrip_DocValues(t *testing.T) {
	dir := newDir(t)
	defer dir.Close()

	f := NewLucene60FieldInfosFormat()
	si := newSegmentInfo(t, "_4", dir)
	ctx := store.IOContext{Context: store.ContextRead}

	fi := index.NewFieldInfoBuilder("score_dv", 0).
		SetDocValuesType(index.DocValuesTypeNumeric).
		SetDocValuesGen(3).
		Build()

	infos := index.NewFieldInfos()
	infos.Add(fi)
	infos.Freeze()

	if err := f.Write(dir, si, "", infos, ctx); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := f.Read(dir, si, "", ctx)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	gotFI := got.GetByName("score_dv")
	if gotFI == nil {
		t.Fatal("dv field not found")
	}
	if gotFI.DocValuesType() != index.DocValuesTypeNumeric {
		t.Errorf("dv type: got %v, want Numeric", gotFI.DocValuesType())
	}
	if gotFI.DocValuesGen() != 3 {
		t.Errorf("dv gen: got %d, want 3", gotFI.DocValuesGen())
	}
}

func TestLucene60FieldInfosFormat_RoundTrip_Attributes(t *testing.T) {
	dir := newDir(t)
	defer dir.Close()

	f := NewLucene60FieldInfosFormat()
	si := newSegmentInfo(t, "_5", dir)
	ctx := store.IOContext{Context: store.ContextRead}

	fi := index.NewFieldInfoBuilder("field_with_attrs", 0).
		SetAttribute("key1", "val1").
		SetAttribute("key2", "val2").
		Build()

	infos := index.NewFieldInfos()
	infos.Add(fi)
	infos.Freeze()

	if err := f.Write(dir, si, "", infos, ctx); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := f.Read(dir, si, "", ctx)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	gotFI := got.GetByName("field_with_attrs")
	if gotFI == nil {
		t.Fatal("attributed field not found")
	}
	attrs := gotFI.GetAttributes()
	if attrs["key1"] != "val1" {
		t.Errorf("key1: got %q, want %q", attrs["key1"], "val1")
	}
	if attrs["key2"] != "val2" {
		t.Errorf("key2: got %q, want %q", attrs["key2"], "val2")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Lucene60PointsFormat
// ─────────────────────────────────────────────────────────────────────────────

func TestLucene60PointsFormat_Name(t *testing.T) {
	f := NewLucene60PointsFormat()
	if f.Name() != "Lucene60PointsFormat" {
		t.Errorf("unexpected name: %q", f.Name())
	}
}

func TestLucene60PointsFormat_FieldsWriterUnsupported(t *testing.T) {
	f := NewLucene60PointsFormat()
	_, err := f.FieldsWriter(nil)
	if err == nil {
		t.Error("expected error from FieldsWriter on legacy format")
	}
}
