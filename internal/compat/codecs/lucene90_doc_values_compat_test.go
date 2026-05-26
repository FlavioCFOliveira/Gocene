// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// lucene90_doc_values_compat_test.go covers Lucene90DocValuesFormat: the
// .dvd payload + .dvm metadata pair, exercising the NUMERIC, BINARY,
// SORTED, SORTED_NUMERIC and SORTED_SET flavours (the doc-values-format
// scenario emits one document carrying all five).
//
// Audit row cited (docs/compat-coverage.tsv, package == "codecs"):
//
//	"Lucene90DocValuesFormat (.dvd/.dvm)" — gap_notes:
//	  "No test reads doc-values segments produced by Lucene; combined
//	   test uses Gocene-only round-trip."
package codecs

import (
	"fmt"
	"testing"

	gcodecs "github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestLucene90DocValues_DataAndMetaEnvelopes opens the .dvd/.dvm pair
// emitted by Lucene and confirms the IndexHeader codec strings + version
// matches Gocene's constants (Lucene90DocValuesDataCodec /
// Lucene90DocValuesMetaCodec).
func TestLucene90DocValues_DataAndMetaEnvelopes(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, "doc-values-format", seed)
			const suffix = "Lucene90_0"
			dvd := findUniqueByExt(t, dir, ".dvd")
			dvm := findUniqueByExt(t, dir, ".dvm")
			version := expectIndexCodecName(t, dir, dvd,
				gcodecs.Lucene90DocValuesDataCodec,
				gcodecs.Lucene90DocValuesVersionStart,
				gcodecs.Lucene90DocValuesVersionCurrent, suffix)
			if version != gcodecs.Lucene90DocValuesVersionCurrent {
				t.Errorf("%s: version=%d, want %d", dvd, version,
					gcodecs.Lucene90DocValuesVersionCurrent)
			}
			expectIndexCodecName(t, dir, dvm,
				gcodecs.Lucene90DocValuesMetaCodec,
				gcodecs.Lucene90DocValuesVersionStart,
				gcodecs.Lucene90DocValuesVersionCurrent, suffix)
		})
	}
}

// openDVProducer opens the PerFieldDocValuesProducer for the "_0" segment in
// dir, reusing the .si / .fnm metadata readers established by the postings
// compat tests. The returned cleanup function closes both the producer and the
// SimpleFSDirectory.
func openDVProducer(t *testing.T, rawDir string) (*gcodecs.PerFieldDocValuesProducer, *index.FieldInfos, func()) {
	t.Helper()
	d, err := store.NewSimpleFSDirectory(rawDir)
	if err != nil {
		t.Fatalf("open dir: %v", err)
	}
	siFormat := gcodecs.NewLucene99SegmentInfoFormat()
	si, err := siFormat.Read(d, "_0", nil, store.IOContextDefault)
	if err != nil {
		_ = d.Close()
		t.Fatalf("read .si: %v", err)
	}
	fiFormat := gcodecs.NewLucene104FieldInfosFormat()
	fn, err := fiFormat.Read(d, si, "", store.IOContextDefault)
	if err != nil {
		_ = d.Close()
		t.Fatalf("read .fnm: %v", err)
	}
	rs := &gcodecs.SegmentReadState{
		Directory:   d,
		SegmentInfo: si,
		FieldInfos:  fn,
	}
	producer, err := gcodecs.NewPerFieldDocValuesProducer(rs)
	if err != nil {
		_ = d.Close()
		t.Fatalf("NewPerFieldDocValuesProducer: %v", err)
	}
	return producer, fn, func() {
		_ = producer.Close()
		_ = d.Close()
	}
}

// TestLucene90DocValues_FooterCRC validates the trailing CRC32 in both
// files (class (a) of the three-class gate).
func TestLucene90DocValues_FooterCRC(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run("", func(t *testing.T) {
			dir := generate(t, "doc-values-format", seed)
			for _, ext := range []string{".dvd", ".dvm"} {
				name := findUniqueByExt(t, dir, ext)
				if err := validateOneEnvelope(t, dir, name); err != nil {
					t.Fatalf("%s: CRC validation failed: %v", name, err)
				}
			}
		})
	}
}

// TestLucene90DocValues_PayloadValues exercises class (b) of the three-class
// gate: Gocene reads Lucene-produced doc-values bytes and recovers exact field
// values for all five DV flavours written by DocValuesFormatScenario.
//
// DocValuesFormatScenario.buildDoc (10 docs, seed S):
//
//	dv_num  (NUMERIC):        value = S + i
//	dv_bin  (BINARY):         "bin-" + (S+i)
//	dv_sorted (SORTED):       "s-" + ((S+i) % 4)
//	dv_sn   (SORTED_NUMERIC): two values per doc: [S+i, (S+i)*2]
//	dv_ss   (SORTED_SET):     two values per doc: sorted["a-"+(S+i)%3, "b-"+(S+i)%5]
//
// Verification strategy: read the first three docs for each field, compare
// against the deterministic formula output. This proves the codec bytes are
// correctly decoded by Gocene (AC#3 for T4642).
func TestLucene90DocValues_PayloadValues(t *testing.T) {
	const seed int64 = 0xC0FFEE // 12648430
	const numCheckDocs = 3

	rawDir := generate(t, "doc-values-format", seed)
	producer, fn, cleanup := openDVProducer(t, rawDir)
	defer cleanup()

	t.Run("NUMERIC", func(t *testing.T) {
		fi := fn.GetByName("dv_num")
		if fi == nil {
			t.Fatal("field dv_num not found in FieldInfos")
		}
		ndv, err := producer.GetNumeric(fi)
		if err != nil {
			t.Fatalf("GetNumeric: %v", err)
		}
		for i := 0; i < numCheckDocs; i++ {
			doc, err := ndv.NextDoc()
			if err != nil {
				t.Fatalf("doc %d NextDoc: %v", i, err)
			}
			if doc != i {
				t.Fatalf("doc %d: NextDoc=%d", i, doc)
			}
			got, err := ndv.LongValue()
			if err != nil {
				t.Fatalf("doc %d LongValue: %v", i, err)
			}
			want := seed + int64(i)
			if got != want {
				t.Errorf("doc %d: got %d, want %d", i, got, want)
			}
		}
	})

	t.Run("BINARY", func(t *testing.T) {
		fi := fn.GetByName("dv_bin")
		if fi == nil {
			t.Fatal("field dv_bin not found in FieldInfos")
		}
		bdv, err := producer.GetBinary(fi)
		if err != nil {
			t.Fatalf("GetBinary: %v", err)
		}
		for i := 0; i < numCheckDocs; i++ {
			doc, err := bdv.NextDoc()
			if err != nil {
				t.Fatalf("doc %d NextDoc: %v", i, err)
			}
			if doc != i {
				t.Fatalf("doc %d: NextDoc=%d", i, doc)
			}
			got, err := bdv.BinaryValue()
			if err != nil {
				t.Fatalf("doc %d BinaryValue: %v", i, err)
			}
			want := fmt.Sprintf("bin-%d", seed+int64(i))
			if string(got) != want {
				t.Errorf("doc %d: got %q, want %q", i, got, want)
			}
		}
	})

	t.Run("SORTED", func(t *testing.T) {
		fi := fn.GetByName("dv_sorted")
		if fi == nil {
			t.Fatal("field dv_sorted not found in FieldInfos")
		}
		sdv, err := producer.GetSorted(fi)
		if err != nil {
			t.Fatalf("GetSorted: %v", err)
		}
		for i := 0; i < numCheckDocs; i++ {
			doc, err := sdv.NextDoc()
			if err != nil {
				t.Fatalf("doc %d NextDoc: %v", i, err)
			}
			if doc != i {
				t.Fatalf("doc %d: NextDoc=%d", i, doc)
			}
			ord, err := sdv.OrdValue()
			if err != nil {
				t.Fatalf("doc %d OrdValue: %v", i, err)
			}
			termBytes, err := sdv.LookupOrd(ord)
			if err != nil {
				t.Fatalf("doc %d LookupOrd(%d): %v", i, ord, err)
			}
			want := fmt.Sprintf("s-%d", (seed+int64(i))%4)
			if string(termBytes) != want {
				t.Errorf("doc %d: got %q, want %q", i, termBytes, want)
			}
		}
	})

	t.Run("SORTED_NUMERIC", func(t *testing.T) {
		fi := fn.GetByName("dv_sn")
		if fi == nil {
			t.Fatal("field dv_sn not found in FieldInfos")
		}
		sndv, err := producer.GetSortedNumeric(fi)
		if err != nil {
			t.Fatalf("GetSortedNumeric: %v", err)
		}
		for i := 0; i < numCheckDocs; i++ {
			doc, err := sndv.NextDoc()
			if err != nil {
				t.Fatalf("doc %d NextDoc: %v", i, err)
			}
			if doc != i {
				t.Fatalf("doc %d: NextDoc=%d", i, doc)
			}
			cnt, err := sndv.DocValueCount()
			if err != nil {
				t.Fatalf("doc %d DocValueCount: %v", i, err)
			}
			if cnt != 2 {
				t.Fatalf("doc %d: count=%d, want 2", i, cnt)
			}
			base := seed + int64(i)
			// Lucene sorts multi-values ascending; base < base*2 always holds
			// for seed=0xC0FFEE and any i in [0,9].
			wantVals := []int64{base, base * 2}
			for j := 0; j < cnt; j++ {
				got, err := sndv.NextValue()
				if err != nil {
					t.Fatalf("doc %d val[%d]: %v", i, j, err)
				}
				if got != wantVals[j] {
					t.Errorf("doc %d val[%d]: got %d, want %d", i, j, got, wantVals[j])
				}
			}
		}
	})

	t.Run("SORTED_SET", func(t *testing.T) {
		fi := fn.GetByName("dv_ss")
		if fi == nil {
			t.Fatal("field dv_ss not found in FieldInfos")
		}
		ssdv, err := producer.GetSortedSet(fi)
		if err != nil {
			t.Fatalf("GetSortedSet: %v", err)
		}
		for i := 0; i < numCheckDocs; i++ {
			doc, err := ssdv.NextDoc()
			if err != nil {
				t.Fatalf("doc %d NextDoc: %v", i, err)
			}
			if doc != i {
				t.Fatalf("doc %d: NextDoc=%d", i, doc)
			}
			base := seed + int64(i)
			// Lucene adds "a-..." and "b-..." values; the sorted order is
			// lexicographic. "a-..." < "b-..." always, so ord 0 = "a-", ord 1 = "b-".
			// We verify the two values via LookupOrd.
			wantTerms := [2]string{
				fmt.Sprintf("a-%d", base%3),
				fmt.Sprintf("b-%d", base%5),
			}
			for j := 0; j < 2; j++ {
				ord, err := ssdv.NextOrd()
				if err != nil {
					t.Fatalf("doc %d ord[%d]: %v", i, j, err)
				}
				if ord < 0 {
					t.Fatalf("doc %d ord[%d]: got -1 (no more ords)", i, j)
				}
				termBytes, err := ssdv.LookupOrd(ord)
				if err != nil {
					t.Fatalf("doc %d LookupOrd(%d): %v", i, ord, err)
				}
				got := string(termBytes)
				// Check that the term matches one of the expected values.
				if got != wantTerms[0] && got != wantTerms[1] {
					t.Errorf("doc %d term[%d]: got %q, want one of %v",
						i, j, got, wantTerms)
				}
			}
			// No more ords for this doc.
			endOrd, err := ssdv.NextOrd()
			if err != nil {
				t.Fatalf("doc %d extra NextOrd: %v", i, err)
			}
			if endOrd != -1 {
				t.Fatalf("doc %d: expected -1 after 2 ords, got %d", i, endOrd)
			}
		}
	})
}
