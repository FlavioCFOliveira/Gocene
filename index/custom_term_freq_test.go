// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// remainingCustomTermFreqBlockers collects the reasons for the tests in this
// file that still fail under the no-skip policy. The postings read-back path
// is now wired, but three end-to-end legs remain incomplete:
const (
	// termVectorsBlocked: the DWPT term-vector path is only stubbed
	// (buildTermVector is a placeholder and flushTermVectors is not invoked).
	termVectorsBlocked = "blocked: term-vector write/flush path not implemented (dwpt.buildTermVector placeholder)"
	// overflowIntBlocked: per-document rollback when a term-frequency sum
	// overflows int is not yet implemented.
	overflowIntBlocked = "blocked: per-document rollback on term-frequency overflow not implemented"
	// fieldInvertStateBlocked: capturing the FieldInvertState requires a
	// SetSimilarity hook on IndexWriterConfig and a test-only similarity.
	fieldInvertStateBlocked = "blocked: IndexWriterConfig.SetSimilarity wiring and FieldInvertState capture hook not implemented"
)

// cannedTermFreqs ports the private CannedTermFreqs TokenStream from
// org.apache.lucene.index.TestCustomTermFreq. It emits a fixed sequence of
// terms, attaching an explicit term frequency to each via TermFrequencyAttribute.
type cannedTermFreqs struct {
	*analysis.BaseTokenStream
	terms       []string
	termFreqs   []int
	termAtt     analysis.CharTermAttribute
	termFreqAtt analysis.TermFrequencyAttribute
	upto        int
}

func newCannedTermFreqs(terms []string, termFreqs []int) *cannedTermFreqs {
	if len(terms) != len(termFreqs) {
		panic("terms.length == termFreqs.length")
	}
	base := analysis.NewBaseTokenStream()
	ts := &cannedTermFreqs{
		BaseTokenStream: base,
		terms:           terms,
		termFreqs:       termFreqs,
		termAtt:         analysis.NewCharTermAttribute(),
		termFreqAtt:     analysis.NewTermFrequencyAttribute(),
	}
	base.AddAttribute(ts.termAtt)
	base.AddAttribute(ts.termFreqAtt)
	return ts
}

func (c *cannedTermFreqs) IncrementToken() (bool, error) {
	if c.upto == len(c.terms) {
		return false, nil
	}
	c.GetAttributeSource().ClearAttributes()
	c.termAtt.AppendString(c.terms[c.upto])
	c.termFreqAtt.SetTermFrequency(c.termFreqs[c.upto])
	c.upto++
	return true, nil
}

func (c *cannedTermFreqs) Reset() error {
	c.upto = 0
	return nil
}

// docsAndFreqsType returns a non-stored TextField type indexing DOCS_AND_FREQS.
func docsAndFreqsType() *document.FieldType {
	ft := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	ft.SetIndexOptions(index.IndexOptionsDocsAndFreqs)
	return ft
}

// newCustomFreqWriter opens a fresh SimpleFSDirectory-backed IndexWriter.
//
// Divergences from Lucene shared by every test below:
//   - MockAnalyzer is replaced by WhitespaceAnalyzer; the analyzer is unused on
//     fields carrying a pre-built TokenStream.
//   - The index is reopened from the directory after commit; Gocene exposes no
//     RandomIndexWriter and no NRT IndexWriter.getReader (matches
//     TestBagOfPositions, TestBinaryTerms).
func newCustomFreqWriter(t *testing.T) (store.Directory, *index.IndexWriter) {
	t.Helper()
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("Failed to open directory: %v", err)
	}
	config := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, config)
	if err != nil {
		dir.Close()
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}
	return dir, w
}

// cannedFreqField builds the Field used throughout the test: a "field"-named
// field backed by a cannedTermFreqs TokenStream and the given FieldType.
func cannedFreqField(t *testing.T, terms []string, freqs []int, ft *document.FieldType) *document.Field {
	t.Helper()
	field, err := document.NewField("field", newCannedTermFreqs(terms, freqs), ft)
	if err != nil {
		t.Fatalf("Failed to create field: %v", err)
	}
	return field
}

// addCannedDoc adds a single document holding one cannedFreqField.
func addCannedDoc(t *testing.T, w *index.IndexWriter, terms []string, freqs []int, ft *document.FieldType) {
	t.Helper()
	doc := document.NewDocument()
	doc.Add(cannedFreqField(t, terms, freqs, ft))
	if err := w.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument failed: %v", err)
	}
}

// commitAndClose commits then closes the writer.
func commitAndClose(t *testing.T, w *index.IndexWriter) {
	t.Helper()
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

// assertCustomTermPostings checks that the term `text` in field "field" has the expected
// (docID, freq) sequence. The field is indexed with DOCS_AND_FREQS, so frequencies
// are requested via index.PostingsFlagFreqs.
func assertCustomTermPostings(t *testing.T, r *index.DirectoryReader, text string, expected []postingEntry) {
	t.Helper()
	terms, err := r.Terms("field")
	if err != nil {
		t.Fatalf("Terms(field): %v", err)
	}
	if terms == nil {
		t.Fatal("Terms(field) is nil")
	}
	te, err := terms.GetIteratorWithSeek(index.NewTerm("field", text))
	if err != nil {
		t.Fatalf("GetIteratorWithSeek(%q): %v", text, err)
	}
	if te == nil {
		t.Fatalf("GetIteratorWithSeek(%q) is nil", text)
	}
	found, err := te.SeekExact(index.NewTerm("field", text))
	if err != nil {
		t.Fatalf("SeekExact(%q): %v", text, err)
	}
	if !found {
		t.Fatalf("term %q not found", text)
	}
	pe, err := te.Postings(index.PostingsFlagFreqs)
	if err != nil {
		t.Fatalf("Postings(%q): %v", text, err)
	}
	if pe == nil {
		t.Fatalf("Postings(%q) is nil", text)
	}
	for i, exp := range expected {
		doc, err := pe.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc at %d for %q: %v", i, text, err)
		}
		if doc != exp.doc {
			t.Fatalf("%s doc[%d] = %d, want %d", text, i, doc, exp.doc)
		}
		freq, err := pe.Freq()
		if err != nil {
			t.Fatalf("Freq at %d for %q: %v", i, text, err)
		}
		if freq != exp.freq {
			t.Fatalf("%s freq[%d] = %d, want %d", text, i, freq, exp.freq)
		}
	}
	if doc, err := pe.NextDoc(); err != nil {
		t.Fatalf("NextDoc after last for %q: %v", text, err)
	} else if doc != -1 {
		t.Fatalf("%s expected no more docs, got %d", text, doc)
	}
}

type postingEntry struct {
	doc  int
	freq int
}

// TestCustomTermFreq_SingletonTermsOneDoc ports
// org.apache.lucene.index.TestCustomTermFreq#testSingletonTermsOneDoc.
func TestCustomTermFreq_SingletonTermsOneDoc(t *testing.T) {
	dir, w := newCustomFreqWriter(t)
	defer dir.Close()

	addCannedDoc(t, w, []string{"foo", "bar"}, []int{42, 128}, docsAndFreqsType())
	commitAndClose(t, w)

	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader failed: %v", err)
	}
	defer r.Close()

	assertCustomTermPostings(t, r, "bar", []postingEntry{{doc: 0, freq: 128}})
	assertCustomTermPostings(t, r, "foo", []postingEntry{{doc: 0, freq: 42}})
}

// TestCustomTermFreq_SingletonTermsTwoDocs ports
// org.apache.lucene.index.TestCustomTermFreq#testSingletonTermsTwoDocs.
func TestCustomTermFreq_SingletonTermsTwoDocs(t *testing.T) {
	dir, w := newCustomFreqWriter(t)
	defer dir.Close()

	ft := docsAndFreqsType()
	addCannedDoc(t, w, []string{"foo", "bar"}, []int{42, 128}, ft)
	addCannedDoc(t, w, []string{"foo", "bar"}, []int{50, 50}, ft)
	commitAndClose(t, w)

	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader failed: %v", err)
	}
	defer r.Close()

	assertCustomTermPostings(t, r, "bar", []postingEntry{{doc: 0, freq: 128}, {doc: 1, freq: 50}})
	assertCustomTermPostings(t, r, "foo", []postingEntry{{doc: 0, freq: 42}, {doc: 1, freq: 50}})
}

// TestCustomTermFreq_RepeatTermsOneDoc ports
// org.apache.lucene.index.TestCustomTermFreq#testRepeatTermsOneDoc.
func TestCustomTermFreq_RepeatTermsOneDoc(t *testing.T) {
	dir, w := newCustomFreqWriter(t)
	defer dir.Close()

	addCannedDoc(t, w,
		[]string{"foo", "bar", "foo", "bar"}, []int{42, 128, 17, 100}, docsAndFreqsType())
	commitAndClose(t, w)

	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader failed: %v", err)
	}
	defer r.Close()

	assertCustomTermPostings(t, r, "bar", []postingEntry{{doc: 0, freq: 228}})
	assertCustomTermPostings(t, r, "foo", []postingEntry{{doc: 0, freq: 59}})
}

// TestCustomTermFreq_RepeatTermsTwoDocs ports
// org.apache.lucene.index.TestCustomTermFreq#testRepeatTermsTwoDocs.
func TestCustomTermFreq_RepeatTermsTwoDocs(t *testing.T) {
	dir, w := newCustomFreqWriter(t)
	defer dir.Close()

	ft := docsAndFreqsType()
	addCannedDoc(t, w, []string{"foo", "bar", "foo", "bar"}, []int{42, 128, 17, 100}, ft)
	addCannedDoc(t, w, []string{"foo", "bar", "foo", "bar"}, []int{50, 60, 70, 80}, ft)
	commitAndClose(t, w)

	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader failed: %v", err)
	}
	defer r.Close()

	assertCustomTermPostings(t, r, "bar", []postingEntry{{doc: 0, freq: 228}, {doc: 1, freq: 140}})
	assertCustomTermPostings(t, r, "foo", []postingEntry{{doc: 0, freq: 59}, {doc: 1, freq: 120}})
}

// TestCustomTermFreq_TotalTermFreq ports
// org.apache.lucene.index.TestCustomTermFreq#testTotalTermFreq.
func TestCustomTermFreq_TotalTermFreq(t *testing.T) {
	dir, w := newCustomFreqWriter(t)
	defer dir.Close()

	ft := docsAndFreqsType()
	addCannedDoc(t, w, []string{"foo", "bar", "foo", "bar"}, []int{42, 128, 17, 100}, ft)
	addCannedDoc(t, w, []string{"foo", "bar", "foo", "bar"}, []int{50, 60, 70, 80}, ft)
	commitAndClose(t, w)

	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader failed: %v", err)
	}
	defer r.Close()

	terms, err := r.Terms("field")
	if err != nil {
		t.Fatalf("Terms(field): %v", err)
	}
	te, err := terms.GetIteratorWithSeek(index.NewTerm("field", "foo"))
	if err != nil {
		t.Fatalf("GetIteratorWithSeek(foo): %v", err)
	}
	if _, err := te.SeekExact(index.NewTerm("field", "foo")); err != nil {
		t.Fatalf("SeekExact(foo): %v", err)
	}
	if got, err := te.TotalTermFreq(); err != nil {
		t.Fatalf("TotalTermFreq(foo): %v", err)
	} else if got != 179 {
		t.Fatalf("foo totalTermFreq = %d, want 179", got)
	}
	if _, err := te.SeekExact(index.NewTerm("field", "bar")); err != nil {
		t.Fatalf("SeekExact(bar): %v", err)
	}
	if got, err := te.TotalTermFreq(); err != nil {
		t.Fatalf("TotalTermFreq(bar): %v", err)
	} else if got != 368 {
		t.Fatalf("bar totalTermFreq = %d, want 368", got)
	}
}

// TestCustomTermFreq_InvalidProx ports
// org.apache.lucene.index.TestCustomTermFreq#testInvalidProx: positions cannot
// be indexed alongside a custom TermFrequencyAttribute.
func TestCustomTermFreq_InvalidProx(t *testing.T) {
	dir, w := newCustomFreqWriter(t)
	defer dir.Close()
	defer w.Close()

	doc := document.NewDocument()
	ft := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	doc.Add(cannedFreqField(t, []string{"foo", "bar", "foo", "bar"}, []int{42, 128, 17, 100}, ft))

	err := w.AddDocument(doc)
	if err == nil {
		t.Fatal("expected error indexing positions with custom TermFrequencyAttribute")
	}
	const want = `field "field": cannot index positions while using custom TermFrequencyAttribute`
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

// TestCustomTermFreq_InvalidDocsOnly ports
// org.apache.lucene.index.TestCustomTermFreq#testInvalidDocsOnly: DOCS-only
// cannot be indexed with a custom TermFrequencyAttribute.
func TestCustomTermFreq_InvalidDocsOnly(t *testing.T) {
	dir, w := newCustomFreqWriter(t)
	defer dir.Close()
	defer w.Close()

	doc := document.NewDocument()
	ft := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	ft.SetIndexOptions(index.IndexOptionsDocs)
	doc.Add(cannedFreqField(t, []string{"foo", "bar", "foo", "bar"}, []int{42, 128, 17, 100}, ft))

	err := w.AddDocument(doc)
	if err == nil {
		t.Fatal("expected error indexing DOCS-only with custom TermFrequencyAttribute")
	}
	const want = `field "field": must index term freq while using custom TermFrequencyAttribute`
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

// TestCustomTermFreq_OverflowInt ports
// org.apache.lucene.index.TestCustomTermFreq#testOverflowInt: the sum of term
// freqs must fit in an int.
func TestCustomTermFreq_OverflowInt(t *testing.T) {
	t.Fatal(overflowIntBlocked)

	dir, w := newCustomFreqWriter(t)
	defer dir.Close()

	ft := document.NewFieldTypeFrom(document.TextFieldTypeNotStored)
	ft.SetIndexOptions(index.IndexOptionsDocs)

	doc := document.NewDocument()
	field, err := document.NewField("field", "this field should be indexed", ft)
	if err != nil {
		t.Fatalf("Failed to create field: %v", err)
	}
	doc.Add(field)
	if err := w.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument failed: %v", err)
	}

	doc2 := document.NewDocument()
	doc2.Add(cannedFreqField(t, []string{"foo", "bar"}, []int{3, math.MaxInt32}, ft))
	if err := w.AddDocument(doc2); err == nil {
		t.Fatal("expected overflow error on term freq sum")
	}

	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader failed: %v", err)
	}
	defer r.Close()
	if got := r.NumDocs(); got != 1 {
		t.Errorf("numDocs = %d, want 1", got)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

// TestCustomTermFreq_InvalidTermVectorPositions ports
// org.apache.lucene.index.TestCustomTermFreq#testInvalidTermVectorPositions.
func TestCustomTermFreq_InvalidTermVectorPositions(t *testing.T) {
	dir, w := newCustomFreqWriter(t)
	defer dir.Close()
	defer w.Close()

	doc := document.NewDocument()
	ft := docsAndFreqsType()
	ft.SetStoreTermVectors(true)
	ft.SetStoreTermVectorPositions(true)
	doc.Add(cannedFreqField(t, []string{"foo", "bar", "foo", "bar"}, []int{42, 128, 17, 100}, ft))

	err := w.AddDocument(doc)
	if err == nil {
		t.Fatal("expected error indexing term vector positions with custom TermFrequencyAttribute")
	}
	const want = `field "field": cannot index term vector positions while using custom TermFrequencyAttribute`
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

// TestCustomTermFreq_InvalidTermVectorOffsets ports
// org.apache.lucene.index.TestCustomTermFreq#testInvalidTermVectorOffsets.
func TestCustomTermFreq_InvalidTermVectorOffsets(t *testing.T) {
	dir, w := newCustomFreqWriter(t)
	defer dir.Close()
	defer w.Close()

	doc := document.NewDocument()
	ft := docsAndFreqsType()
	ft.SetStoreTermVectors(true)
	ft.SetStoreTermVectorOffsets(true)
	doc.Add(cannedFreqField(t, []string{"foo", "bar", "foo", "bar"}, []int{42, 128, 17, 100}, ft))

	err := w.AddDocument(doc)
	if err == nil {
		t.Fatal("expected error indexing term vector offsets with custom TermFrequencyAttribute")
	}
	const want = `field "field": cannot index term vector offsets while using custom TermFrequencyAttribute`
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

// TestCustomTermFreq_TermVectors ports
// org.apache.lucene.index.TestCustomTermFreq#testTermVectors.
func TestCustomTermFreq_TermVectors(t *testing.T) {
	t.Fatal(termVectorsBlocked)

	dir, w := newCustomFreqWriter(t)
	defer dir.Close()

	ft := docsAndFreqsType()
	ft.SetStoreTermVectors(true)
	addCannedDoc(t, w, []string{"foo", "bar", "foo", "bar"}, []int{42, 128, 17, 100}, ft)
	addCannedDoc(t, w, []string{"foo", "bar", "foo", "bar"}, []int{50, 60, 70, 80}, ft)
	commitAndClose(t, w)

	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader failed: %v", err)
	}
	defer r.Close()

	// Expected once term vectors are wired:
	// assertTermVectorDoc(t, r, 0, "bar", 228)
	// assertTermVectorDoc(t, r, 0, "foo", 59)
	// assertTermVectorDoc(t, r, 1, "bar", 140)
	// assertTermVectorDoc(t, r, 1, "foo", 120)
}

// assertTermVectorDoc checks the term-vector entry for a single (docID, term)
// on field "field". For DOCS_AND_FREQS term vectors the total term frequency
// equals the per-document posting frequency.
func assertTermVectorDoc(t *testing.T, r *index.DirectoryReader, docID int, text string, wantFreq int) {
	t.Helper()
	tv, err := r.GetTermVectors(docID)
	if err != nil {
		t.Fatalf("GetTermVectors(%d): %v", docID, err)
	}
	if tv == nil {
		t.Fatalf("GetTermVectors(%d) is nil", docID)
	}
	terms, err := tv.Terms("field")
	if err != nil {
		t.Fatalf("Terms(field) for doc %d: %v", docID, err)
	}
	if terms == nil {
		t.Fatalf("Terms(field) for doc %d is nil", docID)
	}
	te, err := terms.GetIteratorWithSeek(index.NewTerm("field", text))
	if err != nil {
		t.Fatalf("GetIteratorWithSeek(%q) for doc %d: %v", text, docID, err)
	}
	if te == nil {
		t.Fatalf("GetIteratorWithSeek(%q) for doc %d is nil", text, docID)
	}
	found, err := te.SeekExact(index.NewTerm("field", text))
	if err != nil {
		t.Fatalf("SeekExact(%q) for doc %d: %v", text, docID, err)
	}
	if !found {
		t.Fatalf("term %q not found for doc %d", text, docID)
	}
	ttf, err := te.TotalTermFreq()
	if err != nil {
		t.Fatalf("TotalTermFreq(%q) for doc %d: %v", text, docID, err)
	}
	if ttf != int64(wantFreq) {
		t.Errorf("doc %d term %q totalTermFreq = %d, want %d", docID, text, ttf, wantFreq)
	}
	pe, err := te.Postings(index.PostingsFlagFreqs)
	if err != nil {
		t.Fatalf("Postings(%q) for doc %d: %v", text, docID, err)
	}
	if pe == nil {
		t.Fatalf("Postings(%q) for doc %d is nil", text, docID)
	}
	doc, err := pe.NextDoc()
	if err != nil {
		t.Fatalf("NextDoc for doc %d term %q: %v", docID, text, err)
	}
	if doc != docID {
		t.Errorf("doc %d term %q postings doc = %d, want %d", docID, text, doc, docID)
	}
	freq, err := pe.Freq()
	if err != nil {
		t.Fatalf("Freq for doc %d term %q: %v", docID, text, err)
	}
	if freq != wantFreq {
		t.Errorf("doc %d term %q postings freq = %d, want %d", docID, text, freq, wantFreq)
	}
	if next, err := pe.NextDoc(); err != nil {
		t.Fatalf("NextDoc after last for doc %d term %q: %v", docID, text, err)
	} else if next != -1 {
		t.Errorf("doc %d term %q expected no more docs, got %d", docID, text, next)
	}
}

// TestCustomTermFreq_FieldInvertState ports
// org.apache.lucene.index.TestCustomTermFreq#testFieldInvertState. The upstream
// test installs NeverForgetsSimilarity to capture the FieldInvertState and
// asserts its aggregate counters after a single addDocument.
func TestCustomTermFreq_FieldInvertState(t *testing.T) {
	t.Fatal(fieldInvertStateBlocked)

	dir, w := newCustomFreqWriter(t)
	defer dir.Close()
	defer w.Close()

	addCannedDoc(t, w,
		[]string{"foo", "bar", "foo", "bar"}, []int{42, 128, 17, 100}, docsAndFreqsType())

	// FieldInvertState capture deferred. Expected after addDocument:
	// maxTermFrequency 228, uniqueTermCount 2, numOverlap 0, length 287.
}
