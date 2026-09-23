// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestConsistentFieldNumbers.java
// (Apache Lucene 10.5.0).

package index_test

import (
	"math/rand"
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

func noMergeWriter(t testing.TB, dir store.Directory) *index.IndexWriter {
	t.Helper()
	conf := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	conf.SetMergePolicy(index.NewNoMergePolicy())
	return mustNewIndexWriter(t, dir, conf)
}

func mustReadLatestCommit(t testing.TB, dir store.Directory) *index.SegmentInfos {
	t.Helper()
	sis, err := spi.ReadLatestCommit(dir)
	if err != nil {
		t.Fatalf("SegmentInfos.readLatestCommit: %v", err)
	}
	return sis
}

func assertSegmentInfosSize(t testing.TB, expected int, sis *index.SegmentInfos) {
	t.Helper()
	if sis.Size() != expected {
		t.Fatalf("sis.size(): expected %d, got %d", expected, sis.Size())
	}
}

func mustReadFieldInfos(t testing.TB, si *index.SegmentCommitInfo) *index.FieldInfos {
	t.Helper()
	fis, err := index.IndexWriterReadFieldInfos(si)
	if err != nil {
		t.Fatalf("IndexWriter.readFieldInfos: %v", err)
	}
	return fis
}

// assertFieldName renders assertEquals(name, fis.fieldInfo(number).name).
func assertFieldName(t testing.TB, expected string, fis *index.FieldInfos, number int) {
	t.Helper()
	fi := fis.FieldInfoByNumber(number)
	if fi == nil {
		t.Fatalf("fieldInfo(%d): expected %q, got null", number, expected)
	}
	if fi.Name() != expected {
		t.Fatalf("fieldInfo(%d).name: expected %q, got %q", number, expected, fi.Name())
	}
}

func TestConsistentFieldNumbersSameFieldNumbersAcrossSegments(t *testing.T) {
	for i := 0; i < 2; i++ {
		dir := newDirectory()
		writer := noMergeWriter(t, dir)

		d1 := document.NewDocument()
		d1.Add(newTextField(t, "f1", "first field", true))
		d1.Add(newTextField(t, "f2", "second field", true))
		mustAddDocument(t, writer, d1)

		if i == 1 {
			mustClose(t, writer)
			writer = noMergeWriter(t, dir)
		} else {
			mustCommit(t, writer)
		}

		d2 := document.NewDocument()
		d2.Add(newTextField(t, "f2", "second field", false))
		d2.Add(newTextField(t, "f1", "first field", true))
		d2.Add(newTextField(t, "f3", "third field", false))
		d2.Add(newTextField(t, "f4", "fourth field", false))
		mustAddDocument(t, writer, d2)

		mustClose(t, writer)

		sis := mustReadLatestCommit(t, dir)
		assertSegmentInfosSize(t, 2, sis)

		fis1 := mustReadFieldInfos(t, sis.Get(0))
		fis2 := mustReadFieldInfos(t, sis.Get(1))

		assertFieldName(t, "f1", fis1, 0)
		assertFieldName(t, "f2", fis1, 1)
		assertFieldName(t, "f1", fis2, 0)
		assertFieldName(t, "f2", fis2, 1)
		assertFieldName(t, "f3", fis2, 2)
		assertFieldName(t, "f4", fis2, 3)

		writer = mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))
		if err := writer.ForceMerge(1); err != nil {
			t.Fatalf("forceMerge: %v", err)
		}
		mustClose(t, writer)

		sis = mustReadLatestCommit(t, dir)
		assertSegmentInfosSize(t, 1, sis)

		fis3 := mustReadFieldInfos(t, sis.Get(0))

		assertFieldName(t, "f1", fis3, 0)
		assertFieldName(t, "f2", fis3, 1)
		assertFieldName(t, "f3", fis3, 2)
		assertFieldName(t, "f4", fis3, 3)

		mustClose(t, dir)
	}
}

func TestConsistentFieldNumbersAddIndexes(t *testing.T) {
	dir1 := newDirectory()
	dir2 := newDirectory()
	writer := noMergeWriter(t, dir1)

	d1 := document.NewDocument()
	d1.Add(newTextField(t, "f1", "first field", true))
	d1.Add(newTextField(t, "f2", "second field", true))
	mustAddDocument(t, writer, d1)

	mustClose(t, writer)
	writer = noMergeWriter(t, dir2)

	d2 := document.NewDocument()
	d2.Add(newTextField(t, "f2", "second field", true))
	d2.Add(newTextField(t, "f1", "first field", true))
	d2.Add(newTextField(t, "f3", "third field", true))
	d2.Add(newTextField(t, "f4", "fourth field", true))
	mustAddDocument(t, writer, d2)

	mustClose(t, writer)

	writer = noMergeWriter(t, dir1)
	if _, err := writer.AddIndexes(dir2); err != nil {
		t.Fatalf("addIndexes: %v", err)
	}
	mustClose(t, writer)

	sis := mustReadLatestCommit(t, dir1)
	assertSegmentInfosSize(t, 2, sis)

	fis1 := mustReadFieldInfos(t, sis.Get(0))
	fis2 := mustReadFieldInfos(t, sis.Get(1))

	assertFieldName(t, "f1", fis1, 0)
	assertFieldName(t, "f2", fis1, 1)
	// make sure the ordering of the "external" segment is preserved
	assertFieldName(t, "f2", fis2, 0)
	assertFieldName(t, "f1", fis2, 1)
	assertFieldName(t, "f3", fis2, 2)
	assertFieldName(t, "f4", fis2, 3)

	mustClose(t, dir1, dir2)
}

func mustStoredBytesField(t testing.TB, name string, value []byte) *document.StoredField {
	t.Helper()
	f, err := document.NewStoredFieldFromBytes(name, value)
	if err != nil {
		t.Fatalf("StoredField: %v", err)
	}
	return f
}

func assertNoFieldNumber(t testing.TB, fis *index.FieldInfos, number int) {
	t.Helper()
	if fi := fis.FieldInfoByNumber(number); fi != nil {
		t.Fatalf("assertNull(fieldInfo(%d)): got %q", number, fi.Name())
	}
}

func TestConsistentFieldNumbersFieldNumberGaps(t *testing.T) {
	numIters := atLeast(13)
	for i := 0; i < numIters; i++ {
		dir := newDirectory()
		{
			writer := noMergeWriter(t, dir)
			d := document.NewDocument()
			d.Add(newTextField(t, "f1", "d1 first field", true))
			d.Add(newTextField(t, "f2", "d1 second field", true))
			mustAddDocument(t, writer, d)
			mustClose(t, writer)
			sis := mustReadLatestCommit(t, dir)
			assertSegmentInfosSize(t, 1, sis)
			fis1 := mustReadFieldInfos(t, sis.Get(0))
			assertFieldName(t, "f1", fis1, 0)
			assertFieldName(t, "f2", fis1, 1)
		}

		{
			writer := noMergeWriter(t, dir)
			d := document.NewDocument()
			d.Add(newTextField(t, "f1", "d2 first field", true))
			d.Add(mustStoredBytesField(t, "f3", []byte{1, 2, 3}))
			mustAddDocument(t, writer, d)
			mustClose(t, writer)
			sis := mustReadLatestCommit(t, dir)
			assertSegmentInfosSize(t, 2, sis)
			fis1 := mustReadFieldInfos(t, sis.Get(0))
			fis2 := mustReadFieldInfos(t, sis.Get(1))
			assertFieldName(t, "f1", fis1, 0)
			assertFieldName(t, "f2", fis1, 1)
			assertFieldName(t, "f1", fis2, 0)
			assertNoFieldNumber(t, fis2, 1)
			assertFieldName(t, "f3", fis2, 2)
		}

		{
			writer := noMergeWriter(t, dir)
			d := document.NewDocument()
			d.Add(newTextField(t, "f1", "d3 first field", true))
			d.Add(newTextField(t, "f2", "d3 second field", true))
			d.Add(mustStoredBytesField(t, "f3", []byte{1, 2, 3, 4, 5}))
			mustAddDocument(t, writer, d)
			mustClose(t, writer)
			sis := mustReadLatestCommit(t, dir)
			assertSegmentInfosSize(t, 3, sis)
			fis1 := mustReadFieldInfos(t, sis.Get(0))
			fis2 := mustReadFieldInfos(t, sis.Get(1))
			fis3 := mustReadFieldInfos(t, sis.Get(2))
			assertFieldName(t, "f1", fis1, 0)
			assertFieldName(t, "f2", fis1, 1)
			assertFieldName(t, "f1", fis2, 0)
			assertNoFieldNumber(t, fis2, 1)
			assertFieldName(t, "f3", fis2, 2)
			assertFieldName(t, "f1", fis3, 0)
			assertFieldName(t, "f2", fis3, 1)
			assertFieldName(t, "f3", fis3, 2)
		}

		{
			writer := noMergeWriter(t, dir)
			mustDeleteTerm(t, writer, "f1", "d1")
			// nuke the first segment entirely so that the segment with gaps is
			// loaded first!
			if err := writer.ForceMergeDeletes(); err != nil {
				t.Fatalf("forceMergeDeletes: %v", err)
			}
			mustClose(t, writer)
		}

		mustClose(t, dir)
		t.Fatal("org.apache.lucene.tests.util.FailOnNonBulkMergesInfoStream is not ported")
	}
}

func TestConsistentFieldNumbersManyFields(t *testing.T) {
	numDocs := atLeast(200)
	maxFields := atLeast(50)

	docs := make([][4]int, numDocs)
	for i := range docs {
		for j := range docs[i] {
			docs[i][j] = rand.Intn(maxFields)
		}
	}

	dir := newDirectory()
	writer := mustNewIndexWriter(t, dir, newIndexWriterConfigWithAnalyzer(newMockAnalyzer()))

	for i := 0; i < numDocs; i++ {
		d := document.NewDocument()
		for j := range docs[i] {
			d.Add(consistentFieldNumbersGetField(t, docs[i][j]))
		}

		mustAddDocument(t, writer, d)
	}

	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	mustClose(t, writer)

	sis := mustReadLatestCommit(t, dir)
	for si := range sis.Iterator() {
		fis := mustReadFieldInfos(t, si)

		for it := fis.Iterator(); it.HasNext(); {
			fi := it.Next()
			number, err := strconv.Atoi(fi.Name())
			if err != nil {
				t.Fatalf("Integer.parseInt(%q): %v", fi.Name(), err)
			}
			expected := consistentFieldNumbersGetField(t, number)
			if expected.FieldType().IndexOptions() != fi.GetIndexOptions() {
				t.Fatalf("field %s indexOptions: expected %v, got %v", fi.Name(), expected.FieldType().IndexOptions(), fi.GetIndexOptions())
			}
			if expected.FieldType().StoreTermVectors() != fi.HasTermVectors() {
				t.Fatalf("field %s storeTermVectors: expected %v, got %v", fi.Name(), expected.FieldType().StoreTermVectors(), fi.HasTermVectors())
			}
		}
	}

	mustClose(t, dir)
}

// termVectorType renders new FieldType(base) followed by the setter calls
// the private getField(int) makes on it.
func termVectorType(base *document.FieldType, tokenized bool, vectors, offsets, positions bool) *document.FieldType {
	ft := document.NewFieldTypeFrom(base)
	if !tokenized {
		ft.SetTokenized(false)
	}
	if vectors {
		ft.SetStoreTermVectors(true)
	}
	if offsets {
		ft.SetStoreTermVectorOffsets(true)
	}
	if positions {
		ft.SetStoreTermVectorPositions(true)
	}
	return ft
}

// consistentFieldNumbersGetField renders the private getField(int).
func consistentFieldNumbersGetField(t testing.TB, number int) document.IndexableField {
	t.Helper()
	mode := number % 16
	fieldName := strconv.Itoa(number)
	stored, notStored := document.TextFieldTypeStored, document.TextFieldTypeNotStored
	customType := document.NewFieldTypeFrom(stored)
	customType2 := termVectorType(stored, false, false, false, false)
	customType3 := termVectorType(notStored, false, false, false, false)
	customType4 := termVectorType(notStored, false, true, true, false)
	customType5 := termVectorType(notStored, true, true, true, false)
	customType6 := termVectorType(stored, false, true, true, false)
	customType7 := termVectorType(notStored, false, true, true, false)
	customType8 := termVectorType(stored, false, true, false, true)
	customType9 := termVectorType(notStored, true, true, false, true)
	customType10 := termVectorType(stored, false, true, false, true)
	customType11 := termVectorType(notStored, false, true, false, true)
	customType12 := termVectorType(stored, true, true, true, true)
	customType13 := termVectorType(notStored, true, true, true, true)
	customType14 := termVectorType(stored, false, true, true, true)
	customType15 := termVectorType(notStored, false, true, true, true)

	var ft *document.FieldType
	switch mode {
	case 0:
		ft = customType
	case 1:
		return newTextField(t, fieldName, "some text", false)
	case 2:
		ft = customType2
	case 3:
		ft = customType3
	case 4:
		ft = customType4
	case 5:
		ft = customType5
	case 6:
		ft = customType6
	case 7:
		ft = customType7
	case 8:
		ft = customType8
	case 9:
		ft = customType9
	case 10:
		ft = customType10
	case 11:
		ft = customType11
	case 12:
		ft = customType12
	case 13:
		ft = customType13
	case 14:
		ft = customType14
	case 15:
		ft = customType15
	default:
		return nil
	}
	f, err := document.NewField(fieldName, "some text", ft)
	if err != nil {
		t.Fatalf("Field: %v", err)
	}
	return f
}
