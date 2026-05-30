// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"math/rand"
	"strconv"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestDocsAndPositions ports org.apache.lucene.index.TestDocsAndPositions.
//
// The Java suite exercises PostingsEnum: per-document term frequency, position
// iteration (including buffer refill for high-frequency terms), advance/nextDoc
// navigation, and the docID()==-1 contract before the enum is positioned.
//
// Pre-existing infrastructure gap: every test routes through
// LeafReader.Terms(field).iterator().seekExact(...).postings(...), and
// OpenDirectoryReader materialises each segment via NewSegmentReader
// (index/directory_reader.go:462/497), which leaves SegmentReader.coreReaders
// nil. Without the codec-side wiring that loads SegmentCoreReaders from disk,
// LeafReader.Terms returns the "core readers are nil" error and none of the
// assertions can run. Each test therefore skips with the same blocker as
// TestBagOfPositions; unskip once OpenDirectoryReader uses
// NewSegmentReaderWithCore.
//
// Divergences from Lucene shared by every test below:
//   - Lucene drives writes through RandomIndexWriter with a randomized merge
//     policy and MockAnalyzer; Gocene exposes no randomized test-writer
//     wrapper, so these ports use the plain IndexWriter with
//     WhitespaceAnalyzer (the indexed text is purely space-separated tokens,
//     so tokenization is identical).
//   - Lucene reads via IndexWriter.getReader (near-real-time); Gocene's
//     IndexWriter has no NRT reader, so the index is reopened from the
//     directory after commit, matching TestBagOfPositions / TestBinaryTerms.
//   - Lucene's PostingsEnum.ALL / FREQS / NONE flag constants have no Gocene
//     equivalent; TermsEnum.Postings takes a bare int, so 0 is passed.
//   - The Java field-type randomization (omitNorms, term vectors) is dropped;
//     a plain non-stored TextField is used, since none of those options
//     affect frequencies or positions.

const docsAndPositionsBlocked = "blocked: OpenDirectoryReader builds SegmentReader without core readers (index/directory_reader.go:462/497); fix is NewSegmentReaderWithCore"

// getDocsAndPositions mirrors the Java helper of the same name: it resolves the
// PostingsEnum for term bytes in fieldName, or returns nil when the term is
// absent.
func getDocsAndPositions(t *testing.T, air index.LeafReaderInterface, fieldName, term string) index.PostingsEnum {
	t.Helper()
	terms, err := air.Terms(fieldName)
	if err != nil {
		t.Fatalf("Terms(%q) failed: %v", fieldName, err)
	}
	if terms == nil {
		return nil
	}
	te, err := terms.GetIterator()
	if err != nil {
		t.Fatalf("GetIterator failed: %v", err)
	}
	found, err := te.SeekExact(index.NewTerm(fieldName, term))
	if err != nil {
		t.Fatalf("SeekExact(%q) failed: %v", term, err)
	}
	if !found {
		return nil
	}
	pe, err := te.Postings(0) // Java: PostingsEnum.ALL
	if err != nil {
		t.Fatalf("Postings failed: %v", err)
	}
	return pe
}

// docsAndPositionsLeaves writes docs, commits, reopens the directory and
// returns the single leaf reader produced by forceMerge(1).
func docsAndPositionsLeaves(t *testing.T, fieldName string, docs []string) (index.LeafReaderInterface, func()) {
	t.Helper()
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to open directory: %v", err)
	}

	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		dir.Close()
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}

	for _, text := range docs {
		doc := document.NewDocument()
		field, err := document.NewTextField(fieldName, text, false)
		if err != nil {
			t.Fatalf("Failed to create field: %v", err)
		}
		doc.Add(field)
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("Failed to add document: %v", err)
		}
	}

	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge(1) failed: %v", err)
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	reader, err := index.OpenDirectoryReader(dir)
	if err != nil {
		dir.Close()
		t.Fatalf("Failed to open reader: %v", err)
	}
	leaves, err := reader.Leaves()
	if err != nil {
		reader.Close()
		dir.Close()
		t.Fatalf("Failed to get leaves: %v", err)
	}
	if len(leaves) != 1 {
		reader.Close()
		dir.Close()
		t.Fatalf("expected 1 leaf after forceMerge(1), got %d", len(leaves))
	}
	return leaves[0].LeafReader(), func() {
		reader.Close()
		dir.Close()
	}
}

// TestDocsAndPositionsPositionsSimple ports testPositionsSimple: term "1"
// occurs four times per document, at positions 0, 10, 20 and 30.
func TestDocsAndPositionsPositionsSimple(t *testing.T) {
	t.Fatal(docsAndPositionsBlocked)

	const fieldName = "field"
	const text = "1 2 3 4 5 6 7 8 9 10 " +
		"1 2 3 4 5 6 7 8 9 10 " +
		"1 2 3 4 5 6 7 8 9 10 " +
		"1 2 3 4 5 6 7 8 9 10"
	docs := make([]string, 39)
	for i := range docs {
		docs[i] = text
	}

	air, cleanup := docsAndPositionsLeaves(t, fieldName, docs)
	defer cleanup()

	const num = 13 // atLeast(13)
	for i := 0; i < num; i++ {
		pe := getDocsAndPositions(t, air, fieldName, "1")
		if pe == nil {
			t.Fatal("getDocsAndPositions returned nil")
		}
		if air.MaxDoc() == 0 {
			continue
		}
		if _, err := pe.Advance(rand.Intn(air.MaxDoc())); err != nil {
			t.Fatalf("Advance failed: %v", err)
		}
		for {
			for _, want := range []int{0, 10, 20, 30} {
				if freq, _ := pe.Freq(); freq != 4 {
					t.Fatalf("doc %d: Freq() = %d, want 4", pe.DocID(), freq)
				}
				pos, err := pe.NextPosition()
				if err != nil {
					t.Fatalf("NextPosition failed: %v", err)
				}
				if pos != want {
					t.Fatalf("doc %d: NextPosition() = %d, want %d", pe.DocID(), pos, want)
				}
			}
			next, err := pe.NextDoc()
			if err != nil {
				t.Fatalf("NextDoc failed: %v", err)
			}
			if next == index.NO_MORE_DOCS {
				break
			}
		}
	}
}

// TestDocsAndPositionsRandomPositions ports testRandomPositions: random numbers
// are indexed and every recorded position of a randomly chosen term is checked
// against the enum.
func TestDocsAndPositionsRandomPositions(t *testing.T) {
	t.Fatal(docsAndPositionsBlocked)

	const fieldName = "field"
	numDocs := 47 + rand.Intn(47) // atLeast(47)
	const max = 1051
	term := rand.Intn(max)

	docs := make([]string, numDocs)
	positionsInDoc := make([][]int, numDocs)
	for i := 0; i < numDocs; i++ {
		var builder strings.Builder
		var positions []int
		num := 131 + rand.Intn(131) // atLeast(131)
		for j := 0; j < num; j++ {
			nextInt := rand.Intn(max)
			builder.WriteString(strconv.Itoa(nextInt))
			builder.WriteByte(' ')
			if nextInt == term {
				positions = append(positions, j)
			}
		}
		if len(positions) == 0 {
			builder.WriteString(strconv.Itoa(term))
			positions = append(positions, num)
		}
		docs[i] = builder.String()
		positionsInDoc[i] = positions
	}

	air, cleanup := docsAndPositionsLeaves(t, fieldName, docs)
	defer cleanup()

	const iterations = 13 // atLeast(13)
	for i := 0; i < iterations; i++ {
		pe := getDocsAndPositions(t, air, fieldName, strconv.Itoa(term))
		if pe == nil {
			t.Fatal("getDocsAndPositions returned nil")
		}
		maxDoc := air.MaxDoc()
		if rand.Intn(2) == 0 {
			if _, err := pe.NextDoc(); err != nil {
				t.Fatalf("NextDoc failed: %v", err)
			}
		} else {
			if _, err := pe.Advance(rand.Intn(maxDoc)); err != nil {
				t.Fatalf("Advance failed: %v", err)
			}
		}
		for {
			docID := pe.DocID()
			if docID == index.NO_MORE_DOCS {
				break
			}
			// Single leaf: docBase is 0.
			pos := positionsInDoc[docID]
			if freq, _ := pe.Freq(); freq != len(pos) {
				t.Fatalf("doc %d: Freq() = %d, want %d", docID, freq, len(pos))
			}
			howMany := len(pos)
			if len(pos) > 0 && rand.Intn(20) == 0 {
				howMany = len(pos) - rand.Intn(len(pos))
			}
			for j := 0; j < howMany; j++ {
				got, err := pe.NextPosition()
				if err != nil {
					t.Fatalf("NextPosition failed: %v", err)
				}
				if got != pos[j] {
					t.Fatalf("iteration %d doc %d: NextPosition() = %d, want %d", i, docID, got, pos[j])
				}
			}
			if rand.Intn(10) == 0 {
				advanced, err := pe.Advance(docID + 1 + rand.Intn(maxDoc-docID))
				if err != nil {
					t.Fatalf("Advance failed: %v", err)
				}
				if advanced == index.NO_MORE_DOCS {
					break
				}
				continue
			}
			next, err := pe.NextDoc()
			if err != nil {
				t.Fatalf("NextDoc failed: %v", err)
			}
			if next == index.NO_MORE_DOCS {
				break
			}
		}
	}
}

// TestDocsAndPositionsRandomDocs ports testRandomDocs: it verifies per-document
// frequency and advance/nextDoc navigation over a randomly populated field.
func TestDocsAndPositionsRandomDocs(t *testing.T) {
	t.Fatal(docsAndPositionsBlocked)

	const fieldName = "field"
	numDocs := 49 + rand.Intn(49) // atLeast(49)
	const max = 15678
	term := rand.Intn(max)

	docs := make([]string, numDocs)
	freqInDoc := make([]int, numDocs)
	for i := 0; i < numDocs; i++ {
		var builder strings.Builder
		for j := 0; j < 199; j++ {
			nextInt := rand.Intn(max)
			builder.WriteString(strconv.Itoa(nextInt))
			builder.WriteByte(' ')
			if nextInt == term {
				freqInDoc[i]++
			}
		}
		docs[i] = builder.String()
	}

	air, cleanup := docsAndPositionsLeaves(t, fieldName, docs)
	defer cleanup()

	const iterations = 13 // atLeast(13)
	for i := 0; i < iterations; i++ {
		maxDoc := air.MaxDoc()
		// Single leaf: docBase is 0. Java uses TestUtil.docs with FREQS.
		pe := getDocsAndPositions(t, air, fieldName, strconv.Itoa(term))
		if findNextDocsAndPositions(freqInDoc, 0, maxDoc) == int(^uint(0)>>1) {
			if pe != nil {
				t.Fatal("expected nil PostingsEnum when term is absent")
			}
			continue
		}
		if pe == nil {
			t.Fatal("getDocsAndPositions returned nil")
		}
		if _, err := pe.NextDoc(); err != nil {
			t.Fatalf("NextDoc failed: %v", err)
		}
		for j := 0; j < maxDoc; j++ {
			if freqInDoc[j] == 0 {
				continue
			}
			if pe.DocID() != j {
				t.Fatalf("DocID() = %d, want %d", pe.DocID(), j)
			}
			if freq, _ := pe.Freq(); freq != freqInDoc[j] {
				t.Fatalf("doc %d: Freq() = %d, want %d", j, freq, freqInDoc[j])
			}
			if i%2 == 0 && rand.Intn(10) == 0 {
				next := findNextDocsAndPositions(freqInDoc, j+1, maxDoc)
				advancedTo, err := pe.Advance(next)
				if err != nil {
					t.Fatalf("Advance failed: %v", err)
				}
				if next >= maxDoc {
					if advancedTo != index.NO_MORE_DOCS {
						t.Fatalf("Advance(%d) = %d, want NO_MORE_DOCS", next, advancedTo)
					}
				} else if next < advancedTo {
					t.Fatalf("advanced to %d but should be <= %d", advancedTo, next)
				}
			} else if _, err := pe.NextDoc(); err != nil {
				t.Fatalf("NextDoc failed: %v", err)
			}
		}
		if pe.DocID() != index.NO_MORE_DOCS {
			t.Fatalf("final DocID() = %d, want NO_MORE_DOCS", pe.DocID())
		}
	}
}

// findNextDocsAndPositions ports the Java findNext helper: it returns the first
// index in [pos, max) with a non-zero count, or math.MaxInt32 when none exists.
func findNextDocsAndPositions(docs []int, pos, max int) int {
	for i := pos; i < max; i++ {
		if docs[i] != 0 {
			return i
		}
	}
	return int(^uint(0) >> 1)
}

// TestDocsAndPositionsLargeNumberOfPositions ports testLargeNumberOfPositions:
// term "even" occurs 500 times per document, forcing a positions buffer refill.
func TestDocsAndPositionsLargeNumberOfPositions(t *testing.T) {
	t.Fatal(docsAndPositionsBlocked)

	const fieldName = "field"
	const howMany = 1000

	var builder strings.Builder
	for j := 0; j < howMany; j++ {
		if j%2 == 0 {
			builder.WriteString("even ")
		} else {
			builder.WriteString("odd ")
		}
	}
	docs := make([]string, 39)
	for i := range docs {
		docs[i] = builder.String()
	}

	air, cleanup := docsAndPositionsLeaves(t, fieldName, docs)
	defer cleanup()

	const num = 13 // atLeast(13)
	for i := 0; i < num; i++ {
		pe := getDocsAndPositions(t, air, fieldName, "even")
		if pe == nil {
			t.Fatal("getDocsAndPositions returned nil")
		}
		if rand.Intn(2) == 0 {
			if _, err := pe.NextDoc(); err != nil {
				t.Fatalf("NextDoc failed: %v", err)
			}
		} else {
			if _, err := pe.Advance(rand.Intn(air.MaxDoc())); err != nil {
				t.Fatalf("Advance failed: %v", err)
			}
		}
		if freq, _ := pe.Freq(); freq != howMany/2 {
			t.Fatalf("Freq() = %d, want %d", freq, howMany/2)
		}
		for j := 0; j < howMany; j += 2 {
			got, err := pe.NextPosition()
			if err != nil {
				t.Fatalf("NextPosition failed: %v", err)
			}
			if got != j {
				t.Fatalf("position mismatch index %d: NextPosition() = %d, want %d", j, got, j)
			}
		}
	}
}

// TestDocsAndPositionsDocsEnumStart ports testDocsEnumStart: a freshly resolved
// PostingsEnum reports docID()==-1 until nextDoc is called.
func TestDocsAndPositionsDocsEnumStart(t *testing.T) {
	t.Fatal(docsAndPositionsBlocked)

	const fieldName = "foo"
	air, cleanup := docsAndPositionsLeaves(t, fieldName, []string{"bar"})
	defer cleanup()

	disi := getDocsAndPositions(t, air, fieldName, "bar")
	if disi == nil {
		t.Fatal("getDocsAndPositions returned nil")
	}
	if disi.DocID() != -1 {
		t.Fatalf("DocID() before nextDoc = %d, want -1", disi.DocID())
	}
	next, err := disi.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc failed: %v", err)
	}
	if next == index.NO_MORE_DOCS {
		t.Fatal("NextDoc() returned NO_MORE_DOCS, want a document")
	}
}

// TestDocsAndPositionsDocsAndPositionsEnumStart ports
// testDocsAndPositionsEnumStart: same -1 contract for a positions-enabled enum.
func TestDocsAndPositionsDocsAndPositionsEnumStart(t *testing.T) {
	t.Fatal(docsAndPositionsBlocked)

	const fieldName = "foo"
	air, cleanup := docsAndPositionsLeaves(t, fieldName, []string{"bar"})
	defer cleanup()

	disi := getDocsAndPositions(t, air, fieldName, "bar")
	if disi == nil {
		t.Fatal("getDocsAndPositions returned nil")
	}
	if disi.DocID() != -1 {
		t.Fatalf("DocID() before nextDoc = %d, want -1", disi.DocID())
	}
	next, err := disi.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc failed: %v", err)
	}
	if next == index.NO_MORE_DOCS {
		t.Fatal("NextDoc() returned NO_MORE_DOCS, want a document")
	}
}
