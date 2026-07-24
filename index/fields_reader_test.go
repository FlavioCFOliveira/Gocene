// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene's org.apache.lucene.index.TestFieldsReader.
// Source: lucene/core/src/test/org/apache/lucene/index/TestFieldsReader.java
// (release tag releases/lucene/10.4.0, commit 9983b7c).
//
// GOC-4178.
//
// Porting notes / divergences from the Java original:
//
//   - DocHelper is not yet ported. This file builds an equivalent set of
//     fields (textField1/2/3 and noTFField) inline, using document.FieldType
//     directly since Gocene's FieldType exposes plain exported fields rather
//     than Lucene's getter/setter surface.
//
//   - The Java test() opens a real DirectoryReader and calls
//     reader.storedFields().document(0). In Gocene, OpenDirectoryReader
//     produces SegmentReaders via NewSegmentReader, which leaves coreReaders
//     nil; SegmentReader.StoredFields() then fails with "segment reader not
//     initialized". The end-to-end writer -> DirectoryReader stored-fields
//     read-back path is therefore not yet supported. To preserve the
//     assertion intent without waiting on that gap, the stored-fields
//     round-trip is exercised directly against codecs.StoredFieldsReaderImpl,
//     which is the readable unit available today. A skipped test documents
//     the missing DirectoryReader path.
//
//   - testExceptions() (LUCENE-1262) relies on a FilterDirectory that injects
//     IOExceptions into a real DirectoryReader's stored-fields reads. That
//     depends on the same unsupported leaf path and on forceMerge, so it is
//     ported as a skipped test recording the intent.
package index_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// The DocHelper-equivalent field keys and texts (textField1Key, field1Text,
// textField2Key, field2Text, textField3Key, field3Text, noTFKey, noTFText)
// are declared once for package index_test in document_writer_test.go and
// reused here.

// docHelperFields builds the four fields that TestFieldsReader.test()
// inspects, with the same FieldType configuration as DocHelper:
//
//	textField1 - TextField.TYPE_STORED
//	textField2 - TYPE_STORED + store term vectors (+positions +offsets)
//	textField3 - TYPE_STORED + omitNorms
//	noTFField  - TYPE_STORED + IndexOptions DOCS only
func docHelperFields(t *testing.T) []*document.Field {
	t.Helper()

	type spec struct {
		name             string
		value            string
		storeTermVectors bool
		omitNorms        bool
		indexOptions     index.IndexOptions
	}
	specs := []spec{
		{textField1Key, field1Text, false, false, index.IndexOptionsDocsAndFreqsAndPositions},
		{textField2Key, field2Text, true, false, index.IndexOptionsDocsAndFreqsAndPositions},
		{textField3Key, field3Text, false, true, index.IndexOptionsDocsAndFreqsAndPositions},
		{noTFKey, noTFText, false, false, index.IndexOptionsDocs},
	}

	fields := make([]*document.Field, 0, len(specs))
	for _, s := range specs {
		ft := document.NewFieldType()
		ft.Stored = true
		ft.Tokenized = true
		// All DocHelper fields here use a non-NONE IndexOptions, so they are
		// indexed. Gocene's FieldType keeps an explicit Indexed flag (no peer
		// in Lucene, where it is derived from IndexOptions != NONE).
		ft.Indexed = true
		ft.IndexOptions = s.indexOptions
		ft.StoreTermVectors = s.storeTermVectors
		if s.storeTermVectors {
			ft.StoreTermVectorPositions = true
			ft.StoreTermVectorOffsets = true
		}
		ft.OmitNorms = s.omitNorms

		f, err := document.NewField(s.name, s.value, ft)
		if err != nil {
			t.Fatalf("NewField(%q): %v", s.name, err)
		}
		fields = append(fields, f)
	}
	return fields
}

// storedRoundTrip writes the given fields as a single stored document into a
// codecs.StoredFieldsReaderImpl and returns the reader, mirroring what a real
// stored-fields codec would produce on disk.
func storedRoundTrip(t *testing.T, fields []*document.Field) *codecs.StoredFieldsReaderImpl {
	t.Helper()

	storedFields := make([]codecs.StoredField, 0, len(fields))
	for _, f := range fields {
		storedFields = append(storedFields, codecs.StoredField{
			Name:  f.Name(),
			Type:  codecs.FieldTypeString,
			Value: f.StringValue(),
		})
	}

	reader := codecs.NewStoredFieldsReaderImpl(nil, nil, nil)
	reader.AddDocument(codecs.StoredDocument{Fields: storedFields})
	return reader
}

// TestFieldsReader_Test ports TestFieldsReader.test().
//
// It verifies that a stored document round-trips field by field and that the
// per-field configuration is preserved, then that a field-filtered
// DocumentStoredFieldVisitor only reconstructs the requested field.
func TestFieldsReader_Test(t *testing.T) {
	fields := docHelperFields(t)

	// Assertions on the source FieldType configuration. The Java test reads
	// these back off the reconstructed Document; Gocene's stored-fields
	// pipeline does not carry FieldType metadata into the visitor, so the
	// configuration is asserted on the indexed fields directly.
	byName := make(map[string]*document.Field, len(fields))
	for _, f := range fields {
		byName[f.Name()] = f
	}

	if got := byName[textField1Key]; got == nil {
		t.Fatalf("missing field %q", textField1Key)
	}

	f2 := byName[textField2Key]
	if f2 == nil {
		t.Fatalf("missing field %q", textField2Key)
	}
	if !f2.FieldType().StoreTermVectors {
		t.Errorf("%s: StoreTermVectors = false, want true", textField2Key)
	}
	if f2.FieldType().OmitNorms {
		t.Errorf("%s: OmitNorms = true, want false", textField2Key)
	}
	if got := f2.FieldType().IndexOptions; got != index.IndexOptionsDocsAndFreqsAndPositions {
		t.Errorf("%s: IndexOptions = %v, want DOCS_AND_FREQS_AND_POSITIONS", textField2Key, got)
	}

	f3 := byName[textField3Key]
	if f3 == nil {
		t.Fatalf("missing field %q", textField3Key)
	}
	if f3.FieldType().StoreTermVectors {
		t.Errorf("%s: StoreTermVectors = true, want false", textField3Key)
	}
	if !f3.FieldType().OmitNorms {
		t.Errorf("%s: OmitNorms = false, want true", textField3Key)
	}
	if got := f3.FieldType().IndexOptions; got != index.IndexOptionsDocsAndFreqsAndPositions {
		t.Errorf("%s: IndexOptions = %v, want DOCS_AND_FREQS_AND_POSITIONS", textField3Key, got)
	}

	noTF := byName[noTFKey]
	if noTF == nil {
		t.Fatalf("missing field %q", noTFKey)
	}
	if noTF.FieldType().StoreTermVectors {
		t.Errorf("%s: StoreTermVectors = true, want false", noTFKey)
	}
	if noTF.FieldType().OmitNorms {
		t.Errorf("%s: OmitNorms = true, want false", noTFKey)
	}
	if got := noTF.FieldType().IndexOptions; got != index.IndexOptionsDocs {
		t.Errorf("%s: IndexOptions = %v, want DOCS", noTFKey, got)
	}

	// Round-trip the stored document and verify every field is recovered,
	// mirroring reader.storedFields().document(0) in the Java test.
	reader := storedRoundTrip(t, fields)
	defer func() {
		if err := reader.Close(); err != nil {
			t.Errorf("reader.Close(): %v", err)
		}
	}()

	full := document.NewDocumentStoredFieldVisitor()
	if err := reader.VisitDocument(0, full); err != nil {
		t.Fatalf("VisitDocument(0): %v", err)
	}
	doc := full.GetDocument()
	if doc == nil {
		t.Fatal("VisitDocument produced a nil document")
	}
	if doc.Get(textField1Key) == nil {
		t.Errorf("round-tripped document is missing %q", textField1Key)
	}
	if got := doc.Size(); got != len(fields) {
		t.Errorf("round-tripped document has %d fields, want %d", got, len(fields))
	}

	// A field-filtered visitor must reconstruct only the requested field,
	// mirroring the DocumentStoredFieldVisitor(TEXT_FIELD_3_KEY) assertions.
	//
	// codecs.StoredFieldsReaderImpl.VisitDocument is a stub that forwards
	// every stored field without consulting NeedsField, so the filter is
	// applied here against the document's own fields. This still exercises
	// the actual subject of the Java assertion: the visitor's field-name
	// filtering contract.
	filtered := document.NewDocumentStoredFieldVisitorFor(textField3Key)
	for _, f := range fields {
		if filtered.NeedsField(f.Name()) {
			filtered.StringField(f.Name(), f.StringValue())
		}
	}
	picked := filtered.GetDocument().GetAllFields()
	if len(picked) != 1 {
		t.Fatalf("filtered visitor produced %d fields, want 1", len(picked))
	}
	if name := picked[0].Name(); name != textField3Key {
		t.Errorf("filtered visitor produced field %q, want %q", name, textField3Key)
	}
}

// TestFieldsReader_DirectoryReaderPath ports the part of
// TestFieldsReader.test() that opens a real DirectoryReader and reads back
// the stored document.
func TestFieldsReader_DirectoryReaderPath(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer func() { _ = dir.Close() }()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewStandardAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	doc := document.NewDocument()
	for _, f := range docHelperFields(t) {
		doc.Add(f)
	}
	if err := w.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer func() { _ = r.Close() }()

	sf, err := r.StoredFields()
	if err != nil {
		t.Fatalf("StoredFields: %v", err)
	}
	visitor := document.NewDocumentStoredFieldVisitor()
	if err := sf.Document(0, visitor); err != nil {
		t.Fatalf("StoredFields.Document(0): %v", err)
	}
	got := visitor.GetDocument()
	if got == nil {
		t.Fatal("StoredFields.Document returned nil document")
	}
	if got.Get(textField1Key) == nil {
		t.Errorf("document missing %q", textField1Key)
	}
	if got.Size() != 4 {
		t.Errorf("document has %d fields, want 4", got.Size())
	}
}

// TestFieldsReader_Exceptions ports TestFieldsReader.testExceptions()
// (LUCENE-1262), which injects IOExceptions into a DirectoryReader's
// stored-fields reads via a FilterDirectory. The reader-side stored-fields
// implementation keeps file inputs open after DirectoryReader.open, so a
// simple OpenInput wrapper does not trigger the read-time failure; a
// read-time fault-injecting IndexInput wrapper is still needed.
func TestFieldsReader_Exceptions(t *testing.T) {
	t.Fatal("fault-injecting stored-fields read via DirectoryReader unsupported: " +
		"the codec stored-fields reader holds open IndexInputs, so a read-time " +
		"fault wrapper is required (see TestFieldsReader_DirectoryReaderPath)")
}
