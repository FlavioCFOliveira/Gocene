// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build gocene_monsters

// The @Nightly test port of
// lucene/core/src/test/org/apache/lucene/codecs/lucene90/TestLucene90DocValuesFormat.java
// (Apache Lucene 10.5.0), built only with the gocene_monsters tag as Lucene
// runs it only when nightly tests are enabled. The blockers are listed in
// lucene90_doc_values_format_test.go; in addition
// doTestSortedNumericBlocksOfVariousBitsPerValue and assertDVAdvance call
// org.apache.lucene.tests.util.TestUtil.checkReader(IndexReader) (not ported).

package codecs_test

import (
	"bytes"
	"slices"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

const lucene90DVCheckReaderBlocker = "requires org.apache.lucene.tests.util.TestUtil.checkReader(IndexReader) (not ported)"

func TestLucene90DocValuesFormat_testSortedSetVariableLengthManyVsStoredFields(t *testing.T) {
	lucene90DVTestSortedSetVariableLengthManyVsStoredFields(t)
}

func lucene90DVTestSortedSetVariableLengthManyVsStoredFields(t *testing.T) {
	// doTestSortedSetVsStoredFields(TestUtil.nextInt(random(), 1024, 2049), 1, 500, 16, 100)
	t.Fatal(lucene90DVBaseBlocker + ": doTestSortedSetVsStoredFields")
}

func TestLucene90DocValuesFormat_testSortedVariableLengthManyVsStoredFields(t *testing.T) {
	lucene90DVTestSortedVariableLengthManyVsStoredFields(t)
}

func lucene90DVTestSortedVariableLengthManyVsStoredFields(t *testing.T) {
	// doTestSortedVsStoredFields(TestUtil.nextInt(random(), 1024, 2049), 1d, 1, 500)
	t.Fatal(lucene90DVBaseBlocker + ": doTestSortedVsStoredFields")
}

// doTestTermsEnumRandom sets an anonymous
// org.apache.lucene.tests.codecs.asserting.AssertingCodec with
// TestUtil.getPostingsFormatWithOrds(random()), compares against
// SortedSetDocValues.termsEnum() and calls the base-class
// doTestSortedSetEnumAdvanceIndependently.
const lucene90DVTermsEnumRandomBlocker = "requires org.apache.lucene.tests.codecs.asserting.AssertingCodec, " +
	"TestUtil.getPostingsFormatWithOrds, SortedSetDocValues.termsEnum() and " +
	"BaseDocValuesFormatTestCase.doTestSortedSetEnumAdvanceIndependently (not ported)"

func TestLucene90DocValuesFormat_testTermsEnumFixedWidth(t *testing.T) {
	lucene90DVTestTermsEnumFixedWidth(t)
}

func lucene90DVTestTermsEnumFixedWidth(t *testing.T) { t.Fatal(lucene90DVTermsEnumRandomBlocker) }

func TestLucene90DocValuesFormat_testTermsEnumVariableWidth(t *testing.T) {
	lucene90DVTestTermsEnumVariableWidth(t)
}

func lucene90DVTestTermsEnumVariableWidth(t *testing.T) { t.Fatal(lucene90DVTermsEnumRandomBlocker) }

func TestLucene90DocValuesFormat_testTermsEnumRandomMany(t *testing.T) {
	lucene90DVTestTermsEnumRandomMany(t)
}

func lucene90DVTestTermsEnumRandomMany(t *testing.T) { t.Fatal(lucene90DVTermsEnumRandomBlocker) }

func TestLucene90DocValuesFormat_testTermsEnumLongSharedPrefixes(t *testing.T) {
	lucene90DVTestTermsEnumLongSharedPrefixes(t)
}

func lucene90DVTestTermsEnumLongSharedPrefixes(t *testing.T) {
	t.Fatal(lucene90DVTermsEnumRandomBlocker)
}

func TestLucene90DocValuesFormat_testSortedSetAroundBlockSize(t *testing.T) {
	lucene90DVTestSortedSetAroundBlockSize(t)
}

func lucene90DVTestSortedSetAroundBlockSize(t *testing.T) {
	r := luceneRandom(t)
	frontier := 1 << codecs.Lucene90DocValuesDirectMonotonicBlockShift
	for maxDoc := frontier - 1; maxDoc <= frontier+1; maxDoc++ {
		dir := luceneNewDirectory()
		iwc := luceneNewIndexWriterConfig()
		iwc.SetMergePolicy(luceneNewLogMergePolicy(r))
		w, err := index.NewIndexWriter(dir, iwc)
		if err != nil {
			t.Fatal(err)
		}
		out := store.NewByteBuffersDataOutput()
		doc := document.NewDocument()
		field1, err := document.NewSortedSetDocValuesField("sset", [][]byte{{}})
		mustAdd(t, doc, field1, err)
		field2, err := document.NewSortedSetDocValuesField("sset", [][]byte{{}})
		mustAdd(t, doc, field2, err)
		for i := 0; i < maxDoc; i++ {
			s1 := []byte(luceneRandomSimpleString(r, 0, 2))
			s2 := []byte(luceneRandomSimpleString(r, 0, 2))
			field1.SetBytesValue(s1)
			field2.SetBytesValue(s2)
			if _, err := w.AddDocument(doc); err != nil {
				t.Fatal(err)
			}
			set := [][]byte{s1}
			if !bytes.Equal(s1, s2) {
				set = append(set, s2)
			}
			slices.SortFunc(set, bytes.Compare) // new TreeSet<>(Arrays.asList(s1, s2))
			if err := out.WriteVInt(int32(len(set))); err != nil {
				t.Fatal(err)
			}
			for _, ref := range set {
				if err := out.WriteVInt(int32(len(ref))); err != nil {
					t.Fatal(err)
				}
				if err := out.WriteBytes(ref, 0, len(ref)); err != nil {
					t.Fatal(err)
				}
			}
		}

		if err := w.ForceMerge(1); err != nil {
			t.Fatal(err)
		}
		rd, err := index.OpenDirectoryReaderFromWriter(w)
		if err != nil {
			t.Fatal(err)
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		sr := lucene90DVOnlyLeafReader(t, rd)
		if sr.MaxDoc() != maxDoc {
			t.Fatalf("maxDoc = %d, want %d", sr.MaxDoc(), maxDoc)
		}
		values, err := sr.GetSortedSetDocValues("sset")
		if err != nil {
			t.Fatal(err)
		}
		if values == nil {
			t.Fatal("assertNotNull(values)")
		}
		in := out.ToDataInput()
		b := util.NewBytesRefBuilder()
		for i := 0; i < maxDoc; i++ {
			mustDocID(t, i, values.NextDoc)
			numValues, err := store.ReadVInt(in)
			if err != nil {
				t.Fatal(err)
			}
			if int(numValues) != values.DocValueCount() {
				t.Fatalf("docValueCount() = %d, want %d", values.DocValueCount(), numValues)
			}

			for j := 0; j < int(numValues); j++ {
				length, err := store.ReadVInt(in)
				if err != nil {
					t.Fatal(err)
				}
				b.SetLength(int(length))
				b.Grow(b.Length())
				if err := in.ReadBytes(b.Bytes(), 0, b.Length()); err != nil {
					t.Fatal(err)
				}
				ord, err := values.NextOrd()
				if err != nil {
					t.Fatal(err)
				}
				term, err := values.LookupOrd(ord)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(b.Get().ValidBytes(), term) {
					t.Fatalf("lookupOrd = %q, want %q", term, b.Get().ValidBytes())
				}
			}
		}
		if err := rd.Close(); err != nil {
			t.Fatal(err)
		}
		if err := dir.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLucene90DocValuesFormat_testSortedNumericAroundBlockSize(t *testing.T) {
	lucene90DVTestSortedNumericAroundBlockSize(t)
}

func lucene90DVTestSortedNumericAroundBlockSize(t *testing.T) {
	r := luceneRandom(t)
	frontier := 1 << codecs.Lucene90DocValuesDirectMonotonicBlockShift
	for maxDoc := frontier - 1; maxDoc <= frontier+1; maxDoc++ {
		dir := luceneNewDirectory()
		iwc := luceneNewIndexWriterConfig()
		iwc.SetMergePolicy(luceneNewLogMergePolicy(r))
		w, err := index.NewIndexWriter(dir, iwc)
		if err != nil {
			t.Fatal(err)
		}
		buffer := store.NewByteBuffersDataOutput()

		doc := document.NewDocument()
		field1, err := document.NewSortedNumericDocValuesField("snum", []int64{0})
		mustAdd(t, doc, field1, err)
		field2, err := document.NewSortedNumericDocValuesField("snum", []int64{0})
		mustAdd(t, doc, field2, err)
		for i := 0; i < maxDoc; i++ {
			s1 := int64(r.Intn(100))
			s2 := int64(r.Intn(100))
			field1.SetLongValue(s1)
			field2.SetLongValue(s2)
			if _, err := w.AddDocument(doc); err != nil {
				t.Fatal(err)
			}
			if err := buffer.WriteVLong(min(s1, s2)); err != nil {
				t.Fatal(err)
			}
			if err := buffer.WriteVLong(max(s1, s2)); err != nil {
				t.Fatal(err)
			}
		}

		if err := w.ForceMerge(1); err != nil {
			t.Fatal(err)
		}
		rd, err := index.OpenDirectoryReaderFromWriter(w)
		if err != nil {
			t.Fatal(err)
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		sr := lucene90DVOnlyLeafReader(t, rd)
		if sr.MaxDoc() != maxDoc {
			t.Fatalf("maxDoc = %d, want %d", sr.MaxDoc(), maxDoc)
		}
		values, err := sr.GetSortedNumericDocValues("snum")
		if err != nil {
			t.Fatal(err)
		}
		if values == nil {
			t.Fatal("assertNotNull(values)")
		}
		dataInput := buffer.ToDataInput()
		for i := 0; i < maxDoc; i++ {
			mustDocID(t, i, values.NextDoc)
			count, err := values.DocValueCount()
			if err != nil {
				t.Fatal(err)
			}
			if count != 2 {
				t.Fatalf("docValueCount() = %d, want 2", count)
			}
			for k := 0; k < 2; k++ {
				want, err := dataInput.ReadVLong()
				if err != nil {
					t.Fatal(err)
				}
				got, err := values.NextValue()
				if err != nil {
					t.Fatal(err)
				}
				if got != want {
					t.Fatalf("nextValue() = %d, want %d", got, want)
				}
			}
		}
		if err := rd.Close(); err != nil {
			t.Fatal(err)
		}
		if err := dir.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLucene90DocValuesFormat_testSortedNumericBlocksOfVariousBitsPerValue(t *testing.T) {
	lucene90DVTestSortedNumericBlocksOfVariousBitsPerValue(t)
}

func lucene90DVTestSortedNumericBlocksOfVariousBitsPerValue(t *testing.T) {
	// doTestSortedNumericBlocksOfVariousBitsPerValue(() -> TestUtil.nextInt(random(), 1, 3))
	t.Fatal(lucene90DVCheckReaderBlocker)
}

func TestLucene90DocValuesFormat_testSparseSortedNumericBlocksOfVariousBitsPerValue(t *testing.T) {
	lucene90DVTestSparseSortedNumericBlocksOfVariousBitsPerValue(t)
}

func lucene90DVTestSparseSortedNumericBlocksOfVariousBitsPerValue(t *testing.T) {
	// doTestSortedNumericBlocksOfVariousBitsPerValue(() -> TestUtil.nextInt(random(), 0, 2))
	t.Fatal(lucene90DVCheckReaderBlocker)
}

func TestLucene90DocValuesFormat_testNumericBlocksOfVariousBitsPerValue(t *testing.T) {
	lucene90DVTestNumericBlocksOfVariousBitsPerValue(t)
}

func lucene90DVTestNumericBlocksOfVariousBitsPerValue(t *testing.T) {
	// doTestSparseNumericBlocksOfVariousBitsPerValue(1): assertDVIterate + assertDVAdvance
	t.Fatal(lucene90DVBaseBlocker + ": assertDVIterate; " + lucene90DVCheckReaderBlocker)
}

func TestLucene90DocValuesFormat_testSparseNumericBlocksOfVariousBitsPerValue(t *testing.T) {
	lucene90DVTestSparseNumericBlocksOfVariousBitsPerValue(t)
}

func lucene90DVTestSparseNumericBlocksOfVariousBitsPerValue(t *testing.T) {
	// doTestSparseNumericBlocksOfVariousBitsPerValue(random().nextDouble())
	t.Fatal(lucene90DVBaseBlocker + ": assertDVIterate; " + lucene90DVCheckReaderBlocker)
}

// The LUCENE-8585 jump-tables enables O(1) skipping of IndexedDISI blocks, DENSE block lookup
// and numeric multi blocks. This test focuses on testing these jumps.
func TestLucene90DocValuesFormat_testNumericFieldJumpTables(t *testing.T) {
	lucene90DVTestNumericFieldJumpTables(t)
}

func lucene90DVTestNumericFieldJumpTables(t *testing.T) {
	t.Fatal(lucene90DVBaseBlocker + ": assertDVIterate; " + lucene90DVCheckReaderBlocker)
}
