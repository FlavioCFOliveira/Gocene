// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestFieldInfos.java
// (Apache Lucene 10.5.0).

package index

import (
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/spi"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
)

func fieldInfosTestConfig() *IndexWriterConfig {
	return NewIndexWriterConfigWithAnalyzer(testanalysis.NewMockAnalyzer(testanalysis.WHITESPACE, true, 0, nil, true))
}

func fieldInfosAddStringField(t *testing.T, d *document.Document, name, value string) {
	t.Helper()
	f, err := document.NewStringField(name, value, true)
	if err != nil {
		t.Fatalf("new StringField: %v", err)
	}
	d.Add(f)
}

func fieldInfosAddField(t *testing.T, d *document.Document, name, value string, ft *document.FieldType) {
	t.Helper()
	f, err := document.NewField(name, value, ft)
	if err != nil {
		t.Fatalf("new Field: %v", err)
	}
	d.Add(f)
}

func TestFieldInfosFieldInfos(t *testing.T) {
	dir := newDirectory()
	iwc := fieldInfosTestConfig()
	iwc.SetMergePolicy(NewNoMergePolicy())
	writer, err := NewIndexWriter(dir, iwc)
	if err != nil {
		t.Fatalf("new IndexWriter: %v", err)
	}

	d1 := document.NewDocument()
	for i := 0; i < 15; i++ {
		fieldInfosAddStringField(t, d1, fmt.Sprintf("f%d", i), fmt.Sprintf("v%d", i))
	}
	if _, err := writer.AddDocument(d1); err != nil {
		t.Fatalf("addDocument: %v", err)
	}
	if _, err := writer.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	d2 := document.NewDocument()
	fieldInfosAddStringField(t, d2, "f0", "v0")
	fieldInfosAddStringField(t, d2, "f15", "v15")
	fieldInfosAddStringField(t, d2, "f16", "v16")
	if _, err := writer.AddDocument(d2); err != nil {
		t.Fatalf("addDocument: %v", err)
	}
	if _, err := writer.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	d3 := document.NewDocument()
	if _, err := writer.AddDocument(d3); err != nil {
		t.Fatalf("addDocument: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	sis, err := spi.ReadLatestCommit(dir)
	if err != nil {
		t.Fatalf("readLatestCommit: %v", err)
	}
	if sis.Size() != 3 {
		t.Fatalf("expected 3 segments, got %d", sis.Size())
	}

	fis1, err := readFieldInfos(sis.Get(0))
	if err != nil {
		t.Fatalf("readFieldInfos: %v", err)
	}
	fis2, err := readFieldInfos(sis.Get(1))
	if err != nil {
		t.Fatalf("readFieldInfos: %v", err)
	}
	fis3, err := readFieldInfos(sis.Get(2))
	if err != nil {
		t.Fatalf("readFieldInfos: %v", err)
	}

	// testing dense FieldInfos
	it := fis1.Iterator()
	i := 0
	for it.HasNext() {
		fi := it.Next()
		want := fmt.Sprintf("f%d", i)
		if fi.Number() != i {
			t.Fatalf("expected number %d, got %d", i, fi.Number())
		}
		if fi.Name() != want {
			t.Fatalf("expected %q, got %q", want, fi.Name())
		}
		if got := fis1.FieldInfoByNumber(i); got == nil || got.Name() != want { // lookup by number
			t.Fatalf("fieldInfo(%d): expected %q, got %v", i, want, got)
		}
		if got := fis1.FieldInfo(want); got == nil || got.Name() != want { // lookup by name
			t.Fatalf("fieldInfo(%q): expected %q, got %v", want, want, got)
		}
		i++
	}

	// testing sparse FieldInfos
	fieldInfosAssertName(t, fis2.FieldInfoByNumber(0), "f0") // lookup by number
	fieldInfosAssertName(t, fis2.FieldInfo("f0"), "f0")      // lookup by name
	if fis2.FieldInfoByNumber(1) != nil {
		t.Fatal("assertNull(fis2.fieldInfo(1))")
	}
	if fis2.FieldInfo("f1") != nil {
		t.Fatal("assertNull(fis2.fieldInfo(\"f1\"))")
	}
	fieldInfosAssertName(t, fis2.FieldInfoByNumber(15), "f15")
	fieldInfosAssertName(t, fis2.FieldInfo("f15"), "f15")
	fieldInfosAssertName(t, fis2.FieldInfoByNumber(16), "f16")
	fieldInfosAssertName(t, fis2.FieldInfo("f16"), "f16")

	// testing empty FieldInfos
	if fis3.FieldInfoByNumber(0) != nil { // lookup by number
		t.Fatal("assertNull(fis3.fieldInfo(0))")
	}
	if fis3.FieldInfo("f0") != nil { // lookup by name
		t.Fatal("assertNull(fis3.fieldInfo(\"f0\"))")
	}
	if fis3.Size() != 0 {
		t.Fatalf("expected 0, got %d", fis3.Size())
	}
	if fis3.Iterator().HasNext() {
		t.Fatal("assertFalse(it3.hasNext())")
	}
	if err := dir.Close(); err != nil {
		t.Fatalf("close dir: %v", err)
	}
}

func fieldInfosAssertName(t *testing.T, fi *FieldInfo, want string) {
	t.Helper()
	if fi == nil || fi.Name() != want {
		t.Fatalf("expected field %q, got %v", want, fi)
	}
}

func TestFieldInfosFieldAttributes(t *testing.T) {
	dir := newDirectory()
	iwc := fieldInfosTestConfig()
	iwc.SetMergePolicy(NewNoMergePolicy())
	writer, err := NewIndexWriter(dir, iwc)
	if err != nil {
		t.Fatalf("new IndexWriter: %v", err)
	}

	type1 := document.NewFieldType()
	type1.SetStored(true)
	type1.PutAttribute("testKey1", "testValue1")

	d1 := document.NewDocument()
	fieldInfosAddField(t, d1, "f1", "v1", type1)
	type2 := document.NewFieldTypeFrom(type1)
	// changing the value after copying shouldn't impact the original type1
	type2.PutAttribute("testKey1", "testValue2")
	if _, err := writer.AddDocument(d1); err != nil {
		t.Fatalf("addDocument: %v", err)
	}
	if _, err := writer.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	d2 := document.NewDocument()
	type1.PutAttribute("testKey1", "testValueX")
	type1.PutAttribute("testKey2", "testValue2")
	fieldInfosAddField(t, d2, "f1", "v2", type1)
	fieldInfosAddField(t, d2, "f2", "v2", type2)
	if _, err := writer.AddDocument(d2); err != nil {
		t.Fatalf("addDocument: %v", err)
	}
	if _, err := writer.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}

	reader, err := OpenDirectoryReaderFromWriter(writer)
	if err != nil {
		t.Fatalf("DirectoryReader.open(writer): %v", err)
	}
	fis, err := spi.GetMergedFieldInfos(reader)
	if err != nil {
		t.Fatalf("getMergedFieldInfos: %v", err)
	}
	if fis.Size() != 2 {
		t.Fatalf("expected 2, got %d", fis.Size())
	}
	it := fis.Iterator()
	for it.HasNext() {
		fi := it.Next()
		switch fi.Name() {
		case "f1":
			// testKey1 can point to either testValue1 or testValueX based on the order
			// of merge, but we see textValueX winning here since segment_2 is merged on segment_1.
			if got := fi.GetAttribute("testKey1"); got != "testValueX" {
				t.Fatalf("f1 testKey1: expected testValueX, got %q", got)
			}
			if got := fi.GetAttribute("testKey2"); got != "testValue2" {
				t.Fatalf("f1 testKey2: expected testValue2, got %q", got)
			}
		case "f2":
			if got := fi.GetAttribute("testKey1"); got != "testValue2" {
				t.Fatalf("f2 testKey1: expected testValue2, got %q", got)
			}
		default:
			t.Fatal("Unknown field")
		}
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close reader: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	if err := dir.Close(); err != nil {
		t.Fatalf("close dir: %v", err)
	}
}

func TestFieldInfosFieldAttributesSingleSegment(t *testing.T) {
	dir := newDirectory()
	iwc := fieldInfosTestConfig()
	iwc.SetMergePolicy(NewNoMergePolicy())
	writer, err := NewIndexWriter(dir, iwc)
	if err != nil {
		t.Fatalf("new IndexWriter: %v", err)
	}

	d1 := document.NewDocument()
	type1 := document.NewFieldType()
	type1.SetStored(true)
	type1.PutAttribute("att1", "attdoc1")
	fieldInfosAddField(t, d1, "f1", "v1", type1)
	// add field with the same name and an extra attribute
	type1.PutAttribute("att2", "attdoc1")
	fieldInfosAddField(t, d1, "f1", "v1", type1)
	if _, err := writer.AddDocument(d1); err != nil {
		t.Fatalf("addDocument: %v", err)
	}

	d2 := document.NewDocument()
	type1.PutAttribute("att1", "attdoc2")
	type1.PutAttribute("att2", "attdoc2")
	type1.PutAttribute("att3", "attdoc2")
	type2 := document.NewFieldType()
	type2.SetStored(true)
	type2.PutAttribute("att4", "attdoc2")
	fieldInfosAddField(t, d2, "f1", "v2", type1)
	fieldInfosAddField(t, d2, "f2", "v2", type2)
	if _, err := writer.AddDocument(d2); err != nil {
		t.Fatalf("addDocument: %v", err)
	}
	if _, err := writer.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	reader, err := OpenDirectoryReaderFromWriter(writer)
	if err != nil {
		t.Fatalf("DirectoryReader.open(writer): %v", err)
	}
	fis, err := spi.GetMergedFieldInfos(reader)
	if err != nil {
		t.Fatalf("getMergedFieldInfos: %v", err)
	}

	// test that attributes for f1 are introduced by d1,
	// and not modified by d2
	fi1 := fis.FieldInfo("f1")
	if got := fi1.GetAttribute("att1"); got != "attdoc1" {
		t.Fatalf("att1: expected attdoc1, got %q", got)
	}
	if got := fi1.GetAttribute("att2"); got != "attdoc1" {
		t.Fatalf("att2: expected attdoc1, got %q", got)
	}
	if got := fi1.GetAttribute("att3"); got != "" {
		t.Fatalf("assertNull(fi1.getAttribute(\"att3\")): got %q", got)
	}

	// test that attributes for f2 are introduced by d2
	fi2 := fis.FieldInfo("f2")
	if got := fi2.GetAttribute("att4"); got != "attdoc2" {
		t.Fatalf("att4: expected attdoc2, got %q", got)
	}

	if err := reader.Close(); err != nil {
		t.Fatalf("close reader: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	if err := dir.Close(); err != nil {
		t.Fatalf("close dir: %v", err)
	}
}

func TestFieldInfosMergedFieldInfosEmpty(t *testing.T) {
	dir := newDirectory()
	writer, err := NewIndexWriter(dir, fieldInfosTestConfig())
	if err != nil {
		t.Fatalf("new IndexWriter: %v", err)
	}

	reader, err := OpenDirectoryReaderFromWriter(writer)
	if err != nil {
		t.Fatalf("DirectoryReader.open(writer): %v", err)
	}
	actual, err := spi.GetMergedFieldInfos(reader)
	if err != nil {
		t.Fatalf("getMergedFieldInfos: %v", err)
	}

	if actual != EmptyFieldInfos {
		t.Fatal("assertSame(FieldInfos.EMPTY, actual)")
	}

	if err := reader.Close(); err != nil {
		t.Fatalf("close reader: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	if err := dir.Close(); err != nil {
		t.Fatalf("close dir: %v", err)
	}
}

func TestFieldInfosMergedFieldInfosSingleLeaf(t *testing.T) {
	dir := newDirectory()
	writer, err := NewIndexWriter(dir, fieldInfosTestConfig())
	if err != nil {
		t.Fatalf("new IndexWriter: %v", err)
	}

	d1 := document.NewDocument()
	fieldInfosAddStringField(t, d1, "f1", "v1")
	if _, err := writer.AddDocument(d1); err != nil {
		t.Fatalf("addDocument: %v", err)
	}
	if _, err := writer.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	d2 := document.NewDocument()
	fieldInfosAddStringField(t, d2, "f2", "v2")
	if _, err := writer.AddDocument(d2); err != nil {
		t.Fatalf("addDocument: %v", err)
	}
	if _, err := writer.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}

	reader, err := OpenDirectoryReaderFromWriter(writer)
	if err != nil {
		t.Fatalf("DirectoryReader.open(writer): %v", err)
	}
	actual, err := spi.GetMergedFieldInfos(reader)
	if err != nil {
		t.Fatalf("getMergedFieldInfos: %v", err)
	}
	leaves, err := reader.Leaves()
	if err != nil {
		t.Fatalf("leaves: %v", err)
	}
	if len(leaves) != 1 {
		t.Fatalf("expected 1 leaf, got %d", len(leaves))
	}
	expected := leaves[0].LeafReader().GetFieldInfos()
	if expected != actual {
		t.Fatal("assertSame(expected, actual)")
	}

	if err := reader.Close(); err != nil {
		t.Fatalf("close reader: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	if err := dir.Close(); err != nil {
		t.Fatalf("close dir: %v", err)
	}
}

// fieldInfosNoneFieldInfo renders
// new FieldInfo(name, -1, false, false, false, IndexOptions.NONE,
// DocValuesType.NONE, DocValuesSkipIndexType.NONE, -1, new HashMap<>(), 0, 0,
// 0, 0, VectorEncoding.FLOAT32, VectorSimilarityFunction.EUCLIDEAN, false,
// false).
func fieldInfosNoneFieldInfo(name string) *FieldInfo {
	return NewFieldInfo(name, -1, DefaultFieldInfoOptions())
}

func TestFieldInfosFieldNumbersAutoIncrement(t *testing.T) {
	fieldNumbers := NewFieldNumbers("softDeletes", "parentDoc")
	for i := 0; i < 10; i++ {
		fieldNumbers.AddOrGet(fieldInfosNoneFieldInfo(fmt.Sprintf("field%d", i)))
	}
	idx := fieldNumbers.AddOrGet(fieldInfosNoneFieldInfo("EleventhField"))
	if idx != 10 {
		t.Fatalf("Field numbers 0 through 9 were allocated: expected 10, got %d", idx)
	}

	fieldNumbers.Clear()
	idx = fieldNumbers.AddOrGet(fieldInfosNoneFieldInfo("PostClearField"))
	if idx != 0 {
		t.Fatalf("Field numbers should reset after clear(): expected 0, got %d", idx)
	}
}
