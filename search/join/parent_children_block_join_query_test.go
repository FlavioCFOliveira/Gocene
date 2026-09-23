// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// Port of
// lucene/join/src/test/org/apache/lucene/search/join/TestParentChildrenBlockJoinQuery.java
// (Apache Lucene 10.5.0).

func TestParentChildrenBlockJoinQuery(t *testing.T) {
	numParentDocs := 8 + random().Intn(8)
	maxChildDocsPerParent := 8 + random().Intn(8)

	dir := newDirectory()
	iwc := newIndexWriterConfig()
	iwc.SetMergePolicy(newMergePolicyNoMock(t))
	writer := newRandomIndexWriterWithConfig(t, dir, iwc)
	for i := 0; i < numParentDocs; i++ {
		numChildDocs := random().Intn(maxChildDocsPerParent)
		docs := make([]*document.Document, 0, numChildDocs+1)
		for j := 0; j < numChildDocs; j++ {
			childDoc := newTestDocument(
				newStringField(t, "type", "child", false),
				mustNumericDVField(t, "score", int64(j+1)))
			docs = append(docs, childDoc)
		}

		parenDoc := newTestDocument(
			newStringField(t, "type", "parent", false),
			mustNumericDVField(t, "num_child_docs", int64(numChildDocs)))
		docs = append(docs, parenDoc)
		if _, err := writer.AddDocuments(docs); err != nil {
			t.Fatalf("addDocuments: %v", err)
		}
	}

	reader := mustGetReader(t, writer)
	mustClose(t, writer)

	searcher := newSearcher(t, reader)
	parentFilter := NewQueryBitSetProducer(search.NewTermQuery(index.NewTerm("type", "parent")))
	childQuery := search.NewBooleanQueryBuilder().
		Add(search.NewTermQuery(index.NewTerm("type", "child")), search.FILTER).
		Add(numericDocValuesScoreQuery("score"), search.SHOULD).
		Build()

	parentDocs := mustSearch(t, searcher, search.NewTermQuery(index.NewTerm("type", "parent")), numParentDocs)
	if len(parentDocs.ScoreDocs) != numParentDocs {
		t.Fatalf("parent docs: expected %d, got %d", numParentDocs, len(parentDocs.ScoreDocs))
	}
	leaves := mustLeaves(t, reader)
	for _, parentScoreDoc := range parentDocs.ScoreDocs {
		leafReader := leaves[index.ReaderUtilSubIndexLeaves(parentScoreDoc.Doc, leaves)]
		numericDocValuesField, err := leafReader.LeafReader().GetNumericDocValues("num_child_docs")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := numericDocValuesField.Advance(parentScoreDoc.Doc - leafReader.DocBase); err != nil {
			t.Fatal(err)
		}
		expectedChildDocs, err := numericDocValuesField.LongValue()
		if err != nil {
			t.Fatal(err)
		}

		parentChildrenBlockJoinQuery := NewParentChildrenBlockJoinQuery(parentFilter, childQuery, parentScoreDoc.Doc)
		topDocs := mustSearch(t, searcher, parentChildrenBlockJoinQuery, maxChildDocsPerParent)
		if topDocs.TotalHits.Value != expectedChildDocs {
			t.Fatalf("totalHits: expected %d, got %d", expectedChildDocs, topDocs.TotalHits.Value)
		}
		if expectedChildDocs > 0 {
			for i, childScoreDoc := range topDocs.ScoreDocs {
				if want := float32(expectedChildDocs) - float32(i); childScoreDoc.Score != want {
					t.Fatalf("child %d score: expected %v, got %v", i, want, childScoreDoc.Score)
				}
			}
		}
	}

	mustClose(t, reader, dir)
}
