// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of lucene/core/src/test/org/apache/lucene/search/TestSameScoresWithThreads.java
// (Apache Lucene 10.5.0).

package search_test

import (
	"sync"
	"testing"
	"unicode/utf16"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	testanalysis "github.com/FlavioCFOliveira/Gocene/tests/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// lineFileDocsBlocker names org.apache.lucene.tests.util.LineFileDocs.
const lineFileDocsBlocker = "requires org.apache.lucene.tests.util.LineFileDocs (not ported)"

// lineFileDocs renders the members of org.apache.lucene.tests.util.LineFileDocs
// the test uses; the class is not ported.
type lineFileDocs interface {
	NextDoc() (*document.Document, error)
	Close() error
}

func TestSameScoresWithThreads(t *testing.T) {
	dir := newDirectory()
	analyzer := testanalysis.NewMockAnalyzerRandom(random())
	analyzer.SetMaxTokenLength(nextInt(1, index.MAX_TERM_LENGTH))
	w := newRandomIndexWriterWithAnalyzer(t, dir, analyzer)
	// LineFileDocs docs = new LineFileDocs(random());
	t.Fatal(lineFileDocsBlocker)
	var docs lineFileDocs
	charsToIndex := atLeast(100000)
	charsIndexed := 0
	// System.out.println("bytesToIndex=" + charsToIndex);
	for charsIndexed < charsToIndex {
		doc, err := docs.NextDoc()
		if err != nil {
			t.Fatalf("nextDoc: %v", err)
		}
		charsIndexed += len(utf16.Encode([]rune(doc.Get("body").StringValue())))
		mustAddDocument(t, w, doc)
		// System.out.println("  bytes=" + charsIndexed + " add: " + doc);
	}
	r := mustGetReader(t, w)
	// System.out.println("numDocs=" + r.numDocs());
	mustClose(t, w)

	s := newSearcher(t, r)
	terms, err := index.MultiTermsGetTerms(r, "body")
	if err != nil {
		t.Fatalf("MultiTerms.getTerms: %v", err)
	}
	termCount := 0
	termsEnum, err := terms.Iterator()
	if err != nil {
		t.Fatalf("iterator: %v", err)
	}
	for {
		term, err := termsEnum.Next()
		if err != nil {
			t.Fatalf("next: %v", err)
		}
		if term == nil {
			break
		}
		termCount++
	}
	if !(termCount > 0) {
		t.Fatal("assertTrue(termCount > 0)")
	}

	// Target ~10 terms to search:
	chance := 10.0 / float64(termCount)
	termsEnum, err = terms.Iterator()
	if err != nil {
		t.Fatalf("iterator: %v", err)
	}
	type answer struct {
		term *util.BytesRef
		hits *search.TopDocs
	}
	answers := map[string]answer{}
	for {
		next, err := termsEnum.Next()
		if err != nil {
			t.Fatalf("next: %v", err)
		}
		if next == nil {
			break
		}
		if random().Float64() <= chance {
			term := util.DeepCopyOfBytesRef(termsEnum.Term().BytesValue())
			answers[string(term.ValidBytes())] = answer{term: term, hits: mustSearch(t, s, search.NewTermQuery(index.NewTermFromBytesRef("body", term)), 100)}
		}
	}

	if len(answers) != 0 {
		startingGun := make(chan struct{}) // CountDownLatch(1)
		numThreads := 2                    // TEST_NIGHTLY ? TestUtil.nextInt(random(), 2, 5) : 2
		var threads sync.WaitGroup
		for threadID := 0; threadID < numThreads; threadID++ {
			threads.Add(1)
			go func() {
				defer threads.Done()
				<-startingGun
				for i := 0; i < 20; i++ {
					shuffled := make([]answer, 0, len(answers))
					for _, ent := range answers {
						shuffled = append(shuffled, ent)
					}
					random().Shuffle(len(shuffled), func(a, b int) { shuffled[a], shuffled[b] = shuffled[b], shuffled[a] })
					for _, ent := range shuffled {
						actual, err := s.Search(search.NewTermQuery(index.NewTermFromBytesRef("body", ent.term)), 100)
						if err != nil {
							t.Errorf("RuntimeException: %v", err)
							return
						}
						expected := ent.hits
						if expected.TotalHits.Value != actual.TotalHits.Value {
							t.Errorf("totalHits: expected %d, got %d", expected.TotalHits.Value, actual.TotalHits.Value)
							return
						}
						if len(expected.ScoreDocs) != len(actual.ScoreDocs) {
							t.Errorf("query=%s: expected %d hits, got %d", ent.term.Utf8ToString(), len(expected.ScoreDocs), len(actual.ScoreDocs))
							return
						}
						for hit := 0; hit < len(expected.ScoreDocs); hit++ {
							if expected.ScoreDocs[hit].Doc != actual.ScoreDocs[hit].Doc {
								t.Errorf("doc: expected %d, got %d", expected.ScoreDocs[hit].Doc, actual.ScoreDocs[hit].Doc)
								return
							}
							// Floats really should be identical:
							if !(expected.ScoreDocs[hit].Score == actual.ScoreDocs[hit].Score) {
								t.Errorf("score: expected %v, got %v", expected.ScoreDocs[hit].Score, actual.ScoreDocs[hit].Score)
								return
							}
						}
					}
				}
			}()
		}
		close(startingGun)
		threads.Wait()
	}
	mustClose(t, docs, r, dir)
}
