// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene912

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// ─── Lucene912PostingsFormat ──────────────────────────────────────────────────

func TestLucene912PostingsFormat_Name(t *testing.T) {
	f := NewLucene912PostingsFormat()
	if got := f.Name(); got != "Lucene912" {
		t.Errorf("Name() = %q, want %q", got, "Lucene912")
	}
}

func TestLucene912PostingsFormat_FieldsConsumer_ReturnsError(t *testing.T) {
	f := NewLucene912PostingsFormat()
	_, err := f.FieldsConsumer(nil)
	if err == nil {
		t.Fatal("FieldsConsumer: expected error, got nil")
	}
	if !errors.Is(err, ErrWriteNotSupported) {
		t.Errorf("FieldsConsumer error = %v, want ErrWriteNotSupported", err)
	}
}

func TestLucene912PostingsFormat_FieldsProducer_ReturnsError(t *testing.T) {
	f := NewLucene912PostingsFormat()
	_, err := f.FieldsProducer(nil)
	if err == nil {
		t.Fatal("FieldsProducer: expected error, got nil")
	}
}

func TestLucene912PostingsFormat_Constants(t *testing.T) {
	// These values are fixed by the wire format and must not change.
	if BlockSize != 128 {
		t.Errorf("BlockSize = %d, want 128", BlockSize)
	}
	if Level1Factor != 32 {
		t.Errorf("Level1Factor = %d, want 32", Level1Factor)
	}
	if Level1NumDocs != Level1Factor*BlockSize {
		t.Errorf("Level1NumDocs = %d, want %d", Level1NumDocs, Level1Factor*BlockSize)
	}
	if MetaExtension != "psm" {
		t.Errorf("MetaExtension = %q, want %q", MetaExtension, "psm")
	}
	if DocExtension != "doc" {
		t.Errorf("DocExtension = %q, want %q", DocExtension, "doc")
	}
	if PosExtension != "pos" {
		t.Errorf("PosExtension = %q, want %q", PosExtension, "pos")
	}
	if PayExtension != "pay" {
		t.Errorf("PayExtension = %q, want %q", PayExtension, "pay")
	}
}

// ─── IntBlockTermState ───────────────────────────────────────────────────────

func TestIntBlockTermState_Defaults(t *testing.T) {
	s := NewIntBlockTermState()
	if s.LastPosBlockOffset != -1 {
		t.Errorf("LastPosBlockOffset = %d, want -1", s.LastPosBlockOffset)
	}
	if s.SingletonDocID != -1 {
		t.Errorf("SingletonDocID = %d, want -1", s.SingletonDocID)
	}
	if s.DocStartFP != 0 {
		t.Errorf("DocStartFP = %d, want 0", s.DocStartFP)
	}
	if s.PosStartFP != 0 {
		t.Errorf("PosStartFP = %d, want 0", s.PosStartFP)
	}
	if s.PayStartFP != 0 {
		t.Errorf("PayStartFP = %d, want 0", s.PayStartFP)
	}
	if s.BlockTermState == nil {
		t.Error("BlockTermState is nil")
	}
}

// ─── Lucene912Codec ──────────────────────────────────────────────────────────

func TestLucene912Codec_Name(t *testing.T) {
	c := NewLucene912Codec()
	if got := c.Name(); got != "Lucene912" {
		t.Errorf("Name() = %q, want %q", got, "Lucene912")
	}
}

func TestLucene912Codec_PostingsFormat(t *testing.T) {
	c := NewLucene912Codec()
	pf := c.PostingsFormat()
	if pf == nil {
		t.Fatal("PostingsFormat() returned nil")
	}
	if got := pf.Name(); got != "Lucene912" {
		t.Errorf("PostingsFormat().Name() = %q, want %q", got, "Lucene912")
	}
}

func TestLucene912Codec_DelegateFormats_NonNil(t *testing.T) {
	c := NewLucene912Codec()
	if c.StoredFieldsFormat() == nil {
		t.Error("StoredFieldsFormat() is nil")
	}
	if c.FieldInfosFormat() == nil {
		t.Error("FieldInfosFormat() is nil")
	}
	if c.SegmentInfosFormat() == nil {
		t.Error("SegmentInfosFormat() is nil")
	}
	if c.TermVectorsFormat() == nil {
		t.Error("TermVectorsFormat() is nil")
	}
	if c.DocValuesFormat() == nil {
		t.Error("DocValuesFormat() is nil")
	}
}

// ─── Lucene912PostingsReader constructor: error paths ────────────────────────

func TestLucene912PostingsReader_MissingMetaFile(t *testing.T) {
	dir := openDir(t)
	defer dir.Close()

	state := makeMinimalReadState(t, dir)
	_, err := NewLucene912PostingsReader(state)
	if err == nil {
		t.Fatal("NewLucene912PostingsReader: expected error for missing meta file, got nil")
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// makeMinimalReadState builds a SegmentReadState pointing at dir with
// a freshly-constructed SegmentInfo and empty FieldInfos.
func makeMinimalReadState(t *testing.T, dir *store.SimpleFSDirectory) *codecs.SegmentReadState {
	t.Helper()
	si := index.NewSegmentInfo("_seg", 0, dir)
	return &codecs.SegmentReadState{
		Directory:   dir,
		SegmentInfo: si,
		FieldInfos:  &index.FieldInfos{},
	}
}
