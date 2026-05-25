// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blocktree

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
)

// ─── Stats ───────────────────────────────────────────────────────────────────

func TestStats_NewStats(t *testing.T) {
	s := NewStats("seg0", "body")
	if s.Segment != "seg0" {
		t.Fatalf("Segment: got %q want %q", s.Segment, "seg0")
	}
	if s.Field != "body" {
		t.Fatalf("Field: got %q want %q", s.Field, "body")
	}
	if s.BlockCountByPrefixLen == nil {
		t.Fatal("BlockCountByPrefixLen must not be nil")
	}
}

func TestStats_String_ZeroValues(t *testing.T) {
	s := NewStats("seg0", "body")
	out := s.String()
	if out == "" {
		t.Fatal("String() must not be empty")
	}
}

func TestStats_String_WithData(t *testing.T) {
	s := NewStats("seg0", "body")
	s.TotalTermCount = 100
	s.TotalBlockCount = 5
	s.TotalBlockSuffixBytes = 80
	s.TotalUncompressedBlockSuffixBytes = 100
	s.CompressionAlgorithms[int(CompressionNone)] = 3
	s.CompressionAlgorithms[int(CompressionLZ4)] = 2
	out := s.String()
	if out == "" {
		t.Fatal("String() must not be empty")
	}
}

// ─── CompressionAlgorithm ────────────────────────────────────────────────────

func TestCompressionAlgorithm_String(t *testing.T) {
	cases := []struct {
		alg  CompressionAlgorithm
		want string
	}{
		{CompressionNone, "NO_COMPRESSION"},
		{CompressionLowercaseASCII, "LOWERCASE_ASCII"},
		{CompressionLZ4, "LZ4"},
		{CompressionAlgorithm(99), "CompressionAlgorithm(99)"},
	}
	for _, tc := range cases {
		if got := tc.alg.String(); got != tc.want {
			t.Errorf("String(%d): got %q want %q", tc.alg, got, tc.want)
		}
	}
}

func TestCompressionAlgorithm_Code(t *testing.T) {
	if got := CompressionNone.Code(); got != 0x00 {
		t.Fatalf("CompressionNone.Code(): got %d want 0", got)
	}
	if got := CompressionLowercaseASCII.Code(); got != 0x01 {
		t.Fatalf("CompressionLowercaseASCII.Code(): got %d want 1", got)
	}
	if got := CompressionLZ4.Code(); got != 0x02 {
		t.Fatalf("CompressionLZ4.Code(): got %d want 2", got)
	}
}

func TestCompressionAlgorithmByCode_Valid(t *testing.T) {
	for _, code := range []int{0, 1, 2} {
		alg, err := CompressionAlgorithmByCode(code)
		if err != nil {
			t.Fatalf("CompressionAlgorithmByCode(%d): unexpected error: %v", code, err)
		}
		if alg.Code() != code {
			t.Fatalf("CompressionAlgorithmByCode(%d): Code() = %d", code, alg.Code())
		}
	}
}

func TestCompressionAlgorithmByCode_Invalid(t *testing.T) {
	_, err := CompressionAlgorithmByCode(99)
	if err == nil {
		t.Fatal("expected error for unknown compression code")
	}
}

// ─── Version constants ───────────────────────────────────────────────────────

func TestVersionConstants(t *testing.T) {
	if VersionStart != 3 {
		t.Fatalf("VersionStart: got %d want 3", VersionStart)
	}
	if VersionCompressedSuffixes != 5 {
		t.Fatalf("VersionCompressedSuffixes: got %d want 5", VersionCompressedSuffixes)
	}
	if VersionMetaFile != 6 {
		t.Fatalf("VersionMetaFile: got %d want 6", VersionMetaFile)
	}
	if VersionCurrent != VersionMetaFile {
		t.Fatalf("VersionCurrent: got %d, expected VersionMetaFile", VersionCurrent)
	}
}

// ─── OutputFlags constants ────────────────────────────────────────────────────

func TestOutputFlagConstants(t *testing.T) {
	if OutputFlagsNumBits != 2 {
		t.Fatalf("OutputFlagsNumBits: got %d want 2", OutputFlagsNumBits)
	}
	if OutputFlagIsFloor != 0x1 {
		t.Fatalf("OutputFlagIsFloor: got %#x want 0x1", OutputFlagIsFloor)
	}
	if OutputFlagHasTerms != 0x2 {
		t.Fatalf("OutputFlagHasTerms: got %#x want 0x2", OutputFlagHasTerms)
	}
}

// ─── SegmentTermsEnum ─────────────────────────────────────────────────────────

// TestSegmentTermsEnum_NavigationDeferred verifies that navigation methods
// return ErrBlockTraversalNotAvailable (not a panic or unrelated error).
func TestSegmentTermsEnum_NavigationDeferred(t *testing.T) {
	fr := &FieldReader{
		fieldInfo: index.NewFieldInfo("body", 0, index.FieldInfoOptions{
			IndexOptions: index.IndexOptionsDocsAndFreqs,
		}),
	}
	e, err := newSegmentTermsEnum(fr)
	if err != nil {
		t.Fatalf("newSegmentTermsEnum: unexpected error: %v", err)
	}

	if _, err = e.Next(); !errors.Is(err, ErrBlockTraversalNotAvailable) {
		t.Errorf("Next(): want ErrBlockTraversalNotAvailable, got %v", err)
	}
	if _, err = e.SeekCeil(index.NewTerm("body", "foo")); !errors.Is(err, ErrBlockTraversalNotAvailable) {
		t.Errorf("SeekCeil(): want ErrBlockTraversalNotAvailable, got %v", err)
	}
	if _, err = e.SeekExact(index.NewTerm("body", "foo")); !errors.Is(err, ErrBlockTraversalNotAvailable) {
		t.Errorf("SeekExact(): want ErrBlockTraversalNotAvailable, got %v", err)
	}
	if _, err = e.DocFreq(); !errors.Is(err, ErrBlockTraversalNotAvailable) {
		t.Errorf("DocFreq(): want ErrBlockTraversalNotAvailable, got %v", err)
	}
	if _, err = e.TotalTermFreq(); !errors.Is(err, ErrBlockTraversalNotAvailable) {
		t.Errorf("TotalTermFreq(): want ErrBlockTraversalNotAvailable, got %v", err)
	}
	if _, err = e.Postings(0); !errors.Is(err, ErrBlockTraversalNotAvailable) {
		t.Errorf("Postings(): want ErrBlockTraversalNotAvailable, got %v", err)
	}
}

// ─── IntersectTermsEnum ───────────────────────────────────────────────────────

// TestIntersectTermsEnum_NilAutomatonReturnsError verifies that construction
// fails gracefully when compiled is nil.
func TestIntersectTermsEnum_NilAutomatonReturnsError(t *testing.T) {
	fr := &FieldReader{
		fieldInfo: index.NewFieldInfo("body", 0, index.FieldInfoOptions{}),
	}
	_, err := newIntersectTermsEnum(fr, nil, nil)
	if err == nil {
		t.Fatal("expected error for nil compiled automaton")
	}
}

// TestIntersectTermsEnum_SeekNotSupported verifies that SeekCeil and
// SeekExact return errors on a manually-wired instance.
func TestIntersectTermsEnum_SeekNotSupported(t *testing.T) {
	e := &IntersectTermsEnum{
		fr: &FieldReader{
			fieldInfo: index.NewFieldInfo("body", 0, index.FieldInfoOptions{}),
		},
	}
	_ = automaton.Compile(automaton.MakeAnyString()) // sanity check automaton builds

	if _, err := e.SeekCeil(nil); err == nil {
		t.Error("SeekCeil: expected error")
	}
	if _, err := e.SeekExact(nil); err == nil {
		t.Error("SeekExact: expected error")
	}
	if _, err := e.Next(); !errors.Is(err, ErrBlockTraversalNotAvailable) {
		t.Errorf("Next(): want ErrBlockTraversalNotAvailable, got %v", err)
	}
}

// ─── segmentTermsEnumFrame ───────────────────────────────────────────────────

func TestSegmentTermsEnumFrame_GetTermBlockOrd_Leaf(t *testing.T) {
	fr := &FieldReader{
		fieldInfo: index.NewFieldInfo("f", 0, index.FieldInfoOptions{}),
	}
	e, err := newSegmentTermsEnum(fr)
	if err != nil {
		t.Fatal(err)
	}
	f := newSegmentTermsEnumFrame(e, 0)
	f.isLeafBlock = true
	f.nextEnt = 7
	if got := f.getTermBlockOrd(); got != 7 {
		t.Fatalf("getTermBlockOrd leaf: got %d want 7", got)
	}
}

func TestSegmentTermsEnumFrame_GetTermBlockOrd_NonLeaf(t *testing.T) {
	fr := &FieldReader{
		fieldInfo: index.NewFieldInfo("f", 0, index.FieldInfoOptions{}),
	}
	e, err := newSegmentTermsEnum(fr)
	if err != nil {
		t.Fatal(err)
	}
	f := newSegmentTermsEnumFrame(e, 0)
	f.isLeafBlock = false
	f.nextEnt = 7
	if got := f.getTermBlockOrd(); got != 0 {
		t.Fatalf("getTermBlockOrd non-leaf: got %d want 0", got)
	}
}

// ─── intersectTermsEnumFrame ──────────────────────────────────────────────────

func TestIntersectTermsEnumFrame_Construction_NewVersion(t *testing.T) {
	e := &IntersectTermsEnum{
		fr: &FieldReader{
			fieldInfo: index.NewFieldInfo("f", 0, index.FieldInfoOptions{}),
			parent: &Lucene40BlockTreeTermsReader{
				version:        VersionCompressedSuffixes,
				postingsReader: &noopPostingsReader{},
			},
		},
	}
	f := newIntersectTermsEnumFrame(e, 0)
	if f.ord != 0 {
		t.Fatalf("ord: got %d want 0", f.ord)
	}
	if f.suffixLengthBytes == nil {
		t.Fatal("suffixLengthBytes must be allocated for version >= VersionCompressedSuffixes")
	}
	if f.suffixLengthsReader == nil {
		t.Fatal("suffixLengthsReader must not be nil")
	}
}

func TestIntersectTermsEnumFrame_Construction_OldVersion(t *testing.T) {
	e := &IntersectTermsEnum{
		fr: &FieldReader{
			fieldInfo: index.NewFieldInfo("f", 0, index.FieldInfoOptions{}),
			parent: &Lucene40BlockTreeTermsReader{
				version:        VersionMetaLongsRemoved,
				postingsReader: &noopPostingsReader{},
			},
		},
	}
	f := newIntersectTermsEnumFrame(e, 0)
	if f.suffixLengthsReader != f.suffixesReader {
		t.Fatal("old version: suffixLengthsReader should alias suffixesReader")
	}
}

// ─── noopPostingsReader ───────────────────────────────────────────────────────

// noopPostingsReader satisfies codecs.PostingsReaderBase for tests.
type noopPostingsReader struct{}

func (n *noopPostingsReader) Init(_ store.IndexInput, _ *codecs.SegmentReadState) error {
	return nil
}
func (n *noopPostingsReader) NewTermState() *codecs.BlockTermState {
	return codecs.NewBlockTermState()
}
func (n *noopPostingsReader) DecodeTerm(
	_ store.DataInput,
	_ *index.FieldInfo,
	_ *codecs.BlockTermState,
	_ bool,
) error {
	return nil
}
func (n *noopPostingsReader) Postings(
	_ *index.FieldInfo,
	_ *codecs.BlockTermState,
	_ index.PostingsEnum,
	_ int,
) (index.PostingsEnum, error) {
	return nil, errors.New("noop")
}
func (n *noopPostingsReader) Impacts(
	_ *index.FieldInfo,
	_ *codecs.BlockTermState,
	_ int,
) (index.ImpactsEnum, error) {
	return nil, errors.New("noop")
}
func (n *noopPostingsReader) CheckIntegrity() error { return nil }
func (n *noopPostingsReader) Close() error          { return nil }

var _ codecs.PostingsReaderBase = (*noopPostingsReader)(nil)
