// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/test/org/apache/lucene/search/TestMultiThreadTermVectors.java
//
// The upstream test indexes numDocs documents, each carrying a single
// untokenized, stored, term-vector-enabled "field" whose value is
// English.intToEnglish(docID). It then opens a DirectoryReader and reads the
// stored term vectors concurrently from numThreads goroutines, each running
// numIterations passes. Every pass walks every document, pulls its term vectors,
// and verifies that concatenating the term-vector terms reproduces the original
// field value — proving that concurrent term-vector reads return correct,
// consistent results.
//
// This is a faithful port driving the real IndexWriter + DirectoryReader
// term-vector read path. Because the field is not tokenized, each document's
// term vector holds exactly one term: the whole field value. The verification is
// identical to the reference (CheckHits-style equality after String.trim()).
//
// Deviations, documented per the binary-compatibility mandate:
//   - The reference uses the test-framework MockAnalyzer and a LogMergePolicy;
//     this port uses the production WhitespaceAnalyzer (the field is untokenized,
//     so the analyzer is never consulted for it) and the default merge policy.
//     Neither choice affects the term-vector contract under test.
//   - numDocs/numThreads/numIterations are pinned to the non-nightly reference
//     values (50/2/50).

package search_test

import (
	"strings"
	"sync"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"

	// Register the production codec so term vectors are flushed and read back.
	_ "github.com/FlavioCFOliveira/Gocene/codecs"
)

const (
	mttvNumDocs       = 50
	mttvNumThreads    = 2
	mttvNumIterations = 50
)

// mttvIntToEnglish is the faithful port of English.intToEnglish restricted to
// the non-negative range exercised by this test (0..49). The trailing space the
// Java builder leaves on the last word is preserved here and trimmed by the
// verifier, exactly as the reference does.
func mttvIntToEnglish(i int) string {
	var b strings.Builder
	if i == 0 {
		b.WriteString("zero")
		return b.String()
	}
	if i >= 20 {
		switch i / 10 {
		case 9:
			b.WriteString("ninety")
		case 8:
			b.WriteString("eighty")
		case 7:
			b.WriteString("seventy")
		case 6:
			b.WriteString("sixty")
		case 5:
			b.WriteString("fifty")
		case 4:
			b.WriteString("forty")
		case 3:
			b.WriteString("thirty")
		case 2:
			b.WriteString("twenty")
		}
		if i%10 == 0 {
			b.WriteString(" ")
		} else {
			b.WriteString("-")
		}
		i %= 10
	}
	switch i {
	case 19:
		b.WriteString("nineteen ")
	case 18:
		b.WriteString("eighteen ")
	case 17:
		b.WriteString("seventeen ")
	case 16:
		b.WriteString("sixteen ")
	case 15:
		b.WriteString("fifteen ")
	case 14:
		b.WriteString("fourteen ")
	case 13:
		b.WriteString("thirteen ")
	case 12:
		b.WriteString("twelve ")
	case 11:
		b.WriteString("eleven ")
	case 10:
		b.WriteString("ten ")
	case 9:
		b.WriteString("nine ")
	case 8:
		b.WriteString("eight ")
	case 7:
		b.WriteString("seven ")
	case 6:
		b.WriteString("six ")
	case 5:
		b.WriteString("five ")
	case 4:
		b.WriteString("four ")
	case 3:
		b.WriteString("three ")
	case 2:
		b.WriteString("two ")
	case 1:
		b.WriteString("one ")
	case 0:
	}
	return b.String()
}

// mttvTermVectorFieldType mirrors the FieldType the reference builds:
// TextField.TYPE_STORED with tokenized=false and term vectors enabled.
func mttvTermVectorFieldType() *document.FieldType {
	ft := document.NewFieldType()
	ft.Indexed = true
	ft.Stored = true
	ft.Tokenized = false
	ft.IndexOptions = index.IndexOptionsDocsAndFreqsAndPositions
	ft.StoreTermVectors = true
	return ft
}

// buildMultiThreadTermVectorsIndex indexes numDocs untokenized term-vector
// documents and returns the committed directory.
func buildMultiThreadTermVectorsIndex(t *testing.T) store.Directory {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	ft := mttvTermVectorFieldType()
	for i := 0; i < mttvNumDocs; i++ {
		doc := document.NewDocument()
		fld, ferr := document.NewField("field", mttvIntToEnglish(i), ft)
		if ferr != nil {
			t.Fatalf("NewField(%d): %v", i, ferr)
		}
		doc.Add(fld)
		if aerr := w.AddDocument(doc); aerr != nil {
			t.Fatalf("AddDocument(%d): %v", i, aerr)
		}
	}
	if cerr := w.Close(); cerr != nil {
		t.Fatalf("writer.Close: %v", cerr)
	}
	return dir
}

// mttvVerifyVector concatenates the terms of a single term-vector field and
// asserts the trimmed result equals the trimmed english rendering of num,
// mirroring TestMultiThreadTermVectors.verifyVector.
func mttvVerifyVector(t *testing.T, vector index.Terms, num int) {
	t.Helper()
	it, err := vector.GetIterator()
	if err != nil {
		t.Errorf("doc %d: term-vector GetIterator: %v", num, err)
		return
	}
	var temp strings.Builder
	for {
		term, nerr := it.Next()
		if nerr != nil {
			t.Errorf("doc %d: term-vector Next: %v", num, nerr)
			return
		}
		if term == nil {
			break
		}
		temp.WriteString(term.Text())
	}
	if got, want := strings.TrimSpace(temp.String()), strings.TrimSpace(mttvIntToEnglish(num)); got != want {
		t.Errorf("doc %d: term-vector = %q, want %q", num, got, want)
	}
}

// mttvVerifyVectors walks every field of a document's term vectors and verifies
// each, mirroring TestMultiThreadTermVectors.verifyVectors.
func mttvVerifyVectors(t *testing.T, vectors index.Fields, num int) {
	t.Helper()
	it, err := vectors.Iterator()
	if err != nil {
		t.Errorf("doc %d: fields Iterator: %v", num, err)
		return
	}
	for it.HasNext() {
		field, ferr := it.Next()
		if ferr != nil {
			t.Errorf("doc %d: fields Next: %v", num, ferr)
			return
		}
		if field == "" {
			break
		}
		terms, terr := vectors.Terms(field)
		if terr != nil {
			t.Errorf("doc %d field %q: Terms: %v", num, field, terr)
			return
		}
		if terms == nil {
			t.Errorf("doc %d field %q: term vector is nil", num, field)
			return
		}
		mttvVerifyVector(t, terms, num)
	}
}

// TestMultiThreadTermVectors_Concurrency mirrors the Java test() method:
// concurrent goroutines repeatedly read every document's term vectors and
// verify their contents.
func TestMultiThreadTermVectors_Concurrency(t *testing.T) {
	dir := buildMultiThreadTermVectorsIndex(t)
	defer func() { _ = dir.Close() }()

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer func() { _ = reader.Close() }()

	numDocs := reader.NumDocs()
	if numDocs != mttvNumDocs {
		t.Fatalf("NumDocs = %d, want %d", numDocs, mttvNumDocs)
	}

	var wg sync.WaitGroup
	wg.Add(mttvNumThreads)
	for thread := 0; thread < mttvNumThreads; thread++ {
		go func() {
			defer wg.Done()
			for iter := 0; iter < mttvNumIterations; iter++ {
				termVectors, tverr := reader.TermVectors()
				if tverr != nil {
					t.Errorf("TermVectors: %v", tverr)
					return
				}
				for docID := 0; docID < numDocs; docID++ {
					vectors, gerr := termVectors.Get(docID)
					if gerr != nil {
						t.Errorf("TermVectors.Get(%d): %v", docID, gerr)
						return
					}
					if vectors == nil {
						t.Errorf("doc %d: term vectors are nil", docID)
						return
					}
					mttvVerifyVectors(t, vectors, docID)

					vector, verr := vectors.Terms("field")
					if verr != nil {
						t.Errorf("doc %d: Terms(field): %v", docID, verr)
						return
					}
					if vector == nil {
						t.Errorf("doc %d: field term vector is nil", docID)
						return
					}
					mttvVerifyVector(t, vector, docID)
				}
		}
		}()
	}
	wg.Wait()
}