// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build gocene_monsters

// The @Monster and @Nightly test ports of
// lucene/core/src/test/org/apache/lucene/index/TestIndexWriterMaxDocs.java
// (Apache Lucene 10.5.0), built only with the gocene_monsters tag as Lucene
// runs them only when monster/nightly tests are enabled.

package index_test

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// The two hour time was achieved on a Linux 3.13 system with these specs:
// 3-core AMD at 2.5Ghz, 12 GB RAM, 5GB test heap, 2 test JVMs, 2TB SATA.
// @Monster("takes over two hours")
func TestIndexWriterMaxDocsExactlyAtTrueLimit(t *testing.T) {
	dir := newFSDirectoryAt(t, filepath.Join(t.TempDir(), "2BDocs3"))
	iw := nullAnalyzerWriter(t, dir)
	doc := document.NewDocument()
	doc.Add(newStringField(t, "field", "text", false))
	for i := 0; i < index.MaxDocs; i++ {
		mustAddDocument(t, iw, doc)
	}
	mustCommit(t, iw)

	// First unoptimized, then optimized:
	for i := 0; i < 2; i++ {
		ir := mustOpenDirectoryReader(t, dir)
		if ir.MaxDoc() != index.MaxDocs {
			t.Fatalf("maxDoc: expected %d, got %d", index.MaxDocs, ir.MaxDoc())
		}
		if ir.NumDocs() != index.MaxDocs {
			t.Fatalf("numDocs: expected %d, got %d", index.MaxDocs, ir.NumDocs())
		}
		searcher := search.NewIndexSearcher(ir)
		collectorManager, err := search.NewTopScoreDocCollectorManager(10, nil, math.MaxInt32)
		if err != nil {
			t.Fatalf("TopScoreDocCollectorManager: %v", err)
		}
		hits, err := search.SearchWithCollectorManager(searcher, search.NewTermQuery(index.NewTerm("field", "text")), collectorManager)
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		if hits.TotalHits.Value != int64(index.MaxDocs) {
			t.Fatalf("totalHits: expected %d, got %d", index.MaxDocs, hits.TotalHits.Value)
		}

		// Sort by docID reversed:
		sorted, err := searcher.SearchWithSortNoScores(search.NewTermQuery(index.NewTerm("field", "text")), 10,
			search.NewSort(search.NewSortFieldWithReverse("", spi.SortFieldTypeDoc, true)))
		if err != nil {
			t.Fatalf("search(sort): %v", err)
		}
		if sorted.TotalHits.Value != int64(index.MaxDocs) {
			t.Fatalf("totalHits: expected %d, got %d", index.MaxDocs, sorted.TotalHits.Value)
		}
		if len(sorted.ScoreDocs) != 10 {
			t.Fatalf("scoreDocs.length: expected 10, got %d", len(sorted.ScoreDocs))
		}
		if sorted.ScoreDocs[0].Doc != index.MaxDocs-1 {
			t.Fatalf("scoreDocs[0].doc: expected %d, got %d", index.MaxDocs-1, sorted.ScoreDocs[0].Doc)
		}
		mustClose(t, ir)

		if err := iw.ForceMerge(1); err != nil {
			t.Fatalf("forceMerge: %v", err)
		}
	}

	mustClose(t, iw, dir)
}

// LUCENE-6299: Test if addindexes(Dir[]) prevents exceeding max docs.
// @Nightly
func TestIndexWriterMaxDocsAddTooManyIndexesDir(t *testing.T) {
	// we cheat and add the same one over again... IW wants a write lock on each
	t.Fatal(byteBuffersDirectoryLockFactoryMissing)
}
