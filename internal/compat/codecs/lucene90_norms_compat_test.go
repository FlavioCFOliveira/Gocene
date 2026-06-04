// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// lucene90_norms_compat_test.go covers Lucene90NormsFormat: the .nvd
// payload + .nvm metadata pair.
//
// Audit row cited (docs/compat-coverage.tsv, package == "codecs"):
//
//	"Lucene90NormsFormat (.nvd/.nvm)" — gap_notes:
//	  "No golden norms file from Lucene; compatibility test is
//	   Gocene-only."
package codecs

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestLucene90Norms_DataAndMetaEnvelopes validates the codec headers on
// the .nvd / .nvm pair. Norms files DO NOT carry a per-field segment
// suffix (the format is not registered through PerFieldNormsFormat), so
// the expected suffix is the empty string.
func TestLucene90Norms_DataAndMetaEnvelopes(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, "norms-format", seed)
			const suffix = ""
			nvd := findUniqueByExt(t, dir, ".nvd")
			nvm := findUniqueByExt(t, dir, ".nvm")
			expectIndexCodecName(t, dir, nvd, codecs.Lucene90NormsDataCodec,
				0, 32, suffix)
			expectIndexCodecName(t, dir, nvm, codecs.Lucene90NormsMetadataCodec,
				0, 32, suffix)
		})
	}
}

// TestLucene90Norms_FooterCRC validates the trailing CRC32 in both files.
func TestLucene90Norms_FooterCRC(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, "norms-format", seed)
			for _, ext := range []string{".nvd", ".nvm"} {
				name := findUniqueByExt(t, dir, ext)
				if err := validateOneEnvelope(t, dir, name); err != nil {
					t.Fatalf("%s: CRC validation failed: %v", name, err)
				}
			}
		})
	}
}

// openNormsProducer opens the Lucene90NormsProducer for the "_0" segment in
// dir, reusing the .si / .fnm metadata readers established by the other
// codec compat tests. The returned cleanup closes both the producer and the
// SimpleFSDirectory.
func openNormsProducer(t *testing.T, rawDir string) (codecs.NormsProducer, *index.FieldInfos, func()) {
	t.Helper()
	d, err := store.NewSimpleFSDirectory(rawDir)
	if err != nil {
		t.Fatalf("open dir: %v", err)
	}
	siFormat := codecs.NewLucene99SegmentInfoFormat()
	si, err := siFormat.Read(d, "_0", nil, store.IOContextDefault)
	if err != nil {
		_ = d.Close()
		t.Fatalf("read .si: %v", err)
	}
	fiFormat := codecs.NewLucene104FieldInfosFormat()
	fn, err := fiFormat.Read(d, si, "", store.IOContextDefault)
	if err != nil {
		_ = d.Close()
		t.Fatalf("read .fnm: %v", err)
	}
	rs := &codecs.SegmentReadState{
		Directory:   d,
		SegmentInfo: si,
		FieldInfos:  fn,
	}
	format := codecs.NewLucene90NormsFormat()
	producer, err := format.NormsProducer(rs)
	if err != nil {
		_ = d.Close()
		t.Fatalf("NormsProducer: %v", err)
	}
	return producer, fn, func() {
		_ = producer.Close()
		_ = d.Close()
	}
}

// TestLucene90Norms_PayloadValues is the class (b) gate for norms: Gocene
// reads the per-document norm bytes that Lucene 10.4.0 produced and recovers
// the exact value for each document.
//
// NormsFormatScenario (10 docs) indexes a single TextField "body" whose
// doc i carries i+1 distinct tokens, so the BM25 length norm for doc i is
// SmallFloat.intToByte4(i+1). For lengths 1..10 (all below NUM_FREE_VALUES)
// intToByte4 is the identity, so the stored single-byte norm — and therefore
// the value the Gocene producer decodes — must equal i+1 exactly.
//
// This upgrades the norms compat coverage from envelope-only (the gap note in
// docs/compat-coverage.tsv: "No golden norms file from Lucene; compatibility
// test is Gocene-only.") to a true Lucene-write -> Gocene-read parity check.
func TestLucene90Norms_PayloadValues(t *testing.T) {
	const seed int64 = 0xC0FFEE
	const numDocs = 10

	rawDir := generate(t, "norms-format", seed)
	producer, fn, cleanup := openNormsProducer(t, rawDir)
	defer cleanup()

	if err := producer.CheckIntegrity(); err != nil {
		t.Fatalf("CheckIntegrity: %v", err)
	}

	fi := fn.GetByName("body")
	if fi == nil {
		t.Fatal("field body not found in FieldInfos")
	}
	if !fi.HasNorms() {
		t.Fatalf("field body unexpectedly has no norms (omitNorms=%v)", fi.OmitNorms())
	}

	dv, err := producer.GetNorms(fi)
	if err != nil {
		t.Fatalf("GetNorms(body): %v", err)
	}
	if dv == nil {
		t.Fatal("GetNorms(body): nil NumericDocValues")
	}

	for i := 0; i < numDocs; i++ {
		doc, err := dv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc[%d]: %v", i, err)
		}
		if doc != i {
			t.Fatalf("doc[%d] = %d, want %d (every doc has the body field)", i, doc, i)
		}
		got, err := dv.LongValue()
		if err != nil {
			t.Fatalf("LongValue at doc %d: %v", doc, err)
		}
		wantByte, err := util.IntToByte4(i + 1)
		if err != nil {
			t.Fatalf("IntToByte4(%d): %v", i+1, err)
		}
		// The stored norm is a signed single byte; the producer sign-extends
		// via int8. For lengths 1..10 the byte is small and positive.
		want := int64(int8(wantByte))
		if got != want {
			t.Fatalf("norm[doc=%d, length=%d] = %d, want %d (intToByte4=%d)",
				doc, i+1, got, want, wantByte)
		}
	}

	doc, err := dv.NextDoc()
	if err != nil {
		t.Fatalf("trailing NextDoc: %v", err)
	}
	if doc != mathMaxInt32 {
		t.Fatalf("expected NO_MORE_DOCS after %d docs, got doc=%d", numDocs, doc)
	}
}

// mathMaxInt32 is the NO_MORE_DOCS sentinel value (math.MaxInt32), declared
// locally to avoid an extra import in this build-tagged compat file.
const mathMaxInt32 = 1<<31 - 1
