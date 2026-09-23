// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestTermVectorsWriter.java
// (Apache Lucene 10.5.0): tests for writing term vectors.

package index_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
	testutil "github.com/FlavioCFOliveira/Gocene/tests/util"
)

// vectorsFieldType renders new FieldType(base) with term vectors, positions
// and offsets stored.
func vectorsFieldType(base *document.FieldType) *document.FieldType {
	ft := document.NewFieldTypeFrom(base)
	ft.SetStoreTermVectors(true)
	ft.SetStoreTermVectorPositions(true)
	ft.SetStoreTermVectorOffsets(true)
	return ft
}

// fieldVectorTermsEnum renders r.termVectors().get(0).terms("field").iterator().
func fieldVectorTermsEnum(t testing.TB, r index.IndexReaderInterface) index.TermsEnum {
	t.Helper()
	termVectors, err := r.TermVectors()
	if err != nil {
		t.Fatalf("termVectors: %v", err)
	}
	fields, err := termVectors.Get(0)
	if err != nil {
		t.Fatalf("termVectors().get(0): %v", err)
	}
	if fields == nil {
		t.Fatal("termVectors().get(0) is null")
	}
	vector, err := fields.Terms("field")
	if err != nil {
		t.Fatalf("terms(field): %v", err)
	}
	if vector == nil {
		t.Fatal("assertNotNull(vector)")
	}
	termsEnum, err := vector.Iterator()
	if err != nil {
		t.Fatalf("iterator: %v", err)
	}
	return termsEnum
}

func mustNextTerm(t testing.TB, termsEnum index.TermsEnum) *index.Term {
	t.Helper()
	term, err := termsEnum.Next()
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if term == nil {
		t.Fatal("assertNotNull(termsEnum.next())")
	}
	return term
}

func mustPostingsAll(t testing.TB, termsEnum index.TermsEnum) index.PostingsEnum {
	t.Helper()
	dpEnum, err := termsEnum.Postings(spi.PostingsFlagAll)
	if err != nil {
		t.Fatalf("postings: %v", err)
	}
	return dpEnum
}

func assertTotalTermFreq(t testing.TB, termsEnum index.TermsEnum, expected int64) {
	t.Helper()
	if ttf, err := termsEnum.TotalTermFreq(); err != nil || ttf != expected {
		t.Fatalf("totalTermFreq: expected %d, got %d (%v)", expected, ttf, err)
	}
}

func assertHasDoc(t testing.TB, dpEnum index.PostingsEnum) {
	t.Helper()
	if doc, err := dpEnum.NextDoc(); err != nil || doc == spi.NO_MORE_DOCS {
		t.Fatalf("assertTrue(dpEnum.nextDoc() != NO_MORE_DOCS): %d (%v)", doc, err)
	}
}

func assertNoMoreDocs(t testing.TB, dpEnum index.PostingsEnum) {
	t.Helper()
	if doc, err := dpEnum.NextDoc(); err != nil || doc != spi.NO_MORE_DOCS {
		t.Fatalf("nextDoc: expected NO_MORE_DOCS, got %d (%v)", doc, err)
	}
}

// assertNextOffsets renders dpEnum.nextPosition() followed by the
// startOffset/endOffset assertions.
func assertNextOffsets(t testing.TB, dpEnum index.PostingsEnum, start, end int) {
	t.Helper()
	if _, err := dpEnum.NextPosition(); err != nil {
		t.Fatalf("nextPosition: %v", err)
	}
	if got, err := dpEnum.StartOffset(); err != nil || got != start {
		t.Fatalf("startOffset: expected %d, got %d (%v)", start, got, err)
	}
	if got, err := dpEnum.EndOffset(); err != nil || got != end {
		t.Fatalf("endOffset: expected %d, got %d (%v)", end, got, err)
	}
}

// LUCENE-1442
func TestTermVectorsWriterDoubleOffsetCounting(t *testing.T) {
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	doc := document.NewDocument()
	customType := vectorsFieldType(document.StringFieldTypeNotStored)
	f := newField(t, "field", "abcd", customType)
	doc.Add(f)
	doc.Add(f)
	f2 := newField(t, "field", "", customType)
	doc.Add(f2)
	doc.Add(f)
	mustAddDocument(t, w, doc)
	mustClose(t, w)

	r := mustOpenDirectoryReader(t, dir)
	termsEnum := fieldVectorTermsEnum(t, r)
	term := mustNextTerm(t, termsEnum)
	if term.Bytes.String() != "" {
		t.Fatalf("term: expected \"\", got %q", term.Bytes.String())
	}

	// Token "" occurred once
	assertTotalTermFreq(t, termsEnum, 1)

	dpEnum := mustPostingsAll(t, termsEnum)
	assertHasDoc(t, dpEnum)
	assertNextOffsets(t, dpEnum, 8, 8)
	assertNoMoreDocs(t, dpEnum)

	// Token "abcd" occurred three times
	if term := mustNextTerm(t, termsEnum); term.Bytes.String() != "abcd" {
		t.Fatalf("term: expected abcd, got %q", term.Bytes.String())
	}
	dpEnum = mustPostingsAll(t, termsEnum)
	assertTotalTermFreq(t, termsEnum, 3)

	assertHasDoc(t, dpEnum)
	assertNextOffsets(t, dpEnum, 0, 4)
	assertNextOffsets(t, dpEnum, 4, 8)
	assertNextOffsets(t, dpEnum, 8, 12)

	assertNoMoreDocs(t, dpEnum)
	if term, err := termsEnum.Next(); err != nil || term != nil {
		t.Fatalf("assertNull(termsEnum.next()): %v (%v)", term, err)
	}
	mustClose(t, r, dir)
}

// twoInstanceOffsets renders the shared body of the LUCENE-1442/1448 tests
// that add the same field instance twice and check the two offset pairs.
func twoInstanceOffsets(t *testing.T, analyzer analysis.Analyzer, value string, first, second [2]int) {
	t.Helper()
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(analyzer))
	doc := document.NewDocument()
	customType := vectorsFieldType(document.TextFieldTypeNotStored)
	f := newField(t, "field", value, customType)
	doc.Add(f)
	doc.Add(f)
	mustAddDocument(t, w, doc)
	mustClose(t, w)

	r := mustOpenDirectoryReader(t, dir)
	termsEnum := fieldVectorTermsEnum(t, r)
	mustNextTerm(t, termsEnum)
	dpEnum := mustPostingsAll(t, termsEnum)
	assertTotalTermFreq(t, termsEnum, 2)

	assertHasDoc(t, dpEnum)
	assertNextOffsets(t, dpEnum, first[0], first[1])
	assertNextOffsets(t, dpEnum, second[0], second[1])
	assertNoMoreDocs(t, dpEnum)

	mustClose(t, r, dir)
}

// LUCENE-1442
func TestTermVectorsWriterDoubleOffsetCounting2(t *testing.T) {
	twoInstanceOffsets(t, newMockAnalyzer(), "abcd", [2]int{0, 4}, [2]int{5, 9})
}

// LUCENE-1448
func TestTermVectorsWriterEndOffsetPositionCharAnalyzer(t *testing.T) {
	twoInstanceOffsets(t, newMockAnalyzer(), "abcd   ", [2]int{0, 4}, [2]int{8, 12})
}

// LUCENE-1448
func TestTermVectorsWriterEndOffsetPositionWithCachingTokenFilter(t *testing.T) {
	dir := newDirectory()
	analyzer := newMockAnalyzer()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(analyzer))
	doc := document.NewDocument()
	source, err := analyzer.TokenStream("field", strings.NewReader("abcd   "))
	if err != nil {
		t.Fatalf("tokenStream: %v", err)
	}
	stream := analysis.NewCachingTokenFilter(source)
	customType := vectorsFieldType(document.TextFieldTypeNotStored)
	f, err := document.NewField("field", stream, customType)
	if err != nil {
		t.Fatalf("Field: %v", err)
	}
	doc.Add(f)
	doc.Add(f)
	mustAddDocument(t, w, doc)
	if err := stream.Close(); err != nil {
		t.Fatalf("close stream: %v", err)
	}
	mustClose(t, w)

	r := mustOpenDirectoryReader(t, dir)
	termsEnum := fieldVectorTermsEnum(t, r)
	mustNextTerm(t, termsEnum)
	dpEnum := mustPostingsAll(t, termsEnum)
	assertTotalTermFreq(t, termsEnum, 2)

	assertHasDoc(t, dpEnum)
	assertNextOffsets(t, dpEnum, 0, 4)
	assertNextOffsets(t, dpEnum, 8, 12)
	assertNoMoreDocs(t, dpEnum)

	mustClose(t, r, dir)
}

// LUCENE-1448
func TestTermVectorsWriterEndOffsetPositionStopFilter(t *testing.T) {
	analyzer := testanalysis.NewMockAnalyzer(testanalysis.SIMPLE, true, testanalysis.DefaultMaxTokenLength, testanalysis.ENGLISH_STOPSET, true)
	twoInstanceOffsets(t, analyzer, "abcd the", [2]int{0, 4}, [2]int{9, 13})
}

// endOffsetPositionTwoFields renders the shared prologue of the three
// LUCENE-1448 standard-analyzer tests that index distinct field instances.
func endOffsetPositionTwoFields(t *testing.T, values ...string) (store.Directory, *index.DirectoryReader, index.TermsEnum) {
	t.Helper()
	dir := newDirectory()
	w := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	doc := document.NewDocument()
	customType := vectorsFieldType(document.TextFieldTypeNotStored)
	for _, v := range values {
		doc.Add(newField(t, "field", v, customType))
	}
	mustAddDocument(t, w, doc)
	mustClose(t, w)

	r := mustOpenDirectoryReader(t, dir)
	return dir, r, fieldVectorTermsEnum(t, r)
}

// LUCENE-1448
func TestTermVectorsWriterEndOffsetPositionStandard(t *testing.T) {
	dir, r, termsEnum := endOffsetPositionTwoFields(t, "abcd the  ", "crunch man")
	mustNextTerm(t, termsEnum)
	dpEnum := mustPostingsAll(t, termsEnum)

	assertHasDoc(t, dpEnum)
	assertNextOffsets(t, dpEnum, 0, 4)

	mustNextTerm(t, termsEnum)
	dpEnum = mustPostingsAll(t, termsEnum)
	assertHasDoc(t, dpEnum)
	assertNextOffsets(t, dpEnum, 11, 17)

	mustNextTerm(t, termsEnum)
	dpEnum = mustPostingsAll(t, termsEnum)
	assertHasDoc(t, dpEnum)
	assertNextOffsets(t, dpEnum, 18, 21)

	mustClose(t, r, dir)
}

// LUCENE-1448
func TestTermVectorsWriterEndOffsetPositionStandardEmptyField(t *testing.T) {
	dir, r, termsEnum := endOffsetPositionTwoFields(t, "", "crunch man")
	mustNextTerm(t, termsEnum)
	dpEnum := mustPostingsAll(t, termsEnum)

	assertTotalTermFreq(t, termsEnum, 1)
	assertHasDoc(t, dpEnum)
	assertNextOffsets(t, dpEnum, 1, 7)

	mustNextTerm(t, termsEnum)
	dpEnum = mustPostingsAll(t, termsEnum)
	assertHasDoc(t, dpEnum)
	assertNextOffsets(t, dpEnum, 8, 11)

	mustClose(t, r, dir)
}

// LUCENE-1448
func TestTermVectorsWriterEndOffsetPositionStandardEmptyField2(t *testing.T) {
	dir, r, termsEnum := endOffsetPositionTwoFields(t, "abcd", "", "crunch")
	mustNextTerm(t, termsEnum)
	dpEnum := mustPostingsAll(t, termsEnum)

	assertTotalTermFreq(t, termsEnum, 1)
	assertHasDoc(t, dpEnum)
	assertNextOffsets(t, dpEnum, 0, 4)

	mustNextTerm(t, termsEnum)
	dpEnum = mustPostingsAll(t, termsEnum)
	assertHasDoc(t, dpEnum)
	assertNextOffsets(t, dpEnum, 6, 12)

	mustClose(t, r, dir)
}

func termVectorCorruptionWriter(t *testing.T, dir store.Directory) *index.IndexWriter {
	t.Helper()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMaxBufferedDocs(2)
	conf.SetRAMBufferSizeMB(index.DisableAutoFlush)
	conf.SetMergeScheduler(index.NewSerialMergeScheduler())
	conf.SetMergePolicy(index.NewLogDocMergePolicy())
	return mustNewIndexWriter(t, dir, conf)
}

// termVectorCorruptionDocs renders the shared indexing body of
// testTermVectorCorruption and testTermVectorCorruption2.
func termVectorCorruptionDocs(t *testing.T, writer *index.IndexWriter) {
	t.Helper()
	doc := document.NewDocument()
	customType := document.NewFieldType()
	customType.SetStored(true)

	storedField := newField(t, "stored", "stored", customType)
	doc.Add(storedField)
	mustAddDocument(t, writer, doc)
	mustAddDocument(t, writer, doc)

	doc = document.NewDocument()
	doc.Add(storedField)
	customType2 := vectorsFieldType(document.StringFieldTypeNotStored)
	doc.Add(newField(t, "termVector", "termVector", customType2))
	mustAddDocument(t, writer, doc)
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	mustClose(t, writer)
}

// LUCENE-1168
func TestTermVectorsWriterTermVectorCorruption(t *testing.T) {
	dir := newDirectory()
	for iter := 0; iter < 2; iter++ {
		termVectorCorruptionDocs(t, termVectorCorruptionWriter(t, dir))

		reader := mustOpenDirectoryReader(t, dir)
		storedFields, err := reader.StoredFields()
		if err != nil {
			t.Fatalf("storedFields: %v", err)
		}
		termVectors, err := reader.TermVectors()
		if err != nil {
			t.Fatalf("termVectors: %v", err)
		}
		for i := 0; i < reader.NumDocs(); i++ {
			storedDocument(t, storedFields, i)
			if _, err := termVectors.Get(i); err != nil {
				t.Fatalf("termVectors.get(%d): %v", i, err)
			}
		}
		mustClose(t, reader)

		writer := termVectorCorruptionWriter(t, dir)

		ramCopy, err := testutil.RamCopyOf(dir)
		if err != nil {
			t.Fatalf("TestUtil.ramCopyOf: %v", err)
		}
		indexDirs := []store.Directory{store.NewMockDirectoryWrapper(ramCopy)}
		if _, err := writer.AddIndexes(indexDirs...); err != nil {
			t.Fatalf("addIndexes: %v", err)
		}
		if err := writer.ForceMerge(1); err != nil {
			t.Fatalf("forceMerge: %v", err)
		}
		mustClose(t, writer)
	}
	mustClose(t, dir)
}

// LUCENE-1168
func TestTermVectorsWriterTermVectorCorruption2(t *testing.T) {
	dir := newDirectory()
	for iter := 0; iter < 2; iter++ {
		termVectorCorruptionDocs(t, termVectorCorruptionWriter(t, dir))

		reader := mustOpenDirectoryReader(t, dir)
		termVectors, err := reader.TermVectors()
		if err != nil {
			t.Fatalf("termVectors: %v", err)
		}
		for doc, wantNull := range []bool{true, true, false} {
			fields, err := termVectors.Get(doc)
			if err != nil {
				t.Fatalf("termVectors.get(%d): %v", doc, err)
			}
			if (fields == nil) != wantNull {
				t.Fatalf("termVectors().get(%d): null=%v, want null=%v", doc, fields == nil, wantNull)
			}
		}
		mustClose(t, reader)
	}
	mustClose(t, dir)
}

// LUCENE-1168
func TestTermVectorsWriterTermVectorCorruption3(t *testing.T) {
	dir := newDirectory()
	writer := termVectorCorruptionWriter(t, dir)

	doc := document.NewDocument()
	customType := document.NewFieldType()
	customType.SetStored(true)

	doc.Add(newField(t, "stored", "stored", customType))
	customType2 := vectorsFieldType(document.StringFieldTypeNotStored)
	doc.Add(newField(t, "termVector", "termVector", customType2))
	for i := 0; i < 10; i++ {
		mustAddDocument(t, writer, doc)
	}
	mustClose(t, writer)

	writer = termVectorCorruptionWriter(t, dir)
	for i := 0; i < 6; i++ {
		mustAddDocument(t, writer, doc)
	}

	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	mustClose(t, writer)

	reader := mustOpenDirectoryReader(t, dir)
	storedFields, err := reader.StoredFields()
	if err != nil {
		t.Fatalf("storedFields: %v", err)
	}
	termVectors, err := reader.TermVectors()
	if err != nil {
		t.Fatalf("termVectors: %v", err)
	}
	for i := 0; i < 10; i++ {
		if _, err := termVectors.Get(i); err != nil {
			t.Fatalf("termVectors.get(%d): %v", i, err)
		}
		storedDocument(t, storedFields, i)
	}
	mustClose(t, reader, dir)
}

// LUCENE-1008
func TestTermVectorsWriterNoTermVectorAfterTermVector(t *testing.T) {
	dir := newDirectory()
	iw := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	doc := document.NewDocument()
	customType2 := vectorsFieldType(document.TextFieldTypeNotStored)
	doc.Add(newField(t, "tvtest", "a b c", customType2))
	mustAddDocument(t, iw, doc)
	doc = document.NewDocument()
	doc.Add(newTextField(t, "tvtest", "x y z", false))
	mustAddDocument(t, iw, doc)
	// Make first segment
	mustCommit(t, iw)

	customType := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	customType.SetStoreTermVectors(true)
	doc = document.NewDocument()
	doc.Add(newField(t, "tvtest", "a b c", customType))
	mustAddDocument(t, iw, doc)
	// Make 2nd segment
	mustCommit(t, iw)

	if err := iw.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	mustClose(t, iw, dir)
}

// LUCENE-1010
func TestTermVectorsWriterNoTermVectorAfterTermVectorMerge(t *testing.T) {
	dir := newDirectory()
	iw := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
	doc := document.NewDocument()
	customType := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	customType.SetStoreTermVectors(true)
	doc.Add(newField(t, "tvtest", "a b c", customType))
	mustAddDocument(t, iw, doc)
	mustCommit(t, iw)

	doc = document.NewDocument()
	doc.Add(newTextField(t, "tvtest", "x y z", false))
	mustAddDocument(t, iw, doc)
	// Make first segment
	mustCommit(t, iw)

	if err := iw.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}

	customType2 := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	customType2.SetStoreTermVectors(true)
	doc.Add(newField(t, "tvtest", "a b c", customType2))
	doc = document.NewDocument()
	mustAddDocument(t, iw, doc)
	// Make 2nd segment
	mustCommit(t, iw)
	if err := iw.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}

	mustClose(t, iw, dir)
}

func textNotStoredWith(configure func(*document.FieldType)) *document.FieldType {
	ft := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	configure(ft)
	return ft
}

// In a single doc, for the same field, mix the term vectors up
func TestTermVectorsWriterInconsistentTermVectorOptions(t *testing.T) {
	// no vectors + vectors
	termVectorsWriterDoTestMixup(t,
		textNotStoredWith(func(*document.FieldType) {}),
		textNotStoredWith(func(ft *document.FieldType) { ft.SetStoreTermVectors(true) }))

	// vectors + vectors with pos
	termVectorsWriterDoTestMixup(t,
		textNotStoredWith(func(ft *document.FieldType) { ft.SetStoreTermVectors(true) }),
		textNotStoredWith(func(ft *document.FieldType) {
			ft.SetStoreTermVectors(true)
			ft.SetStoreTermVectorPositions(true)
		}))

	// vectors + vectors with off
	termVectorsWriterDoTestMixup(t,
		textNotStoredWith(func(ft *document.FieldType) { ft.SetStoreTermVectors(true) }),
		textNotStoredWith(func(ft *document.FieldType) {
			ft.SetStoreTermVectors(true)
			ft.SetStoreTermVectorOffsets(true)
		}))

	// vectors with pos + vectors with pos + off
	termVectorsWriterDoTestMixup(t,
		textNotStoredWith(func(ft *document.FieldType) {
			ft.SetStoreTermVectors(true)
			ft.SetStoreTermVectorPositions(true)
		}),
		textNotStoredWith(func(ft *document.FieldType) {
			ft.SetStoreTermVectors(true)
			ft.SetStoreTermVectorPositions(true)
			ft.SetStoreTermVectorOffsets(true)
		}))

	// vectors with pos + vectors with pos + pay
	termVectorsWriterDoTestMixup(t,
		textNotStoredWith(func(ft *document.FieldType) {
			ft.SetStoreTermVectors(true)
			ft.SetStoreTermVectorPositions(true)
		}),
		textNotStoredWith(func(ft *document.FieldType) {
			ft.SetStoreTermVectors(true)
			ft.SetStoreTermVectorPositions(true)
			ft.SetStoreTermVectorPayloads(true)
		}))
}

func termVectorsWriterDoTestMixup(t *testing.T, ft1, ft2 *document.FieldType) {
	t.Helper()
	dir := newDirectory()
	iw := newRandomIndexWriter(t, dir)

	// add 3 good docs
	for i := 0; i < 3; i++ {
		doc := document.NewDocument()
		doc.Add(newStringField(t, "id", strconv.Itoa(i), false))
		if _, err := iw.AddDocument(doc); err != nil {
			t.Fatalf("addDocument: %v", err)
		}
	}

	// add broken doc
	doc := document.NewDocument()
	doc.Add(newField(t, "field", "value1", ft1))
	doc.Add(newField(t, "field", "value2", ft2))

	// ensure broken doc hits exception
	_, err := iw.AddDocument(doc)
	if err == nil {
		t.Fatal("expected IllegalArgumentException from addDocument")
	}
	message := err.Error()
	if !strings.HasPrefix(message, "all instances of a given field name must have the same term vectors settings") &&
		!strings.HasPrefix(message, "Inconsistency of field data structures across documents for field [field]") {
		t.Fatalf("unexpected message: %q", message)
	}
	// ensure good docs are still ok
	ir := mustGetReaderRIW(t, iw)
	assertNumDocs(t, 3, ir)

	mustClose(t, ir, iw, dir)
}

// LUCENE-5611: don't abort segment when term vector settings are wrong
func TestTermVectorsWriterNoAbortOnBadTVSettings(t *testing.T) {
	dir := newDirectory()
	// Don't use RandomIndexWriter because we want to be sure both docs go to 1 seg:
	iwc := index.NewIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iw := mustNewIndexWriter(t, dir, iwc)

	doc := document.NewDocument()
	mustAddDocument(t, iw, doc)
	ft := document.NewFieldTypeFrom(document.StoredFieldType)
	ft.SetStoreTermVectors(true)
	ft.Freeze()
	doc.Add(newField(t, "field", "value", ft))

	if _, err := iw.AddDocument(doc); err == nil {
		t.Fatal("expected IllegalArgumentException from addDocument")
	}

	r := openReaderFromWriter(t, iw)

	// Make sure the exc didn't lose our first document:
	assertNumDocs(t, 1, r)
	mustClose(t, iw, r, dir)
}
