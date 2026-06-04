// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Round-trip tests for Lucene90NormsConsumer.AddNormsField /
// Lucene90NormsProducer.GetNorms (rmp #19, S3).
//
// These tests verify that the per-field norms encoding produced by the
// consumer is read back identically by the producer across every encoding
// path the format selects: constant fields (bytesPerValue==0), fixed-width
// fields at 1/2/4/8 bytes, all-docs-present (dense) docsWithField, partial
// (sparse) docsWithField with single- and multi-block IndexedDISI jump
// tables, and the empty field. They also confirm the CodecUtil framing
// (IndexHeader / Footer / checksum) verifies on both the .nvd and .nvm
// files via Producer.CheckIntegrity.
//
// Source of truth: org.apache.lucene.codecs.lucene90.Lucene90NormsConsumer
// and Lucene90NormsProducer (Apache Lucene 10.4.0).

package codecs_test

import (
	"crypto/rand"
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// normsDoc is a single (docID, value) norm pair.
type normsDoc struct {
	doc   int
	value int64
}

// sliceNormsIterator is a single-pass codecs.NormsIterator over an ascending
// list of (docID, value) pairs, exactly mirroring the iteration contract the
// real flush path feeds into AddNormsField.
type sliceNormsIterator struct {
	docs []normsDoc
	pos  int
}

func newSliceNormsIterator(docs []normsDoc) *sliceNormsIterator {
	return &sliceNormsIterator{docs: docs, pos: -1}
}

func (it *sliceNormsIterator) Next() bool {
	it.pos++
	return it.pos < len(it.docs)
}

func (it *sliceNormsIterator) DocID() int {
	return it.docs[it.pos].doc
}

func (it *sliceNormsIterator) LongValue() int64 {
	return it.docs[it.pos].value
}

// normsRTField bundles a field's metadata with the norm values it should
// round-trip.
type normsRTField struct {
	name   string
	number int
	docs   []normsDoc
}

// buildAllDocs returns a value for every doc in [0, maxDoc) using gen.
func buildAllDocs(maxDoc int, gen func(doc int) int64) []normsDoc {
	out := make([]normsDoc, maxDoc)
	for d := 0; d < maxDoc; d++ {
		out[d] = normsDoc{doc: d, value: gen(d)}
	}
	return out
}

// runNormsRoundTrip writes every field via AddNormsField into a fresh
// single-segment .nvd/.nvm pair, closes the consumer, then reopens a producer
// and asserts that every (docID, norm) pair round-trips exactly. The producer
// integrity check (.nvd/.nvm checksum + footer) is also asserted.
func runNormsRoundTrip(t *testing.T, maxDoc int, fields []normsRTField) {
	t.Helper()

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	si := index.NewSegmentInfo("_0", maxDoc, dir)
	if err := si.SetID(id); err != nil {
		t.Fatalf("SetID: %v", err)
	}

	// Build FieldInfos covering exactly the fields under test, all
	// norm-bearing (indexed with freqs and norms not omitted).
	fis := index.NewFieldInfos()
	infos := make(map[string]*index.FieldInfo, len(fields))
	for _, f := range fields {
		fi := index.NewFieldInfo(f.name, f.number, index.FieldInfoOptions{
			IndexOptions: index.IndexOptionsDocsAndFreqsAndPositions,
		})
		if !fi.HasNorms() {
			t.Fatalf("field %q misconfigured: HasNorms()=false", f.name)
		}
		if err := fis.Add(fi); err != nil {
			t.Fatalf("FieldInfos.Add(%q): %v", f.name, err)
		}
		infos[f.name] = fi
	}

	writeState := &codecs.SegmentWriteState{
		Directory:   dir,
		SegmentInfo: si,
		FieldInfos:  fis,
	}

	format := codecs.NewLucene90NormsFormat()
	consumer, err := format.NormsConsumer(writeState)
	if err != nil {
		t.Fatalf("NormsConsumer: %v", err)
	}
	for _, f := range fields {
		if err := consumer.AddNormsField(infos[f.name], newSliceNormsIterator(f.docs)); err != nil {
			t.Fatalf("AddNormsField(%q): %v", f.name, err)
		}
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("consumer.Close: %v", err)
	}

	if !dir.FileExists("_0.nvd") {
		t.Fatal("expected _0.nvd to exist")
	}
	if !dir.FileExists("_0.nvm") {
		t.Fatal("expected _0.nvm to exist")
	}

	readState := &codecs.SegmentReadState{
		Directory:   dir,
		SegmentInfo: si,
		FieldInfos:  fis,
	}
	producer, err := format.NormsProducer(readState)
	if err != nil {
		t.Fatalf("NormsProducer: %v", err)
	}
	defer func() {
		if err := producer.Close(); err != nil {
			t.Errorf("producer.Close: %v", err)
		}
	}()

	// Framing/checksum verification over both files.
	if err := producer.CheckIntegrity(); err != nil {
		t.Fatalf("producer.CheckIntegrity: %v", err)
	}

	for _, f := range fields {
		assertFieldRoundTrips(t, producer, infos[f.name], f.docs)
	}
}

// assertFieldRoundTrips iterates the producer's NumericDocValues for one
// field and checks that it yields exactly the expected (docID, value) pairs.
func assertFieldRoundTrips(t *testing.T, producer codecs.NormsProducer, fi *index.FieldInfo, want []normsDoc) {
	t.Helper()

	dv, err := producer.GetNorms(fi)
	if err != nil {
		t.Fatalf("GetNorms(%q): %v", fi.Name(), err)
	}
	if dv == nil {
		t.Fatalf("GetNorms(%q): nil NumericDocValues", fi.Name())
	}

	for i, exp := range want {
		doc, err := dv.NextDoc()
		if err != nil {
			t.Fatalf("field %q: NextDoc[%d]: %v", fi.Name(), i, err)
		}
		if doc != exp.doc {
			t.Fatalf("field %q: doc[%d] = %d, want %d", fi.Name(), i, doc, exp.doc)
		}
		got, err := dv.LongValue()
		if err != nil {
			t.Fatalf("field %q: LongValue at doc %d: %v", fi.Name(), doc, err)
		}
		if got != exp.value {
			t.Fatalf("field %q: norm[doc=%d] = %d, want %d", fi.Name(), doc, got, exp.value)
		}
	}

	doc, err := dv.NextDoc()
	if err != nil {
		t.Fatalf("field %q: trailing NextDoc: %v", fi.Name(), err)
	}
	if doc != math.MaxInt32 {
		t.Fatalf("field %q: expected NO_MORE_DOCS after %d values, got doc=%d", fi.Name(), len(want), doc)
	}
}

// TestLucene90Norms_RoundTrip_DenseConstant covers an all-docs-present field
// whose values are constant (min==max => bytesPerValue 0, value stored in
// metadata).
func TestLucene90Norms_RoundTrip_DenseConstant(t *testing.T) {
	const maxDoc = 64
	runNormsRoundTrip(t, maxDoc, []normsRTField{
		{name: "constNorm", number: 0, docs: buildAllDocs(maxDoc, func(int) int64 { return 7 })},
	})
}

// TestLucene90Norms_RoundTrip_DenseWidths covers all-docs-present fields whose
// value ranges force each fixed-width encoding (1/2/4/8 bytes). Each field is
// tested in its own segment because every field in a dense-all segment must
// span the same maxDoc.
func TestLucene90Norms_RoundTrip_DenseWidths(t *testing.T) {
	cases := []struct {
		name string
		gen  func(doc int) int64
	}{
		{
			// 1-byte signed range: values in [-50, 77].
			name: "width1",
			gen:  func(d int) int64 { return int64(d%128) - 50 },
		},
		{
			// 2-byte range: spans well beyond a byte but inside a short.
			name: "width2",
			gen:  func(d int) int64 { return int64(d*200) - 5000 },
		},
		{
			// 4-byte range: includes a value beyond Short.MAX_VALUE.
			name: "width4",
			gen:  func(d int) int64 { return int64(d)*100000 - 1 },
		},
		{
			// 8-byte range: includes a value beyond Integer.MAX_VALUE.
			name: "width8",
			gen: func(d int) int64 {
				if d == 0 {
					return math.MinInt64 / 2
				}
				return int64(d) * 5_000_000_000
			},
		},
	}
	const maxDoc = 96
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			runNormsRoundTrip(t, maxDoc, []normsRTField{
				{name: tc.name, number: 0, docs: buildAllDocs(maxDoc, tc.gen)},
			})
		})
	}
}

// TestLucene90Norms_RoundTrip_Sparse covers fields where only a subset of
// docs have a value, exercising the IndexedDISI docsWithField jump table for
// both constant and fixed-width values, single-block and multi-block layouts.
func TestLucene90Norms_RoundTrip_Sparse(t *testing.T) {
	t.Run("sparse-singleblock-width1", func(t *testing.T) {
		// Every 3rd doc within one IndexedDISI block (< 65536).
		const maxDoc = 600
		var docs []normsDoc
		for d := 0; d < maxDoc; d += 3 {
			docs = append(docs, normsDoc{doc: d, value: int64(d%200) - 100})
		}
		runNormsRoundTrip(t, maxDoc, []normsRTField{
			{name: "sparse1", number: 0, docs: docs},
		})
	})

	t.Run("sparse-singleblock-constant", func(t *testing.T) {
		const maxDoc = 500
		var docs []normsDoc
		for d := 5; d < maxDoc; d += 7 {
			docs = append(docs, normsDoc{doc: d, value: 42})
		}
		runNormsRoundTrip(t, maxDoc, []normsRTField{
			{name: "sparseConst", number: 0, docs: docs},
		})
	})

	t.Run("sparse-multiblock-width4", func(t *testing.T) {
		// Span three IndexedDISI 65536-doc blocks so the jump table has
		// multiple entries; mix dense and sparse regions.
		const maxDoc = 200000
		var docs []normsDoc
		// Dense region in block 0.
		for d := 0; d < 50000; d++ {
			docs = append(docs, normsDoc{doc: d, value: int64(d)*1000 - 7})
		}
		// Sparse region spanning block 1.
		for d := 70000; d < 130000; d += 11 {
			docs = append(docs, normsDoc{doc: d, value: int64(d)*1000 - 7})
		}
		// A few docs in block 2.
		for d := 150000; d < 150100; d++ {
			docs = append(docs, normsDoc{doc: d, value: int64(d)*1000 - 7})
		}
		runNormsRoundTrip(t, maxDoc, []normsRTField{
			{name: "sparseMulti", number: 0, docs: docs},
		})
	})
}

// TestLucene90Norms_RoundTrip_Empty covers a field with no value-bearing
// documents (numDocsWithValue==0 => docsWithFieldOffset==-2).
func TestLucene90Norms_RoundTrip_Empty(t *testing.T) {
	const maxDoc = 32
	runNormsRoundTrip(t, maxDoc, []normsRTField{
		{name: "emptyNorm", number: 0, docs: nil},
	})
}

// TestLucene90Norms_RoundTrip_MultiField writes several fields with mixed
// encodings into one segment and round-trips them all, verifying that the
// interleaved per-field metadata in .nvm and the per-field data in .nvd stay
// correctly associated. All non-empty fields are all-docs-present so they can
// coexist at one maxDoc; the empty field needs no docs.
func TestLucene90Norms_RoundTrip_MultiField(t *testing.T) {
	const maxDoc = 80
	runNormsRoundTrip(t, maxDoc, []normsRTField{
		{name: "f_const", number: 0, docs: buildAllDocs(maxDoc, func(int) int64 { return 3 })},
		{name: "f_w1", number: 1, docs: buildAllDocs(maxDoc, func(d int) int64 { return int64(d % 100) })},
		{name: "f_w2", number: 2, docs: buildAllDocs(maxDoc, func(d int) int64 { return int64(d)*300 - 1000 })},
		{name: "f_w8", number: 3, docs: buildAllDocs(maxDoc, func(d int) int64 { return int64(d) * 9_000_000_000 })},
		{name: "f_empty", number: 4, docs: nil},
	})
}

// TestLucene90Norms_RoundTrip_TypicalByteNorms covers the most common real
// case: every doc has a norm in the BM25 single-byte range [0, 255], stored
// as a 1-byte field. The values are written by the consumer as signed bytes
// and must read back through int8 sign-extension identically.
func TestLucene90Norms_RoundTrip_TypicalByteNorms(t *testing.T) {
	const maxDoc = 256
	runNormsRoundTrip(t, maxDoc, []normsRTField{
		// Norm bytes 0..127 stay positive; 128..255 become negative after
		// the int8 cast, which is exactly how Lucene stores SmallFloat norm
		// bytes. The round-trip must preserve the signed interpretation.
		{name: "byteNorm", number: 0, docs: buildAllDocs(maxDoc, func(d int) int64 {
			return int64(int8(byte(d)))
		})},
	})
}
