// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blocktree

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestVersionCompressedSuffixes verifies that the unexported alias equals the
// exported constant.
func TestVersionCompressedSuffixes(t *testing.T) {
	if versionCompressedSuffixes != VersionCompressedSuffixes {
		t.Errorf("versionCompressedSuffixes: got %d want %d",
			versionCompressedSuffixes, VersionCompressedSuffixes)
	}
}

// TestFieldReader_HasFreqs verifies HasFreqs depends on IndexOptions.
func TestFieldReader_HasFreqs(t *testing.T) {
	tests := []struct {
		name  string
		opts  index.IndexOptions
		want  bool
	}{
		{"docs only", index.IndexOptionsDocs, false},
		{"docs and freqs", index.IndexOptionsDocsAndFreqs, true},
		{"docs freqs positions", index.IndexOptionsDocsAndFreqsAndPositions, true},
		{"docs freqs positions offsets", index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets, true},
	}
	for _, tc := range tests {
		fr := &FieldReader{
			fieldInfo: index.NewFieldInfo("f", 0, index.FieldInfoOptions{
				IndexOptions: tc.opts,
			}),
		}
		if got := fr.HasFreqs(); got != tc.want {
			t.Errorf("%s: HasFreqs() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestFieldReader_HasPositions verifies HasPositions depends on IndexOptions.
func TestFieldReader_HasPositions(t *testing.T) {
	tests := []struct {
		name  string
		opts  index.IndexOptions
		want  bool
	}{
		{"docs only", index.IndexOptionsDocs, false},
		{"docs and freqs", index.IndexOptionsDocsAndFreqs, false},
		{"docs freqs positions", index.IndexOptionsDocsAndFreqsAndPositions, true},
	}
	for _, tc := range tests {
		fr := &FieldReader{
			fieldInfo: index.NewFieldInfo("f", 0, index.FieldInfoOptions{
				IndexOptions: tc.opts,
			}),
		}
		if got := fr.HasPositions(); got != tc.want {
			t.Errorf("%s: HasPositions() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestFieldReader_HasOffsets verifies HasOffsets depends on IndexOptions.
func TestFieldReader_HasOffsets(t *testing.T) {
	tests := []struct {
		name  string
		opts  index.IndexOptions
		want  bool
	}{
		{"docs only", index.IndexOptionsDocs, false},
		{"docs freqs positions", index.IndexOptionsDocsAndFreqsAndPositions, false},
		{"docs freqs positions offsets", index.IndexOptionsDocsAndFreqsAndPositionsAndOffsets, true},
	}
	for _, tc := range tests {
		fr := &FieldReader{
			fieldInfo: index.NewFieldInfo("f", 0, index.FieldInfoOptions{
				IndexOptions: tc.opts,
			}),
		}
		if got := fr.HasOffsets(); got != tc.want {
			t.Errorf("%s: HasOffsets() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestFieldReader_HasPayloads verifies HasPayloads returns the field's payloads flag.
func TestFieldReader_HasPayloads(t *testing.T) {
	// FieldInfo must have positions to accept SetStorePayloads.
	fi := index.NewFieldInfo("f", 0, index.FieldInfoOptions{
		IndexOptions: index.IndexOptionsDocsAndFreqsAndPositions,
	})
	fi.SetStorePayloads()
	fr := &FieldReader{fieldInfo: fi}
	if !fr.HasPayloads() {
		t.Error("HasPayloads: want true after SetStorePayloads")
	}

	fiNoPayload := index.NewFieldInfo("f", 0, index.FieldInfoOptions{
		IndexOptions: index.IndexOptionsDocsAndFreqsAndPositions,
	})
	frNoPayload := &FieldReader{fieldInfo: fiNoPayload}
	if frNoPayload.HasPayloads() {
		t.Error("HasPayloads: want false before SetStorePayloads")
	}
}

// TestFieldReader_GetDocCountAndSums verifies that GetDocCount, GetSumDocFreq,
// and GetSumTotalTermFreq return the field values directly.
func TestFieldReader_GetDocCountAndSums(t *testing.T) {
	fr := &FieldReader{
		docCount:         7,
		sumDocFreq:       100,
		sumTotalTermFreq: 250,
	}
	docCount, err := fr.GetDocCount()
	if err != nil {
		t.Fatalf("GetDocCount: %v", err)
	}
	if docCount != 7 {
		t.Errorf("GetDocCount: got %d want 7", docCount)
	}
	sumDF, err := fr.GetSumDocFreq()
	if err != nil {
		t.Fatalf("GetSumDocFreq: %v", err)
	}
	if sumDF != 100 {
		t.Errorf("GetSumDocFreq: got %d want 100", sumDF)
	}
	sumTTF, err := fr.GetSumTotalTermFreq()
	if err != nil {
		t.Fatalf("GetSumTotalTermFreq: %v", err)
	}
	if sumTTF != 250 {
		t.Errorf("GetSumTotalTermFreq: got %d want 250", sumTTF)
	}
}

// TestFieldReader_ZeroSums verifies that zero-valued fields are returned correctly.
func TestFieldReader_ZeroSums(t *testing.T) {
	fr := &FieldReader{}
	docCount, err := fr.GetDocCount()
	if err != nil {
		t.Fatalf("GetDocCount: %v", err)
	}
	if docCount != 0 {
		t.Errorf("GetDocCount: got %d want 0", docCount)
	}
}

// TestFieldReader_GetIterator verifies GetIterator returns a non-nil
// SegmentTermsEnum even on a bare-bones FieldReader.
func TestFieldReader_GetIterator(t *testing.T) {
	fr := &FieldReader{
		fieldInfo: index.NewFieldInfo("f", 0, index.FieldInfoOptions{}),
	}
	te, err := fr.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator: %v", err)
	}
	if te == nil {
		t.Fatal("GetIterator returned nil")
	}
}

// TestFieldReader_GetPostingsReaderNoSuchTerm verifies that looking up a
// non-existent term returns (nil, nil).
func TestFieldReader_GetPostingsReaderNoSuchTerm(t *testing.T) {
	fr := &FieldReader{
		fieldInfo: index.NewFieldInfo("f", 0, index.FieldInfoOptions{}),
	}
	pe, err := fr.GetPostingsReader("nonexistent", 0)
	if err != nil {
		t.Fatalf("GetPostingsReader: %v", err)
	}
	if pe != nil {
		t.Error("GetPostingsReader: expected nil for missing term")
	}
}

// TestReadBytesRef_Zero verifies that readBytesRef handles a zero-length slice.
func TestReadBytesRef_Zero(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	out, err := dir.CreateOutput("zero_br.dat", store.IOContext{})
	if err != nil {
		t.Fatal(err)
	}
	// Write VInt(0) meaning length=0.
	if err := store.WriteVInt(out, 0); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}

	in, err := dir.OpenInput("zero_br.dat", store.IOContext{})
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	br, err := readBytesRef(in)
	if err != nil {
		t.Fatalf("readBytesRef: %v", err)
	}
	if br.Length != 0 {
		t.Errorf("Length: got %d want 0", br.Length)
	}
}
