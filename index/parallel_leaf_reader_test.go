// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestParallelLeafReader.java
// (Apache Lucene 10.5.0).

package index

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/store"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
)

func parallelLeafReaderConfig() *IndexWriterConfig {
	return NewIndexWriterConfigWithAnalyzer(testanalysis.NewMockAnalyzer(testanalysis.WHITESPACE, true, 0, nil, true))
}

func parallelLeafReaderTextField(t *testing.T, d *document.Document, name, value string) {
	t.Helper()
	f, err := document.NewTextField(name, value, true)
	if err != nil {
		t.Fatalf("newTextField: %v", err)
	}
	d.Add(f)
}

// parallelLeafReaderGetOnlyLeafReader renders
// LuceneTestCase.getOnlyLeafReader(DirectoryReader.open(dir)).
func parallelLeafReaderGetOnlyLeafReader(t *testing.T, dir store.Directory) LeafReader {
	t.Helper()
	reader, err := OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("DirectoryReader.open: %v", err)
	}
	return parallelLeafReaderOnlyLeaf(t, reader)
}

func parallelLeafReaderOnlyLeaf(t *testing.T, reader IndexReader) LeafReader {
	t.Helper()
	subReaders, err := reader.Leaves()
	if err != nil {
		t.Fatalf("leaves: %v", err)
	}
	if len(subReaders) != 1 {
		t.Fatalf("reader has %d segments instead of exactly one", len(subReaders))
	}
	return subReaders[0].LeafReader()
}

// parallelLeafReaderStoredValue renders
// `pr.storedFields().document(docID).get(field)` (null when absent).
func parallelLeafReaderStoredValue(t *testing.T, r LeafReader, docID int, field string) *string {
	t.Helper()
	storedFields, err := r.StoredFields()
	if err != nil {
		t.Fatalf("storedFields: %v", err)
	}
	visitor := document.NewDocumentStoredFieldVisitor()
	if err := storedFields.Document(docID, visitor); err != nil {
		t.Fatalf("document(%d): %v", docID, err)
	}
	f := visitor.GetDocument().Get(field)
	if f == nil {
		return nil
	}
	v := f.StringValue()
	return &v
}

func parallelLeafReaderAssertStored(t *testing.T, r LeafReader, field string, want *string) {
	t.Helper()
	got := parallelLeafReaderStoredValue(t, r, 0, field)
	switch {
	case want == nil && got != nil:
		t.Fatalf("assertNull(document(0).get(%q)): got %q", field, *got)
	case want != nil && (got == nil || *got != *want):
		t.Fatalf("document(0).get(%q): expected %q, got %v", field, *want, got)
	}
}

func parallelLeafReaderAssertTerms(t *testing.T, r LeafReader, field string, present bool) {
	t.Helper()
	terms, err := r.Terms(field)
	if err != nil {
		t.Fatalf("terms(%q): %v", field, err)
	}
	if present && terms == nil {
		t.Fatalf("assertNotNull(pr.terms(%q))", field)
	}
	if !present && terms != nil {
		t.Fatalf("assertNull(pr.terms(%q))", field)
	}
}

// TestParallelLeafReaderQueries ports testQueries, whose parallel() fixture
// validates the ParallelLeafReader with TestUtil.checkReader(IndexReader).
func TestParallelLeafReaderQueries(t *testing.T) {
	t.Fatal("org.apache.lucene.tests.util.TestUtil#checkReader(IndexReader) is not ported")
}

func TestParallelLeafReaderFieldNames(t *testing.T) {
	dir1 := parallelLeafReaderGetDir1(t)
	dir2 := parallelLeafReaderGetDir2(t)
	pr, err := NewParallelLeafReader(
		parallelLeafReaderGetOnlyLeafReader(t, dir1),
		parallelLeafReaderGetOnlyLeafReader(t, dir2))
	if err != nil {
		t.Fatalf("new ParallelLeafReader: %v", err)
	}
	fieldInfos := pr.GetFieldInfos()
	if fieldInfos.Size() != 4 {
		t.Fatalf("expected 4 fields, got %d", fieldInfos.Size())
	}
	for _, f := range []string{"f1", "f2", "f3", "f4"} {
		if fieldInfos.FieldInfo(f) == nil {
			t.Fatalf("assertNotNull(fieldInfos.fieldInfo(%q))", f)
		}
	}
	if err := pr.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := dir1.Close(); err != nil {
		t.Fatalf("close dir1: %v", err)
	}
	if err := dir2.Close(); err != nil {
		t.Fatalf("close dir2: %v", err)
	}
}

func TestParallelLeafReaderRefCounts1(t *testing.T) {
	dir1 := parallelLeafReaderGetDir1(t)
	dir2 := parallelLeafReaderGetDir2(t)
	// close subreaders, ParallelReader will not change refCounts, but close on its own close
	ir1 := parallelLeafReaderGetOnlyLeafReader(t, dir1)
	ir2 := parallelLeafReaderGetOnlyLeafReader(t, dir2)
	pr, err := NewParallelLeafReader(ir1, ir2)
	if err != nil {
		t.Fatalf("new ParallelLeafReader: %v", err)
	}

	// check RefCounts
	if ir1.GetRefCount() != 1 || ir2.GetRefCount() != 1 {
		t.Fatalf("expected refCounts 1/1, got %d/%d", ir1.GetRefCount(), ir2.GetRefCount())
	}
	if err := pr.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if ir1.GetRefCount() != 0 || ir2.GetRefCount() != 0 {
		t.Fatalf("expected refCounts 0/0, got %d/%d", ir1.GetRefCount(), ir2.GetRefCount())
	}
	if err := dir1.Close(); err != nil {
		t.Fatalf("close dir1: %v", err)
	}
	if err := dir2.Close(); err != nil {
		t.Fatalf("close dir2: %v", err)
	}
}

func TestParallelLeafReaderRefCounts2(t *testing.T) {
	dir1 := parallelLeafReaderGetDir1(t)
	dir2 := parallelLeafReaderGetDir2(t)
	ir1 := parallelLeafReaderGetOnlyLeafReader(t, dir1)
	ir2 := parallelLeafReaderGetOnlyLeafReader(t, dir2)
	// don't close subreaders, so ParallelReader will increment refcounts
	pr, err := NewParallelLeafReaderWithClose(false, ir1, ir2)
	if err != nil {
		t.Fatalf("new ParallelLeafReader: %v", err)
	}
	// check RefCounts
	if ir1.GetRefCount() != 2 || ir2.GetRefCount() != 2 {
		t.Fatalf("expected refCounts 2/2, got %d/%d", ir1.GetRefCount(), ir2.GetRefCount())
	}
	if err := pr.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if ir1.GetRefCount() != 1 || ir2.GetRefCount() != 1 {
		t.Fatalf("expected refCounts 1/1, got %d/%d", ir1.GetRefCount(), ir2.GetRefCount())
	}
	if err := ir1.Close(); err != nil {
		t.Fatalf("close ir1: %v", err)
	}
	if err := ir2.Close(); err != nil {
		t.Fatalf("close ir2: %v", err)
	}
	if ir1.GetRefCount() != 0 || ir2.GetRefCount() != 0 {
		t.Fatalf("expected refCounts 0/0, got %d/%d", ir1.GetRefCount(), ir2.GetRefCount())
	}
	if err := dir1.Close(); err != nil {
		t.Fatalf("close dir1: %v", err)
	}
	if err := dir2.Close(); err != nil {
		t.Fatalf("close dir2: %v", err)
	}
}

func TestParallelLeafReaderCloseInnerReader(t *testing.T) {
	dir1 := parallelLeafReaderGetDir1(t)
	ir1 := parallelLeafReaderGetOnlyLeafReader(t, dir1)

	// with overlapping
	pr, err := NewParallelLeafReaderFull(true, []LeafReader{ir1}, []LeafReader{ir1})
	if err != nil {
		t.Fatalf("new ParallelLeafReader: %v", err)
	}

	if err := ir1.Close(); err != nil {
		t.Fatalf("close ir1: %v", err)
	}

	// should already be closed because inner reader is closed!
	err = func() error {
		storedFields, err := pr.StoredFields()
		if err != nil {
			return err
		}
		return storedFields.Document(0, document.NewDocumentStoredFieldVisitor())
	}()
	var ace *store.AlreadyClosedException
	if !errors.As(err, &ace) {
		t.Fatalf("expected AlreadyClosedException, got %v", err)
	}

	// noop:
	if err := pr.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := dir1.Close(); err != nil {
		t.Fatalf("close dir1: %v", err)
	}
}

func TestParallelLeafReaderIncompatibleIndexes(t *testing.T) {
	// two documents:
	dir1 := parallelLeafReaderGetDir1(t)

	// one document only:
	dir2 := newDirectory()
	w2, err := NewIndexWriter(dir2, parallelLeafReaderConfig())
	if err != nil {
		t.Fatalf("new IndexWriter: %v", err)
	}
	d3 := document.NewDocument()
	parallelLeafReaderTextField(t, d3, "f3", "v1")
	if _, err := w2.AddDocument(d3); err != nil {
		t.Fatalf("addDocument: %v", err)
	}
	if err := w2.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	ir1 := parallelLeafReaderGetOnlyLeafReader(t, dir1)
	ir2 := parallelLeafReaderGetOnlyLeafReader(t, dir2)

	// indexes don't have the same number of documents
	if _, err := NewParallelLeafReader(ir1, ir2); err == nil {
		t.Fatal("expected IllegalArgumentException")
	}
	if _, err := NewParallelLeafReaderFull(rarely() || atLeast(1) > 1, []LeafReader{ir1, ir2}, []LeafReader{ir1, ir2}); err == nil {
		t.Fatal("expected IllegalArgumentException")
	}

	// check RefCounts
	if ir1.GetRefCount() != 1 || ir2.GetRefCount() != 1 {
		t.Fatalf("expected refCounts 1/1, got %d/%d", ir1.GetRefCount(), ir2.GetRefCount())
	}
	if err := ir1.Close(); err != nil {
		t.Fatalf("close ir1: %v", err)
	}
	if err := ir2.Close(); err != nil {
		t.Fatalf("close ir2: %v", err)
	}
	if err := dir1.Close(); err != nil {
		t.Fatalf("close dir1: %v", err)
	}
	if err := dir2.Close(); err != nil {
		t.Fatalf("close dir2: %v", err)
	}
}

func TestParallelLeafReaderIgnoreStoredFields(t *testing.T) {
	dir1 := parallelLeafReaderGetDir1(t)
	dir2 := parallelLeafReaderGetDir2(t)
	ir1 := parallelLeafReaderGetOnlyLeafReader(t, dir1)
	ir2 := parallelLeafReaderGetOnlyLeafReader(t, dir2)
	v1 := "v1"

	// with overlapping
	pr, err := NewParallelLeafReaderFull(false, []LeafReader{ir1, ir2}, []LeafReader{ir1})
	if err != nil {
		t.Fatalf("new ParallelLeafReader: %v", err)
	}
	parallelLeafReaderAssertStored(t, pr, "f1", &v1)
	parallelLeafReaderAssertStored(t, pr, "f2", &v1)
	parallelLeafReaderAssertStored(t, pr, "f3", nil)
	parallelLeafReaderAssertStored(t, pr, "f4", nil)
	// check that fields are there
	parallelLeafReaderAssertTerms(t, pr, "f1", true)
	parallelLeafReaderAssertTerms(t, pr, "f2", true)
	parallelLeafReaderAssertTerms(t, pr, "f3", true)
	parallelLeafReaderAssertTerms(t, pr, "f4", true)
	if err := pr.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// no stored fields at all
	pr, err = NewParallelLeafReaderFull(false, []LeafReader{ir2}, []LeafReader{})
	if err != nil {
		t.Fatalf("new ParallelLeafReader: %v", err)
	}
	parallelLeafReaderAssertStored(t, pr, "f1", nil)
	parallelLeafReaderAssertStored(t, pr, "f2", nil)
	parallelLeafReaderAssertStored(t, pr, "f3", nil)
	parallelLeafReaderAssertStored(t, pr, "f4", nil)
	// check that fields are there
	parallelLeafReaderAssertTerms(t, pr, "f1", false)
	parallelLeafReaderAssertTerms(t, pr, "f2", false)
	parallelLeafReaderAssertTerms(t, pr, "f3", true)
	parallelLeafReaderAssertTerms(t, pr, "f4", true)
	if err := pr.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// without overlapping
	pr, err = NewParallelLeafReaderFull(true, []LeafReader{ir2}, []LeafReader{ir1})
	if err != nil {
		t.Fatalf("new ParallelLeafReader: %v", err)
	}
	parallelLeafReaderAssertStored(t, pr, "f1", &v1)
	parallelLeafReaderAssertStored(t, pr, "f2", &v1)
	parallelLeafReaderAssertStored(t, pr, "f3", nil)
	parallelLeafReaderAssertStored(t, pr, "f4", nil)
	// check that fields are there
	parallelLeafReaderAssertTerms(t, pr, "f1", false)
	parallelLeafReaderAssertTerms(t, pr, "f2", false)
	parallelLeafReaderAssertTerms(t, pr, "f3", true)
	parallelLeafReaderAssertTerms(t, pr, "f4", true)
	if err := pr.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// no main readers
	if _, err := NewParallelLeafReaderFull(true, []LeafReader{}, []LeafReader{ir1}); err == nil {
		t.Fatal("expected IllegalArgumentException")
	}

	if err := dir1.Close(); err != nil {
		t.Fatalf("close dir1: %v", err)
	}
	if err := dir2.Close(); err != nil {
		t.Fatalf("close dir2: %v", err)
	}
}

func parallelLeafReaderWriteDir(t *testing.T, docs [][2][2]string) store.Directory {
	t.Helper()
	dir := newDirectory()
	conf := parallelLeafReaderConfig()
	conf.SetMergePolicy(NewLogDocMergePolicy())
	w, err := NewIndexWriter(dir, conf)
	if err != nil {
		t.Fatalf("new IndexWriter: %v", err)
	}
	for _, fields := range docs {
		d := document.NewDocument()
		for _, f := range fields {
			parallelLeafReaderTextField(t, d, f[0], f[1])
		}
		if _, err := w.AddDocument(d); err != nil {
			t.Fatalf("addDocument: %v", err)
		}
	}
	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return dir
}

func parallelLeafReaderGetDir1(t *testing.T) store.Directory {
	return parallelLeafReaderWriteDir(t, [][2][2]string{
		{{"f1", "v1"}, {"f2", "v1"}},
		{{"f1", "v2"}, {"f2", "v2"}},
	})
}

func parallelLeafReaderGetDir2(t *testing.T) store.Directory {
	return parallelLeafReaderWriteDir(t, [][2][2]string{
		{{"f3", "v1"}, {"f4", "v1"}},
		{{"f3", "v2"}, {"f4", "v2"}},
	})
}

// parallelLeafReaderSortedIndex writes two empty documents in two commits
// under the given index sort (nil for none) and force-merges them.
func parallelLeafReaderSortedIndex(t *testing.T, sortField string, twoCommits bool) (store.Directory, IndexReader) {
	t.Helper()
	dir := newDirectory()
	iwc := parallelLeafReaderConfig()
	if sortField != "" {
		iwc.SetIndexSort(NewSortFromFields([]SortField{*NewSortField(sortField, SortTypeInt)}))
	}
	w, err := NewIndexWriter(dir, iwc)
	if err != nil {
		t.Fatalf("new IndexWriter: %v", err)
	}
	if _, err := w.AddDocument(document.NewDocument()); err != nil {
		t.Fatalf("addDocument: %v", err)
	}
	if twoCommits {
		if _, err := w.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
	}
	if _, err := w.AddDocument(document.NewDocument()); err != nil {
		t.Fatalf("addDocument: %v", err)
	}
	if twoCommits {
		if err := w.ForceMerge(1); err != nil {
			t.Fatalf("forceMerge: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	r, err := OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("DirectoryReader.open: %v", err)
	}
	return dir, r
}

// TestParallelLeafReaderWithIndexSort1: not ok to have one leaf w/ index sort
// and another with a different index sort.
func TestParallelLeafReaderWithIndexSort1(t *testing.T) {
	dir1, r1 := parallelLeafReaderSortedIndex(t, "foo", true)
	dir2, r2 := parallelLeafReaderSortedIndex(t, "bar", true)

	_, err := NewParallelLeafReader(parallelLeafReaderOnlyLeaf(t, r1), parallelLeafReaderOnlyLeaf(t, r2))
	if err == nil {
		t.Fatal("expected IllegalArgumentException")
	}
	want := "cannot combine LeafReaders that have different index sorts: saw both sort=<int: \"foo\"> and <int: \"bar\">"
	if err.Error() != want {
		t.Fatalf("expected %q, got %q", want, err.Error())
	}
	for _, c := range []interface{ Close() error }{r1, dir1, r2, dir2} {
		if err := c.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
	}
}

// TestParallelLeafReaderWithIndexSort2: ok to have one leaf w/ index sort and
// the other with no sort.
func TestParallelLeafReaderWithIndexSort2(t *testing.T) {
	dir1, r1 := parallelLeafReaderSortedIndex(t, "foo", true)
	dir2, r2 := parallelLeafReaderSortedIndex(t, "", false)

	pr, err := NewParallelLeafReaderWithClose(false, parallelLeafReaderOnlyLeaf(t, r1), parallelLeafReaderOnlyLeaf(t, r2))
	if err != nil {
		t.Fatalf("new ParallelLeafReader: %v", err)
	}
	if err := pr.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	pr, err = NewParallelLeafReaderWithClose(false, parallelLeafReaderOnlyLeaf(t, r2), parallelLeafReaderOnlyLeaf(t, r1))
	if err != nil {
		t.Fatalf("new ParallelLeafReader: %v", err)
	}
	if err := pr.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	for _, c := range []interface{ Close() error }{r1, dir1, r2, dir2} {
		if err := c.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
	}
}

func TestParallelLeafReaderWithDocValuesUpdates(t *testing.T) {
	dir1 := newDirectory()
	w1, err := NewIndexWriter(dir1, parallelLeafReaderConfig())
	if err != nil {
		t.Fatalf("new IndexWriter: %v", err)
	}
	d := document.NewDocument()
	name, err := document.NewTextField("name", "billy", false)
	if err != nil {
		t.Fatalf("newTextField: %v", err)
	}
	d.Add(name)
	age, err := document.NewNumericDocValuesField("age", 21)
	if err != nil {
		t.Fatalf("new NumericDocValuesField: %v", err)
	}
	d.Add(age)
	if _, err := w1.AddDocument(d); err != nil {
		t.Fatalf("addDocument: %v", err)
	}
	if _, err := w1.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if _, err := w1.UpdateNumericDocValue(NewTerm("name", "billy"), "age", 22); err != nil {
		t.Fatalf("updateNumericDocValue: %v", err)
	}
	if err := w1.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	r1, err := OpenDirectoryReader(dir1)
	if err != nil {
		t.Fatalf("DirectoryReader.open: %v", err)
	}
	lr, err := NewParallelLeafReaderWithClose(false, parallelLeafReaderOnlyLeaf(t, r1))
	if err != nil {
		t.Fatalf("new ParallelLeafReader: %v", err)
	}

	dv, err := lr.GetNumericDocValues("age")
	if err != nil {
		t.Fatalf("getNumericDocValues: %v", err)
	}
	if dv == nil {
		t.Fatal("getNumericDocValues(\"age\") returned null")
	}
	doc, err := dv.NextDoc()
	if err != nil || doc != 0 {
		t.Fatalf("nextDoc: expected 0, got %d (%v)", doc, err)
	}
	v, err := dv.LongValue()
	if err != nil || v != 22 {
		t.Fatalf("longValue: expected 22, got %d (%v)", v, err)
	}

	if fi := lr.GetFieldInfos().FieldInfo("age"); fi == nil || fi.DocValuesGen() != 1 {
		t.Fatalf("expected docValuesGen 1, got %v", fi)
	}

	for _, c := range []interface{ Close() error }{lr, r1, dir1} {
		if err := c.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
	}
}
