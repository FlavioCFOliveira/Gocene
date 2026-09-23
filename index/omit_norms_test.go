// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/index/TestOmitNorms.java
// (Apache Lucene 10.5.0).

package index_test

import (
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// omitNormsTextType renders new FieldType(TextField.TYPE_NOT_STORED) with the
// given omitNorms and storeTermVectors=false.
func omitNormsTextType(omitNorms bool) *document.FieldType {
	ft := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	ft.SetOmitNorms(omitNorms)
	ft.SetStoreTermVectors(false)
	return ft
}

// TestOmitNormsMixedMergeThrowsError tests that merging of docs with different
// omitNorms throws error.
func TestOmitNormsMixedMergeThrowsError(t *testing.T) {
	ram := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMaxBufferedDocs(3)
	iwc.SetMergePolicy(newLogMergePolicyWithMergeFactor(2))
	writer := mustNewIndexWriter(t, ram, iwc)
	d := document.NewDocument()

	// this field will have norms
	fieldType1 := omitNormsTextType(false)
	d.Add(newField(t, "f1", "This field has norms", fieldType1))

	// this field will NOT have norms
	fieldType2 := omitNormsTextType(true)
	d.Add(newField(t, "f2", "This field has NO norms in all docs", fieldType2))

	for i := 0; i < 30; i++ {
		mustAddDocument(t, writer, d)
	}

	// reverse omitNorms options for f1 and f2
	d2 := document.NewDocument()
	d2.Add(newField(t, "f1", "This field has NO norms", fieldType2))
	d2.Add(newField(t, "f2", "This field has norms", fieldType1))

	_, err := writer.AddDocument(d2)
	if err == nil {
		t.Fatal("expected IllegalArgumentException")
	}
	if want := "cannot change field \"f1\" from omitNorms=false to inconsistent omitNorms=true"; err.Error() != want {
		t.Fatalf("expected %q, got %q", want, err.Error())
	}

	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	mustClose(t, writer)

	reader := getOnlyLeafReader(t, mustOpenDirectoryReader(t, ram))
	fi := reader.GetFieldInfos()
	// assert original omitNorms
	if fi.FieldInfo("f1").OmitNorms() {
		t.Fatal("OmitNorms field bit must not be set.")
	}
	if !fi.FieldInfo("f2").OmitNorms() {
		t.Fatal("OmitNorms field bit must be set.")
	}

	mustClose(t, reader, ram)
}

// TestOmitNormsMixedRAM makes sure first adding docs that do not omitNorms for
// field X, then adding docs that do omitNorms for that same field.
func TestOmitNormsMixedRAM(t *testing.T) {
	ram := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMaxBufferedDocs(10)
	iwc.SetMergePolicy(newLogMergePolicyWithMergeFactor(2))
	writer := mustNewIndexWriter(t, ram, iwc)
	d := document.NewDocument()

	// this field will have norms
	d.Add(newTextField(t, "f1", "This field has norms", false))

	// this field will NOT have norms
	customType := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	customType.SetOmitNorms(true)
	d.Add(newField(t, "f2", "This field has NO norms in all docs", customType))

	for i := 0; i < 5; i++ {
		mustAddDocument(t, writer, d)
	}
	for i := 0; i < 20; i++ {
		mustAddDocument(t, writer, d)
	}

	// force merge
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}

	// flush
	mustClose(t, writer)

	reader := getOnlyLeafReader(t, mustOpenDirectoryReader(t, ram))
	fi := reader.GetFieldInfos()
	if fi.FieldInfo("f1").OmitNorms() {
		t.Fatal("OmitNorms field bit should not be set.")
	}
	if !fi.FieldInfo("f2").OmitNorms() {
		t.Fatal("OmitNorms field bit should be set.")
	}

	mustClose(t, reader, ram)
}

func omitNormsAssertNoNrm(t *testing.T, dir store.Directory) {
	t.Helper()
	files, err := dir.ListAll()
	if err != nil {
		t.Fatalf("listAll: %v", err)
	}
	for _, f := range files {
		// TODO: this relies upon filenames
		if strings.HasSuffix(f, ".nrm") || strings.HasSuffix(f, ".len") {
			t.Fatalf("unexpected norms file %s", f)
		}
	}
}

// TestOmitNormsNoNrmFile verifies no *.nrm exists when all fields omit norms.
func TestOmitNormsNoNrmFile(t *testing.T) {
	ram := newDirectory()
	iwc := newIndexWriterConfigWithAnalyzer(newMockAnalyzer())
	iwc.SetMaxBufferedDocs(3)
	iwc.SetMergePolicy(newLogMergePolicy())
	writer := mustNewIndexWriter(t, ram, iwc)
	lmp := writer.GetConfig().GetMergePolicy().(logMergePolicy)
	lmp.SetMergeFactor(2)
	lmp.SetNoCFSRatio(0.0)
	d := document.NewDocument()

	customType := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	customType.SetOmitNorms(true)
	d.Add(newField(t, "f1", "This field has no norms", customType))

	for i := 0; i < 30; i++ {
		mustAddDocument(t, writer, d)
	}

	mustCommit(t, writer)

	omitNormsAssertNoNrm(t, ram)

	// force merge
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("forceMerge: %v", err)
	}
	// flush
	mustClose(t, writer)

	omitNormsAssertNoNrm(t, ram)
	mustClose(t, ram)
}
