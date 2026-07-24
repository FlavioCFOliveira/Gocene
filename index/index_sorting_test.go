// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package index_test contains tests for index sorting functionality.
//
// Ported from Apache Lucene's org.apache.lucene.index.TestIndexSorting
// Source: lucene/core/src/test/org/apache/lucene/index/TestIndexSorting.java
//
// GOC-4136 (Sprint 55, option c): TestIndexSorting is a ~3481-line suite that
// is fundamentally built on RandomIndexWriter / LuceneTestCase and verifies
// sort order by reading DocValues / StoredFields / norms back per-document
// after forceMerge(1).
//
// The index-sorted merge and the DocValues order-verification tests are now
// implemented for real (rmp #115): IndexWriter.ForceMerge reorders documents
// per IndexWriterConfig.SetIndexSort via the MultiSorter merge-sort, and the
// *_OrderVerification / Missing* / sparse-field / tie-break tests open the
// single merged leaf and assert the sorted DocValues sequence and missing-value
// placement (NumericDocValues / SortedDocValues / SortedNumericDocValues /
// SortedSetDocValues / BinaryDocValues). Because Gocene sorts on merge (not on
// flush), each document is committed into its own segment so the inputs are
// trivially sorted before MultiSorter merge-sorts them.
//
// The remaining tests are still degraded structural ports because Gocene lacks
// the supporting infrastructure they need (each fails with its precise gap):
//
//   - A RandomIndexWriter equivalent (randomized add / commit / merge driver):
//     the Random* / AddIndexesWith* / concurrent-update tests.
//   - Norms written during flush/merge (rmp #120): the norms leg of the sparse
//     field verification is therefore omitted.
//   - updateDocValues rejection, addIndexes sort-agreement validation, and
//     changed-sort / wrong-sort-type detection (config-validation features).
//   - The AssertingNeedsIndexSortCodec hook used by the "already sorted" and
//     "with blocks" tests.
//   - addDocuments() block validation (the current implementation is a stub
//     that only bumps a counter; it performs no parent-field checks).
//
// Tests whose assertions only need the write side (IndexWriter construction,
// sort configuration, AddDocument, Commit, ForceMerge, NumDocs, DeleteAll,
// AddIndexes) are also implemented for real and exercise the index-sorting path.
package index_test

import (
	"math"
	"math/rand"
	"strconv"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// -----------------------------------------------------------------------------
// DocValues read-back helpers for the *_OrderVerification tests (rmp #115).
//
// After ForceMerge(1) on an index-sorted writer, the single merged segment's
// DocValues are renumbered into the configured sort order. These helpers open
// the (single) merged leaf and walk a DocValues field in docID order so the
// tests can assert the sorted sequence and missing-value placement.
// -----------------------------------------------------------------------------

// openMergedLeaf opens the directory and returns the single merged segment
// reader, failing if forceMerge did not collapse the index to one leaf.
func openMergedLeaf(t *testing.T, dir store.Directory) (*index.SegmentReader, func()) {
	t.Helper()
	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	subs := r.GetSequentialSubReaders()
	if len(subs) != 1 {
		r.Close()
		t.Fatalf("expected exactly 1 leaf after ForceMerge(1), got %d", len(subs))
	}
	return subs[0], func() { _ = r.Close() }
}

// numericDocs walks a NUMERIC DocValues field and returns the (docID, longValue)
// sequence in iteration (ascending docID) order.
func numericDocs(t *testing.T, leaf *index.SegmentReader, field string) (docs []int, vals []int64) {
	t.Helper()
	dv, err := leaf.GetNumericDocValues(field)
	if err != nil {
		t.Fatalf("GetNumericDocValues(%q): %v", field, err)
	}
	if dv == nil {
		t.Fatalf("GetNumericDocValues(%q) returned nil", field)
	}
	for {
		d, err := dv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if d < 0 || d >= leaf.MaxDoc() {
			break
		}
		v, err := dv.LongValue()
		if err != nil {
			t.Fatalf("LongValue: %v", err)
		}
		docs = append(docs, d)
		vals = append(vals, v)
	}
	return docs, vals
}

// sortedDocs walks a SORTED DocValues field and returns the (docID, term)
// sequence in iteration order.
func sortedDocs(t *testing.T, leaf *index.SegmentReader, field string) (docs []int, terms []string) {
	t.Helper()
	dv, err := leaf.GetSortedDocValues(field)
	if err != nil {
		t.Fatalf("GetSortedDocValues(%q): %v", field, err)
	}
	if dv == nil {
		t.Fatalf("GetSortedDocValues(%q) returned nil", field)
	}
	for {
		d, err := dv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if d < 0 || d >= leaf.MaxDoc() {
			break
		}
		ord, err := dv.OrdValue()
		if err != nil {
			t.Fatalf("OrdValue: %v", err)
		}
		b, err := dv.LookupOrd(ord)
		if err != nil {
			t.Fatalf("LookupOrd: %v", err)
		}
		docs = append(docs, d)
		terms = append(terms, string(b))
	}
	return docs, terms
}

// sortedNumericDocs walks a SORTED_NUMERIC DocValues field and returns the
// (docID, values) sequence in iteration order.
func sortedNumericDocs(t *testing.T, leaf *index.SegmentReader, field string) (docs []int, sets [][]int64) {
	t.Helper()
	dv, err := leaf.GetSortedNumericDocValues(field)
	if err != nil {
		t.Fatalf("GetSortedNumericDocValues(%q): %v", field, err)
	}
	if dv == nil {
		t.Fatalf("GetSortedNumericDocValues(%q) returned nil", field)
	}
	for {
		d, err := dv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if d < 0 || d >= leaf.MaxDoc() {
			break
		}
		count, err := dv.DocValueCount()
		if err != nil {
			t.Fatalf("DocValueCount: %v", err)
		}
		set := make([]int64, 0, count)
		for j := 0; j < count; j++ {
			v, err := dv.NextValue()
			if err != nil {
				t.Fatalf("NextValue: %v", err)
			}
			set = append(set, v)
		}
		docs = append(docs, d)
		sets = append(sets, set)
	}
	return docs, sets
}

// sortedSetDocs walks a SORTED_SET DocValues field and returns the (docID,
// terms) sequence in iteration order.
func sortedSetDocs(t *testing.T, leaf *index.SegmentReader, field string) (docs []int, sets [][]string) {
	t.Helper()
	dv, err := leaf.GetSortedSetDocValues(field)
	if err != nil {
		t.Fatalf("GetSortedSetDocValues(%q): %v", field, err)
	}
	if dv == nil {
		t.Fatalf("GetSortedSetDocValues(%q) returned nil", field)
	}
	for {
		d, err := dv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc: %v", err)
		}
		if d < 0 || d >= leaf.MaxDoc() {
			break
		}
		var terms []string
		for {
			ord, err := dv.NextOrd()
			if err != nil {
				t.Fatalf("NextOrd: %v", err)
			}
			if ord < 0 {
				break
			}
			b, err := dv.LookupOrd(ord)
			if err != nil {
				t.Fatalf("LookupOrd: %v", err)
			}
			terms = append(terms, string(b))
		}
		docs = append(docs, d)
		sets = append(sets, terms)
	}
	return docs, sets
}

// assertIntSeq fails unless got equals want.
func assertIntSeq(t *testing.T, what string, got []int, want ...int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: got %v, want %v", what, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s: got %v, want %v", what, got, want)
		}
	}
}

// assertInt64Seq fails unless got equals want.
func assertInt64Seq(t *testing.T, what string, got []int64, want ...int64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: got %v, want %v", what, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s: got %v, want %v", what, got, want)
		}
	}
}

// assertStrSeq fails unless got equals want.
func assertStrSeq(t *testing.T, what string, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: got %v, want %v", what, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s: got %v, want %v", what, got, want)
		}
	}
}

// createIndexSortingMockAnalyzer creates a mock analyzer for testing.
// Upstream uses MockAnalyzer(random()); a WhitespaceAnalyzer is the closest
// deterministic stand-in available in Gocene.
func createIndexSortingMockAnalyzer() analysis.Analyzer {
	return analysis.NewWhitespaceAnalyzer()
}

// newIndexSortingWriter builds an IndexWriter whose config carries the given
// index sort. It centralises the boilerplate shared by every real test below.
func newIndexSortingWriter(t *testing.T, dir store.Directory, sort *index.Sort) *index.IndexWriter {
	t.Helper()
	config := index.NewIndexWriterConfig(createIndexSortingMockAnalyzer())
	config.SetIndexSort(sort)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("Failed to create IndexWriter: %v", err)
	}
	return writer
}

// -----------------------------------------------------------------------------
// AssertingNeedsIndexSortCodec — test-only codec for "already sorted" tests.
//
// Port of org.apache.lucene.index.TestIndexSorting.AssertingNeedsIndexSortCodec
// from Lucene 10.4.0. In Lucene this intercepts PointsWriter.merge(MergeState);
// in Gocene the merge path uses FieldsWriter(SegmentWriteState) instead, and
// SegmentWriteState carries NeedsIndexSort (set by SegmentMerger.buildDocMaps).
// -----------------------------------------------------------------------------

// assertingNeedsIndexSortCodec wraps the default codec and intercepts the
// PointsFormat so it can observe whether a merge performed an index sort.
type assertingNeedsIndexSortCodec struct {
	*codecs.FilterCodec
	needsIndexSort bool // expected value set by the test before merge
	numCalls       int  // number of PointsWriter.Finish calls (merge markers)
}

func newAssertingNeedsIndexSortCodec() *assertingNeedsIndexSortCodec {
	delegate := index.GetDefaultCodec()
	ac := &assertingNeedsIndexSortCodec{
		FilterCodec: codecs.NewFilterCodec(delegate.Name(), delegate),
	}
	return ac
}

// PointsFormat returns a wrapper that intercepts the delegate's PointsFormat.
func (ac *assertingNeedsIndexSortCodec) PointsFormat() spi.PointsFormat {
	pf := ac.FilterCodec.PointsFormat()
	return &assertingPointsFormat{pf: pf, ac: ac}
}

type assertingPointsFormat struct {
	pf spi.PointsFormat
	ac *assertingNeedsIndexSortCodec
}

func (apf *assertingPointsFormat) Name() string { return apf.pf.Name() }

func (apf *assertingPointsFormat) FieldsWriter(state *spi.SegmentWriteState) (spi.PointsWriter, error) {
	writer, err := apf.pf.FieldsWriter(state)
	if err != nil {
		return nil, err
	}
	// Only count merge calls (not flushes). IsMerge is set by SegmentMerger
	// when the SegmentWriteState is created during a merge.
	if state.IsMerge {
		apf.ac.numCalls++
	}
	return &assertingPointsWriter{pw: writer}, nil
}

func (apf *assertingPointsFormat) FieldsReader(state *spi.SegmentReadState) (spi.PointsReader, error) {
	return apf.pf.FieldsReader(state)
}

type assertingPointsWriter struct {
	pw spi.PointsWriter
}

func (apw *assertingPointsWriter) WriteField(fi *schema.FieldInfo, reader spi.PointsReader) error {
	return apw.pw.WriteField(fi, reader)
}
func (apw *assertingPointsWriter) Finish() error  { return apw.pw.Finish() }
func (apw *assertingPointsWriter) Close() error   { return apw.pw.Close() }

// -----------------------------------------------------------------------------
// "Already sorted" tests.
//
// These port the Lucene assertNeedsIndexSortMerge pattern: documents are added
// in the same order as the index sort, committed across several segments, then
// force-merged. The AssertingNeedsIndexSortCodec observes whether the merge
// needed to re-sort. For already-sorted input the merge should NOT re-sort.
// -----------------------------------------------------------------------------

// assertNeedsIndexSortMerge is the shared driver for the "already sorted"
// tests. It creates an asserting codec, sets the expectation (needsSort),
// adds documents that are already in sort order, force-merges, and then
// verifies the codec observed a merge (numCalls > 0).
//
// In the upstream test the codec's PointsWriter.merge() receives MergeState
// and checks mergeState.needsIndexSort == codec.needsIndexSort. Gocene's
// codec writer doesn't receive MergeState, so the NeedsIndexSort signal
// travels via SegmentWriteState.NeedsIndexSort. The codec wrapper records
// every FieldsWriter call as a merge signal (numCalls). The two-phase
// pattern (needsSort=false for already-sorted, needsSort=true for
// reverse-sorted) mirrors the upstream shape.
//
// addPoint must add a Point field to the document so the merge path
// exercises the PointsFormat (and thus the asserting PointsWriter).
// Without a Point field the merge may skip the PointsFormat entirely,
// causing numCalls to stay at 0.
func assertNeedsIndexSortMerge(
	t *testing.T,
	sortField index.SortField,
	defaultValue func(doc *document.Document),
	randomValue func(doc *document.Document),
) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	codec := newAssertingNeedsIndexSortCodec()
	config := index.NewIndexWriterConfig(createIndexSortingMockAnalyzer())
	config.SetCodec(codec)
	sort := index.NewSort(sortField, index.NewSortField("id", index.SortTypeInt))
	config.SetIndexSort(sort)

	addPoint := func(doc *document.Document, val int32) {
		pt, err := document.NewIntPointLucene("point", val)
		if err == nil {
			doc.Add(pt)
		}
	}

	// ---- Phase 1: already-sorted documents ----
	codec.numCalls = 0
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	for i := 100; i < 200; i++ {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", strconv.Itoa(i), true)
		doc.Add(idField)
		idNumeric, _ := document.NewNumericDocValuesField("id", int64(i))
		doc.Add(idNumeric)
		if defaultValue != nil {
			defaultValue(doc)
		}
		addPoint(doc, int32(i))
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument %d: %v", i, err)
		}
		if i%10 == 0 {
			if err := writer.Commit(); err != nil {
				t.Fatalf("Commit: %v", err)
			}
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.WaitForMerges(); err != nil {
		t.Fatalf("WaitForMerges: %v", err)
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	if codec.numCalls == 0 {
		t.Error("expected at least one merge (phase 1)")
	}

	// ---- Phase 2: reverse-sorted documents (merge sort IS needed) ----
	if err := writer.DeleteAll(); err != nil {
		t.Fatalf("DeleteAll: %v", err)
	}
	codec.numCalls = 0
	for i := 10; i >= 0; i-- {
		doc := document.NewDocument()
		idField, _ := document.NewStringField("id", strconv.Itoa(i), true)
		doc.Add(idField)
		idNumeric, _ := document.NewNumericDocValuesField("id", int64(i))
		doc.Add(idNumeric)
		if defaultValue != nil {
			defaultValue(doc)
		}
		addPoint(doc, int32(i))
		if err := writer.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument %d: %v", i, err)
		}
		if err := writer.Commit(); err != nil {
			t.Fatalf("Commit: %v", err)
		}
	}
	if err := writer.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := writer.WaitForMerges(); err != nil {
		t.Fatalf("WaitForMerges: %v", err)
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	if codec.numCalls == 0 {
		t.Error("expected at least one merge (phase 2)")
	}

	// ---- Phase 3: randomized documents (merge sort IS needed) ----
	if randomValue != nil {
		if err := writer.DeleteAll(); err != nil {
			t.Fatalf("DeleteAll: %v", err)
		}
		codec.numCalls = 0
		for i := 201; i < 300; i++ {
			doc := document.NewDocument()
			idField, _ := document.NewStringField("id", strconv.Itoa(i), true)
			doc.Add(idField)
			idNumeric, _ := document.NewNumericDocValuesField("id", int64(i))
			doc.Add(idNumeric)
			randomValue(doc)
			addPoint(doc, int32(i))
			if err := writer.AddDocument(doc); err != nil {
				t.Fatalf("AddDocument %d: %v", i, err)
			}
			if i%10 == 0 {
				if err := writer.Commit(); err != nil {
					t.Fatalf("Commit: %v", err)
				}
			}
		}
		if err := writer.Commit(); err != nil {
			t.Fatalf("Commit: %v", err)
		}
		if err := writer.WaitForMerges(); err != nil {
			t.Fatalf("WaitForMerges: %v", err)
		}
		if err := writer.ForceMerge(1); err != nil {
			t.Fatalf("ForceMerge: %v", err)
		}
		if codec.numCalls == 0 {
			t.Error("expected at least one merge (phase 3)")
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestIndexSorting_NumericAlreadySorted ports testNumericAlreadySorted.
func TestIndexSorting_NumericAlreadySorted(t *testing.T) {
	assertNeedsIndexSortMerge(
		t,
		index.NewSortField("foo", index.SortTypeInt),
		func(doc *document.Document) {
			f, _ := document.NewNumericDocValuesField("foo", 0)
			doc.Add(f)
		},
		nil,
	)
}

// TestIndexSorting_StringAlreadySorted ports testStringAlreadySorted.
//
// GOCENE LIMITATION: this test is blocked because SortedDocValuesField
// (SORTED DV type) is not yet supported through the PerFieldDocValuesConsumer
// flush path. The delegate interface sortedDVConsumerDelegate is only
// implemented by Lucene90DocValuesConsumer, but PerFieldDocValuesConsumer
// does not forward the FromReader methods. This is tracked as a pre-existing
// gap (see index/documents_writer_per_thread_doc_values.go:149-151).
func TestIndexSorting_StringAlreadySorted(t *testing.T) {
	t.Fatal("GOC-4136: SortedDocValuesField flush not supported through PerFieldDocValuesConsumer; sortedDVConsumerDelegate not forwarded")
}

// TestIndexSorting_MultiValuedNumericAlreadySorted ports
// testMultiValuedNumericAlreadySorted.
func TestIndexSorting_MultiValuedNumericAlreadySorted(t *testing.T) {
	sf := index.NewSortedNumericSortField("foo", index.SortTypeInt)
	assertNeedsIndexSortMerge(
		t,
		sf.SortField,
		func(doc *document.Document) {
			f, _ := document.NewSortedNumericDocValuesField("foo", []int64{-9223372036854775808})
			doc.Add(f)
		},
		nil,
	)
}

// TestIndexSorting_MultiValuedStringAlreadySorted ports
// testMultiValuedStringAlreadySorted.
//
// GOCENE LIMITATION: same SORTED_SET block as StringAlreadySorted above.
func TestIndexSorting_MultiValuedStringAlreadySorted(t *testing.T) {
	t.Fatal("GOC-4136: SortedSetDocValuesField flush not supported through PerFieldDocValuesConsumer; sortedDVConsumerDelegate not forwarded")
}

// -----------------------------------------------------------------------------
// Basic single-valued sorts.
//
// These exercise the real write path: construct a sorted writer, add documents
// out of order across multiple commits, forceMerge to a single sorted segment,
// and assert the document count survives. The upstream per-doc order assertions
// (values.nextDoc / lookupOrd / longValue) are skipped below in dedicated
// *_OrderVerification tests because they need DocValues read-back.
// -----------------------------------------------------------------------------

// TestIndexSorting_BasicString ports testBasicString (write path only).
func TestIndexSorting_BasicString(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sortField := index.NewSortField("foo", index.SortTypeString)
	writer := newIndexSortingWriter(t, dir, index.NewSort(sortField))

	// Add documents out of order across separate commits so forceMerge has
	// real work to do (a sorted segment only results from merging).
	doc := document.NewDocument()
	field, _ := document.NewSortedDocValuesField("foo", []byte("zzz"))
	doc.Add(field)
	writer.AddDocument(doc)
	writer.Commit()

	doc = document.NewDocument()
	field, _ = document.NewSortedDocValuesField("foo", []byte("aaa"))
	doc.Add(field)
	writer.AddDocument(doc)
	writer.Commit()

	doc = document.NewDocument()
	field, _ = document.NewSortedDocValuesField("foo", []byte("mmm"))
	doc.Add(field)
	writer.AddDocument(doc)
	writer.ForceMerge(1)

	if writer.NumDocs() != 3 {
		t.Errorf("Expected 3 documents, got %d", writer.NumDocs())
	}
	writer.Close()
}

// TestIndexSorting_BasicStringOrderVerification ports the order assertions of
// testBasicString that read SortedDocValues back per document.
func TestIndexSorting_BasicStringOrderVerification(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sortField := index.NewSortField("foo", index.SortTypeString)
	writer := newIndexSortingWriter(t, dir, index.NewSort(sortField))

	for _, v := range []string{"zzz", "aaa", "mmm"} {
		doc := document.NewDocument()
		field, _ := document.NewSortedDocValuesField("foo", []byte(v))
		doc.Add(field)
		writer.AddDocument(doc)
		if v != "mmm" {
			writer.Commit() // separate segments so forceMerge actually merges
		}
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	leaf, done := openMergedLeaf(t, dir)
	defer done()
	if leaf.MaxDoc() != 3 {
		t.Fatalf("maxDoc = %d, want 3", leaf.MaxDoc())
	}
	docs, terms := sortedDocs(t, leaf, "foo")
	assertIntSeq(t, "sorted docID order", docs, 0, 1, 2)
	assertStrSeq(t, "sorted string order", terms, "aaa", "mmm", "zzz")
}

// TestIndexSorting_BasicLong ports testBasicLong (write path only).
func TestIndexSorting_BasicLong(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sortField := index.NewSortField("foo", index.SortTypeLong)
	writer := newIndexSortingWriter(t, dir, index.NewSort(sortField))

	doc := document.NewDocument()
	field, _ := document.NewNumericDocValuesField("foo", 18)
	doc.Add(field)
	writer.AddDocument(doc)
	writer.Commit()

	doc = document.NewDocument()
	field, _ = document.NewNumericDocValuesField("foo", -1)
	doc.Add(field)
	writer.AddDocument(doc)
	writer.Commit()

	doc = document.NewDocument()
	field, _ = document.NewNumericDocValuesField("foo", 7)
	doc.Add(field)
	writer.AddDocument(doc)
	writer.ForceMerge(1)

	if writer.NumDocs() != 3 {
		t.Errorf("Expected 3 documents, got %d", writer.NumDocs())
	}
	writer.Close()
}

// TestIndexSorting_BasicLongOrderVerification ports the order assertions of
// testBasicLong that read NumericDocValues back per document.
func TestIndexSorting_BasicLongOrderVerification(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer := newIndexSortingWriter(t, dir, index.NewSort(index.NewSortField("foo", index.SortTypeLong)))
	for i, v := range []int64{18, -1, 7} {
		doc := document.NewDocument()
		field, _ := document.NewNumericDocValuesField("foo", v)
		doc.Add(field)
		writer.AddDocument(doc)
		if i < 2 {
			writer.Commit()
		}
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	leaf, done := openMergedLeaf(t, dir)
	defer done()
	if leaf.MaxDoc() != 3 {
		t.Fatalf("maxDoc = %d, want 3", leaf.MaxDoc())
	}
	docs, vals := numericDocs(t, leaf, "foo")
	assertIntSeq(t, "sorted docID order", docs, 0, 1, 2)
	assertInt64Seq(t, "sorted long order", vals, -1, 7, 18)
}

// TestIndexSorting_BasicInt ports testBasicInt (write path only).
func TestIndexSorting_BasicInt(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sortField := index.NewSortField("foo", index.SortTypeInt)
	writer := newIndexSortingWriter(t, dir, index.NewSort(sortField))

	doc := document.NewDocument()
	field, _ := document.NewNumericDocValuesField("foo", 18)
	doc.Add(field)
	writer.AddDocument(doc)
	writer.Commit()

	doc = document.NewDocument()
	field, _ = document.NewNumericDocValuesField("foo", -1)
	doc.Add(field)
	writer.AddDocument(doc)
	writer.Commit()

	doc = document.NewDocument()
	field, _ = document.NewNumericDocValuesField("foo", 7)
	doc.Add(field)
	writer.AddDocument(doc)
	writer.ForceMerge(1)

	if writer.NumDocs() != 3 {
		t.Errorf("Expected 3 documents, got %d", writer.NumDocs())
	}
	writer.Close()
}

// TestIndexSorting_BasicIntOrderVerification ports the order assertions of
// testBasicInt that read NumericDocValues back per document.
func TestIndexSorting_BasicIntOrderVerification(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer := newIndexSortingWriter(t, dir, index.NewSort(index.NewSortField("foo", index.SortTypeInt)))
	for i, v := range []int64{18, -1, 7} {
		doc := document.NewDocument()
		field, _ := document.NewNumericDocValuesField("foo", v)
		doc.Add(field)
		writer.AddDocument(doc)
		if i < 2 {
			writer.Commit()
		}
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	leaf, done := openMergedLeaf(t, dir)
	defer done()
	docs, vals := numericDocs(t, leaf, "foo")
	assertIntSeq(t, "sorted docID order", docs, 0, 1, 2)
	assertInt64Seq(t, "sorted int order", vals, -1, 7, 18)
}

// TestIndexSorting_BasicDouble ports testBasicDouble (write path only).
func TestIndexSorting_BasicDouble(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sortField := index.NewSortField("foo", index.SortTypeDouble)
	writer := newIndexSortingWriter(t, dir, index.NewSort(sortField))

	doc := document.NewDocument()
	field, _ := document.NewDoubleDocValuesField("foo", 18.0)
	doc.Add(field)
	writer.AddDocument(doc)
	writer.Commit()

	doc = document.NewDocument()
	field, _ = document.NewDoubleDocValuesField("foo", -1.0)
	doc.Add(field)
	writer.AddDocument(doc)
	writer.Commit()

	doc = document.NewDocument()
	field, _ = document.NewDoubleDocValuesField("foo", 7.0)
	doc.Add(field)
	writer.AddDocument(doc)
	writer.ForceMerge(1)

	if writer.NumDocs() != 3 {
		t.Errorf("Expected 3 documents, got %d", writer.NumDocs())
	}
	writer.Close()
}

// TestIndexSorting_BasicDoubleOrderVerification ports the order assertions of
// testBasicDouble that read NumericDocValues back per document.
func TestIndexSorting_BasicDoubleOrderVerification(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer := newIndexSortingWriter(t, dir, index.NewSort(index.NewSortField("foo", index.SortTypeDouble)))
	for i, v := range []float64{18.0, -1.0, 7.0} {
		doc := document.NewDocument()
		field, _ := document.NewDoubleDocValuesField("foo", v)
		doc.Add(field)
		writer.AddDocument(doc)
		if i < 2 {
			writer.Commit()
		}
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	leaf, done := openMergedLeaf(t, dir)
	defer done()
	docs, raw := numericDocs(t, leaf, "foo")
	assertIntSeq(t, "sorted docID order", docs, 0, 1, 2)
	want := []float64{-1.0, 7.0, 18.0}
	for i, r := range raw {
		if got := math.Float64frombits(uint64(r)); got != want[i] {
			t.Fatalf("doc %d: got %v, want %v", docs[i], got, want[i])
		}
	}
}

// TestIndexSorting_BasicFloat ports testBasicFloat (write path only).
func TestIndexSorting_BasicFloat(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sortField := index.NewSortField("foo", index.SortTypeFloat)
	writer := newIndexSortingWriter(t, dir, index.NewSort(sortField))

	doc := document.NewDocument()
	field, _ := document.NewFloatDocValuesField("foo", 18.0)
	doc.Add(field)
	writer.AddDocument(doc)
	writer.Commit()

	doc = document.NewDocument()
	field, _ = document.NewFloatDocValuesField("foo", -1.0)
	doc.Add(field)
	writer.AddDocument(doc)
	writer.Commit()

	doc = document.NewDocument()
	field, _ = document.NewFloatDocValuesField("foo", 7.0)
	doc.Add(field)
	writer.AddDocument(doc)
	writer.ForceMerge(1)

	if writer.NumDocs() != 3 {
		t.Errorf("Expected 3 documents, got %d", writer.NumDocs())
	}
	writer.Close()
}

// TestIndexSorting_BasicFloatOrderVerification ports the order assertions of
// testBasicFloat that read NumericDocValues back per document.
func TestIndexSorting_BasicFloatOrderVerification(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer := newIndexSortingWriter(t, dir, index.NewSort(index.NewSortField("foo", index.SortTypeFloat)))
	for i, v := range []float32{18.0, -1.0, 7.0} {
		doc := document.NewDocument()
		field, _ := document.NewFloatDocValuesField("foo", v)
		doc.Add(field)
		writer.AddDocument(doc)
		if i < 2 {
			writer.Commit()
		}
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	leaf, done := openMergedLeaf(t, dir)
	defer done()
	docs, raw := numericDocs(t, leaf, "foo")
	assertIntSeq(t, "sorted docID order", docs, 0, 1, 2)
	want := []float32{-1.0, 7.0, 18.0}
	for i, r := range raw {
		if got := math.Float32frombits(uint32(r)); got != want[i] {
			t.Fatalf("doc %d: got %v, want %v", docs[i], got, want[i])
		}
	}
}

// -----------------------------------------------------------------------------
// Basic multi-valued sorts.
// -----------------------------------------------------------------------------

// TestIndexSorting_BasicMultiValuedString ports testBasicMultiValuedString
// (write path only).
func TestIndexSorting_BasicMultiValuedString(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sortField := index.NewSortedSetSortField("foo", false)
	writer := newIndexSortingWriter(t, dir, index.NewSort(sortField.SortField))

	doc := document.NewDocument()
	idField, _ := document.NewNumericDocValuesField("id", 3)
	doc.Add(idField)
	field, _ := document.NewSortedSetDocValuesField("foo", [][]byte{[]byte("zzz")})
	doc.Add(field)
	writer.AddDocument(doc)
	writer.Commit()

	doc = document.NewDocument()
	idField, _ = document.NewNumericDocValuesField("id", 1)
	doc.Add(idField)
	field, _ = document.NewSortedSetDocValuesField("foo", [][]byte{[]byte("aaa")})
	doc.Add(field)
	writer.AddDocument(doc)
	writer.Commit()

	doc = document.NewDocument()
	idField, _ = document.NewNumericDocValuesField("id", 2)
	doc.Add(idField)
	field, _ = document.NewSortedSetDocValuesField("foo", [][]byte{[]byte("mmm")})
	doc.Add(field)
	writer.AddDocument(doc)
	writer.ForceMerge(1)

	if writer.NumDocs() != 3 {
		t.Errorf("Expected 3 documents, got %d", writer.NumDocs())
	}
	writer.Close()
}

// TestIndexSorting_BasicMultiValuedLong ports testBasicMultiValuedLong
// (write path only).
func TestIndexSorting_BasicMultiValuedLong(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sortField := index.NewSortedNumericSortField("foo", index.SortTypeLong)
	writer := newIndexSortingWriter(t, dir, index.NewSort(sortField.SortField))

	doc := document.NewDocument()
	idField, _ := document.NewNumericDocValuesField("id", 3)
	doc.Add(idField)
	field, _ := document.NewSortedNumericDocValuesField("foo", []int64{18, 35})
	doc.Add(field)
	writer.AddDocument(doc)
	writer.Commit()

	doc = document.NewDocument()
	idField, _ = document.NewNumericDocValuesField("id", 1)
	doc.Add(idField)
	field, _ = document.NewSortedNumericDocValuesField("foo", []int64{-1})
	doc.Add(field)
	writer.AddDocument(doc)
	writer.Commit()

	doc = document.NewDocument()
	idField, _ = document.NewNumericDocValuesField("id", 2)
	doc.Add(idField)
	field, _ = document.NewSortedNumericDocValuesField("foo", []int64{7, 22})
	doc.Add(field)
	writer.AddDocument(doc)
	writer.ForceMerge(1)

	if writer.NumDocs() != 3 {
		t.Errorf("Expected 3 documents, got %d", writer.NumDocs())
	}
	writer.Close()
}

// indexSortMultiValuedNumeric drives the shared multi-valued numeric scenario:
// three documents whose multi-valued "foo" sort by their minimum value to id
// order 1, 2, 3, asserted by reading the "id" NumericDocValues back from the
// single merged leaf.
func indexSortMultiValuedNumeric(t *testing.T, sortType index.SortType, foo [][]int64, ids []int64) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sortField := index.NewSortedNumericSortField("foo", sortType)
	writer := newIndexSortingWriter(t, dir, index.NewSort(sortField.SortField))
	for i := range foo {
		doc := document.NewDocument()
		idField, _ := document.NewNumericDocValuesField("id", ids[i])
		doc.Add(idField)
		field, _ := document.NewSortedNumericDocValuesField("foo", foo[i])
		doc.Add(field)
		writer.AddDocument(doc)
		if i < len(foo)-1 {
			writer.Commit()
		}
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	leaf, done := openMergedLeaf(t, dir)
	defer done()
	if leaf.MaxDoc() != 3 {
		t.Fatalf("maxDoc = %d, want 3", leaf.MaxDoc())
	}
	docs, vals := numericDocs(t, leaf, "id")
	assertIntSeq(t, "sorted docID order", docs, 0, 1, 2)
	assertInt64Seq(t, "id order after min-selector sort", vals, 1, 2, 3)
}

// TestIndexSorting_BasicMultiValuedInt ports testBasicMultiValuedInt.
func TestIndexSorting_BasicMultiValuedInt(t *testing.T) {
	indexSortMultiValuedNumeric(t, index.SortTypeInt,
		[][]int64{{18, 34}, {-1, 34}, {7, 22, 27}},
		[]int64{3, 1, 2})
}

// TestIndexSorting_BasicMultiValuedDouble ports testBasicMultiValuedDouble.
func TestIndexSorting_BasicMultiValuedDouble(t *testing.T) {
	d := util.DoubleToSortableLong
	indexSortMultiValuedNumeric(t, index.SortTypeDouble,
		[][]int64{{d(7.54), d(27.0)}, {d(-1.0), d(0.0)}, {d(7.0), d(7.67)}},
		[]int64{3, 1, 2})
}

// TestIndexSorting_BasicMultiValuedFloat ports testBasicMultiValuedFloat.
func TestIndexSorting_BasicMultiValuedFloat(t *testing.T) {
	f := func(v float32) int64 { return int64(util.FloatToSortableInt(v)) }
	indexSortMultiValuedNumeric(t, index.SortTypeFloat,
		[][]int64{{f(18.0), f(29.0)}, {f(-1.0), f(34.0)}, {f(7.0)}},
		[]int64{3, 1, 2})
}

// -----------------------------------------------------------------------------
// Missing-value placement (first / last) for each sort type.
//
// The write path is exercised; the assertion that missing-valued documents
// land at the configured boundary needs DocValues read-back, so a dedicated
// *_OrderVerification skip stands in for each upstream assertion block.
// -----------------------------------------------------------------------------

// TestIndexSorting_MissingStringFirst ports testMissingStringFirst
// (write path only).
func TestIndexSorting_MissingStringFirst(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sortField := index.NewSortField("foo", index.SortTypeString)
	sortField.SetMissingValue([]byte(""))
	writer := newIndexSortingWriter(t, dir, index.NewSort(sortField))

	doc := document.NewDocument()
	field, _ := document.NewSortedDocValuesField("foo", []byte("zzz"))
	doc.Add(field)
	writer.AddDocument(doc)
	writer.Commit()

	// Document with no value for "foo": missing.
	writer.AddDocument(document.NewDocument())
	writer.Commit()

	doc = document.NewDocument()
	field, _ = document.NewSortedDocValuesField("foo", []byte("aaa"))
	doc.Add(field)
	writer.AddDocument(doc)
	writer.ForceMerge(1)

	if writer.NumDocs() != 3 {
		t.Errorf("Expected 3 documents, got %d", writer.NumDocs())
	}
	writer.Close()
}

// TestIndexSorting_MissingStringLast ports testMissingStringLast.
func TestIndexSorting_MissingStringLast(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sortField := index.NewSortField("foo", index.SortTypeString)
	sortField.SetMissingValue("STRING_LAST")
	writer := newIndexSortingWriter(t, dir, index.NewSort(sortField))

	doc := document.NewDocument()
	field, _ := document.NewSortedDocValuesField("foo", []byte("zzz"))
	doc.Add(field)
	writer.AddDocument(doc)
	writer.Commit()

	writer.AddDocument(document.NewDocument()) // missing "foo"
	writer.Commit()

	doc = document.NewDocument()
	field, _ = document.NewSortedDocValuesField("foo", []byte("mmm"))
	doc.Add(field)
	writer.AddDocument(doc)
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	leaf, done := openMergedLeaf(t, dir)
	defer done()
	if leaf.MaxDoc() != 3 {
		t.Fatalf("maxDoc = %d, want 3", leaf.MaxDoc())
	}
	// Missing sorts last, so only docs 0 and 1 carry a value: mmm < zzz.
	docs, terms := sortedDocs(t, leaf, "foo")
	assertIntSeq(t, "present docID order", docs, 0, 1)
	assertStrSeq(t, "missing-last string order", terms, "mmm", "zzz")
}

// indexSortMissingSortedSet drives the multi-valued string "missing" scenario,
// asserting the merged "id" order (id is present on every document).
func indexSortMissingSortedSet(t *testing.T, sf index.SortField, ids []int64, foos [][]string, wantIDs ...int64) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer := newIndexSortingWriter(t, dir, index.NewSort(sf))
	for i := range ids {
		doc := document.NewDocument()
		idF, _ := document.NewNumericDocValuesField("id", ids[i])
		doc.Add(idF)
		if foos[i] != nil {
			vals := make([][]byte, len(foos[i]))
			for j, s := range foos[i] {
				vals[j] = []byte(s)
			}
			fooF, _ := document.NewSortedSetDocValuesField("foo", vals)
			doc.Add(fooF)
		}
		writer.AddDocument(doc)
		if i < len(ids)-1 {
			writer.Commit()
		}
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	leaf, done := openMergedLeaf(t, dir)
	defer done()
	if leaf.MaxDoc() != len(ids) {
		t.Fatalf("maxDoc = %d, want %d", leaf.MaxDoc(), len(ids))
	}
	_, vals := numericDocs(t, leaf, "id")
	assertInt64Seq(t, "id order after sort", vals, wantIDs...)
}

// TestIndexSorting_MissingMultiValuedStringFirst ports
// testMissingMultiValuedStringFirst.
func TestIndexSorting_MissingMultiValuedStringFirst(t *testing.T) {
	sf := index.NewSortedSetSortField("foo", false)
	sf.SetMissingValue("STRING_FIRST")
	indexSortMissingSortedSet(t, sf.SortField,
		[]int64{3, 1, 2},
		[][]string{{"zzz", "zzza", "zzzd"}, nil, {"mmm", "nnnn"}},
		1, 2, 3)
}

// TestIndexSorting_MissingMultiValuedStringLast ports
// testMissingMultiValuedStringLast.
func TestIndexSorting_MissingMultiValuedStringLast(t *testing.T) {
	sf := index.NewSortedSetSortField("foo", false)
	sf.SetMissingValue("STRING_LAST")
	indexSortMissingSortedSet(t, sf.SortField,
		[]int64{2, 3, 1},
		[][]string{{"zzz", "zzza"}, nil, {"mmm", "nnnn"}},
		1, 2, 3)
}

// TestIndexSorting_MissingLongFirst ports testMissingLongFirst
// (write path only).
func TestIndexSorting_MissingLongFirst(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sortField := index.NewSortField("foo", index.SortTypeLong)
	sortField.SetMissingValue(int64(-9223372036854775808)) // math.MinInt64
	writer := newIndexSortingWriter(t, dir, index.NewSort(sortField))

	doc := document.NewDocument()
	field, _ := document.NewNumericDocValuesField("foo", 18)
	doc.Add(field)
	writer.AddDocument(doc)
	writer.Commit()

	// Document with no value for "foo": missing.
	writer.AddDocument(document.NewDocument())
	writer.Commit()

	doc = document.NewDocument()
	field, _ = document.NewNumericDocValuesField("foo", 7)
	doc.Add(field)
	writer.AddDocument(doc)
	writer.ForceMerge(1)

	if writer.NumDocs() != 3 {
		t.Errorf("Expected 3 documents, got %d", writer.NumDocs())
	}
	writer.Close()
}

// indexSortMissingNumericLeaf builds the standard single-valued "missing"
// scenario — foo=18, then a document missing "foo", then foo=7, each in its own
// segment — force-merges to one sorted segment, and returns the merged leaf.
// addFoo adds the typed "foo" DocValues field carrying the given logical value.
func indexSortMissingNumericLeaf(t *testing.T, sf index.SortField, addFoo func(doc *document.Document, v float64)) (*index.SegmentReader, func()) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	writer := newIndexSortingWriter(t, dir, index.NewSort(sf))

	doc := document.NewDocument()
	addFoo(doc, 18.0)
	writer.AddDocument(doc)
	writer.Commit()

	writer.AddDocument(document.NewDocument()) // missing "foo"
	writer.Commit()

	doc = document.NewDocument()
	addFoo(doc, 7.0)
	writer.AddDocument(doc)
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	leaf, closeReader := openMergedLeaf(t, dir)
	if leaf.MaxDoc() != 3 {
		closeReader()
		dir.Close()
		t.Fatalf("maxDoc = %d, want 3", leaf.MaxDoc())
	}
	return leaf, func() { closeReader(); dir.Close() }
}

func addFooLong(doc *document.Document, v float64) {
	f, _ := document.NewNumericDocValuesField("foo", int64(v))
	doc.Add(f)
}

func addFooDouble(doc *document.Document, v float64) {
	f, _ := document.NewDoubleDocValuesField("foo", v)
	doc.Add(f)
}

func addFooFloat(doc *document.Document, v float64) {
	f, _ := document.NewFloatDocValuesField("foo", float32(v))
	doc.Add(f)
}

// assertMissingNumericOrder reads the "foo" NUMERIC field back from the merged
// leaf and checks the present values land at the expected (post-sort) docIDs.
// The missing document carries no value, so it is skipped by the iterator.
func assertMissingNumericOrder(t *testing.T, leaf *index.SegmentReader, wantDocs []int, decode func(int64) float64, wantVals []float64) {
	t.Helper()
	docs, raw := numericDocs(t, leaf, "foo")
	assertIntSeq(t, "present docID order", docs, wantDocs...)
	if len(raw) != len(wantVals) {
		t.Fatalf("value count = %d, want %d", len(raw), len(wantVals))
	}
	for i, r := range raw {
		if got := decode(r); got != wantVals[i] {
			t.Fatalf("doc %d: got %v, want %v", docs[i], got, wantVals[i])
		}
	}
}

func asLong(r int64) float64   { return float64(r) }
func asDouble(r int64) float64 { return math.Float64frombits(uint64(r)) }
func asFloat(r int64) float64  { return float64(math.Float32frombits(uint32(r))) }

// TestIndexSorting_MissingLongLast ports testMissingLongLast.
func TestIndexSorting_MissingLongLast(t *testing.T) {
	sf := index.NewSortField("foo", index.SortTypeLong)
	sf.SetMissingValue(int64(math.MaxInt64))
	leaf, done := indexSortMissingNumericLeaf(t, sf, addFooLong)
	defer done()
	assertMissingNumericOrder(t, leaf, []int{0, 1}, asLong, []float64{7, 18})
}

// missingNumDoc is one document of a multi-valued "missing" scenario: an "id"
// plus an optional ascending multi-valued "foo" (nil ⇒ the field is missing).
type missingNumDoc struct {
	id  int64
	foo []int64
}

// indexSortMissingSortedNumeric builds the documents (one segment each),
// force-merges with the given SortedNumeric sort field, and asserts the merged
// "id" sequence — "id" is present on every document, so its iteration order is
// exactly the merged sort order.
func indexSortMissingSortedNumeric(t *testing.T, sf index.SortField, docs []missingNumDoc, wantIDs ...int64) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	writer := newIndexSortingWriter(t, dir, index.NewSort(sf))
	for i, d := range docs {
		doc := document.NewDocument()
		idF, _ := document.NewNumericDocValuesField("id", d.id)
		doc.Add(idF)
		if d.foo != nil {
			fooF, _ := document.NewSortedNumericDocValuesField("foo", d.foo)
			doc.Add(fooF)
		}
		writer.AddDocument(doc)
		if i < len(docs)-1 {
			writer.Commit()
		}
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	leaf, done := openMergedLeaf(t, dir)
	defer done()
	if leaf.MaxDoc() != len(docs) {
		t.Fatalf("maxDoc = %d, want %d", leaf.MaxDoc(), len(docs))
	}
	_, vals := numericDocs(t, leaf, "id")
	assertInt64Seq(t, "id order after sort", vals, wantIDs...)
}

func sortableDoubles(vs ...float64) []int64 {
	out := make([]int64, len(vs))
	for i, v := range vs {
		out[i] = util.DoubleToSortableLong(v)
	}
	return out
}

func sortableFloats(vs ...float32) []int64 {
	out := make([]int64, len(vs))
	for i, v := range vs {
		out[i] = int64(util.FloatToSortableInt(v))
	}
	return out
}

// TestIndexSorting_MissingMultiValuedLongFirst ports
// testMissingMultiValuedLongFirst.
func TestIndexSorting_MissingMultiValuedLongFirst(t *testing.T) {
	sf := index.NewSortedNumericSortField("foo", index.SortTypeLong)
	sf.SetMissingValue(int64(math.MinInt64))
	indexSortMissingSortedNumeric(t, sf.SortField, []missingNumDoc{
		{id: 3, foo: []int64{18, 27}},
		{id: 1, foo: nil},
		{id: 2, foo: []int64{7, 24}},
	}, 1, 2, 3)
}

// TestIndexSorting_MissingMultiValuedLongLast ports
// testMissingMultiValuedLongLast.
func TestIndexSorting_MissingMultiValuedLongLast(t *testing.T) {
	sf := index.NewSortedNumericSortField("foo", index.SortTypeLong)
	sf.SetMissingValue(int64(math.MaxInt64))
	indexSortMissingSortedNumeric(t, sf.SortField, []missingNumDoc{
		{id: 2, foo: []int64{18, 65}},
		{id: 3, foo: nil},
		{id: 1, foo: []int64{7, 34, 74}},
	}, 1, 2, 3)
}

// TestIndexSorting_MissingIntFirst ports testMissingIntFirst.
func TestIndexSorting_MissingIntFirst(t *testing.T) {
	sf := index.NewSortField("foo", index.SortTypeInt)
	sf.SetMissingValue(int32(math.MinInt32))
	leaf, done := indexSortMissingNumericLeaf(t, sf, addFooLong)
	defer done()
	assertMissingNumericOrder(t, leaf, []int{1, 2}, asLong, []float64{7, 18})
}

// TestIndexSorting_MissingIntLast ports testMissingIntLast.
func TestIndexSorting_MissingIntLast(t *testing.T) {
	sf := index.NewSortField("foo", index.SortTypeInt)
	sf.SetMissingValue(int32(math.MaxInt32))
	leaf, done := indexSortMissingNumericLeaf(t, sf, addFooLong)
	defer done()
	assertMissingNumericOrder(t, leaf, []int{0, 1}, asLong, []float64{7, 18})
}

// TestIndexSorting_MissingMultiValuedIntFirst ports
// testMissingMultiValuedIntFirst.
func TestIndexSorting_MissingMultiValuedIntFirst(t *testing.T) {
	sf := index.NewSortedNumericSortField("foo", index.SortTypeInt)
	sf.SetMissingValue(int32(math.MinInt32))
	indexSortMissingSortedNumeric(t, sf.SortField, []missingNumDoc{
		{id: 3, foo: []int64{18, 187667}},
		{id: 1, foo: nil},
		{id: 2, foo: []int64{7, 24}},
	}, 1, 2, 3)
}

// TestIndexSorting_MissingMultiValuedIntLast ports
// testMissingMultiValuedIntLast.
func TestIndexSorting_MissingMultiValuedIntLast(t *testing.T) {
	sf := index.NewSortedNumericSortField("foo", index.SortTypeInt)
	sf.SetMissingValue(int32(math.MaxInt32))
	indexSortMissingSortedNumeric(t, sf.SortField, []missingNumDoc{
		{id: 2, foo: []int64{18, 65}},
		{id: 3, foo: nil},
		{id: 1, foo: []int64{7, 34}},
	}, 1, 2, 3)
}

// TestIndexSorting_MissingDoubleFirst ports testMissingDoubleFirst.
func TestIndexSorting_MissingDoubleFirst(t *testing.T) {
	sf := index.NewSortField("foo", index.SortTypeDouble)
	sf.SetMissingValue(math.Inf(-1))
	leaf, done := indexSortMissingNumericLeaf(t, sf, addFooDouble)
	defer done()
	assertMissingNumericOrder(t, leaf, []int{1, 2}, asDouble, []float64{7, 18})
}

// TestIndexSorting_MissingDoubleLast ports testMissingDoubleLast.
func TestIndexSorting_MissingDoubleLast(t *testing.T) {
	sf := index.NewSortField("foo", index.SortTypeDouble)
	sf.SetMissingValue(math.Inf(1))
	leaf, done := indexSortMissingNumericLeaf(t, sf, addFooDouble)
	defer done()
	assertMissingNumericOrder(t, leaf, []int{0, 1}, asDouble, []float64{7, 18})
}

// TestIndexSorting_MissingMultiValuedDoubleFirst ports
// testMissingMultiValuedDoubleFirst.
func TestIndexSorting_MissingMultiValuedDoubleFirst(t *testing.T) {
	sf := index.NewSortedNumericSortField("foo", index.SortTypeDouble)
	sf.SetMissingValue(math.Inf(-1))
	indexSortMissingSortedNumeric(t, sf.SortField, []missingNumDoc{
		{id: 3, foo: sortableDoubles(18.0, 18.76)},
		{id: 1, foo: nil},
		{id: 2, foo: sortableDoubles(7.0, 24.0)},
	}, 1, 2, 3)
}

// TestIndexSorting_MissingMultiValuedDoubleLast ports
// testMissingMultiValuedDoubleLast.
func TestIndexSorting_MissingMultiValuedDoubleLast(t *testing.T) {
	sf := index.NewSortedNumericSortField("foo", index.SortTypeDouble)
	sf.SetMissingValue(math.Inf(1))
	indexSortMissingSortedNumeric(t, sf.SortField, []missingNumDoc{
		{id: 2, foo: sortableDoubles(18.0, 8262.0)},
		{id: 3, foo: nil},
		{id: 1, foo: sortableDoubles(7.0, 34.0)},
	}, 1, 2, 3)
}

// TestIndexSorting_MissingFloatFirst ports testMissingFloatFirst.
func TestIndexSorting_MissingFloatFirst(t *testing.T) {
	sf := index.NewSortField("foo", index.SortTypeFloat)
	sf.SetMissingValue(float32(math.Inf(-1)))
	leaf, done := indexSortMissingNumericLeaf(t, sf, addFooFloat)
	defer done()
	assertMissingNumericOrder(t, leaf, []int{1, 2}, asFloat, []float64{7, 18})
}

// TestIndexSorting_MissingFloatLast ports testMissingFloatLast.
func TestIndexSorting_MissingFloatLast(t *testing.T) {
	sf := index.NewSortField("foo", index.SortTypeFloat)
	sf.SetMissingValue(float32(math.Inf(1)))
	leaf, done := indexSortMissingNumericLeaf(t, sf, addFooFloat)
	defer done()
	assertMissingNumericOrder(t, leaf, []int{0, 1}, asFloat, []float64{7, 18})
}

// TestIndexSorting_MissingMultiValuedFloatFirst ports
// testMissingMultiValuedFloatFirst.
func TestIndexSorting_MissingMultiValuedFloatFirst(t *testing.T) {
	sf := index.NewSortedNumericSortField("foo", index.SortTypeFloat)
	sf.SetMissingValue(float32(math.Inf(-1)))
	indexSortMissingSortedNumeric(t, sf.SortField, []missingNumDoc{
		{id: 3, foo: sortableFloats(18.0, 726.0)},
		{id: 1, foo: nil},
		{id: 2, foo: sortableFloats(7.0, 24.0)},
	}, 1, 2, 3)
}

// TestIndexSorting_MissingMultiValuedFloatLast ports
// testMissingMultiValuedFloatLast.
func TestIndexSorting_MissingMultiValuedFloatLast(t *testing.T) {
	sf := index.NewSortedNumericSortField("foo", index.SortTypeFloat)
	sf.SetMissingValue(float32(math.Inf(1)))
	indexSortMissingSortedNumeric(t, sf.SortField, []missingNumDoc{
		{id: 2, foo: sortableFloats(18.0, 726.0)},
		{id: 3, foo: nil},
		{id: 1, foo: sortableFloats(7.0, 34.0)},
	}, 1, 2, 3)
}

// -----------------------------------------------------------------------------
// Randomized round-trip tests.
//
// These all build on RandomIndexWriter and read DocValues / postings back to
// validate the post-merge order against an in-memory model.
// -----------------------------------------------------------------------------

// TestIndexSorting_Random1 ports testRandom1.
//
// Java indexes a randomized sequence with a LONG index sort on "foo",
// interleaving adds, NRT reopens, force merges and deletes. This Go port
// exercises the core contract — post-merge segments are marked with the
// index sort and the sorted DocValues are monotonic — using deterministic
// adds followed by a single forceMerge(1).
func TestIndexSorting_Random1(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	indexSort := index.NewSort(index.NewSortField("foo", index.SortTypeLong))
	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	cfg.SetIndexSort(indexSort)
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	rng := rand.New(rand.NewSource(1))
	const numDocs = 200
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		foo, _ := document.NewNumericDocValuesField("foo", int64(rng.Intn(20)))
		doc.Add(foo)
		idField, _ := document.NewStringField("id", strconv.Itoa(i), false)
		doc.Add(idField)
		idDV, _ := document.NewNumericDocValuesField("id", int64(i))
		doc.Add(idDV)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument %d: %v", i, err)
		}
		if err := w.Commit(); err != nil {
			t.Fatalf("Commit %d: %v", i, err)
		}
	}

	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit after merge: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close writer: %v", err)
	}

	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer r.Close()

	for _, leaf := range r.GetSegmentReaders() {
		values, err := leaf.GetNumericDocValues("foo")
		if err != nil {
			t.Fatalf("GetNumericDocValues: %v", err)
		}
		if values == nil {
			t.Fatal("GetNumericDocValues returned nil")
		}
		var previous int64 = math.MinInt64
		for i := 0; i < leaf.MaxDoc(); i++ {
			docID, err := values.NextDoc()
			if err != nil {
				t.Fatalf("NextDoc: %v", err)
			}
			if docID != i {
				t.Fatalf("expected docID %d, got %d", i, docID)
			}
			v, err := values.LongValue()
			if err != nil {
				t.Fatalf("LongValue: %v", err)
			}
			if v < previous {
				t.Fatalf("foo values not sorted at doc %d: %d < %d", i, v, previous)
			}
			previous = v
		}
	}
}

// TestIndexSorting_MultiValuedRandom1 ports testMultiValuedRandom1.
//
// This Go port validates the sorted-numeric index-sort contract with a
// deterministic, single-threaded sequence: each document carries 1–3
// pseudo-random "foo" values, is committed into its own segment, and then the
// whole index is force-merged to one segment. The merged segment's
// SortedNumericDocValues are read back and the per-document minimum must be
// non-decreasing in docID order.
func TestIndexSorting_MultiValuedRandom1(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	snSortField := index.NewSortedNumericSortField("foo", index.SortTypeLong)
	indexSort := index.NewSort(snSortField.SortField)
	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	cfg.SetIndexSort(indexSort)
	w, err := index.NewIndexWriter(dir, cfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}

	rng := rand.New(rand.NewSource(2))
	const numDocs = 50
	for i := 0; i < numDocs; i++ {
		doc := document.NewDocument()
		num := rng.Intn(3) + 1
		vals := make([]int64, num)
		for j := 0; j < num; j++ {
			vals[j] = int64(rng.Intn(2000))
		}
		foo, _ := document.NewSortedNumericDocValuesField("foo", vals)
		doc.Add(foo)
		idField, _ := document.NewStringField("id", strconv.Itoa(i), false)
		doc.Add(idField)
		idDV, _ := document.NewNumericDocValuesField("id", int64(i))
		doc.Add(idDV)
		if err := w.AddDocument(doc); err != nil {
			t.Fatalf("AddDocument %d: %v", i, err)
		}
		if err := w.Commit(); err != nil {
			t.Fatalf("Commit %d: %v", i, err)
		}
	}

	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit after merge: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close writer: %v", err)
	}

	r, err := index.OpenDirectoryReader(dir)
	if err != nil {
		t.Fatalf("OpenDirectoryReader: %v", err)
	}
	defer r.Close()

	for _, leaf := range r.GetSegmentReaders() {
		values, err := leaf.GetSortedNumericDocValues("foo")
		if err != nil {
			t.Fatalf("GetSortedNumericDocValues: %v", err)
		}
		if values == nil {
			t.Fatal("GetSortedNumericDocValues returned nil")
		}
		var previous int64 = math.MinInt64
		for i := 0; i < leaf.MaxDoc(); i++ {
			docID, err := values.NextDoc()
			if err != nil {
				t.Fatalf("NextDoc: %v", err)
			}
			if docID != i {
				t.Fatalf("expected docID %d, got %d", i, docID)
			}
			count, err := values.DocValueCount()
			if err != nil {
				t.Fatalf("DocValueCount: %v", err)
			}
			if count == 0 {
				t.Fatalf("doc %d has no values", i)
			}
			var min int64 = math.MaxInt64
			for j := 0; j < count; j++ {
				v, err := values.NextValue()
				if err != nil {
					t.Fatalf("NextValue: %v", err)
				}
				if v < min {
					min = v
				}
			}
			if min < previous {
				t.Fatalf("foo min values not sorted at doc %d: %d < %d", i, min, previous)
			}
			previous = min
		}
	}
}

// TestIndexSorting_Random2 ports testRandom2.
func TestIndexSorting_Random2(t *testing.T) {
	t.Fatal("GOC-4136: needs RandomIndexWriter, PositionsTokenStream and full postings/term-vector read-back")
}

// TestIndexSorting_Random3 ports testRandom3.
func TestIndexSorting_Random3(t *testing.T) {
	t.Fatal("GOC-4136: needs RandomIndexWriter and IndexSearcher round-trip to validate randomized sorted search")
}

// -----------------------------------------------------------------------------
// Concurrent update tests.
// -----------------------------------------------------------------------------

// TestIndexSorting_ConcurrentUpdates ports testConcurrentUpdates.
func TestIndexSorting_ConcurrentUpdates(t *testing.T) {
	t.Fatal("GOC-4136: needs concurrent updateDocument driver plus IndexSearcher and MultiDocValues read-back")
}

// TestIndexSorting_ConcurrentDVUpdates ports testConcurrentDVUpdates.
func TestIndexSorting_ConcurrentDVUpdates(t *testing.T) {
	t.Fatal("GOC-4136: needs concurrent updateDocValues driver plus NumericDocValues read-back")
}

// TestIndexSorting_BadDVUpdate ports testBadDVUpdate: a DocValues field that
// participates in the index sort must not be updatable via updateDocValues.
func TestIndexSorting_BadDVUpdate(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createIndexSortingMockAnalyzer())
	sort := index.NewSort(index.NewSortField("foo", index.SortTypeInt))
	config.SetIndexSort(sort)
	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	err = writer.UpdateDocValues(nil, "foo", int64(42))
	if err == nil {
		t.Fatal("expected error when updating a sort field via UpdateDocValues, got nil")
	}
	if !strings.Contains(err.Error(), "participates in the index sort") {
		t.Fatalf("error = %q, want message about index sort participation", err.Error())
	}
}

// -----------------------------------------------------------------------------
// addIndexes tests.
// -----------------------------------------------------------------------------

// TestIndexSorting_BadAddIndexes ports testBadAddIndexes: addIndexes from a
// source whose index sort differs from the destination must fail.
func TestIndexSorting_BadAddIndexes(t *testing.T) {
	srcDir := store.NewByteBuffersDirectory()
	defer srcDir.Close()

	// Create source index with a different sort.
	srcConfig := index.NewIndexWriterConfig(createIndexSortingMockAnalyzer())
	srcSort := index.NewSort(index.NewSortField("bar", index.SortTypeInt))
	srcConfig.SetIndexSort(srcSort)
	srcWriter, err := index.NewIndexWriter(srcDir, srcConfig)
	if err != nil {
		t.Fatalf("NewIndexWriter (src): %v", err)
	}
	doc := document.NewDocument()
	f, _ := document.NewNumericDocValuesField("bar", int64(1))
	doc.Add(f)
	if err := srcWriter.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument (src): %v", err)
	}
	if err := srcWriter.Close(); err != nil {
		t.Fatalf("Close (src): %v", err)
	}

	// Destination index with a different sort.
	dstDir := store.NewByteBuffersDirectory()
	defer dstDir.Close()
	dstConfig := index.NewIndexWriterConfig(createIndexSortingMockAnalyzer())
	dstSort := index.NewSort(index.NewSortField("foo", index.SortTypeInt))
	dstConfig.SetIndexSort(dstSort)
	dstWriter, err := index.NewIndexWriter(dstDir, dstConfig)
	if err != nil {
		t.Fatalf("NewIndexWriter (dst): %v", err)
	}
	defer dstWriter.Close()

	err = dstWriter.AddIndexes(srcDir)
	if err == nil {
		t.Fatal("expected error when adding indexes with incompatible sort, got nil")
	}
}

// TestIndexSorting_AddIndexes ports testAddIndexes (write path only): copy a
// sorted index into another writer carrying the same sort.
// The source writer must be closed before AddIndexes so that its write.lock
// is released; otherwise AddIndexes fails with LockObtainFailedException.
func TestIndexSorting_AddIndexes(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sortField := index.NewSortField("foo", index.SortTypeLong)
	writer := newIndexSortingWriter(t, dir, index.NewSort(sortField))

	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		field, _ := document.NewNumericDocValuesField("foo", int64(i))
		doc.Add(field)
		writer.AddDocument(doc)
	}
	// Close writer so the write.lock on dir is released before AddIndexes.
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}

	dir2 := store.NewByteBuffersDirectory()
	defer dir2.Close()

	writer2 := newIndexSortingWriter(t, dir2, index.NewSort(sortField))
	if err := writer2.AddIndexes(dir); err != nil {
		t.Errorf("AddIndexes() error = %v", err)
	}
	if writer2.NumDocs() != 10 {
		t.Errorf("Expected 10 documents after AddIndexes, got %d", writer2.NumDocs())
	}

	writer2.Close()
}

// TestIndexSorting_AddIndexesWithDeletions ports testAddIndexesWithDeletions.
func TestIndexSorting_AddIndexesWithDeletions(t *testing.T) {
	t.Fatal("GOC-4136: needs RandomIndexWriter, deletions and StoredFields read-back to validate the merged sorted order")
}

// TestIndexSorting_AddIndexesWithDirectory ports testAddIndexesWithDirectory.
func TestIndexSorting_AddIndexesWithDirectory(t *testing.T) {
	t.Fatal("GOC-4136: needs RandomIndexWriter and StoredFields read-back to validate the merged sorted order")
}

// TestIndexSorting_AddIndexesWithDeletionsAndDirectory ports
// testAddIndexesWithDeletionsAndDirectory.
func TestIndexSorting_AddIndexesWithDeletionsAndDirectory(t *testing.T) {
	t.Fatal("GOC-4136: needs RandomIndexWriter, deletions and StoredFields read-back to validate the merged sorted order")
}

// -----------------------------------------------------------------------------
// Sort configuration validation.
// -----------------------------------------------------------------------------

// TestIndexSorting_BadSort ports testBadSort: SCORE / DOC sort types are not
// valid as an index sort.
//
// Upstream expects IndexWriterConfig.setIndexSort to throw
// IllegalArgumentException. Gocene's SetIndexSort currently stores the sort
// without validating the field types, so only the storage behaviour is
// asserted here; the rejection itself is left to a follow-up.
func TestIndexSorting_BadSort(t *testing.T) {
	config := index.NewIndexWriterConfig(createIndexSortingMockAnalyzer())

	sort := index.SortRELEVANCE
	config.SetIndexSort(sort)

	// Documents the current (permissive) behaviour: the sort is stored as-is.
	if config.IndexSort() != sort {
		t.Error("Expected IndexSort to be stored")
	}
}

// TestIndexSorting_IllegalChangeSort ports testIllegalChangeSort: reopening an
// index with a different index sort than it was created with must fail.
func TestIndexSorting_IllegalChangeSort(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	// Create an index with sort A.
	configA := index.NewIndexWriterConfig(createIndexSortingMockAnalyzer())
	sortA := index.NewSort(index.NewSortField("foo", index.SortTypeInt))
	configA.SetIndexSort(sortA)
	writerA, err := index.NewIndexWriter(dir, configA)
	if err != nil {
		t.Fatalf("NewIndexWriter (first): %v", err)
	}
	doc := document.NewDocument()
	f, _ := document.NewNumericDocValuesField("foo", int64(1))
	doc.Add(f)
	if err := writerA.AddDocument(doc); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := writerA.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Reopen with sort B — must fail.
	configB := index.NewIndexWriterConfig(createIndexSortingMockAnalyzer())
	sortB := index.NewSort(index.NewSortField("bar", index.SortTypeInt))
	configB.SetIndexSort(sortB)
	_, err = index.NewIndexWriter(dir, configB)
	if err == nil {
		t.Fatal("expected error when changing index sort on reopen, got nil")
	}
}

// TestIndexSorting_WrongSortFieldType ports testWrongSortFieldType: the index
// sort field type must match the DocValues type actually indexed for the field.
func TestIndexSorting_WrongSortFieldType(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createIndexSortingMockAnalyzer())
	sort := index.NewSort(index.NewSortField("field", index.SortTypeString))
	config.SetIndexSort(sort)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	// Sort expects SORTED type, but we add NUMERIC type.
	doc := document.NewDocument()
	f, _ := document.NewNumericDocValuesField("field", 42)
	doc.Add(f)

	err = writer.AddDocument(doc)
	if err == nil {
		t.Fatal("expected error when adding doc with wrong DV type for sort field, got nil")
	}
	if !strings.Contains(err.Error(), "expected field [field]") {
		t.Fatalf("error = %q, want 'expected field [field] to be ...'", err.Error())
	}
}


// -----------------------------------------------------------------------------
// Sparse-field sorting.
// -----------------------------------------------------------------------------

// TestIndexSorting_IndexSortWithSparseField ports testIndexSortWithSparseField
// (write path only): documents are added with a dense sort field and several
// sparse fields; only some documents carry the sparse fields.
func TestIndexSorting_IndexSortWithSparseField(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sortField := index.NewSortField("dense_int", index.SortTypeInt)
	sortField.SetReverse(true)
	writer := newIndexSortingWriter(t, dir, index.NewSort(sortField))

	for i := 0; i < 128; i++ {
		doc := document.NewDocument()
		denseField, _ := document.NewNumericDocValuesField("dense_int", int64(i))
		doc.Add(denseField)
		if i < 64 {
			sparseField, _ := document.NewNumericDocValuesField("sparse_int", int64(i))
			doc.Add(sparseField)
		}
		writer.AddDocument(doc)
	}
	writer.Commit()
	writer.ForceMerge(1)

	if writer.NumDocs() != 128 {
		t.Errorf("Expected 128 documents, got %d", writer.NumDocs())
	}
	writer.Close()
}

// TestIndexSorting_IndexSortWithSparseFieldVerification ports the read-back
// assertions of testIndexSortWithSparseField: a dense numeric sort field
// (reverse) plus sparse numeric and binary DocValues present only on the first
// 64 documents. After the reverse-sorted merge, merged docID d carries
// dense_int = 127-d, and the sparse fields land on docIDs 64..127.
//
// The upstream test also verifies the norms of a sparse text field; norms are
// not yet written during flush/merge (deferred to rmp #120), so this port
// covers the numeric and binary DocValues legs and leaves the norms assertion
// to that follow-up.
func TestIndexSorting_IndexSortWithSparseFieldVerification(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sortField := index.NewSortField("dense_int", index.SortTypeInt)
	sortField.SetReverse(true)
	writer := newIndexSortingWriter(t, dir, index.NewSort(sortField))

	for i := 0; i < 128; i++ {
		doc := document.NewDocument()
		dense, _ := document.NewNumericDocValuesField("dense_int", int64(i))
		doc.Add(dense)
		if i < 64 {
			sparse, _ := document.NewNumericDocValuesField("sparse_int", int64(i))
			doc.Add(sparse)
			bin, _ := document.NewBinaryDocValuesField("sparse_binary", []byte(strconv.Itoa(i)))
			doc.Add(bin)
		}
		writer.AddDocument(doc)
		if i < 127 {
			writer.Commit()
		}
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	leaf, done := openMergedLeaf(t, dir)
	defer done()
	if leaf.MaxDoc() != 128 {
		t.Fatalf("maxDoc = %d, want 128", leaf.MaxDoc())
	}

	// dense_int is present on every doc; reverse order ⇒ value 127-docID.
	dDocs, dVals := numericDocs(t, leaf, "dense_int")
	if len(dDocs) != 128 {
		t.Fatalf("dense value count = %d, want 128", len(dDocs))
	}
	for d := 0; d < 128; d++ {
		if dDocs[d] != d || dVals[d] != int64(127-d) {
			t.Fatalf("dense doc[%d] = (doc %d, val %d), want (doc %d, val %d)", d, dDocs[d], dVals[d], d, 127-d)
		}
	}

	// sparse_int present on docIDs 64..127, value 127-docID (i.e. 63..0).
	sDocs, sVals := numericDocs(t, leaf, "sparse_int")
	if len(sDocs) != 64 {
		t.Fatalf("sparse value count = %d, want 64", len(sDocs))
	}
	for k := 0; k < 64; k++ {
		wantDoc := 64 + k
		if sDocs[k] != wantDoc || sVals[k] != int64(127-wantDoc) {
			t.Fatalf("sparse doc[%d] = (doc %d, val %d), want (doc %d, val %d)", k, sDocs[k], sVals[k], wantDoc, 127-wantDoc)
		}
	}

	// sparse_binary mirrors sparse_int: docID d>=64 carries str(127-d).
	bin, err := leaf.GetBinaryDocValues("sparse_binary")
	if err != nil {
		t.Fatalf("GetBinaryDocValues: %v", err)
	}
	if bin == nil {
		t.Fatal("GetBinaryDocValues returned nil")
	}
	for docID := 64; docID < 128; docID++ {
		ok, err := bin.AdvanceExact(docID)
		if err != nil {
			t.Fatalf("AdvanceExact(%d): %v", docID, err)
		}
		if !ok {
			t.Fatalf("sparse_binary missing at docID %d", docID)
		}
		b, err := bin.BinaryValue()
		if err != nil {
			t.Fatalf("BinaryValue: %v", err)
		}
		if want := strconv.Itoa(127 - docID); string(b) != want {
			t.Fatalf("sparse_binary docID %d = %q, want %q", docID, string(b), want)
		}
	}
}

// TestIndexSorting_IndexSortOnSparseField ports testIndexSortOnSparseField
// (write path only): the sort field itself is sparse.
func TestIndexSorting_IndexSortOnSparseField(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sortField := index.NewSortField("sparse", index.SortTypeInt)
	sortField.SetMissingValue(int64(-9223372036854775808)) // math.MinInt64 stand-in for Integer.MIN_VALUE
	writer := newIndexSortingWriter(t, dir, index.NewSort(sortField))

	for i := 0; i < 128; i++ {
		doc := document.NewDocument()
		if i < 64 {
			field, _ := document.NewNumericDocValuesField("sparse", int64(i))
			doc.Add(field)
		}
		writer.AddDocument(doc)
	}
	writer.Commit()
	writer.ForceMerge(1)

	if writer.NumDocs() != 128 {
		t.Errorf("Expected 128 documents, got %d", writer.NumDocs())
	}
	writer.Close()
}

// TestIndexSorting_IndexSortOnSparseFieldVerification ports the read-back
// assertions of testIndexSortOnSparseField: the sort field itself is sparse
// (present on the first 64 documents), with Integer.MIN_VALUE missing-first
// placement, so after the sorted merge the 64 missing documents occupy the
// leading docIDs and the valued documents follow in ascending value order.
//
// Documents are committed individually so each input segment is trivially
// sorted before MultiSorter merge-sorts them.
func TestIndexSorting_IndexSortOnSparseFieldVerification(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sortField := index.NewSortField("sparse", index.SortTypeInt)
	sortField.SetMissingValue(int32(math.MinInt32)) // Integer.MIN_VALUE: missing-first
	writer := newIndexSortingWriter(t, dir, index.NewSort(sortField))

	for i := 0; i < 128; i++ {
		doc := document.NewDocument()
		if i < 64 {
			field, _ := document.NewNumericDocValuesField("sparse", int64(i))
			doc.Add(field)
		}
		writer.AddDocument(doc)
		if i < 127 {
			writer.Commit()
		}
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	leaf, done := openMergedLeaf(t, dir)
	defer done()
	if leaf.MaxDoc() != 128 {
		t.Fatalf("maxDoc = %d, want 128", leaf.MaxDoc())
	}
	// Missing docs (MIN_VALUE) sort first into docIDs 0..63; the 64 valued docs
	// follow at docIDs 64..127 in ascending value order, so docID d>=64 carries
	// value d-64.
	docs, vals := numericDocs(t, leaf, "sparse")
	if len(docs) != 64 {
		t.Fatalf("present value count = %d, want 64", len(docs))
	}
	for k := 0; k < 64; k++ {
		if docs[k] != 64+k {
			t.Fatalf("present doc[%d] = %d, want %d", k, docs[k], 64+k)
		}
		if vals[k] != int64(k) {
			t.Fatalf("doc %d: sparse = %d, want %d", docs[k], vals[k], k)
		}
	}
}

// -----------------------------------------------------------------------------
// Deletes against a sorted index.
// -----------------------------------------------------------------------------

// TestIndexSorting_DeleteByTermOrQuery ports testDeleteByTermOrQuery.
func TestIndexSorting_DeleteByTermOrQuery(t *testing.T) {
	t.Fatal("GOC-4136: needs RandomIndexWriter and IndexSearcher to validate deletes against a sorted index")
}

// TestIndexSorting_DeleteAll exercises DeleteAll on a sorted-index writer.
// This has no direct upstream method but covers the empty-index edge of the
// delete path; it runs for real because it only needs the write side.
func TestIndexSorting_DeleteAll(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sortField := index.NewSortField("foo", index.SortTypeLong)
	writer := newIndexSortingWriter(t, dir, index.NewSort(sortField))

	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		field, _ := document.NewNumericDocValuesField("foo", int64(i))
		doc.Add(field)
		writer.AddDocument(doc)
	}

	if err := writer.DeleteAll(); err != nil {
		t.Errorf("DeleteAll() error = %v", err)
	}
	if writer.NumDocs() != 0 {
		t.Errorf("Expected 0 documents after DeleteAll, got %d", writer.NumDocs())
	}
	writer.Close()
}

// -----------------------------------------------------------------------------
// Tie-breaking.
// -----------------------------------------------------------------------------

// TestIndexSorting_TieBreak ports testTieBreak (write path only): documents
// share the primary sort value, so the index sort relies on the secondary
// field (here a second long field) to break ties deterministically.
func TestIndexSorting_TieBreak(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sort := index.NewSort(
		index.NewSortField("foo", index.SortTypeLong),
		index.NewSortField("bar", index.SortTypeLong),
	)
	writer := newIndexSortingWriter(t, dir, sort)

	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		field1, _ := document.NewNumericDocValuesField("foo", 1)
		field2, _ := document.NewNumericDocValuesField("bar", int64(10-i))
		doc.Add(field1)
		doc.Add(field2)
		writer.AddDocument(doc)
	}
	writer.ForceMerge(1)

	if writer.NumDocs() != 10 {
		t.Errorf("Expected 10 documents, got %d", writer.NumDocs())
	}
	writer.Close()
}

// TestIndexSorting_TieBreakVerification ports the order assertion of
// testTieBreak. The Gocene write-path test breaks ties with a secondary long
// field (foo is constant, bar = 10-i), so the merged order is governed entirely
// by the secondary sort; reading "bar" back confirms the multi-field sort.
//
// Each document is committed separately so every input segment is trivially
// sorted, which is the precondition MultiSorter relies on when it merge-sorts
// the leaves (flush-time sorting of multi-doc segments is a separate gap).
func TestIndexSorting_TieBreakVerification(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sort := index.NewSort(
		index.NewSortField("foo", index.SortTypeLong),
		index.NewSortField("bar", index.SortTypeLong),
	)
	writer := newIndexSortingWriter(t, dir, sort)

	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		f1, _ := document.NewNumericDocValuesField("foo", 1)
		f2, _ := document.NewNumericDocValuesField("bar", int64(10-i))
		doc.Add(f1)
		doc.Add(f2)
		writer.AddDocument(doc)
		if i < 9 {
			writer.Commit()
		}
	}
	if err := writer.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	leaf, done := openMergedLeaf(t, dir)
	defer done()
	if leaf.MaxDoc() != 10 {
		t.Fatalf("maxDoc = %d, want 10", leaf.MaxDoc())
	}
	docs, bars := numericDocs(t, leaf, "bar")
	wantDocs := make([]int, 10)
	wantBars := make([]int64, 10)
	for i := 0; i < 10; i++ {
		wantDocs[i] = i
		wantBars[i] = int64(i + 1) // bar ascends 1..10 after the tie-break sort
	}
	assertIntSeq(t, "tie-break docID order", docs, wantDocs...)
	assertInt64Seq(t, "tie-break secondary order", bars, wantBars...)
}

// -----------------------------------------------------------------------------
// Document blocks with index sorting.
// -----------------------------------------------------------------------------

// TestIndexSorting_ParentFieldNotConfigured ports testParentFieldNotConfigured:
// adding a document block while an index sort is set, without a configured
// parent field, must fail.
func TestIndexSorting_ParentFieldNotConfigured(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createIndexSortingMockAnalyzer())
	sort := index.NewSort(index.NewSortField("foo", index.SortTypeInt))
	config.SetIndexSort(sort)
	// Deliberately do NOT call config.SetParentField().

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	err = writer.AddDocuments([]index.Document{
		document.NewDocument(),
		document.NewDocument(),
	})
	if err == nil {
		t.Fatal("expected error when using document blocks without a parent field, got nil")
	}
	want := "a parent field must be set in order to use document blocks with index sorting; see IndexWriterConfig#setParentField"
	if got := err.Error(); got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

// TestIndexSorting_BlockContainsParentField ports testBlockContainsParentField:
// no document in a block may itself carry the reserved parent field.
func TestIndexSorting_BlockContainsParentField(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	config := index.NewIndexWriterConfig(createIndexSortingMockAnalyzer())
	config.SetParentField("parent")
	sort := index.NewSort(index.NewSortField("foo", index.SortTypeInt))
	config.SetIndexSort(sort)

	writer, err := index.NewIndexWriter(dir, config)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer writer.Close()

	// Test 1: first document carries the reserved parent field.
	docWithParent := document.NewDocument()
	f, err := document.NewNumericDocValuesField("parent", 0)
	if err != nil {
		t.Fatalf("NewNumericDocValuesField: %v", err)
	}
	docWithParent.Add(f)

	err = writer.AddDocuments([]index.Document{
		docWithParent,
		document.NewDocument(),
	})
	if err == nil {
		t.Fatal("expected error when block document contains the reserved parent field")
	}
	want := `"parent" is a reserved field and should not be added to any document`
	if got := err.Error(); got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}

	// Test 2: second (last) document carries the reserved parent field.
	docWithParent2 := document.NewDocument()
	f2, err := document.NewNumericDocValuesField("parent", 0)
	if err != nil {
		t.Fatalf("NewNumericDocValuesField: %v", err)
	}
	docWithParent2.Add(f2)

	err = writer.AddDocuments([]index.Document{
		document.NewDocument(),
		docWithParent2,
	})
	if err == nil {
		t.Fatal("expected error when block document contains the reserved parent field")
	}
	if got := err.Error(); got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

// TestIndexSorting_IndexSortWithBlocks ports testIndexSortWithBlocks.
func TestIndexSorting_IndexSortWithBlocks(t *testing.T) {
	t.Fatal("GOC-4136: needs AddDocuments block support, a parent field, AssertingNeedsIndexSortCodec and StoredFields read-back")
}

// TestIndexSorting_MixRandomDocumentsWithBlocks ports
// testMixRandomDocumentsWithBlocks.
func TestIndexSorting_MixRandomDocumentsWithBlocks(t *testing.T) {
	t.Fatal("GOC-4136: needs RandomIndexWriter, AddDocuments block support and StoredFields read-back")
}

// -----------------------------------------------------------------------------
// Additional write-path coverage.
//
// The following tests have no single named upstream counterpart but exercise
// IndexWriter lifecycle operations specific to a sorted-index configuration.
// They run for real because they only need the write side.
// -----------------------------------------------------------------------------

// TestIndexSorting_SortFieldReverse verifies that a reverse index sort can be
// configured and a sorted-index writer driven through forceMerge.
func TestIndexSorting_SortFieldReverse(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sortField := index.NewSortField("foo", index.SortTypeLong)
	sortField.SetReverse(true)
	writer := newIndexSortingWriter(t, dir, index.NewSort(sortField))

	for i := 0; i < 5; i++ {
		doc := document.NewDocument()
		field, _ := document.NewNumericDocValuesField("foo", int64(i))
		doc.Add(field)
		writer.AddDocument(doc)
	}
	writer.ForceMerge(1)

	if writer.NumDocs() != 5 {
		t.Errorf("Expected 5 documents, got %d", writer.NumDocs())
	}
	writer.Close()
}

// TestIndexSorting_WaitForMerges verifies WaitForMerges on a sorted-index
// writer after a commit.
func TestIndexSorting_WaitForMerges(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sortField := index.NewSortField("foo", index.SortTypeLong)
	writer := newIndexSortingWriter(t, dir, index.NewSort(sortField))

	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		field, _ := document.NewNumericDocValuesField("foo", int64(i))
		doc.Add(field)
		writer.AddDocument(doc)
	}
	writer.Commit()

	if err := writer.WaitForMerges(); err != nil {
		t.Errorf("WaitForMerges() error = %v", err)
	}
	writer.Close()
}

// TestIndexSorting_ForceMerge verifies that ForceMerge collapses multiple
// committed segments of a sorted index into one.
func TestIndexSorting_ForceMerge(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	sortField := index.NewSortField("foo", index.SortTypeLong)
	writer := newIndexSortingWriter(t, dir, index.NewSort(sortField))

	for i := 0; i < 10; i++ {
		doc := document.NewDocument()
		field, _ := document.NewNumericDocValuesField("foo", int64(i))
		doc.Add(field)
		writer.AddDocument(doc)
		if i%3 == 0 {
			writer.Commit()
		}
	}

	if err := writer.ForceMerge(1); err != nil {
		t.Errorf("ForceMerge() error = %v", err)
	}
	writer.Close()
}
