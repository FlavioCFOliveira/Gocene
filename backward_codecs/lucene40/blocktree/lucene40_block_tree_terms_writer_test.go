// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blocktree

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestWireFormatConstants verifies the terms dictionary file-extension and codec
// name constants.
func TestWireFormatConstants(t *testing.T) {
	if termsExtension != "tim" {
		t.Errorf("termsExtension: got %q want %q", termsExtension, "tim")
	}
	if termsCodecName != "BlockTreeTermsDict" {
		t.Errorf("termsCodecName: got %q want %q", termsCodecName, "BlockTreeTermsDict")
	}
	if termsIndexExt != "tip" {
		t.Errorf("termsIndexExt: got %q want %q", termsIndexExt, "tip")
	}
	if termsIndexCodec != "BlockTreeTermsIndex" {
		t.Errorf("termsIndexCodec: got %q want %q", termsIndexCodec, "BlockTreeTermsIndex")
	}
	if termsMetaExt != "tmd" {
		t.Errorf("termsMetaExt: got %q want %q", termsMetaExt, "tmd")
	}
	if termsMetaCodecName != "BlockTreeTermsMeta" {
		t.Errorf("termsMetaCodecName: got %q want %q", termsMetaCodecName, "BlockTreeTermsMeta")
	}
}

// TestCheckFooter_Truncated verifies that checkFooter returns an error when the
// remaining bytes are fewer than the footer length.
func TestCheckFooter_Truncated(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	out, err := dir.CreateOutput("trunc.dat", store.IOContext{})
	if err != nil {
		t.Fatal(err)
	}
	if err := out.WriteInt(0); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}

	in, err := dir.OpenInput("trunc.dat", store.IOContext{})
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	csum := store.NewBufferedChecksumIndexInput(in)
	if err := checkFooter(csum); err == nil {
		t.Error("expected error for truncated file (remaining < footerLen)")
	}
}

// TestCheckFooter_Extended verifies that checkFooter returns an error when extra
// data follows the footer region.
func TestCheckFooter_Extended(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	out, err := dir.CreateOutput("ext.dat", store.IOContext{})
	if err != nil {
		t.Fatal(err)
	}
	// Write several integers, then append a 16-byte footer-like block.
	for i := int32(0); i < 5; i++ {
		if err := out.WriteInt(i); err != nil {
			t.Fatal(err)
		}
	}
	// 16 bytes of footer-looking data at the end.
	if err := out.WriteLong(0); err != nil {
		t.Fatal(err)
	}
	if err := out.WriteLong(0); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}

	in, err := dir.OpenInput("ext.dat", store.IOContext{})
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	csum := store.NewBufferedChecksumIndexInput(in)
	if err := checkFooter(csum); err == nil {
		t.Error("expected error for extended file (remaining > footerLen)")
	}
}

// TestCheckFooter_MagicMismatch verifies that checkFooter catches a wrong magic
// value.
func TestCheckFooter_MagicMismatch(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	out, err := dir.CreateOutput("magic.dat", store.IOContext{})
	if err != nil {
		t.Fatal(err)
	}
	// Write 16 bytes: wrong magic + alg=0 + checksum=0.
	if err := out.WriteInt(-559038737); err != nil { // 0xDEADBEEF as signed int32
		t.Fatal(err)
	}
	if err := out.WriteInt(0); err != nil {
		t.Fatal(err)
	}
	if err := out.WriteLong(0); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}

	in, err := dir.OpenInput("magic.dat", store.IOContext{})
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	csum := store.NewBufferedChecksumIndexInput(in)
	if err := checkFooter(csum); err == nil {
		t.Error("expected error for mismatched magic value")
	}
}

// TestCheckFooter_WrongAlgorithmID verifies that checkFooter catches a non-zero
// algorithm ID.
func TestCheckFooter_WrongAlgorithmID(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	out, err := dir.CreateOutput("alg.dat", store.IOContext{})
	if err != nil {
		t.Fatal(err)
	}
	const footerMagic = int32(^0x3FD76C17)
	if err := out.WriteInt(footerMagic); err != nil {
		t.Fatal(err)
	}
	if err := out.WriteInt(1); err != nil { // non-zero algorithm ID
		t.Fatal(err)
	}
	if err := out.WriteLong(0); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}

	in, err := dir.OpenInput("alg.dat", store.IOContext{})
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	csum := store.NewBufferedChecksumIndexInput(in)
	if err := checkFooter(csum); err == nil {
		t.Error("expected error for non-zero algorithm ID")
	}
}

// TestFieldReader_GetMinMaxNil verifies that GetMin and GetMax return (nil, nil)
// when minTerm and maxTerm are nil.
func TestFieldReader_GetMinMaxNil(t *testing.T) {
	fr := &FieldReader{}
	min, err := fr.GetMin()
	if err != nil {
		t.Fatalf("GetMin: %v", err)
	}
	if min != nil {
		t.Errorf("GetMin: want nil, got %v", min)
	}
	max, err := fr.GetMax()
	if err != nil {
		t.Fatalf("GetMax: %v", err)
	}
	if max != nil {
		t.Errorf("GetMax: want nil, got %v", max)
	}
}

// TestFieldReader_GetStatsNotAvailable verifies that GetStats returns
// ErrBlockTraversalNotAvailable.
func TestFieldReader_GetStatsNotAvailable(t *testing.T) {
	fr := &FieldReader{}
	_, err := fr.GetStats()
	if err != ErrBlockTraversalNotAvailable {
		t.Errorf("GetStats: got %v want %v", err, ErrBlockTraversalNotAvailable)
	}
}

// TestFieldReader_Size verifies Size returns the numTerms field.
func TestFieldReader_Size(t *testing.T) {
	fr := &FieldReader{numTerms: 42}
	if got := fr.Size(); got != 42 {
		t.Errorf("Size: got %d want 42", got)
	}
	fr.numTerms = 0
	if got := fr.Size(); got != 0 {
		t.Errorf("Size (zero): got %d want 0", got)
	}
}

// TestLucene40BlockTreeTermsReader_SizeAndString verifies the reader-level
// methods work on a properly initialized instance.
func TestLucene40BlockTreeTermsReader_SizeAndString(t *testing.T) {
	r := &Lucene40BlockTreeTermsReader{
		segment: "testseg",
		postingsReader: &noopPostingsReader{},
	}
	if r.Size() != 0 {
		t.Errorf("Size: got %d want 0", r.Size())
	}
	if s := r.String(); s == "" {
		t.Error("String: expected non-empty")
	}
}
