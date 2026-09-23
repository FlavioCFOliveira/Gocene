// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestDocValues.java
// (Apache Lucene 10.5.0): tests helper methods in DocValues.

package index_test

import (
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// docValuesGetBinaryMissing names the DocValues helper every test but the
// last reaches.
const docValuesGetBinaryMissing = "org.apache.lucene.index.DocValues#getBinary(LeafReader, String) is not ported"

// docValuesTestReader renders the shared prologue: an IndexWriter with
// newIndexWriterConfig(null), one document, an NRT reader and its only leaf.
func docValuesTestReader(t *testing.T, fields ...document.IndexableField) (store.Directory, *index.IndexWriter, *index.DirectoryReader, index.LeafReader) {
	t.Helper()
	dir := newDirectory()
	iw := mustNewIndexWriter(t, dir, index.NewIndexWriterConfigWithAnalyzer(nil))
	doc := document.NewDocument()
	for _, f := range fields {
		doc.Add(f)
	}
	mustAddDocument(t, iw, doc)
	dr := openReaderFromWriter(t, iw)
	r := getOnlyLeafReader(t, dr)
	return dir, iw, dr, r
}

func assertDocValuesOK(t *testing.T, what string, dv any, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
	if dv == nil {
		t.Fatalf("assertNotNull(%s)", what)
	}
}

func assertDocValuesIllegalState(t *testing.T, what string, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected IllegalStateException from %s", what)
	}
}

// If the field doesn't exist, we return empty instances: it can easily happen
// that a segment just doesn't have any docs with the field.
func TestDocValuesEmptyIndex(t *testing.T) {
	dir, iw, dr, _ := docValuesTestReader(t)
	defer mustClose(t, dr, iw, dir)

	// ok
	t.Fatal(docValuesGetBinaryMissing)
}

// field just doesnt have any docvalues at all: exception
func TestDocValuesMisconfiguredField(t *testing.T) {
	dir, iw, dr, _ := docValuesTestReader(t, newStringField(t, "foo", "bar", false))
	defer mustClose(t, dr, iw, dir)

	// errors
	t.Fatal(docValuesGetBinaryMissing)
}

// field with numeric docvalues
func TestDocValuesNumericField(t *testing.T) {
	f, err := document.NewNumericDocValuesField("foo", 3)
	if err != nil {
		t.Fatalf("NumericDocValuesField: %v", err)
	}
	dir, iw, dr, r := docValuesTestReader(t, f)
	defer mustClose(t, dr, iw, dir)

	// ok
	numeric, err := index.GetNumeric(r, "foo")
	assertDocValuesOK(t, "DocValues.getNumeric(r, \"foo\")", numeric, err)
	sortedNumeric, err := index.GetSortedNumeric(r, "foo")
	assertDocValuesOK(t, "DocValues.getSortedNumeric(r, \"foo\")", sortedNumeric, err)

	// errors
	t.Fatal(docValuesGetBinaryMissing)
}

// field with binary docvalues
func TestDocValuesBinaryField(t *testing.T) {
	f, err := document.NewBinaryDocValuesField("foo", []byte("bar"))
	if err != nil {
		t.Fatalf("BinaryDocValuesField: %v", err)
	}
	dir, iw, dr, _ := docValuesTestReader(t, f)
	defer mustClose(t, dr, iw, dir)

	// ok
	t.Fatal(docValuesGetBinaryMissing)
}

// field with sorted docvalues
func TestDocValuesSortedField(t *testing.T) {
	f, err := document.NewSortedDocValuesField("foo", []byte("bar"))
	if err != nil {
		t.Fatalf("SortedDocValuesField: %v", err)
	}
	dir, iw, dr, r := docValuesTestReader(t, f)
	defer mustClose(t, dr, iw, dir)

	// ok
	sorted, err := index.GetSorted(r, "foo")
	assertDocValuesOK(t, "DocValues.getSorted(r, \"foo\")", sorted, err)
	sortedSet, err := index.GetSortedSet(r, "foo")
	assertDocValuesOK(t, "DocValues.getSortedSet(r, \"foo\")", sortedSet, err)

	// errors
	t.Fatal(docValuesGetBinaryMissing)
}

// field with sortedset docvalues
func TestDocValuesSortedSetField(t *testing.T) {
	f, err := document.NewSortedSetDocValuesField("foo", [][]byte{[]byte("bar")})
	if err != nil {
		t.Fatalf("SortedSetDocValuesField: %v", err)
	}
	dir, iw, dr, r := docValuesTestReader(t, f)
	defer mustClose(t, dr, iw, dir)

	// ok
	sortedSet, err := index.GetSortedSet(r, "foo")
	assertDocValuesOK(t, "DocValues.getSortedSet(r, \"foo\")", sortedSet, err)

	// errors
	t.Fatal(docValuesGetBinaryMissing)
}

// field with sortednumeric docvalues
func TestDocValuesSortedNumericField(t *testing.T) {
	f, err := document.NewSortedNumericDocValuesField("foo", []int64{3})
	if err != nil {
		t.Fatalf("SortedNumericDocValuesField: %v", err)
	}
	dir, iw, dr, r := docValuesTestReader(t, f)
	defer mustClose(t, dr, iw, dir)

	// ok
	sortedNumeric, err := index.GetSortedNumeric(r, "foo")
	assertDocValuesOK(t, "DocValues.getSortedNumeric(r, \"foo\")", sortedNumeric, err)

	// errors
	t.Fatal(docValuesGetBinaryMissing)
}

func TestDocValuesAddNullNumericDocValues(t *testing.T) {
	dir := newDirectory()
	iw := mustNewIndexWriter(t, dir, index.NewIndexWriterConfigWithAnalyzer(nil))
	defer mustClose(t, iw, dir)
	doc := document.NewDocument()
	if rand.Intn(2) == 0 {
		t.Fatal("org.apache.lucene.document.NumericDocValuesField(String, Long) accepting a null value is not ported")
	}
	f, err := document.NewBinaryDocValuesField("foo", nil)
	if err != nil {
		t.Fatalf("new BinaryDocValuesField(\"foo\", null): %v", err)
	}
	doc.Add(f)
	_, err = iw.AddDocument(doc)
	if err == nil {
		t.Fatal("expected IllegalArgumentException from addDocument")
	}
	if got := err.Error(); got != "field=\"foo\": null value not allowed" {
		t.Fatalf("message: expected %q, got %q", "field=\"foo\": null value not allowed", got)
	}
}
