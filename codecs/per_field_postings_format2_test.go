// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs_test

// Port of
// lucene/core/src/test/org/apache/lucene/codecs/perfield/TestPerFieldPostingsFormat2.java
// (Apache Lucene 10.5.0).
//
// Blockers: testMergeUnusedPerFieldCodec and testChangeCodecAndMerge use the
// nested MockCodec, which extends org.apache.lucene.tests.codecs.asserting.AssertingCodec
// (not ported); testChangeCodecAndMerge also searches through
// LuceneTestCase.newSearcher (org.apache.lucene.tests.search.AssertingIndexSearcher).
// testSameCodecDifferentInstance, testSameCodecDifferentParams and
// testMergeCalledOnTwoFormats configure anonymous AssertingCodec subclasses
// (testSameCodecDifferentParams also needs the test-framework
// LuceneVarGapFixedInterval postings format). testStressPerFieldCodec runs in
// full. LuceneTestCase.newField(String, String, FieldType) is rendered without
// its random field-type variation, as elsewhere in the Gocene test ports.

import (
	"fmt"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// perFieldPostingsFormat2NewWriter renders TestPerFieldPostingsFormat2.newWriter.
func perFieldPostingsFormat2NewWriter(t *testing.T, dir store.Directory, conf *index.IndexWriterConfig) *index.IndexWriter {
	t.Helper()
	logByteSizeMergePolicy := index.NewLogDocMergePolicy()
	logByteSizeMergePolicy.SetNoCFSRatio(0.0) // make sure we use plain
	// files
	conf.SetMergePolicy(logByteSizeMergePolicy)

	writer, err := index.NewIndexWriter(dir, conf)
	if err != nil {
		t.Fatalf("new IndexWriter: %v", err)
	}
	return writer
}

// Test that heterogeneous index segments are merge successfully
func TestPerFieldPostingsFormat2_testMergeUnusedPerFieldCodec(t *testing.T) {
	t.Fatal("requires TestPerFieldPostingsFormat2.MockCodec extending " +
		"org.apache.lucene.tests.codecs.asserting.AssertingCodec (not ported)")
}

// Test that heterogeneous index segments are merged sucessfully
func TestPerFieldPostingsFormat2_testChangeCodecAndMerge(t *testing.T) {
	t.Fatal("requires TestPerFieldPostingsFormat2.MockCodec extending " +
		"org.apache.lucene.tests.codecs.asserting.AssertingCodec and " +
		"org.apache.lucene.tests.search.AssertingIndexSearcher (not ported)")
}

// Test per field codec support - adding fields with random codecs
func TestPerFieldPostingsFormat2_testStressPerFieldCodec(t *testing.T) {
	r := luceneRandom(t)
	dir := luceneNewDirectory()
	const docsPerRound = 97
	numRounds := luceneAtLeast(r, 1)
	for i := 0; i < numRounds; i++ {
		num := luceneNextInt(r, 30, 60)
		config := luceneNewIndexWriterConfig()
		config.SetOpenMode(index.CreateOrAppend)
		writer := perFieldPostingsFormat2NewWriter(t, dir, config)
		for j := 0; j < docsPerRound; j++ {
			doc := document.NewDocument()
			for k := 0; k < num; k++ {
				customType := document.NewFieldTypeFrom(document.TextFieldTYPENOTSTORED)
				customType.SetTokenized(r.Intn(2) == 0)
				customType.SetOmitNorms(r.Intn(2) == 0)
				field, err := document.NewField(fmt.Sprintf("%d", k), util.RandomRealisticUnicodeString(r, 0, 128), customType)
				if err != nil {
					t.Fatal(err)
				}
				doc.Add(field)
			}
			if _, err := writer.AddDocument(doc); err != nil {
				t.Fatalf("addDocument: %v", err)
			}
		}
		if r.Intn(2) == 0 {
			if err := writer.ForceMerge(1); err != nil {
				t.Fatalf("forceMerge: %v", err)
			}
		}
		if _, err := writer.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
		stats, err := writer.GetDocStats()
		if err != nil {
			t.Fatal(err)
		}
		if stats.MaxDoc != (i+1)*docsPerRound {
			t.Fatalf("maxDoc = %d, want %d", stats.MaxDoc, (i+1)*docsPerRound)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if err := dir.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPerFieldPostingsFormat2_testSameCodecDifferentInstance(t *testing.T) {
	t.Fatal("requires an anonymous org.apache.lucene.tests.codecs.asserting.AssertingCodec subclass (not ported)")
}

func TestPerFieldPostingsFormat2_testSameCodecDifferentParams(t *testing.T) {
	t.Fatal("requires an anonymous org.apache.lucene.tests.codecs.asserting.AssertingCodec subclass and " +
		"org.apache.lucene.tests.codecs.blockterms.LuceneVarGapFixedInterval (not ported)")
}

func TestPerFieldPostingsFormat2_testMergeCalledOnTwoFormats(t *testing.T) {
	t.Fatal("requires an anonymous org.apache.lucene.tests.codecs.asserting.AssertingCodec subclass (not ported)")
}
