// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs_test

// Port of
// lucene/core/src/test/org/apache/lucene/codecs/lucene90/TestLucene90DocValuesFormat.java
// (Apache Lucene 10.5.0). The @Nightly methods live in
// lucene90_doc_values_format_monster_test.go (gocene_monsters tag).
//
// Every test method body is a function taking the running *testing.T so that
// TestLucene90DocValuesFormatMergeInstance, which inherits all of them, runs
// the same bodies (its only override, shouldTestMergeInstance(), is read by
// the base-class helpers alone).
//
// Blockers:
//   - the class extends org.apache.lucene.tests.index.BaseCompressingDocValuesFormatTestCase
//     (not ported): the inherited methods, and the own methods that call its
//     helpers doTestSortedSetVsStoredFields / doTestSortedVsStoredFields /
//     assertDVIterate / doTestSortedSetEnumAdvanceIndependently, fail naming it;
//   - spi.NumericDocValues lacks NumericDocValues.longValues (the bulk fetch);
//   - spi.SortedDocValues / spi.SortedSetDocValues lack termsEnum() and the
//     Lucene90DocValuesProducer overrides of termsEnum() and lookupTerm(BytesRef);
//   - spi.DocValuesSkipper lacks advance(int), minDocID(int) and maxDocID(int).
// BaseIndexFileFormatTestCase.setUp installs getCodec() (TestUtil.getDefaultCodec())
// as the default codec; the ports rely on Gocene's default codec.

import (
	"fmt"
	"math/rand"
	"slices"
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	testindex "github.com/FlavioCFOliveira/Gocene/tests/index"
)

const (
	lucene90DVBaseBlocker      = "requires org.apache.lucene.tests.index.BaseCompressingDocValuesFormatTestCase (not ported)"
	lucene90DVTermsEnumBlocker = "requires SortedDocValues.termsEnum()/SortedSetDocValues.termsEnum() " +
		"(missing from spi.SortedDocValues and spi.SortedSetDocValues) and the Lucene90DocValuesProducer " +
		"termsEnum()/lookupTerm(BytesRef) overrides (not ported)"
)

func TestLucene90DocValuesFormat_BaseCompressingDocValuesFormatTestCase(t *testing.T) {
	t.Fatal(lucene90DVBaseBlocker)
}

// TODO: these big methods can easily blow up some of the other ram-hungry codecs...
// for now just keep them here, as we want to test this for this format.

func TestLucene90DocValuesFormat_testSortedSetVariableLengthBigVsStoredFields(t *testing.T) {
	lucene90DVTestSortedSetVariableLengthBigVsStoredFields(t)
}

func lucene90DVTestSortedSetVariableLengthBigVsStoredFields(t *testing.T) {
	// doTestSortedSetVsStoredFields(numDocs, 1, 32766, 16, 100)
	t.Fatal(lucene90DVBaseBlocker + ": doTestSortedSetVsStoredFields")
}

func TestLucene90DocValuesFormat_testSortedVariableLengthBigVsStoredFields(t *testing.T) {
	lucene90DVTestSortedVariableLengthBigVsStoredFields(t)
}

func lucene90DVTestSortedVariableLengthBigVsStoredFields(t *testing.T) {
	// doTestSortedVsStoredFields(atLeast(100), 1d, 1, 32766)
	t.Fatal(lucene90DVBaseBlocker + ": doTestSortedVsStoredFields")
}

func TestLucene90DocValuesFormat_testSparseDocValuesVsStoredFields(t *testing.T) {
	lucene90DVTestSparseDocValuesVsStoredFields(t)
}

func lucene90DVTestSparseDocValuesVsStoredFields(t *testing.T) {
	r := luceneRandom(t)
	numIterations := luceneAtLeast(r, 1)
	for i := 0; i < numIterations; i++ {
		lucene90DVDoTestSparseDocValuesVsStoredFields(t, r)
	}
}

func TestLucene90DocValuesFormat_testDenseNumericLongValuesBulkFetch(t *testing.T) {
	lucene90DVTestDenseNumericLongValuesBulkFetch(t)
}

func lucene90DVTestDenseNumericLongValuesBulkFetch(t *testing.T) {
	// doTestDenseNumericLongValuesBulkFetch(8..64) reads through
	// NumericDocValues.longValues(int, int[], long[], long) and
	// longValues(int, int[], int, long[], int, long).
	t.Fatal("requires NumericDocValues.longValues bulk fetch (missing from spi.NumericDocValues)")
}

func mustAdd(t *testing.T, doc *document.Document, f any, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	doc.Add(f.(document.IndexableField))
}

func lucene90DVDoTestSparseDocValuesVsStoredFields(t *testing.T, r *rand.Rand) {
	t.Helper()
	values := make([]int64, luceneNextInt(r, 1, 500))
	for i := range values {
		values[i] = int64(r.Uint64())
	}

	// newFSDirectory(createTempDir()): FSDirectory.open(path) wrapped in a MockDirectoryWrapper.
	fsDir, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := store.NewMockDirectoryWrapper(fsDir)
	conf := luceneNewIndexWriterConfig()
	conf.SetMergeScheduler(index.NewSerialMergeScheduler())
	writer, err := testindex.NewRandomIndexWriterWithConfig(r, dir, conf)
	if err != nil {
		t.Fatalf("new RandomIndexWriter: %v", err)
	}

	// sparse compression is only enabled if less than 1% of docs have a value
	const avgGap = 100

	numDocs := luceneAtLeast(r, 200)
	for i := r.Intn(avgGap * 2); i >= 0; i-- {
		if _, err := writer.AddDocument(document.NewDocument()); err != nil {
			t.Fatalf("addDocument: %v", err)
		}
	}
	maxNumValuesPerDoc := 1
	if r.Intn(2) != 0 {
		maxNumValuesPerDoc = luceneNextInt(r, 2, 5)
	}
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()

		// single-valued
		docValue := values[r.Intn(len(values))]
		s := strconv.FormatInt(docValue, 10)
		f1, err := document.NewNumericDocValuesField("numeric", docValue)
		mustAdd(t, doc, f1, err)
		f2, err := document.NewSortedDocValuesField("sorted", []byte(s))
		mustAdd(t, doc, f2, err)
		f3, err := document.NewBinaryDocValuesField("binary", []byte(s))
		mustAdd(t, doc, f3, err)
		f4, err := document.NewStoredFieldFromInt64("value", docValue)
		mustAdd(t, doc, f4, err)

		// multi-valued
		numValues := luceneNextInt(r, 1, maxNumValuesPerDoc)
		for j := 0; j < numValues; j++ {
			docValue = values[r.Intn(len(values))]
			s = strconv.FormatInt(docValue, 10)
			g1, err := document.NewSortedNumericDocValuesField("sorted_numeric", []int64{docValue})
			mustAdd(t, doc, g1, err)
			g2, err := document.NewSortedSetDocValuesField("sorted_set", [][]byte{[]byte(s)})
			mustAdd(t, doc, g2, err)
			g3, err := document.NewStoredFieldFromInt64("values", docValue)
			mustAdd(t, doc, g3, err)
		}

		if _, err := writer.AddDocument(doc); err != nil {
			t.Fatalf("addDocument: %v", err)
		}

		// add a gap
		for j := luceneNextInt(r, 0, avgGap*2); j >= 0; j-- {
			if _, err := writer.AddDocument(document.NewDocument()); err != nil {
				t.Fatalf("addDocument: %v", err)
			}
		}
	}

	if r.Intn(2) == 0 {
		if err := writer.ForceMerge(1); err != nil {
			t.Fatalf("forceMerge: %v", err)
		}
	}

	indexReader, err := writer.GetReader()
	if err != nil {
		t.Fatalf("getReader: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	leaves, err := indexReader.Leaves()
	if err != nil {
		t.Fatal(err)
	}
	for _, context := range leaves {
		reader := context.LeafReader()
		numeric, err := index.GetNumeric(reader, "numeric")
		if err != nil {
			t.Fatal(err)
		}
		sorted, err := index.GetSorted(reader, "sorted")
		if err != nil {
			t.Fatal(err)
		}
		binary, err := index.GetBinary(reader, "binary")
		if err != nil {
			t.Fatal(err)
		}
		sortedNumeric, err := index.GetSortedNumeric(reader, "sorted_numeric")
		if err != nil {
			t.Fatal(err)
		}
		sortedSet, err := index.GetSortedSet(reader, "sorted_set")
		if err != nil {
			t.Fatal(err)
		}

		storedFields, err := reader.StoredFields()
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < reader.MaxDoc(); i++ {
			doc := luceneStoredDocument(t, storedFields, i)
			valueField := doc.Get("value")

			if valueField == nil {
				if !(numeric.DocID() < i) {
					t.Fatalf("%d vs %d", numeric.DocID(), i)
				}
			} else {
				value := luceneNumericValueInt64(t, valueField.NumericValue())
				mustDocID(t, i, numeric.NextDoc)
				mustDocID(t, i, binary.NextDoc)
				mustDocID(t, i, sorted.NextDoc)
				got, err := numeric.LongValue()
				if err != nil {
					t.Fatal(err)
				}
				if got != value {
					t.Fatalf("numeric.longValue() = %d, want %d", got, value)
				}
				ord, err := sorted.OrdValue()
				if err != nil {
					t.Fatal(err)
				}
				if !(ord >= 0) {
					t.Fatalf("sorted.ordValue() = %d", ord)
				}
				term, err := sorted.LookupOrd(ord)
				if err != nil {
					t.Fatal(err)
				}
				if string(term) != strconv.FormatInt(value, 10) {
					t.Fatalf("sorted.lookupOrd = %q, want %d", term, value)
				}
				bv, err := binary.BinaryValue()
				if err != nil {
					t.Fatal(err)
				}
				if string(bv) != strconv.FormatInt(value, 10) {
					t.Fatalf("binary.binaryValue = %q, want %d", bv, value)
				}
			}

			valuesFields := doc.GetFieldsByName("values")
			if len(valuesFields) == 0 {
				if !(sortedNumeric.DocID() < i) {
					t.Fatalf("%d vs %d", sortedNumeric.DocID(), i)
				}
			} else {
				valueSet := make(map[int64]bool)
				for _, sf := range valuesFields {
					valueSet[luceneNumericValueInt64(t, sf.NumericValue())] = true
				}

				mustDocID(t, i, sortedNumeric.NextDoc)
				count, err := sortedNumeric.DocValueCount()
				if err != nil {
					t.Fatal(err)
				}
				if count != len(valuesFields) {
					t.Fatalf("sortedNumeric.docValueCount() = %d, want %d", count, len(valuesFields))
				}
				for j := 0; j < count; j++ {
					v, err := sortedNumeric.NextValue()
					if err != nil {
						t.Fatal(err)
					}
					if !valueSet[v] {
						t.Fatalf("sortedNumeric value %d not in %v", v, valueSet)
					}
				}
				mustDocID(t, i, sortedSet.NextDoc)

				if sortedSet.DocValueCount() != len(valueSet) {
					t.Fatalf("sortedSet.docValueCount() = %d, want %d", sortedSet.DocValueCount(), len(valueSet))
				}
				for j := 0; j < sortedSet.DocValueCount(); j++ {
					ord, err := sortedSet.NextOrd()
					if err != nil {
						t.Fatal(err)
					}
					term, err := sortedSet.LookupOrd(ord)
					if err != nil {
						t.Fatal(err)
					}
					v, err := strconv.ParseInt(string(term), 10, 64)
					if err != nil {
						t.Fatal(err)
					}
					if !valueSet[v] {
						t.Fatalf("sortedSet value %d not in %v", v, valueSet)
					}
				}
			}
		}
	}

	if err := indexReader.Close(); err != nil {
		t.Fatal(err)
	}
	if err := dir.Close(); err != nil {
		t.Fatal(err)
	}
}

func mustDocID(t *testing.T, want int, next func() (int, error)) {
	t.Helper()
	got, err := next()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("nextDoc() = %d, want %d", got, want)
	}
}

func TestLucene90DocValuesFormat_testReseekAfterSkipDecompression(t *testing.T) {
	lucene90DVTestReseekAfterSkipDecompression(t)
}

func lucene90DVTestReseekAfterSkipDecompression(t *testing.T) {
	// ssdvMulti.lookupTerm(BytesRef) must reach the Lucene90DocValuesProducer
	// override, whose re-seek after skip-decompression the test exercises.
	t.Fatal(lucene90DVTermsEnumBlocker)
}

func TestLucene90DocValuesFormat_testLargeTermsCompression(t *testing.T) {
	lucene90DVTestLargeTermsCompression(t)
}

func lucene90DVTestLargeTermsCompression(t *testing.T) {
	r := luceneRandom(t)
	const cardinality = 64
	valuesSet := make(map[string]bool)
	for i := 0; i < cardinality; i++ {
		length := luceneNextInt(r, 512, 1024)
		valuesSet[luceneRandomSimpleString(r, 0, length)] = true
	}
	valuesCount := len(valuesSet)
	values := make([]string, 0, valuesCount)
	for v := range valuesSet {
		values = append(values, v)
	}
	slices.Sort(values) // new ArrayList<>(HashSet): the order only picks which value each doc gets

	directory := luceneNewDirectory()
	defer func() {
		if err := directory.Close(); err != nil {
			t.Error(err)
		}
	}()
	var analyzer analysis.Analyzer = analysis.NewStandardAnalyzer()
	config := index.NewIndexWriterConfigWithAnalyzer(analyzer)
	lucene90DVSetCodec(t, config)
	config.SetUseCompoundFile(false)
	writer, err := index.NewIndexWriter(directory, config)
	if err != nil {
		t.Fatalf("new IndexWriter: %v", err)
	}
	for i := 0; i < 256; i++ {
		doc := document.NewDocument()
		f1, err := document.NewStringField("id", fmt.Sprintf("Doc%d", i), false)
		mustAdd(t, doc, f1, err)
		f2, err := document.NewSortedDocValuesField("sdv", []byte(values[i%valuesCount]))
		mustAdd(t, doc, f2, err)
		if _, err := writer.AddDocument(doc); err != nil {
			t.Fatalf("addDocument: %v", err)
		}
	}
	if _, err := writer.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatal(err)
	}
	ireader, err := index.OpenDirectoryReaderFromWriter(writer)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	reader := lucene90DVOnlyLeafReader(t, ireader)
	// Check values count.
	ssdvMulti, err := reader.GetSortedDocValues("sdv")
	if err != nil {
		t.Fatal(err)
	}
	if ssdvMulti.GetValueCount() != valuesCount {
		t.Fatalf("getValueCount() = %d, want %d", ssdvMulti.GetValueCount(), valuesCount)
	}
	if err := ireader.Close(); err != nil {
		t.Fatal(err)
	}
}

// lucene90DVSetCodec renders config.setCodec(getCodec()) with
// getCodec() = TestUtil.getDefaultCodec().
func lucene90DVSetCodec(t *testing.T, config *index.IndexWriterConfig) {
	t.Helper()
	codec, err := codecs.GetDefault()
	if err != nil {
		t.Fatalf("default codec: %v", err)
	}
	config.SetCodec(codec)
}

// lucene90DVOnlyLeafReader renders LuceneTestCase.getOnlyLeafReader(IndexReader).
func lucene90DVOnlyLeafReader(t *testing.T, r *index.DirectoryReader) index.LeafReader {
	t.Helper()
	leaves, err := r.Leaves()
	if err != nil {
		t.Fatal(err)
	}
	if len(leaves) != 1 {
		t.Fatalf("reader has %d segments instead of exactly one", len(leaves))
	}
	return leaves[0].LeafReader()
}

func TestLucene90DocValuesFormat_testSortedTermsDictLookupOrd(t *testing.T) {
	lucene90DVTestSortedTermsDictLookupOrd(t)
}

func lucene90DVTestSortedTermsDictLookupOrd(t *testing.T) {
	t.Fatal(lucene90DVTermsEnumBlocker)
}

func TestLucene90DocValuesFormat_testSortedSetTermsDictLookupOrd(t *testing.T) {
	lucene90DVTestSortedSetTermsDictLookupOrd(t)
}

func lucene90DVTestSortedSetTermsDictLookupOrd(t *testing.T) {
	t.Fatal(lucene90DVTermsEnumBlocker)
}

// Exercise the logic that leverages the first term of a block as a dictionary for suffixes of
// other terms
func TestLucene90DocValuesFormat_testTermsEnumDictionary(t *testing.T) {
	lucene90DVTestTermsEnumDictionary(t)
}

func lucene90DVTestTermsEnumDictionary(t *testing.T) {
	t.Fatal(lucene90DVTermsEnumBlocker)
}

// Testing termsEnum seekCeil edge case, where inconsistent internal state led to
// IndexOutOfBoundsException
// see https://github.com/apache/lucene/pull/12555 for details
func TestLucene90DocValuesFormat_testTermsEnumConsistency(t *testing.T) {
	lucene90DVTestTermsEnumConsistency(t)
}

func lucene90DVTestTermsEnumConsistency(t *testing.T) {
	t.Fatal(lucene90DVTermsEnumBlocker)
}

func TestLucene90DocValuesFormat_testSkipIndexStoredSeparately(t *testing.T) {
	lucene90DVTestSkipIndexStoredSeparately(t)
}

func lucene90DVTestSkipIndexStoredSeparately(t *testing.T) {
	// The test reads the skipper through DocValuesSkipper.docCount(), advance(int),
	// minDocID(int), maxDocID(int), minValue() and maxValue().
	t.Fatal("requires DocValuesSkipper.advance(int), minDocID(int) and maxDocID(int) " +
		"(missing from spi.DocValuesSkipper)")
}
