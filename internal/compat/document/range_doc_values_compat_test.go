// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// range_doc_values_compat_test.go verifies that Gocene can read range
// doc-values bytes written by Apache Lucene 10.4.0 and that the encoded
// bytes match Gocene's EncodeDoubleRangeLucene / EncodeLongRangeLucene
// output byte-for-byte.
//
// Audit row (docs/compat-coverage.tsv, row 101):
//
//	"Range field doc-values encoding" — gap_notes:
//	  "No cross-engine fixture for range encodings."
package document

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	gindex "github.com/FlavioCFOliveira/Gocene/index"
)

// TestRangeDocValues_DoubleRangePayloadValues verifies that the double
// range doc-values bytes produced by Lucene 10.4.0's
// DoubleRangeDocValuesField are byte-identical to Gocene's
// EncodeDoubleRangeLucene output.
//
// DocumentRangeDocValuesScenario.buildDoc defines:
//
//	dbl_range (DoubleRangeDocValuesField):
//	  min = [1.0+i, 10.0+i*2]
//	  max = [5.0+i, 20.0+i*2]
func TestRangeDocValues_DoubleRangePayloadValues(t *testing.T) {
	const seed int64 = 0xC0FFEE
	const numDocs = 3

	rawDir := generate(t, ScenarioDocumentRangeDV, seed)
	producer, fn, cleanup := openDVProducer(t, rawDir)
	defer cleanup()

	fi := fn.GetByName("dbl_range")
	if fi == nil {
		t.Fatal("field dbl_range not found")
	}
	bdv, err := producer.GetBinary(fi)
	if err != nil {
		t.Fatalf("GetBinary: %v", err)
	}

	for i := 0; i < numDocs; i++ {
		docID, err := bdv.NextDoc()
		if err != nil {
			t.Fatalf("doc %d NextDoc: %v", i, err)
		}
		if docID != i {
			t.Fatalf("doc %d: NextDoc=%d", i, docID)
		}

		got, err := bdv.BinaryValue()
		if err != nil {
			t.Fatalf("doc %d BinaryValue: %v", i, err)
		}

		min := []float64{1.0 + float64(i), 10.0 + float64(i)*2}
		max := []float64{5.0 + float64(i), 20.0 + float64(i)*2}
		want, err := document.EncodeDoubleRangeLucene(min, max)
		if err != nil {
			t.Fatalf("doc %d EncodeDoubleRangeLucene: %v", i, err)
		}

		if !bytes.Equal(got, want) {
			t.Errorf("doc %d: bytes %x, want %x", i, got, want)
		}
	}
}

// TestRangeDocValues_LongRangePayloadValues verifies that the long range
// doc-values bytes produced by Lucene 10.4.0's LongRangeDocValuesField are
// byte-identical to Gocene's EncodeLongRangeLucene output.
//
// DocumentRangeDocValuesScenario.buildDoc defines:
//
//	long_range (LongRangeDocValuesField):
//	  min = [100+i, 1000+i*10]
//	  max = [500+i*2, 5000+i*20]
func TestRangeDocValues_LongRangePayloadValues(t *testing.T) {
	const seed int64 = 0xC0FFEE
	const numDocs = 3

	rawDir := generate(t, ScenarioDocumentRangeDV, seed)
	producer, fn, cleanup := openDVProducer(t, rawDir)
	defer cleanup()

	fi := fn.GetByName("long_range")
	if fi == nil {
		t.Fatal("field long_range not found")
	}
	bdv, err := producer.GetBinary(fi)
	if err != nil {
		t.Fatalf("GetBinary: %v", err)
	}

	for i := 0; i < numDocs; i++ {
		docID, err := bdv.NextDoc()
		if err != nil {
			t.Fatalf("doc %d NextDoc: %v", i, err)
		}
		if docID != i {
			t.Fatalf("doc %d: NextDoc=%d", i, docID)
		}

		got, err := bdv.BinaryValue()
		if err != nil {
			t.Fatalf("doc %d BinaryValue: %v", i, err)
		}

		min := []int64{100 + int64(i), 1000 + int64(i)*10}
		max := []int64{500 + int64(i)*2, 5000 + int64(i)*20}
		want, err := document.EncodeLongRangeLucene(min, max)
		if err != nil {
			t.Fatalf("doc %d EncodeLongRangeLucene: %v", i, err)
		}

		if !bytes.Equal(got, want) {
			t.Errorf("doc %d: bytes %x, want %x", i, got, want)
		}
	}
}

// TestRangeDocValues_AllSeeds verifies range DV payloads at both canary seeds.
func TestRangeDocValues_AllSeeds(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run(fmt.Sprintf("seed=%d", seed), func(t *testing.T) {
			rawDir := generate(t, ScenarioDocumentRangeDV, seed)
			producer, fn, cleanup := openDVProducer(t, rawDir)
			defer cleanup()

			for _, name := range []string{"dbl_range", "long_range"} {
				fi := fn.GetByName(name)
				if fi == nil {
					t.Errorf("field %q not found", name)
					continue
				}
				if fi.DocValuesType() != gindex.DocValuesTypeBinary {
					t.Errorf("%q: expected Binary DV, got %v", name, fi.DocValuesType())
					continue
				}
				bdv, err := producer.GetBinary(fi)
				if err != nil {
					t.Errorf("GetBinary(%q): %v", name, err)
					continue
				}
				docID, err := bdv.NextDoc()
				if err != nil {
					t.Errorf("%q NextDoc: %v", name, err)
					continue
				}
				if docID < 0 {
					t.Errorf("%q: no docs", name)
					continue
				}
				raw, err := bdv.BinaryValue()
				if err != nil {
					t.Errorf("%q BinaryValue: %v", name, err)
				} else if len(raw) == 0 {
					t.Errorf("%q: empty binary value", name)
				}
			}
		})
	}
}
