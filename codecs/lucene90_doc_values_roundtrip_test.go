// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Round-trip tests for Lucene90DocValuesConsumer / Lucene90DocValuesProducer.
//
// Each test writes one or more DV fields via lucene90DVConsumer (accessed
// through the internal API), then reads them back via lucene90DVProducer and
// asserts value equality. This validates the byte-level write/read correctness
// without going through IndexWriter.
//
// Tests are in package codecs (not codecs_test) so they can access the
// unexported dvSorted* / dvBinary* iterator interfaces required by the
// consumer's direct API.

package codecs

import (
	"crypto/rand"
	"fmt"
	"sort"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// dvRTSegmentState creates a temp FS-backed directory and minimal
// SegmentWriteState / SegmentReadState for round-trip tests.
//
// Using SimpleFSDirectory (not ByteBuffersDirectory) ensures the on-disk
// ChecksumIndexOutput BE vs ByteBuffersDataOutput LE endian difference
// is exercised with the correct reader.
func dvRTSegmentState(t *testing.T, maxDoc int, fis *index.FieldInfos) (
	*SegmentWriteState, *SegmentReadState, func()) {
	t.Helper()
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	si := index.NewSegmentInfo("_rt", maxDoc, dir)
	if err := si.SetID(id); err != nil {
		t.Fatalf("SetID: %v", err)
	}
	ws := &SegmentWriteState{
		Directory:   dir,
		SegmentInfo: si,
		FieldInfos:  fis,
	}
	rs := &SegmentReadState{
		Directory:   dir,
		SegmentInfo: si,
		FieldInfos:  fis,
	}
	cleanup := func() { _ = dir.Close() }
	return ws, rs, cleanup
}

// dvRTNumericField returns a *index.FieldInfo for a NUMERIC DV field.
func dvRTNumericField(name string, number int) *index.FieldInfo {
	return index.NewFieldInfo(name, number, index.FieldInfoOptions{
		DocValuesType: index.DocValuesTypeNumeric,
	})
}

// dvRTBinaryField returns a *index.FieldInfo for a BINARY DV field.
func dvRTBinaryField(name string, number int) *index.FieldInfo {
	return index.NewFieldInfo(name, number, index.FieldInfoOptions{
		DocValuesType: index.DocValuesTypeBinary,
	})
}

// dvRTSortedField returns a *index.FieldInfo for a SORTED DV field.
func dvRTSortedField(name string, number int) *index.FieldInfo {
	return index.NewFieldInfo(name, number, index.FieldInfoOptions{
		DocValuesType: index.DocValuesTypeSorted,
	})
}

// dvRTSortedSetField returns a *index.FieldInfo for a SORTED_SET DV field.
func dvRTSortedSetField(name string, number int) *index.FieldInfo {
	return index.NewFieldInfo(name, number, index.FieldInfoOptions{
		DocValuesType: index.DocValuesTypeSortedSet,
	})
}

// dvRTSortedNumericField returns a *index.FieldInfo for a SORTED_NUMERIC DV field.
func dvRTSortedNumericField(name string, number int) *index.FieldInfo {
	return index.NewFieldInfo(name, number, index.FieldInfoOptions{
		DocValuesType: index.DocValuesTypeSortedNumeric,
	})
}

// ---------------------------------------------------------------------------
// dvSortedNumericValues adapter for simple numeric slices
// ---------------------------------------------------------------------------

// dvRTNumericValues implements dvSortedNumericValues from a flat doc→value map.
// Each doc has exactly one value (single-valued numeric).
type dvRTNumericValues struct {
	docs   []int
	values []int64
	pos    int
	doc    int
}

func newDVRTNumericValues(docs []int, values []int64) *dvRTNumericValues {
	return &dvRTNumericValues{docs: docs, values: values, pos: -1, doc: -1}
}

func (v *dvRTNumericValues) Reset() error { v.pos = -1; v.doc = -1; return nil }
func (v *dvRTNumericValues) DocID() int   { return v.doc }
func (v *dvRTNumericValues) NextDoc() (int, error) {
	v.pos++
	if v.pos >= len(v.docs) {
		v.doc = dvNoMoreDocs
		return dvNoMoreDocs, nil
	}
	v.doc = v.docs[v.pos]
	return v.doc, nil
}
func (v *dvRTNumericValues) DocValueCount() (int, error) { return 1, nil }
func (v *dvRTNumericValues) NextValue() (int64, error) {
	return v.values[v.pos], nil
}

// ---------------------------------------------------------------------------
// dvBinaryValues adapter
// ---------------------------------------------------------------------------

type dvRTBinaryValues struct {
	docs   []int
	values [][]byte
	pos    int
	doc    int
}

func newDVRTBinaryValues(docs []int, values [][]byte) *dvRTBinaryValues {
	return &dvRTBinaryValues{docs: docs, values: values, pos: -1, doc: -1}
}

func (v *dvRTBinaryValues) Reset() error { v.pos = -1; v.doc = -1; return nil }
func (v *dvRTBinaryValues) DocID() int   { return v.doc }
func (v *dvRTBinaryValues) NextDoc() (int, error) {
	v.pos++
	if v.pos >= len(v.docs) {
		v.doc = dvNoMoreDocs
		return dvNoMoreDocs, nil
	}
	v.doc = v.docs[v.pos]
	return v.doc, nil
}
func (v *dvRTBinaryValues) BinaryValue() ([]byte, error) { return v.values[v.pos], nil }

// ---------------------------------------------------------------------------
// dvSortedValues adapter
// ---------------------------------------------------------------------------

// dvRTSortedValues provides sorted values from a pre-sorted unique term list
// and per-doc ordinals.
type dvRTSortedValues struct {
	docs  []int
	ords  []int
	terms [][]byte
	pos   int
	doc   int
}

func newDVRTSortedValues(docs []int, ords []int, terms [][]byte) *dvRTSortedValues {
	return &dvRTSortedValues{docs: docs, ords: ords, terms: terms, pos: -1, doc: -1}
}

func (v *dvRTSortedValues) Reset() error { v.pos = -1; v.doc = -1; return nil }
func (v *dvRTSortedValues) DocID() int   { return v.doc }
func (v *dvRTSortedValues) NextDoc() (int, error) {
	v.pos++
	if v.pos >= len(v.docs) {
		v.doc = dvNoMoreDocs
		return dvNoMoreDocs, nil
	}
	v.doc = v.docs[v.pos]
	return v.doc, nil
}
func (v *dvRTSortedValues) OrdValue() (int, error) { return v.ords[v.pos], nil }
func (v *dvRTSortedValues) LookupOrd(ord int) ([]byte, error) {
	if ord < 0 || ord >= len(v.terms) {
		return nil, fmt.Errorf("dvRTSortedValues: ord %d out of range [0,%d)", ord, len(v.terms))
	}
	return v.terms[ord], nil
}
func (v *dvRTSortedValues) GetValueCount() int { return len(v.terms) }

// ---------------------------------------------------------------------------
// dvSortedSetValues adapter
// ---------------------------------------------------------------------------

type dvRTSortedSetEntry struct {
	doc  int
	ords []int
}

type dvRTSortedSetValues struct {
	entries []dvRTSortedSetEntry
	terms   [][]byte
	pos     int
	doc     int
	ordPos  int
}

func newDVRTSortedSetValues(entries []dvRTSortedSetEntry, terms [][]byte) *dvRTSortedSetValues {
	return &dvRTSortedSetValues{entries: entries, terms: terms, pos: -1, doc: -1}
}

func (v *dvRTSortedSetValues) Reset() error { v.pos = -1; v.doc = -1; v.ordPos = 0; return nil }
func (v *dvRTSortedSetValues) DocID() int   { return v.doc }
func (v *dvRTSortedSetValues) NextDoc() (int, error) {
	v.pos++
	if v.pos >= len(v.entries) {
		v.doc = dvNoMoreDocs
		return dvNoMoreDocs, nil
	}
	v.doc = v.entries[v.pos].doc
	v.ordPos = 0
	return v.doc, nil
}
func (v *dvRTSortedSetValues) NextOrd() (int, error) {
	ords := v.entries[v.pos].ords
	if v.ordPos >= len(ords) {
		return -1, nil
	}
	o := ords[v.ordPos]
	v.ordPos++
	return o, nil
}

// NextOrdTerm returns unique terms in ordinal order (independent of doc position).
// addTermsDict calls Reset() then iterates NextOrdTerm(); it must not rely on pos.
func (v *dvRTSortedSetValues) NextOrdTerm() ([]byte, error) {
	if v.ordPos >= len(v.terms) {
		return nil, nil
	}
	t := v.terms[v.ordPos]
	v.ordPos++
	return t, nil
}
func (v *dvRTSortedSetValues) LookupOrd(ord int) ([]byte, error) {
	if ord < 0 || ord >= len(v.terms) {
		return nil, fmt.Errorf("dvRTSortedSetValues: ord %d out of range", ord)
	}
	return v.terms[ord], nil
}
func (v *dvRTSortedSetValues) GetValueCount() int { return len(v.terms) }
func (v *dvRTSortedSetValues) DocValueCount() (int, error) {
	if v.pos < 0 || v.pos >= len(v.entries) {
		return 0, nil
	}
	return len(v.entries[v.pos].ords), nil
}

// ---------------------------------------------------------------------------
// dvSortedNumericValues adapter (multi-value)
// ---------------------------------------------------------------------------

type dvRTSortedNumericEntry struct {
	doc    int
	values []int64
}

type dvRTSortedNumericValues struct {
	entries []dvRTSortedNumericEntry
	pos     int
	doc     int
	valPos  int
}

func newDVRTSortedNumericValues(entries []dvRTSortedNumericEntry) *dvRTSortedNumericValues {
	return &dvRTSortedNumericValues{entries: entries, pos: -1, doc: -1}
}

func (v *dvRTSortedNumericValues) Reset() error { v.pos = -1; v.doc = -1; v.valPos = 0; return nil }
func (v *dvRTSortedNumericValues) DocID() int   { return v.doc }
func (v *dvRTSortedNumericValues) NextDoc() (int, error) {
	v.pos++
	if v.pos >= len(v.entries) {
		v.doc = dvNoMoreDocs
		return dvNoMoreDocs, nil
	}
	v.doc = v.entries[v.pos].doc
	v.valPos = 0
	return v.doc, nil
}
func (v *dvRTSortedNumericValues) DocValueCount() (int, error) {
	if v.pos < 0 || v.pos >= len(v.entries) {
		return 0, nil
	}
	return len(v.entries[v.pos].values), nil
}
func (v *dvRTSortedNumericValues) NextValue() (int64, error) {
	val := v.entries[v.pos].values[v.valPos]
	v.valPos++
	return val, nil
}

// ---------------------------------------------------------------------------
// TestLucene90DV_Numeric_DenseConst
// ---------------------------------------------------------------------------

// TestLucene90DV_Numeric_DenseConst writes a dense NUMERIC field where all
// docs have the same constant value (bitsPerValue==0 encoding).
func TestLucene90DV_Numeric_DenseConst(t *testing.T) {
	const maxDoc = 10
	const constVal = int64(42)

	fi := dvRTNumericField("num", 0)
	fis := index.NewFieldInfos()
	if err := fis.Add(fi); err != nil {
		t.Fatalf("fis.Add: %v", err)
	}

	ws, rs, cleanup := dvRTSegmentState(t, maxDoc, fis)
	defer cleanup()

	// Write
	consumer, err := newLucene90DVConsumer(ws, Lucene90DocValuesDefaultSkipIndexIntervalSize)
	if err != nil {
		t.Fatalf("newLucene90DVConsumer: %v", err)
	}
	docs := make([]int, maxDoc)
	vals := make([]int64, maxDoc)
	for i := range docs {
		docs[i] = i
		vals[i] = constVal
	}
	if err := consumer.AddNumericField(fi, newDVRTNumericValues(docs, vals)); err != nil {
		t.Fatalf("AddNumericField: %v", err)
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("consumer.Close: %v", err)
	}

	// Read
	producer, err := newLucene90DVProducer(rs)
	if err != nil {
		t.Fatalf("newLucene90DVProducer: %v", err)
	}
	defer producer.Close() //nolint:errcheck

	ndv, err := producer.GetNumeric(fi)
	if err != nil {
		t.Fatalf("GetNumeric: %v", err)
	}
	if ndv == nil {
		t.Fatal("GetNumeric returned nil")
	}
	for _, doc := range docs {
		d, err := ndv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc at %d: %v", doc, err)
		}
		if d != doc {
			t.Fatalf("NextDoc: got %d, want %d", d, doc)
		}
		v, err := ndv.LongValue()
		if err != nil {
			t.Fatalf("LongValue at %d: %v", doc, err)
		}
		if v != constVal {
			t.Fatalf("doc %d: got %d, want %d", doc, v, constVal)
		}
	}
}

// ---------------------------------------------------------------------------
// TestLucene90DV_Numeric_DenseDelta
// ---------------------------------------------------------------------------

// TestLucene90DV_Numeric_DenseDelta writes a dense NUMERIC field with
// monotonically increasing values (delta compression path).
func TestLucene90DV_Numeric_DenseDelta(t *testing.T) {
	const maxDoc = 50

	fi := dvRTNumericField("num", 0)
	fis := index.NewFieldInfos()
	if err := fis.Add(fi); err != nil {
		t.Fatalf("fis.Add: %v", err)
	}

	ws, rs, cleanup := dvRTSegmentState(t, maxDoc, fis)
	defer cleanup()

	docs := make([]int, maxDoc)
	vals := make([]int64, maxDoc)
	for i := range docs {
		docs[i] = i
		vals[i] = int64(i * 100)
	}

	consumer, err := newLucene90DVConsumer(ws, Lucene90DocValuesDefaultSkipIndexIntervalSize)
	if err != nil {
		t.Fatalf("newLucene90DVConsumer: %v", err)
	}
	if err := consumer.AddNumericField(fi, newDVRTNumericValues(docs, vals)); err != nil {
		t.Fatalf("AddNumericField: %v", err)
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("consumer.Close: %v", err)
	}

	producer, err := newLucene90DVProducer(rs)
	if err != nil {
		t.Fatalf("newLucene90DVProducer: %v", err)
	}
	defer producer.Close() //nolint:errcheck

	ndv, err := producer.GetNumeric(fi)
	if err != nil {
		t.Fatalf("GetNumeric: %v", err)
	}
	for i, doc := range docs {
		d, err := ndv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc[%d]: %v", i, err)
		}
		if d != doc {
			t.Fatalf("NextDoc[%d]: got %d, want %d", i, d, doc)
		}
		v, err := ndv.LongValue()
		if err != nil {
			t.Fatalf("LongValue[%d]: %v", i, err)
		}
		if v != vals[i] {
			t.Fatalf("doc %d: got %d, want %d", doc, v, vals[i])
		}
	}
}

// ---------------------------------------------------------------------------
// TestLucene90DV_Numeric_Sparse
// ---------------------------------------------------------------------------

// TestLucene90DV_Numeric_Sparse writes a sparse NUMERIC field (not all docs
// have a value), exercising the DISI path in the producer.
func TestLucene90DV_Numeric_Sparse(t *testing.T) {
	const maxDoc = 20

	fi := dvRTNumericField("num", 0)
	fis := index.NewFieldInfos()
	if err := fis.Add(fi); err != nil {
		t.Fatalf("fis.Add: %v", err)
	}

	ws, rs, cleanup := dvRTSegmentState(t, maxDoc, fis)
	defer cleanup()

	// Only even docs get a value
	var docs []int
	var vals []int64
	for i := 0; i < maxDoc; i += 2 {
		docs = append(docs, i)
		vals = append(vals, int64(i*7))
	}

	consumer, err := newLucene90DVConsumer(ws, Lucene90DocValuesDefaultSkipIndexIntervalSize)
	if err != nil {
		t.Fatalf("newLucene90DVConsumer: %v", err)
	}
	if err := consumer.AddNumericField(fi, newDVRTNumericValues(docs, vals)); err != nil {
		t.Fatalf("AddNumericField: %v", err)
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("consumer.Close: %v", err)
	}

	producer, err := newLucene90DVProducer(rs)
	if err != nil {
		t.Fatalf("newLucene90DVProducer: %v", err)
	}
	defer producer.Close() //nolint:errcheck

	ndv, err := producer.GetNumeric(fi)
	if err != nil {
		t.Fatalf("GetNumeric: %v", err)
	}
	for i, doc := range docs {
		d, err := ndv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc[%d]: %v", i, err)
		}
		if d != doc {
			t.Fatalf("NextDoc[%d]: got %d, want %d", i, d, doc)
		}
		v, err := ndv.LongValue()
		if err != nil {
			t.Fatalf("LongValue[%d]: %v", i, err)
		}
		if v != vals[i] {
			t.Fatalf("doc %d: got %d, want %d", doc, v, vals[i])
		}
	}
}

// ---------------------------------------------------------------------------
// TestLucene90DV_Numeric_BlockBoundary
// ---------------------------------------------------------------------------

// TestLucene90DV_Numeric_BlockBoundary exercises crossing NumericBlockSize
// (16384) to trigger the varying-BPV block reader.
func TestLucene90DV_Numeric_BlockBoundary(t *testing.T) {
	// 2 full blocks + a partial block
	const maxDoc = Lucene90DocValuesNumericBlockSize*2 + 100

	fi := dvRTNumericField("num", 0)
	fis := index.NewFieldInfos()
	if err := fis.Add(fi); err != nil {
		t.Fatalf("fis.Add: %v", err)
	}

	ws, rs, cleanup := dvRTSegmentState(t, maxDoc, fis)
	defer cleanup()

	docs := make([]int, maxDoc)
	vals := make([]int64, maxDoc)
	for i := range docs {
		docs[i] = i
		// Use different magnitudes per block to force varying BPV
		switch {
		case i < Lucene90DocValuesNumericBlockSize:
			vals[i] = int64(i % 128) // 7 bits
		case i < Lucene90DocValuesNumericBlockSize*2:
			vals[i] = int64(i % 65536) // 16 bits
		default:
			vals[i] = int64(i)
		}
	}

	consumer, err := newLucene90DVConsumer(ws, Lucene90DocValuesDefaultSkipIndexIntervalSize)
	if err != nil {
		t.Fatalf("newLucene90DVConsumer: %v", err)
	}
	if err := consumer.AddNumericField(fi, newDVRTNumericValues(docs, vals)); err != nil {
		t.Fatalf("AddNumericField: %v", err)
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("consumer.Close: %v", err)
	}

	producer, err := newLucene90DVProducer(rs)
	if err != nil {
		t.Fatalf("newLucene90DVProducer: %v", err)
	}
	defer producer.Close() //nolint:errcheck

	ndv, err := producer.GetNumeric(fi)
	if err != nil {
		t.Fatalf("GetNumeric: %v", err)
	}
	for i, doc := range docs {
		d, err := ndv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc[%d]: %v", i, err)
		}
		if d != doc {
			t.Fatalf("NextDoc[%d]: got %d, want %d", i, d, doc)
		}
		v, err := ndv.LongValue()
		if err != nil {
			t.Fatalf("LongValue[%d]: %v", i, err)
		}
		if v != vals[i] {
			t.Fatalf("doc %d: got %d, want %d", doc, v, vals[i])
		}
	}
}

// ---------------------------------------------------------------------------
// TestLucene90DV_Binary_FixedWidth
// ---------------------------------------------------------------------------

// TestLucene90DV_Binary_FixedWidth tests a dense binary field where all
// values have the same length (fixed-width path in the producer).
func TestLucene90DV_Binary_FixedWidth(t *testing.T) {
	const maxDoc = 20
	const fixedLen = 8

	fi := dvRTBinaryField("bin", 0)
	fis := index.NewFieldInfos()
	if err := fis.Add(fi); err != nil {
		t.Fatalf("fis.Add: %v", err)
	}

	ws, rs, cleanup := dvRTSegmentState(t, maxDoc, fis)
	defer cleanup()

	docs := make([]int, maxDoc)
	values := make([][]byte, maxDoc)
	for i := range docs {
		docs[i] = i
		v := make([]byte, fixedLen)
		for j := range v {
			v[j] = byte((i + j) % 256)
		}
		values[i] = v
	}

	consumer, err := newLucene90DVConsumer(ws, Lucene90DocValuesDefaultSkipIndexIntervalSize)
	if err != nil {
		t.Fatalf("newLucene90DVConsumer: %v", err)
	}
	if err := consumer.AddBinaryField(fi, newDVRTBinaryValues(docs, values)); err != nil {
		t.Fatalf("AddBinaryField: %v", err)
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("consumer.Close: %v", err)
	}

	producer, err := newLucene90DVProducer(rs)
	if err != nil {
		t.Fatalf("newLucene90DVProducer: %v", err)
	}
	defer producer.Close() //nolint:errcheck

	bdv, err := producer.GetBinary(fi)
	if err != nil {
		t.Fatalf("GetBinary: %v", err)
	}
	if bdv == nil {
		t.Fatal("GetBinary returned nil")
	}
	for i, doc := range docs {
		d, err := bdv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc[%d]: %v", i, err)
		}
		if d != doc {
			t.Fatalf("NextDoc[%d]: got %d, want %d", i, d, doc)
		}
		bv, err := bdv.BinaryValue()
		if err != nil {
			t.Fatalf("BinaryValue[%d]: %v", i, err)
		}
		if len(bv) != len(values[i]) {
			t.Fatalf("doc %d: len got %d, want %d", doc, len(bv), len(values[i]))
		}
		for j := range bv {
			if bv[j] != values[i][j] {
				t.Fatalf("doc %d byte %d: got %d, want %d", doc, j, bv[j], values[i][j])
			}
		}
	}
}

// ---------------------------------------------------------------------------
// TestLucene90DV_Binary_VarWidth
// ---------------------------------------------------------------------------

// TestLucene90DV_Binary_VarWidth tests a dense binary field with variable
// length values (address-table path in the producer).
func TestLucene90DV_Binary_VarWidth(t *testing.T) {
	const maxDoc = 15

	fi := dvRTBinaryField("bin", 0)
	fis := index.NewFieldInfos()
	if err := fis.Add(fi); err != nil {
		t.Fatalf("fis.Add: %v", err)
	}

	ws, rs, cleanup := dvRTSegmentState(t, maxDoc, fis)
	defer cleanup()

	docs := make([]int, maxDoc)
	values := make([][]byte, maxDoc)
	for i := range docs {
		docs[i] = i
		v := make([]byte, i+1) // lengths: 1, 2, ..., maxDoc
		for j := range v {
			v[j] = byte((i*3 + j) % 256)
		}
		values[i] = v
	}

	consumer, err := newLucene90DVConsumer(ws, Lucene90DocValuesDefaultSkipIndexIntervalSize)
	if err != nil {
		t.Fatalf("newLucene90DVConsumer: %v", err)
	}
	if err := consumer.AddBinaryField(fi, newDVRTBinaryValues(docs, values)); err != nil {
		t.Fatalf("AddBinaryField: %v", err)
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("consumer.Close: %v", err)
	}

	producer, err := newLucene90DVProducer(rs)
	if err != nil {
		t.Fatalf("newLucene90DVProducer: %v", err)
	}
	defer producer.Close() //nolint:errcheck

	bdv, err := producer.GetBinary(fi)
	if err != nil {
		t.Fatalf("GetBinary: %v", err)
	}
	for i, doc := range docs {
		d, err := bdv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc[%d]: %v", i, err)
		}
		if d != doc {
			t.Fatalf("NextDoc[%d]: got %d, want %d", i, d, doc)
		}
		bv, err := bdv.BinaryValue()
		if err != nil {
			t.Fatalf("BinaryValue[%d]: %v", i, err)
		}
		if len(bv) != len(values[i]) {
			t.Fatalf("doc %d: len got %d, want %d", doc, len(bv), len(values[i]))
		}
		for j := range bv {
			if bv[j] != values[i][j] {
				t.Fatalf("doc %d byte %d: got %d, want %d", doc, j, bv[j], values[i][j])
			}
		}
	}
}

// ---------------------------------------------------------------------------
// TestLucene90DV_Sorted_FewTerms
// ---------------------------------------------------------------------------

// buildSortedTermsAndOrds builds a sorted unique term list and per-doc
// ordinal slice from raw doc→term maps.
func buildSortedTermsAndOrds(docTerms map[int]string) (docs []int, ords []int, terms [][]byte) {
	// collect unique terms
	termSet := make(map[string]struct{})
	for _, v := range docTerms {
		termSet[v] = struct{}{}
	}
	termList := make([]string, 0, len(termSet))
	for t := range termSet {
		termList = append(termList, t)
	}
	sort.Strings(termList)
	termIndex := make(map[string]int, len(termList))
	for i, t := range termList {
		termIndex[t] = i
		terms = append(terms, []byte(t))
	}
	// sorted docs
	for d := range docTerms {
		docs = append(docs, d)
	}
	sort.Ints(docs)
	for _, d := range docs {
		ords = append(ords, termIndex[docTerms[d]])
	}
	return docs, ords, terms
}

// TestLucene90DV_Sorted_FewTerms writes a SORTED field with few unique terms
// and reads back, validating OrdValue and LookupOrd.
func TestLucene90DV_Sorted_FewTerms(t *testing.T) {
	const maxDoc = 10

	fi := dvRTSortedField("sorted", 0)
	fis := index.NewFieldInfos()
	if err := fis.Add(fi); err != nil {
		t.Fatalf("fis.Add: %v", err)
	}

	ws, rs, cleanup := dvRTSegmentState(t, maxDoc, fis)
	defer cleanup()

	docTerms := map[int]string{
		0: "apple", 1: "banana", 2: "apple",
		3: "cherry", 4: "banana", 5: "apple",
		6: "cherry", 7: "banana", 8: "cherry",
		9: "apple",
	}
	docs, ords, terms := buildSortedTermsAndOrds(docTerms)

	consumer, err := newLucene90DVConsumer(ws, Lucene90DocValuesDefaultSkipIndexIntervalSize)
	if err != nil {
		t.Fatalf("newLucene90DVConsumer: %v", err)
	}
	if err := consumer.AddSortedField(fi, newDVRTSortedValues(docs, ords, terms)); err != nil {
		t.Fatalf("AddSortedField: %v", err)
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("consumer.Close: %v", err)
	}

	producer, err := newLucene90DVProducer(rs)
	if err != nil {
		t.Fatalf("newLucene90DVProducer: %v", err)
	}
	defer producer.Close() //nolint:errcheck

	sdv, err := producer.GetSorted(fi)
	if err != nil {
		t.Fatalf("GetSorted: %v", err)
	}
	if sdv == nil {
		t.Fatal("GetSorted returned nil")
	}

	for i, doc := range docs {
		d, err := sdv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc[%d]: %v", i, err)
		}
		if d != doc {
			t.Fatalf("NextDoc[%d]: got %d, want %d", i, d, doc)
		}
		ord, err := sdv.OrdValue()
		if err != nil {
			t.Fatalf("OrdValue[%d]: %v", i, err)
		}
		if ord != ords[i] {
			t.Fatalf("doc %d: ord got %d, want %d", doc, ord, ords[i])
		}
		termBytes, err := sdv.LookupOrd(ord)
		if err != nil {
			t.Fatalf("LookupOrd(%d): %v", ord, err)
		}
		if string(termBytes) != string(terms[ord]) {
			t.Fatalf("doc %d: term got %q, want %q", doc, termBytes, terms[ord])
		}
	}
}

// ---------------------------------------------------------------------------
// TestLucene90DV_Sorted_BlockBoundary
// ---------------------------------------------------------------------------

// TestLucene90DV_Sorted_BlockBoundary exercises the LZ4 terms dict block
// boundary (TermsDictBlockLZ4Size=64 terms per block).
func TestLucene90DV_Sorted_BlockBoundary(t *testing.T) {
	// 3 full LZ4 blocks = 192 distinct terms
	const numTerms = Lucene90DocValuesTermsDictBlockLZ4Size*3 + 5
	const maxDoc = numTerms

	fi := dvRTSortedField("sorted", 0)
	fis := index.NewFieldInfos()
	if err := fis.Add(fi); err != nil {
		t.Fatalf("fis.Add: %v", err)
	}

	ws, rs, cleanup := dvRTSegmentState(t, maxDoc, fis)
	defer cleanup()

	// Generate numTerms distinct sorted terms
	terms := make([][]byte, numTerms)
	for i := range terms {
		terms[i] = []byte(fmt.Sprintf("term%06d", i))
	}

	// All docs have term with same index as doc
	docs := make([]int, maxDoc)
	ords := make([]int, maxDoc)
	for i := range docs {
		docs[i] = i
		ords[i] = i
	}

	consumer, err := newLucene90DVConsumer(ws, Lucene90DocValuesDefaultSkipIndexIntervalSize)
	if err != nil {
		t.Fatalf("newLucene90DVConsumer: %v", err)
	}
	if err := consumer.AddSortedField(fi, newDVRTSortedValues(docs, ords, terms)); err != nil {
		t.Fatalf("AddSortedField: %v", err)
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("consumer.Close: %v", err)
	}

	producer, err := newLucene90DVProducer(rs)
	if err != nil {
		t.Fatalf("newLucene90DVProducer: %v", err)
	}
	defer producer.Close() //nolint:errcheck

	sdv, err := producer.GetSorted(fi)
	if err != nil {
		t.Fatalf("GetSorted: %v", err)
	}
	for i, doc := range docs {
		d, err := sdv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc[%d]: %v", i, err)
		}
		if d != doc {
			t.Fatalf("NextDoc[%d]: got %d, want %d", i, d, doc)
		}
		ord, err := sdv.OrdValue()
		if err != nil {
			t.Fatalf("OrdValue[%d]: %v", i, err)
		}
		if ord != ords[i] {
			t.Fatalf("doc %d: ord got %d, want %d", doc, ord, ords[i])
		}
		termBytes, err := sdv.LookupOrd(ord)
		if err != nil {
			t.Fatalf("LookupOrd(%d): %v", ord, err)
		}
		if string(termBytes) != string(terms[ord]) {
			t.Fatalf("doc %d: term got %q, want %q", doc, termBytes, terms[ord])
		}
	}
}

// ---------------------------------------------------------------------------
// TestLucene90DV_SortedSet_SingleValue
// ---------------------------------------------------------------------------

// TestLucene90DV_SortedSet_SingleValue tests a SORTED_SET field where each
// doc has exactly one value (single-value fast path in the producer).
func TestLucene90DV_SortedSet_SingleValue(t *testing.T) {
	const maxDoc = 8

	fi := dvRTSortedSetField("ss", 0)
	fis := index.NewFieldInfos()
	if err := fis.Add(fi); err != nil {
		t.Fatalf("fis.Add: %v", err)
	}

	ws, rs, cleanup := dvRTSegmentState(t, maxDoc, fis)
	defer cleanup()

	terms := [][]byte{
		[]byte("aaa"), []byte("bbb"), []byte("ccc"),
	}
	// Map doc → single ord
	entries := []dvRTSortedSetEntry{
		{doc: 0, ords: []int{0}},
		{doc: 1, ords: []int{1}},
		{doc: 2, ords: []int{2}},
		{doc: 3, ords: []int{0}},
		{doc: 4, ords: []int{1}},
		{doc: 5, ords: []int{2}},
		{doc: 6, ords: []int{0}},
		{doc: 7, ords: []int{2}},
	}

	consumer, err := newLucene90DVConsumer(ws, Lucene90DocValuesDefaultSkipIndexIntervalSize)
	if err != nil {
		t.Fatalf("newLucene90DVConsumer: %v", err)
	}
	if err := consumer.AddSortedSetField(fi, newDVRTSortedSetValues(entries, terms)); err != nil {
		t.Fatalf("AddSortedSetField: %v", err)
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("consumer.Close: %v", err)
	}

	producer, err := newLucene90DVProducer(rs)
	if err != nil {
		t.Fatalf("newLucene90DVProducer: %v", err)
	}
	defer producer.Close() //nolint:errcheck

	ssdv, err := producer.GetSortedSet(fi)
	if err != nil {
		t.Fatalf("GetSortedSet: %v", err)
	}
	if ssdv == nil {
		t.Fatal("GetSortedSet returned nil")
	}

	for i, e := range entries {
		d, err := ssdv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc[%d]: %v", i, err)
		}
		if d != e.doc {
			t.Fatalf("NextDoc[%d]: got %d, want %d", i, d, e.doc)
		}
		for j, wantOrd := range e.ords {
			ord, err := ssdv.NextOrd()
			if err != nil {
				t.Fatalf("NextOrd[%d][%d]: %v", i, j, err)
			}
			if ord != wantOrd {
				t.Fatalf("doc %d ord[%d]: got %d, want %d", e.doc, j, ord, wantOrd)
			}
		}
		// Verify exhaustion
		termOrd, err := ssdv.NextOrd()
		if err != nil {
			t.Fatalf("NextOrd exhaustion[%d]: %v", i, err)
		}
		if termOrd != -1 {
			t.Fatalf("doc %d: expected -1 after last ord, got %d", e.doc, termOrd)
		}
	}
}

// ---------------------------------------------------------------------------
// TestLucene90DV_SortedSet_MultiValue
// ---------------------------------------------------------------------------

// TestLucene90DV_SortedSet_MultiValue tests a SORTED_SET field where docs
// have multiple values (multi-valued path).
func TestLucene90DV_SortedSet_MultiValue(t *testing.T) {
	const maxDoc = 6

	fi := dvRTSortedSetField("ss", 0)
	fis := index.NewFieldInfos()
	if err := fis.Add(fi); err != nil {
		t.Fatalf("fis.Add: %v", err)
	}

	ws, rs, cleanup := dvRTSegmentState(t, maxDoc, fis)
	defer cleanup()

	terms := [][]byte{
		[]byte("alpha"), []byte("beta"), []byte("gamma"), []byte("delta"),
	}
	entries := []dvRTSortedSetEntry{
		{doc: 0, ords: []int{0, 1}},
		{doc: 1, ords: []int{2}},
		{doc: 2, ords: []int{0, 2, 3}},
		{doc: 3, ords: []int{1, 3}},
		{doc: 4, ords: []int{0}},
		{doc: 5, ords: []int{3}},
	}

	consumer, err := newLucene90DVConsumer(ws, Lucene90DocValuesDefaultSkipIndexIntervalSize)
	if err != nil {
		t.Fatalf("newLucene90DVConsumer: %v", err)
	}
	if err := consumer.AddSortedSetField(fi, newDVRTSortedSetValues(entries, terms)); err != nil {
		t.Fatalf("AddSortedSetField: %v", err)
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("consumer.Close: %v", err)
	}

	producer, err := newLucene90DVProducer(rs)
	if err != nil {
		t.Fatalf("newLucene90DVProducer: %v", err)
	}
	defer producer.Close() //nolint:errcheck

	ssdv, err := producer.GetSortedSet(fi)
	if err != nil {
		t.Fatalf("GetSortedSet: %v", err)
	}

	for i, e := range entries {
		d, err := ssdv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc[%d]: %v", i, err)
		}
		if d != e.doc {
			t.Fatalf("NextDoc[%d]: got %d, want %d", i, d, e.doc)
		}
		for j, wantOrd := range e.ords {
			ord, err := ssdv.NextOrd()
			if err != nil {
				t.Fatalf("NextOrd[%d][%d]: %v", i, j, err)
			}
			if ord != wantOrd {
				t.Fatalf("doc %d ord[%d]: got %d, want %d", e.doc, j, ord, wantOrd)
			}
		}
		termOrd, err := ssdv.NextOrd()
		if err != nil {
			t.Fatalf("NextOrd exhaustion[%d]: %v", i, err)
		}
		if termOrd != -1 {
			t.Fatalf("doc %d: expected -1 after last ord, got %d", e.doc, termOrd)
		}
	}
}

// ---------------------------------------------------------------------------
// TestLucene90DV_SortedNumeric_SingleValue
// ---------------------------------------------------------------------------

// TestLucene90DV_SortedNumeric_SingleValue tests a SORTED_NUMERIC field where
// each doc has exactly one value (single-value fast path).
func TestLucene90DV_SortedNumeric_SingleValue(t *testing.T) {
	const maxDoc = 10

	fi := dvRTSortedNumericField("sn", 0)
	fis := index.NewFieldInfos()
	if err := fis.Add(fi); err != nil {
		t.Fatalf("fis.Add: %v", err)
	}

	ws, rs, cleanup := dvRTSegmentState(t, maxDoc, fis)
	defer cleanup()

	entries := make([]dvRTSortedNumericEntry, maxDoc)
	for i := range entries {
		entries[i] = dvRTSortedNumericEntry{doc: i, values: []int64{int64(i * 11)}}
	}

	consumer, err := newLucene90DVConsumer(ws, Lucene90DocValuesDefaultSkipIndexIntervalSize)
	if err != nil {
		t.Fatalf("newLucene90DVConsumer: %v", err)
	}
	if err := consumer.AddSortedNumericField(fi, newDVRTSortedNumericValues(entries)); err != nil {
		t.Fatalf("AddSortedNumericField: %v", err)
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("consumer.Close: %v", err)
	}

	producer, err := newLucene90DVProducer(rs)
	if err != nil {
		t.Fatalf("newLucene90DVProducer: %v", err)
	}
	defer producer.Close() //nolint:errcheck

	sndv, err := producer.GetSortedNumeric(fi)
	if err != nil {
		t.Fatalf("GetSortedNumeric: %v", err)
	}
	if sndv == nil {
		t.Fatal("GetSortedNumeric returned nil")
	}

	for i, e := range entries {
		d, err := sndv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc[%d]: %v", i, err)
		}
		if d != e.doc {
			t.Fatalf("NextDoc[%d]: got %d, want %d", i, d, e.doc)
		}
		cnt, err := sndv.DocValueCount()
		if err != nil {
			t.Fatalf("DocValueCount[%d]: %v", i, err)
		}
		if cnt != len(e.values) {
			t.Fatalf("doc %d: count got %d, want %d", e.doc, cnt, len(e.values))
		}
		for j, wantV := range e.values {
			v, err := sndv.NextValue()
			if err != nil {
				t.Fatalf("NextValue[%d][%d]: %v", i, j, err)
			}
			if v != wantV {
				t.Fatalf("doc %d val[%d]: got %d, want %d", e.doc, j, v, wantV)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// TestLucene90DV_SortedNumeric_MultiValue
// ---------------------------------------------------------------------------

// TestLucene90DV_SortedNumeric_MultiValue tests a SORTED_NUMERIC field where
// docs have multiple values (address-table path in the producer).
func TestLucene90DV_SortedNumeric_MultiValue(t *testing.T) {
	const maxDoc = 5

	fi := dvRTSortedNumericField("sn", 0)
	fis := index.NewFieldInfos()
	if err := fis.Add(fi); err != nil {
		t.Fatalf("fis.Add: %v", err)
	}

	ws, rs, cleanup := dvRTSegmentState(t, maxDoc, fis)
	defer cleanup()

	entries := []dvRTSortedNumericEntry{
		{doc: 0, values: []int64{1, 3, 5}},
		{doc: 1, values: []int64{2}},
		{doc: 2, values: []int64{10, 20, 30, 40}},
		{doc: 3, values: []int64{7, 8}},
		{doc: 4, values: []int64{100}},
	}

	consumer, err := newLucene90DVConsumer(ws, Lucene90DocValuesDefaultSkipIndexIntervalSize)
	if err != nil {
		t.Fatalf("newLucene90DVConsumer: %v", err)
	}
	if err := consumer.AddSortedNumericField(fi, newDVRTSortedNumericValues(entries)); err != nil {
		t.Fatalf("AddSortedNumericField: %v", err)
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("consumer.Close: %v", err)
	}

	producer, err := newLucene90DVProducer(rs)
	if err != nil {
		t.Fatalf("newLucene90DVProducer: %v", err)
	}
	defer producer.Close() //nolint:errcheck

	sndv, err := producer.GetSortedNumeric(fi)
	if err != nil {
		t.Fatalf("GetSortedNumeric: %v", err)
	}

	for i, e := range entries {
		d, err := sndv.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc[%d]: %v", i, err)
		}
		if d != e.doc {
			t.Fatalf("NextDoc[%d]: got %d, want %d", i, d, e.doc)
		}
		cnt, err := sndv.DocValueCount()
		if err != nil {
			t.Fatalf("DocValueCount[%d]: %v", i, err)
		}
		if cnt != len(e.values) {
			t.Fatalf("doc %d: count got %d, want %d", e.doc, cnt, len(e.values))
		}
		for j, wantV := range e.values {
			v, err := sndv.NextValue()
			if err != nil {
				t.Fatalf("NextValue[%d][%d]: %v", i, j, err)
			}
			if v != wantV {
				t.Fatalf("doc %d val[%d]: got %d, want %d", e.doc, j, v, wantV)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// TestLucene90DV_MultipleFields
// ---------------------------------------------------------------------------

// TestLucene90DV_MultipleFields writes multiple DV fields of different types
// in a single segment and reads them all back.
func TestLucene90DV_MultipleFields(t *testing.T) {
	const maxDoc = 5

	fiNum := dvRTNumericField("num", 0)
	fiBin := dvRTBinaryField("bin", 1)
	fiSN := dvRTSortedNumericField("sn", 2)

	fis := index.NewFieldInfos()
	for _, fi := range []*index.FieldInfo{fiNum, fiBin, fiSN} {
		if err := fis.Add(fi); err != nil {
			t.Fatalf("fis.Add: %v", err)
		}
	}

	ws, rs, cleanup := dvRTSegmentState(t, maxDoc, fis)
	defer cleanup()

	numDocs := []int{0, 1, 2, 3, 4}
	numVals := []int64{10, 20, 30, 40, 50}
	binValues := [][]byte{
		[]byte("hello"), []byte("world"), []byte("foo"), []byte("bar"), []byte("baz"),
	}
	snEntries := []dvRTSortedNumericEntry{
		{doc: 0, values: []int64{1, 2}},
		{doc: 1, values: []int64{3}},
		{doc: 2, values: []int64{4, 5, 6}},
		{doc: 3, values: []int64{7}},
		{doc: 4, values: []int64{8, 9}},
	}

	consumer, err := newLucene90DVConsumer(ws, Lucene90DocValuesDefaultSkipIndexIntervalSize)
	if err != nil {
		t.Fatalf("newLucene90DVConsumer: %v", err)
	}
	if err := consumer.AddNumericField(fiNum, newDVRTNumericValues(numDocs, numVals)); err != nil {
		t.Fatalf("AddNumericField: %v", err)
	}
	if err := consumer.AddBinaryField(fiBin, newDVRTBinaryValues(numDocs, binValues)); err != nil {
		t.Fatalf("AddBinaryField: %v", err)
	}
	if err := consumer.AddSortedNumericField(fiSN, newDVRTSortedNumericValues(snEntries)); err != nil {
		t.Fatalf("AddSortedNumericField: %v", err)
	}
	if err := consumer.Close(); err != nil {
		t.Fatalf("consumer.Close: %v", err)
	}

	producer, err := newLucene90DVProducer(rs)
	if err != nil {
		t.Fatalf("newLucene90DVProducer: %v", err)
	}
	defer producer.Close() //nolint:errcheck

	// Validate numeric
	ndv, err := producer.GetNumeric(fiNum)
	if err != nil {
		t.Fatalf("GetNumeric: %v", err)
	}
	for i, doc := range numDocs {
		d, _ := ndv.NextDoc()
		if d != doc {
			t.Fatalf("num NextDoc[%d]: got %d, want %d", i, d, doc)
		}
		v, _ := ndv.LongValue()
		if v != numVals[i] {
			t.Fatalf("num doc %d: got %d, want %d", doc, v, numVals[i])
		}
	}

	// Validate binary
	bdv, err := producer.GetBinary(fiBin)
	if err != nil {
		t.Fatalf("GetBinary: %v", err)
	}
	for i, doc := range numDocs {
		d, _ := bdv.NextDoc()
		if d != doc {
			t.Fatalf("bin NextDoc[%d]: got %d, want %d", i, d, doc)
		}
		bv, _ := bdv.BinaryValue()
		if string(bv) != string(binValues[i]) {
			t.Fatalf("bin doc %d: got %q, want %q", doc, bv, binValues[i])
		}
	}

	// Validate sorted numeric
	sndv, err := producer.GetSortedNumeric(fiSN)
	if err != nil {
		t.Fatalf("GetSortedNumeric: %v", err)
	}
	for i, e := range snEntries {
		d, _ := sndv.NextDoc()
		if d != e.doc {
			t.Fatalf("sn NextDoc[%d]: got %d, want %d", i, d, e.doc)
		}
		cnt, _ := sndv.DocValueCount()
		if cnt != len(e.values) {
			t.Fatalf("sn doc %d count: got %d, want %d", e.doc, cnt, len(e.values))
		}
		for j, wantV := range e.values {
			v, _ := sndv.NextValue()
			if v != wantV {
				t.Fatalf("sn doc %d val[%d]: got %d, want %d", e.doc, j, v, wantV)
			}
		}
	}
}
