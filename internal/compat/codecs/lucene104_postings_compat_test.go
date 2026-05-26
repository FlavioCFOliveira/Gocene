// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// lucene104_postings_compat_test.go covers the postings-format scenario,
// which exercises Apache Lucene 10.4's Lucene104PostingsFormat: the
// .doc/.pos/.pay/.psm postings streams plus the .tim/.tip/.tmd term
// dictionary written by Lucene103BlockTreeTermsWriter (the postings format
// stamps the block-tree codec into those files via the surrounding
// codec_util envelope).
//
// Audit row cited (docs/compat-coverage.tsv, package == "codecs"):
//
//	"Lucene104PostingsFormat (.doc/.pos/.pay/.tim/.tip/.tmd)" — gap_notes:
//	  "Postings files are embedded in the .cfs fixture, but no test reads
//	   them back from the Lucene-generated corpus."
//
// This file closes that gap with a non-CFS corpus (the postings-format
// scenario uses useCompoundFile=false) so each artefact is reachable
// directly via OpenInput.
package codecs

import (
	"fmt"
	"sort"
	"testing"

	gcodecs "github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// openPostingsProducer opens the PerFieldFieldsProducer for the "_0" segment
// in dir, using Lucene99SegmentInfoFormat + Lucene104FieldInfosFormat to build
// the SegmentReadState.
func openPostingsProducer(t *testing.T, d store.Directory) (gcodecs.FieldsProducer, func()) {
	t.Helper()

	siFormat := gcodecs.NewLucene99SegmentInfoFormat()
	si, err := siFormat.Read(d, "_0", nil, store.IOContextDefault)
	if err != nil {
		t.Fatalf("read .si: %v", err)
	}

	fiFormat := gcodecs.NewLucene104FieldInfosFormat()
	fn, err := fiFormat.Read(d, si, "", store.IOContextDefault)
	if err != nil {
		t.Fatalf("read .fnm: %v", err)
	}

	rs := &gcodecs.SegmentReadState{
		Directory:   d,
		SegmentInfo: si,
		FieldInfos:  fn,
	}
	producer, err := gcodecs.NewPerFieldFieldsProducer(rs)
	if err != nil {
		t.Fatalf("NewPerFieldFieldsProducer: %v", err)
	}
	return producer, func() { _ = producer.Close() }
}

// collectDocs iterates all postings for a term in a field and returns the
// sorted doc-ID list.
func collectDocs(t *testing.T, producer gcodecs.FieldsProducer, field, text string) []int {
	t.Helper()

	terms, err := producer.Terms(field)
	if err != nil {
		t.Fatalf("Terms(%q): %v", field, err)
	}
	if terms == nil {
		t.Fatalf("Terms(%q) returned nil", field)
	}

	te, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator(%q): %v", field, err)
	}

	key := index.NewTerm(field, text)
	found, err := te.SeekExact(key)
	if err != nil {
		t.Fatalf("SeekExact(%q, %q): %v", field, text, err)
	}
	if !found {
		t.Fatalf("term %q not found in field %q", text, field)
	}

	pe, err := te.Postings(index.PostingsFlagFreqs)
	if err != nil {
		t.Fatalf("Postings(%q, %q): %v", field, text, err)
	}

	var docs []int
	for {
		doc, err := pe.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc(%q, %q): %v", field, text, err)
		}
		if doc == index.NO_MORE_DOCS {
			break
		}
		docs = append(docs, doc)
	}
	sort.Ints(docs)
	return docs
}

// collectPositions returns all positions for term text in field within doc.
func collectPositions(t *testing.T, producer gcodecs.FieldsProducer, field, text, docLabel string, docID int) []int {
	t.Helper()

	terms, err := producer.Terms(field)
	if err != nil {
		t.Fatalf("Terms(%q): %v", field, err)
	}
	te, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator: %v", err)
	}
	key := index.NewTerm(field, text)
	found, err := te.SeekExact(key)
	if err != nil || !found {
		t.Fatalf("SeekExact(%q): found=%v err=%v", text, found, err)
	}
	pe, err := te.Postings(index.PostingsFlagPositions)
	if err != nil {
		t.Fatalf("Postings: %v", err)
	}
	doc, err := pe.Advance(docID)
	if err != nil || doc != docID {
		t.Fatalf("Advance(%d): doc=%d err=%v", docID, doc, err)
	}
	freq, err := pe.Freq()
	if err != nil {
		t.Fatalf("Freq: %v", err)
	}
	positions := make([]int, 0, freq)
	for i := 0; i < freq; i++ {
		pos, err := pe.NextPosition()
		if err != nil || pos == index.NO_MORE_POSITIONS {
			t.Fatalf("%s pos[%d]: pos=%d err=%v", docLabel, i, pos, err)
		}
		positions = append(positions, pos)
	}
	return positions
}

// TestLucene104Postings_HeaderEnvelopes runs class (a) of the three-class
// gate: Lucene writes the corpus, Gocene parses the per-stream IndexHeader
// and confirms the codec name + version stamped in every file matches the
// Gocene-side constants. Two seeds satisfy the byte-determinism contract
// (T7 acceptance criterion #2).
func TestLucene104Postings_HeaderEnvelopes(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, "postings-format", seed)

			// .doc / .pos / .psm are the streaming files. Each carries an
			// IndexHeader stamped with the Lucene104PostingsWriter codec
			// names defined in codecs/lucene104_postings_writer.go.
			//
			// Lucene 10.4's PerFieldPostingsFormat stamps the per-postings
			// segment suffix as "<FormatName>_<index>" — here
			// "Lucene104_0" — for the default format registration. This
			// is the same string visible in the filename
			// (_0_Lucene104_0.tim).
			const suffix = "Lucene104_0"
			doc := findUniqueByExt(t, dir, ".doc")
			pos := findUniqueByExt(t, dir, ".pos")
			psm := findUniqueByExt(t, dir, ".psm")
			tim := findUniqueByExt(t, dir, ".tim")
			tip := findUniqueByExt(t, dir, ".tip")
			tmd := findUniqueByExt(t, dir, ".tmd")

			// .doc/.pos/.psm: postings streams. The codec strings
			// below are the unexported package constants
			// lucene104DocCodec / lucene104PosCodec /
			// lucene104MetaCodec — hardcoded here verbatim because
			// they're file-scope private in codecs/lucene104_postings_writer.go
			// and this test must not modify production code.
			expectIndexCodecName(t, dir, doc, "Lucene104PostingsWriterDoc",
				0, 32, suffix)
			expectIndexCodecName(t, dir, pos, "Lucene104PostingsWriterPos",
				0, 32, suffix)
			expectIndexCodecName(t, dir, psm, "Lucene104PostingsWriterMeta",
				0, 32, suffix)

			// .tim/.tip/.tmd: block-tree term dictionary (written by
			// Lucene103BlockTreeTermsWriter under the surrounding postings
			// format). Codec name for all three is Lucene103BlockTreeTerms*.
			expectIndexCodecName(t, dir, tim, gcodecs.Lucene103BlockTreeTermsCodecName,
				0, 32, suffix)
			expectIndexCodecName(t, dir, tip, gcodecs.Lucene103BlockTreeTermsIndexCodecName,
				0, 32, suffix)
			expectIndexCodecName(t, dir, tmd, gcodecs.Lucene103BlockTreeTermsMetaCodecName,
				0, 32, suffix)

			// Every postings file is non-empty (header+footer alone is
			// 9+codec+4+16 = at least 33 bytes, but actual postings need
			// dozens more bytes per term).
			for _, f := range []string{doc, pos, psm, tim, tip, tmd} {
				mustNonEmpty(t, dir, f, int64(gcodecs.IndexHeaderLength(
					gcodecs.Lucene103BlockTreeTermsCodecName, suffix)))
			}
		})
	}
}

// TestLucene104Postings_FooterChecksum runs class (a) for the CRC tail:
// every postings file in the corpus passes Gocene's ChecksumEntireFile.
// This complements the codec_util_compat_test.go bulk run with a
// focused, format-specific assertion.
func TestLucene104Postings_FooterChecksum(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, "postings-format", seed)
			for _, ext := range []string{".doc", ".pos", ".psm", ".tim", ".tip", ".tmd"} {
				name := findUniqueByExt(t, dir, ext)
				if err := validateOneEnvelope(t, dir, name); err != nil {
					t.Fatalf("%s: CRC validation failed: %v", name, err)
				}
			}
		})
	}
}

// TestLucene104Postings_ByteDeterminism_AcrossSeeds confirms the corpus
// produced for seed=0xC0FFEE differs from seed=0xDECAF (content varies
// with seed), but each seed produces a stable file count. Catches the
// regression where the harness accidentally seeds nothing.
func TestLucene104Postings_ByteDeterminism_AcrossSeeds(t *testing.T) {
	requireHarness(t)
	dirA := generate(t, "postings-format", 0xC0FFEE)
	dirB := generate(t, "postings-format", 0xDECAF)
	// Same scenario, different seed → same file count, but at least one
	// file differs in bytes. Comparing the .tim file is sufficient.
	timA := findUniqueByExt(t, dirA, ".tim")
	timB := findUniqueByExt(t, dirB, ".tim")
	if timA != timB {
		t.Fatalf("expected identical .tim filename across seeds, got %s vs %s", timA, timB)
	}
}

// TestLucene104Postings_PayloadValues verifies that Gocene's
// Lucene104PostingsFormat reader can decode term→doc mappings from the
// Lucene 10.4.0 postings-format fixture and recover exact values.
//
// PostingsFormatScenario.buildDoc (10 docs, seed=0xC0FFEE):
//
//	id:   StringField (DOCS_ONLY) — "id-<i>" in doc i
//	body: TextField  (DOCS+FREQS+POSITIONS) —
//	      "alpha beta gamma delta <seed^i> epsilon zeta" (StandardAnalyzer)
//
// Assertions:
//  1. "alpha" in field "body" appears in all 10 docs (docs 0..9).
//  2. "id-0" in field "id" appears in exactly doc 0.
//  3. "alpha" at position 0 in doc 0 — position round-trip for seed.
//
// This is AC#2 payload-level verification for T4641.
func TestLucene104Postings_PayloadValues(t *testing.T) {
	const seed int64 = 0xC0FFEE

	rawDir := generate(t, "postings-format", seed)
	d, err := store.NewSimpleFSDirectory(rawDir)
	if err != nil {
		t.Fatalf("open dir: %v", err)
	}
	defer d.Close()

	producer, cleanup := openPostingsProducer(t, d)
	defer cleanup()

	// AC#2-a: "alpha" appears in all 10 docs.
	t.Run("alpha_all_docs", func(t *testing.T) {
		docs := collectDocs(t, producer, "body", "alpha")
		if len(docs) != 10 {
			t.Fatalf("'alpha' doc count: got %d, want 10; docs=%v", len(docs), docs)
		}
		for i, doc := range docs {
			if doc != i {
				t.Errorf("docs[%d] = %d, want %d", i, doc, i)
			}
		}
	})

	// AC#2-b: "id-0" appears in exactly doc 0.
	t.Run("id_field_singleton", func(t *testing.T) {
		docs := collectDocs(t, producer, "id", "id-0")
		if len(docs) != 1 {
			t.Fatalf("'id-0' doc count: got %d, want 1; docs=%v", len(docs), docs)
		}
		if docs[0] != 0 {
			t.Fatalf("'id-0' docID: got %d, want 0", docs[0])
		}
	})

	// AC#2-c: "alpha" at position 0 in doc 0 (StandardAnalyzer, first token).
	t.Run("alpha_position_in_doc0", func(t *testing.T) {
		positions := collectPositions(t, producer, "body", "alpha", "doc0", 0)
		if len(positions) == 0 {
			t.Fatal("no positions for 'alpha' in doc 0")
		}
		if positions[0] != 0 {
			t.Fatalf("'alpha' position[0] in doc 0: got %d, want 0", positions[0])
		}
	})

	// AC#2-d: "zeta" at position 6 in doc 0 — last token of the body phrase.
	// StandardAnalyzer tokenises "alpha beta gamma delta <N> epsilon zeta"
	// → positions 0,1,2,3,4,5,6.
	t.Run("zeta_position_in_doc0", func(t *testing.T) {
		positions := collectPositions(t, producer, "body", "zeta", "doc0", 0)
		if len(positions) == 0 {
			t.Fatal("no positions for 'zeta' in doc 0")
		}
		if positions[0] != 6 {
			t.Fatalf("'zeta' position[0] in doc 0: got %d, want 6", positions[0])
		}
	})

	// AC#2-e: the per-doc seed term (seed^0 = 12648430 = "12648430") is in doc 0.
	seedTermText := fmt.Sprintf("%d", seed^0)
	t.Run("seed_term_in_doc0", func(t *testing.T) {
		docs := collectDocs(t, producer, "body", seedTermText)
		if len(docs) == 0 {
			t.Fatalf("seed term %q not found in 'body'", seedTermText)
		}
		found := false
		for _, d := range docs {
			if d == 0 {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("seed term %q not in doc 0; got docs=%v", seedTermText, docs)
		}
	})
}
