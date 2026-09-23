// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestMultiDocValues.java
// (Apache Lucene 10.5.0): tests MultiDocValues versus ordinary segment merging.

package index_test

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	testindex "github.com/FlavioCFOliveira/Gocene/tests/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// multiDocValuesConfig renders newIndexWriterConfig(random(), null) followed by
// iwc.setMergePolicy(newLogMergePolicy()).
func multiDocValuesConfig() *index.IndexWriterConfig {
	iwc := newIndexWriterConfigWithAnalyzer(nil)
	iwc.SetMergePolicy(newLogMergePolicy())
	return iwc
}

// multiDocValuesReaders renders the shared tail of every test: getReader,
// forceMerge(1), getReader, getOnlyLeafReader, close.
func multiDocValuesReaders(t *testing.T, iw *testindex.RandomIndexWriter) (*index.DirectoryReader, *index.DirectoryReader, index.LeafReader) {
	t.Helper()
	ir, err := iw.GetReader()
	if err != nil {
		t.Fatalf("getReader: %v", err)
	}
	if err := iw.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	ir2, err := iw.GetReader()
	if err != nil {
		t.Fatalf("getReader: %v", err)
	}
	merged := getOnlyLeafReader(t, ir2)
	mustClose(t, iw)
	return ir, ir2, merged
}

func multiDocValuesMaybeCommit(t *testing.T, iw *testindex.RandomIndexWriter) {
	t.Helper()
	if rand.Intn(17) == 0 {
		if _, err := iw.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
	}
}

func mdvAdd(t *testing.T, iw *testindex.RandomIndexWriter, doc *document.Document) {
	t.Helper()
	if _, err := iw.AddDocument(doc); err != nil {
		t.Fatalf("addDocument: %v", err)
	}
}

func mdvNext(t *testing.T, it interface{ NextDoc() (int, error) }) int {
	t.Helper()
	doc, err := it.NextDoc()
	if err != nil {
		t.Fatalf("nextDoc: %v", err)
	}
	return doc
}

func mdvCheck(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestMultiDocValuesNumerics(t *testing.T) {
	dir := newDirectory()
	doc := document.NewDocument()
	field, err := document.NewNumericDocValuesField("numbers", 0)
	mdvCheck(t, err)
	doc.Add(field)

	iw := newRandomIndexWriterWithConfig(t, dir, multiDocValuesConfig())

	numDocs := atLeast(50)
	for i := 0; i < numDocs; i++ {
		field.SetLongValue(int64(rand.Uint64()))
		mdvAdd(t, iw, doc)
		multiDocValuesMaybeCommit(t, iw)
	}
	ir, ir2, merged := multiDocValuesReaders(t, iw)

	multi, err := index.MultiDocValuesGetNumericValues(ir, "numbers")
	mdvCheck(t, err)
	single, err := merged.GetNumericDocValues("numbers")
	mdvCheck(t, err)
	for i := 0; i < numDocs; i++ {
		if got := mdvNext(t, multi); got != i {
			t.Fatalf("multi.nextDoc: expected %d, got %d", i, got)
		}
		if got := mdvNext(t, single); got != i {
			t.Fatalf("single.nextDoc: expected %d, got %d", i, got)
		}
		sv, err := single.LongValue()
		mdvCheck(t, err)
		mv, err := multi.LongValue()
		mdvCheck(t, err)
		if sv != mv {
			t.Fatalf("doc %d: expected %d, got %d", i, sv, mv)
		}
	}
	a, err := merged.GetNumericDocValues("numbers")
	mdvCheck(t, err)
	b, err := index.MultiDocValuesGetNumericValues(ir, "numbers")
	mdvCheck(t, err)
	mdvTestRandomAdvance(t, a, b)
	a, err = merged.GetNumericDocValues("numbers")
	mdvCheck(t, err)
	b, err = index.MultiDocValuesGetNumericValues(ir, "numbers")
	mdvCheck(t, err)
	mdvTestRandomAdvanceExact(t, a, b, merged.MaxDoc())

	mustClose(t, ir, ir2, dir)
}

func TestMultiDocValuesBinary(t *testing.T) {
	dir := newDirectory()
	doc := document.NewDocument()
	field, err := document.NewBinaryDocValuesField("bytes", []byte{})
	mdvCheck(t, err)
	doc.Add(field)

	iw := newRandomIndexWriterWithConfig(t, dir, multiDocValuesConfig())

	numDocs := atLeast(50)
	r := rand.New(rand.NewSource(rand.Int63()))
	for i := 0; i < numDocs; i++ {
		field.SetBytesValue([]byte(util.RandomUnicodeString(r, 20)))
		mdvAdd(t, iw, doc)
		multiDocValuesMaybeCommit(t, iw)
	}
	ir, ir2, merged := multiDocValuesReaders(t, iw)

	multi, err := index.MultiDocValuesGetBinaryValues(ir, "bytes")
	mdvCheck(t, err)
	single, err := merged.GetBinaryDocValues("bytes")
	mdvCheck(t, err)
	for i := 0; i < numDocs; i++ {
		if got := mdvNext(t, multi); got != i {
			t.Fatalf("multi.nextDoc: expected %d, got %d", i, got)
		}
		if got := mdvNext(t, single); got != i {
			t.Fatalf("single.nextDoc: expected %d, got %d", i, got)
		}
		sv, err := single.BinaryValue()
		mdvCheck(t, err)
		expected := append([]byte(nil), sv...)
		actual, err := multi.BinaryValue()
		mdvCheck(t, err)
		if !bytes.Equal(expected, actual) {
			t.Fatalf("doc %d: expected %v, got %v", i, expected, actual)
		}
	}
	a, err := merged.GetBinaryDocValues("bytes")
	mdvCheck(t, err)
	b, err := index.MultiDocValuesGetBinaryValues(ir, "bytes")
	mdvCheck(t, err)
	mdvTestRandomAdvance(t, a, b)
	a, err = merged.GetBinaryDocValues("bytes")
	mdvCheck(t, err)
	b, err = index.MultiDocValuesGetBinaryValues(ir, "bytes")
	mdvCheck(t, err)
	mdvTestRandomAdvanceExact(t, a, b, merged.MaxDoc())

	mustClose(t, ir, ir2, dir)
}

func mdvCheckSorted(t *testing.T, single, multi index.SortedDocValues) {
	t.Helper()
	so, err := single.OrdValue()
	mdvCheck(t, err)
	mo, err := multi.OrdValue()
	mdvCheck(t, err)
	// check value
	sv, err := single.LookupOrd(so)
	mdvCheck(t, err)
	expected := append([]byte(nil), sv...)
	actual, err := multi.LookupOrd(mo)
	mdvCheck(t, err)
	if !bytes.Equal(expected, actual) {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
	// check ord
	if so != mo {
		t.Fatalf("expected ord %d, got %d", so, mo)
	}
}

func TestMultiDocValuesSorted(t *testing.T) {
	dir := newDirectory()
	doc := document.NewDocument()
	field, err := document.NewSortedDocValuesField("bytes", []byte{})
	mdvCheck(t, err)
	doc.Add(field)

	iw := newRandomIndexWriterWithConfig(t, dir, multiDocValuesConfig())

	numDocs := atLeast(50)
	r := rand.New(rand.NewSource(rand.Int63()))
	for i := 0; i < numDocs; i++ {
		field.SetBytesValue([]byte(util.RandomUnicodeString(r, 20)))
		if rand.Intn(7) == 0 {
			mdvAdd(t, iw, document.NewDocument())
		}
		mdvAdd(t, iw, doc)
		multiDocValuesMaybeCommit(t, iw)
	}
	ir, ir2, merged := multiDocValuesReaders(t, iw)
	multi, err := index.MultiDocValuesGetSortedValues(ir, "bytes")
	mdvCheck(t, err)
	single, err := merged.GetSortedDocValues("bytes")
	mdvCheck(t, err)
	if single.GetValueCount() != multi.GetValueCount() {
		t.Fatalf("expected valueCount %d, got %d", single.GetValueCount(), multi.GetValueCount())
	}
	for {
		sd := mdvNext(t, single)
		md := mdvNext(t, multi)
		if sd != md {
			t.Fatalf("expected doc %d, got %d", sd, md)
		}
		if single.DocID() == index.NO_MORE_DOCS {
			break
		}
		mdvCheckSorted(t, single, multi)
	}
	a, err := merged.GetSortedDocValues("bytes")
	mdvCheck(t, err)
	b, err := index.MultiDocValuesGetSortedValues(ir, "bytes")
	mdvCheck(t, err)
	mdvTestRandomAdvance(t, a, b)
	a, err = merged.GetSortedDocValues("bytes")
	mdvCheck(t, err)
	b, err = index.MultiDocValuesGetSortedValues(ir, "bytes")
	mdvCheck(t, err)
	mdvTestRandomAdvanceExact(t, a, b, merged.MaxDoc())
	mustClose(t, ir, ir2, dir)
}

// TestMultiDocValuesSortedWithLotsOfDups tries to make more dups than testSorted.
func TestMultiDocValuesSortedWithLotsOfDups(t *testing.T) {
	dir := newDirectory()
	doc := document.NewDocument()
	field, err := document.NewSortedDocValuesField("bytes", []byte{})
	mdvCheck(t, err)
	doc.Add(field)

	iw := newRandomIndexWriterWithConfig(t, dir, multiDocValuesConfig())

	numDocs := atLeast(50)
	r := rand.New(rand.NewSource(rand.Int63()))
	for i := 0; i < numDocs; i++ {
		field.SetBytesValue([]byte(util.RandomSimpleString(r, 0, 2)))
		mdvAdd(t, iw, doc)
		multiDocValuesMaybeCommit(t, iw)
	}
	ir, ir2, merged := multiDocValuesReaders(t, iw)

	multi, err := index.MultiDocValuesGetSortedValues(ir, "bytes")
	mdvCheck(t, err)
	single, err := merged.GetSortedDocValues("bytes")
	mdvCheck(t, err)
	if single.GetValueCount() != multi.GetValueCount() {
		t.Fatalf("expected valueCount %d, got %d", single.GetValueCount(), multi.GetValueCount())
	}
	for i := 0; i < numDocs; i++ {
		if got := mdvNext(t, multi); got != i {
			t.Fatalf("multi.nextDoc: expected %d, got %d", i, got)
		}
		if got := mdvNext(t, single); got != i {
			t.Fatalf("single.nextDoc: expected %d, got %d", i, got)
		}
		mdvCheckSorted(t, single, multi)
	}
	a, err := merged.GetSortedDocValues("bytes")
	mdvCheck(t, err)
	b, err := index.MultiDocValuesGetSortedValues(ir, "bytes")
	mdvCheck(t, err)
	mdvTestRandomAdvance(t, a, b)
	a, err = merged.GetSortedDocValues("bytes")
	mdvCheck(t, err)
	b, err = index.MultiDocValuesGetSortedValues(ir, "bytes")
	mdvCheck(t, err)
	mdvTestRandomAdvanceExact(t, a, b, merged.MaxDoc())

	mustClose(t, ir, ir2, dir)
}

func mdvCheckSortedSet(t *testing.T, single, multi index.SortedSetDocValues) {
	t.Helper()
	if multi == nil {
		if single != nil {
			t.Fatal("assertNull(single)")
		}
		return
	}
	if single.GetValueCount() != multi.GetValueCount() {
		t.Fatalf("expected valueCount %d, got %d", single.GetValueCount(), multi.GetValueCount())
	}
	// check values
	for i := 0; i < single.GetValueCount(); i++ {
		sv, err := single.LookupOrd(i)
		mdvCheck(t, err)
		expected := append([]byte(nil), sv...)
		actual, err := multi.LookupOrd(i)
		mdvCheck(t, err)
		if !bytes.Equal(expected, actual) {
			t.Fatalf("ord %d: expected %v, got %v", i, expected, actual)
		}
	}
	// check ord list
	for {
		docID := mdvNext(t, single)
		if got := mdvNext(t, multi); got != docID {
			t.Fatalf("expected doc %d, got %d", docID, got)
		}
		if docID == index.NO_MORE_DOCS {
			break
		}
		if single.DocValueCount() != multi.DocValueCount() {
			t.Fatalf("expected docValueCount %d, got %d", single.DocValueCount(), multi.DocValueCount())
		}
		for i := 0; i < single.DocValueCount(); i++ {
			so, err := single.NextOrd()
			mdvCheck(t, err)
			mo, err := multi.NextOrd()
			mdvCheck(t, err)
			if so != mo {
				t.Fatalf("expected ord %d, got %d", so, mo)
			}
		}
	}
}

func mdvSortedSetTest(t *testing.T, value func(r *rand.Rand) string) {
	dir := newDirectory()

	iw := newRandomIndexWriterWithConfig(t, dir, multiDocValuesConfig())

	numDocs := atLeast(50)
	r := rand.New(rand.NewSource(rand.Int63()))
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		numValues := rand.Intn(5)
		for j := 0; j < numValues; j++ {
			f, err := document.NewSortedSetDocValuesField("bytes", [][]byte{[]byte(value(r))})
			mdvCheck(t, err)
			doc.Add(f)
		}
		mdvAdd(t, iw, doc)
		multiDocValuesMaybeCommit(t, iw)
	}
	ir, ir2, merged := multiDocValuesReaders(t, iw)

	multi, err := index.MultiDocValuesGetSortedSetValues(ir, "bytes")
	mdvCheck(t, err)
	single, err := merged.GetSortedSetDocValues("bytes")
	mdvCheck(t, err)
	mdvCheckSortedSet(t, single, multi)
	a, err := merged.GetSortedSetDocValues("bytes")
	mdvCheck(t, err)
	b, err := index.MultiDocValuesGetSortedSetValues(ir, "bytes")
	mdvCheck(t, err)
	mdvTestRandomAdvance(t, a, b)
	a, err = merged.GetSortedSetDocValues("bytes")
	mdvCheck(t, err)
	b, err = index.MultiDocValuesGetSortedSetValues(ir, "bytes")
	mdvCheck(t, err)
	mdvTestRandomAdvanceExact(t, a, b, merged.MaxDoc())

	mustClose(t, ir, ir2, dir)
}

func TestMultiDocValuesSortedSet(t *testing.T) {
	mdvSortedSetTest(t, func(r *rand.Rand) string { return util.RandomUnicodeString(r, 20) })
}

// TestMultiDocValuesSortedSetWithDups tries to make more dups than testSortedSet.
func TestMultiDocValuesSortedSetWithDups(t *testing.T) {
	mdvSortedSetTest(t, func(r *rand.Rand) string { return util.RandomSimpleString(r, 0, 2) })
}

func TestMultiDocValuesSortedNumeric(t *testing.T) {
	dir := newDirectory()

	iw := newRandomIndexWriterWithConfig(t, dir, multiDocValuesConfig())

	numDocs := atLeast(50)
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		numValues := rand.Intn(5)
		for j := 0; j < numValues; j++ {
			f, err := document.NewSortedNumericDocValuesField("nums", []int64{int64(rand.Uint64())})
			mdvCheck(t, err)
			doc.Add(f)
		}
		mdvAdd(t, iw, doc)
		multiDocValuesMaybeCommit(t, iw)
	}
	ir, ir2, merged := multiDocValuesReaders(t, iw)

	multi, err := index.MultiDocValuesGetSortedNumericValues(ir, "nums")
	mdvCheck(t, err)
	single, err := merged.GetSortedNumericDocValues("nums")
	mdvCheck(t, err)
	if multi == nil {
		if single != nil {
			t.Fatal("assertNull(single)")
		}
	} else {
		// check values
		for i := 0; i < numDocs; i++ {
			if i > single.DocID() {
				sd := mdvNext(t, single)
				if md := mdvNext(t, multi); sd != md {
					t.Fatalf("expected doc %d, got %d", sd, md)
				}
			}
			if i == single.DocID() {
				sc, err := single.DocValueCount()
				mdvCheck(t, err)
				mc, err := multi.DocValueCount()
				mdvCheck(t, err)
				if sc != mc {
					t.Fatalf("expected docValueCount %d, got %d", sc, mc)
				}
				for j := 0; j < sc; j++ {
					sv, err := single.NextValue()
					mdvCheck(t, err)
					mv, err := multi.NextValue()
					mdvCheck(t, err)
					if sv != mv {
						t.Fatalf("expected value %d, got %d", sv, mv)
					}
				}
			}
		}
	}
	a, err := merged.GetSortedNumericDocValues("nums")
	mdvCheck(t, err)
	b, err := index.MultiDocValuesGetSortedNumericValues(ir, "nums")
	mdvCheck(t, err)
	mdvTestRandomAdvance(t, a, b)
	a, err = merged.GetSortedNumericDocValues("nums")
	mdvCheck(t, err)
	b, err = index.MultiDocValuesGetSortedNumericValues(ir, "nums")
	mdvCheck(t, err)
	mdvTestRandomAdvanceExact(t, a, b, merged.MaxDoc())

	mustClose(t, ir, ir2, dir)
}

func mdvTestRandomAdvance(t *testing.T, iter1, iter2 util.DocIdSetIterator) {
	t.Helper()
	if iter1.DocID() != -1 || iter2.DocID() != -1 {
		t.Fatalf("expected -1/-1, got %d/%d", iter1.DocID(), iter2.DocID())
	}
	for iter1.DocID() != index.NO_MORE_DOCS {
		if rand.Intn(2) == 0 {
			a := mdvNext(t, iter1)
			if b := mdvNext(t, iter2); a != b {
				t.Fatalf("nextDoc: expected %d, got %d", a, b)
			}
		} else {
			target := iter1.DocID() + nextInt(1, 100)
			a, err := iter1.Advance(target)
			mdvCheck(t, err)
			b, err := iter2.Advance(target)
			mdvCheck(t, err)
			if a != b {
				t.Fatalf("advance(%d): expected %d, got %d", target, a, b)
			}
		}
	}
}

func mdvTestRandomAdvanceExact(t *testing.T, iter1, iter2 index.DocValuesIterator, maxDoc int) {
	t.Helper()
	for target := rand.Intn(min(maxDoc, 10)); target < maxDoc; target += rand.Intn(10) {
		exists1, err := iter1.AdvanceExact(target)
		mdvCheck(t, err)
		exists2, err := iter2.AdvanceExact(target)
		mdvCheck(t, err)
		if exists1 != exists2 {
			t.Fatalf("advanceExact(%d): expected %t, got %t", target, exists1, exists2)
		}
	}
}
