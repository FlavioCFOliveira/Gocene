// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs_test

// Port of
// lucene/core/src/test/org/apache/lucene/codecs/lucene90/TestLucene90StoredFieldsFormatHighCompression.java
// (Apache Lucene 10.5.0). The class extends
// org.apache.lucene.tests.index.BaseStoredFieldsFormatTestCase (not ported);
// its inherited test methods are represented by one failing test naming it.
// testInvalidOptions renders the Java null Mode by an out-of-range enum value,
// the only Go value that is not a valid Mode.

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
)

func TestLucene90StoredFieldsFormatHighCompression_BaseStoredFieldsFormatTestCase(t *testing.T) {
	t.Fatal("requires org.apache.lucene.tests.index.BaseStoredFieldsFormatTestCase (not ported)")
}

// Change compression params (leaving it the same for old segments) and tests that nothing breaks.
func TestLucene90StoredFieldsFormatHighCompression_testMixedCompressions(t *testing.T) {
	r := luceneRandom(t)
	dir := luceneNewDirectory()
	modes := []codecs.Lucene104CodecMode{codecs.Lucene104CodecBestSpeed, codecs.Lucene104CodecBestCompression}
	for i := 0; i < 10; i++ {
		iwc := luceneNewIndexWriterConfig()
		iwc.SetCodec(codecs.NewLucene104CodecWithMode(modes[r.Intn(len(modes))]))
		iw, err := index.NewIndexWriter(dir, luceneNewIndexWriterConfig())
		if err != nil {
			t.Fatalf("new IndexWriter: %v", err)
		}
		doc := document.NewDocument()
		f1, err := document.NewStoredField("field1", "value1")
		if err != nil {
			t.Fatal(err)
		}
		doc.Add(f1)
		f2, err := document.NewStoredField("field2", "value2")
		if err != nil {
			t.Fatal(err)
		}
		doc.Add(f2)
		if _, err := iw.AddDocument(doc); err != nil {
			t.Fatalf("addDocument: %v", err)
		}
		if r.Intn(4) == 0 {
			if err := iw.ForceMerge(1); err != nil {
				t.Fatalf("forceMerge: %v", err)
			}
		}
		if _, err := iw.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
		if err := iw.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
	}

	ir, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if got := ir.NumDocs(); got != 10 {
		t.Fatalf("numDocs = %d, want 10", got)
	}
	storedFields, err := ir.StoredFields()
	if err != nil {
		t.Fatalf("storedFields: %v", err)
	}
	for i := 0; i < 10; i++ {
		doc := luceneStoredDocument(t, storedFields, i)
		if v, _ := luceneDocGet(doc, "field1"); v != "value1" {
			t.Fatalf("doc %d field1 = %q, want value1", i, v)
		}
		if v, _ := luceneDocGet(doc, "field2"); v != "value2" {
			t.Fatalf("doc %d field2 = %q, want value2", i, v)
		}
	}
	if err := ir.Close(); err != nil {
		t.Fatal(err)
	}
	// checkindex
	if err := dir.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestLucene90StoredFieldsFormatHighCompression_testInvalidOptions(t *testing.T) {
	luceneExpectPanic(t, func() {
		codecs.NewLucene104CodecWithMode(codecs.Lucene104CodecMode(-1))
	})

	luceneExpectPanic(t, func() {
		lucene90.NewLucene90StoredFieldsFormatWithMode(lucene90.Lucene90StoredFieldsMode(-1))
	})
}
