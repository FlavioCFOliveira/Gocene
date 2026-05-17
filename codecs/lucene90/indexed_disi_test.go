// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene90

import (
	"errors"
	"math/rand/v2"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestIndexedDISI_SparseRoundTrip writes a sparse doc-id sequence (<= 4095
// per block) and verifies NextDoc/Index returns the original docs in order
// with monotone ordinals.
func TestIndexedDISI_SparseRoundTrip(t *testing.T) {
	docs := []int{1, 17, 256, 1<<16 + 5, 1<<16 + 100, 3<<16 + 42}
	verifyIndexedDISIRoundTrip(t, docs)
}

// TestIndexedDISI_DenseRoundTrip writes a single block whose cardinality
// exceeds maxArrayLength, forcing the DENSE encoding. Verifies that the
// FixedBitSet + (optional) rank table path reconstructs every doc.
func TestIndexedDISI_DenseRoundTrip(t *testing.T) {
	docs := make([]int, 0, 5000)
	for i := 0; i < 5000; i += 1 { // every doc in [0, 5000)
		docs = append(docs, i)
	}
	verifyIndexedDISIRoundTrip(t, docs)
}

// TestIndexedDISI_AllRoundTrip writes a block with every doc set (cardinality
// == blockSize), forcing the ALL encoding (header-only, empty body).
func TestIndexedDISI_AllRoundTrip(t *testing.T) {
	docs := make([]int, 0, blockSize)
	for i := 0; i < blockSize; i++ {
		docs = append(docs, i)
	}
	verifyIndexedDISIRoundTrip(t, docs)
}

// TestIndexedDISI_MultiBlockMix exercises all three encodings within the
// same stream, plus the jump-table path: SPARSE block 0, ALL block 1,
// DENSE block 2.
func TestIndexedDISI_MultiBlockMix(t *testing.T) {
	docs := []int{}
	// SPARSE block 0
	docs = append(docs, 1, 100, 1000)
	// ALL block 1
	for i := blockSize; i < 2*blockSize; i++ {
		docs = append(docs, i)
	}
	// DENSE block 2 (5000 docs in [2*blockSize, 2*blockSize+5000))
	for i := 0; i < 5000; i++ {
		docs = append(docs, 2*blockSize+i)
	}
	verifyIndexedDISIRoundTrip(t, docs)
}

// TestIndexedDISI_AdvanceJumps covers the jump-table fast-path: a multi-
// block stream where Advance lands several blocks ahead of the cursor.
func TestIndexedDISI_AdvanceJumps(t *testing.T) {
	docs := []int{
		1, 100, // block 0
		1 << 16, 1<<16 + 5, // block 1
		2 << 16, 2<<16 + 10, // block 2
		3 << 16, 3<<16 + 1, // block 3
	}
	cardinality := int64(len(docs))
	compressed, jumpEntries := writeIndexedDISI(t, docs)
	disi, err := openIndexedDISI(t, compressed, jumpEntries, cardinality)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Jump directly into block 3.
	got, err := disi.Advance(3 << 16)
	if err != nil {
		t.Fatalf("Advance(3<<16): %v", err)
	}
	if got != 3<<16 {
		t.Fatalf("Advance landed on %d, want %d", got, 3<<16)
	}
}

// TestIndexedDISI_AdvanceExact exercises the exact-position query path.
func TestIndexedDISI_AdvanceExact(t *testing.T) {
	docs := []int{1, 17, 256, 1024}
	cardinality := int64(len(docs))
	compressed, jumpEntries := writeIndexedDISI(t, docs)
	disi, err := openIndexedDISI(t, compressed, jumpEntries, cardinality)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	for _, d := range docs {
		exists, err := disi.AdvanceExact(d)
		if err != nil {
			t.Fatalf("AdvanceExact(%d): %v", d, err)
		}
		if !exists {
			t.Errorf("AdvanceExact(%d) reported missing", d)
		}
	}
	// A doc that does not exist.
	exists, err := disi.AdvanceExact(2)
	if err != nil {
		t.Fatalf("AdvanceExact(2): %v", err)
	}
	if exists {
		t.Errorf("AdvanceExact(2) reported present")
	}
}

// TestIndexedDISI_RandomMixed feeds a deterministic pseudo-random sequence
// across several blocks and verifies the round-trip.
func TestIndexedDISI_RandomMixed(t *testing.T) {
	r := rand.New(rand.NewPCG(0x9E3779B97F4A7C15, 0x1234))
	docs := make([]int, 0, 2_000)
	prev := -1
	for len(docs) < cap(docs) {
		next := prev + 1 + r.IntN(64)
		if next >= 4*blockSize {
			break
		}
		docs = append(docs, next)
		prev = next
	}
	verifyIndexedDISIRoundTrip(t, docs)
}

// TestIndexedDISI_InvalidDenseRankPower checks the constructor's rejection
// of out-of-range rank-power values.
func TestIndexedDISI_InvalidDenseRankPower(t *testing.T) {
	for _, p := range []byte{0, 1, 6, 16, 100} {
		if _, err := NewIndexedDISIWithSlices(nil, nil, 0, p, 1); err == nil {
			t.Errorf("denseRankPower=%d: expected error, got nil", int8(p))
		}
	}
	// 0xFF = int8(-1) should be accepted (DENSE ranks disabled).
	if _, err := NewIndexedDISIWithSlices(emptyIndexInput(), nil, 0, 0xFF, 1); err != nil {
		t.Errorf("denseRankPower=-1: unexpected error: %v", err)
	}
}

// TestIndexedDISI_IntoBitSetIsExplicitlyUnsupported verifies the public
// contract that IntoBitSet returns ErrIntoBitSetNotSupported.
func TestIndexedDISI_IntoBitSetIsExplicitlyUnsupported(t *testing.T) {
	d := &IndexedDISI{}
	if err := d.IntoBitSet(0, nil, 0); !errors.Is(err, ErrIntoBitSetNotSupported) {
		t.Fatalf("IntoBitSet error = %v, want %v", err, ErrIntoBitSetNotSupported)
	}
}

// -----------------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------------

// verifyIndexedDISIRoundTrip writes docs via WriteBitSet then enumerates
// the resulting IndexedDISI with NextDoc, asserting it returns docs in
// order with the correct Index (ordinal) values.
func verifyIndexedDISIRoundTrip(t *testing.T, docs []int) {
	t.Helper()

	compressed, jumpEntries := writeIndexedDISI(t, docs)
	disi, err := openIndexedDISI(t, compressed, jumpEntries, int64(len(docs)))
	if err != nil {
		t.Fatalf("openIndexedDISI: %v", err)
	}

	for ord, want := range docs {
		got, err := disi.NextDoc()
		if err != nil {
			t.Fatalf("NextDoc(ord=%d): %v", ord, err)
		}
		if got != want {
			t.Fatalf("NextDoc(ord=%d) = %d, want %d", ord, got, want)
		}
		if disi.Index() != ord {
			t.Fatalf("Index after NextDoc(ord=%d) = %d, want %d", ord, disi.Index(), ord)
		}
	}
	// One more NextDoc must hit the sentinel.
	got, err := disi.NextDoc()
	if err != nil {
		t.Fatalf("trailing NextDoc: %v", err)
	}
	if got != search.NO_MORE_DOCS {
		t.Fatalf("trailing NextDoc = %d, want NO_MORE_DOCS", got)
	}
}

// writeIndexedDISI drives WriteBitSet against a freshly-created file in
// an in-memory ByteBuffersDirectory and returns the produced byte stream
// plus the jump-table-entry count.
func writeIndexedDISI(t *testing.T, docs []int) ([]byte, int) {
	t.Helper()

	dir := store.NewByteBuffersDirectory()
	out, err := dir.CreateOutput("disi", store.IOContext{Context: store.ContextWrite})
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}

	it := &sliceDocIdSetIterator{docs: docs, cur: -1}
	jumpEntries, err := WriteBitSet(it, out)
	if err != nil {
		_ = out.Close()
		t.Fatalf("WriteBitSet: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close output: %v", err)
	}

	// Read everything back via an IndexInput.
	in, err := dir.OpenInput("disi", store.IOContext{Context: store.ContextRead})
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	defer in.Close()
	n := in.Length()
	bytes := make([]byte, n)
	if err := in.ReadBytes(bytes); err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}
	return bytes, int(jumpEntries)
}

// openIndexedDISI builds an IndexedDISI from a byte slice. We wrap the
// bytes in a fresh ByteBuffersDirectory file so we get a real IndexInput
// with seek/slice/randomAccess semantics rather than a DataInput-only
// view.
func openIndexedDISI(t *testing.T, compressed []byte, jumpEntries int, cardinality int64) (*IndexedDISI, error) {
	t.Helper()

	dir := store.NewByteBuffersDirectory()
	out, err := dir.CreateOutput("disi", store.IOContext{Context: store.ContextWrite})
	if err != nil {
		return nil, err
	}
	if err := out.WriteBytes(compressed); err != nil {
		_ = out.Close()
		return nil, err
	}
	if err := out.Close(); err != nil {
		return nil, err
	}
	in, err := dir.OpenInput("disi", store.IOContext{Context: store.ContextRead})
	if err != nil {
		return nil, err
	}
	disi, err := NewIndexedDISI(in, 0, int64(len(compressed)), jumpEntries, DefaultDenseRankPower, cardinality)
	if err != nil {
		_ = in.Close()
		return nil, err
	}
	t.Cleanup(func() { _ = disi.Close() })
	return disi, nil
}

// emptyIndexInput returns a zero-length IndexInput, suitable for
// constructor argument validation only.
func emptyIndexInput() store.IndexInput {
	dir := store.NewByteBuffersDirectory()
	out, err := dir.CreateOutput("empty", store.IOContext{Context: store.ContextWrite})
	if err != nil {
		panic(err)
	}
	_ = out.Close()
	in, err := dir.OpenInput("empty", store.IOContext{Context: store.ContextRead})
	if err != nil {
		panic(err)
	}
	return in
}

// sliceDocIdSetIterator is a minimal DocIdSetIterator over a sorted slice
// of doc IDs. NextDoc advances strictly forward; Advance binary-searches.
type sliceDocIdSetIterator struct {
	docs []int
	cur  int
}

func (it *sliceDocIdSetIterator) DocID() int {
	if it.cur < 0 {
		return -1
	}
	if it.cur >= len(it.docs) {
		return search.NO_MORE_DOCS
	}
	return it.docs[it.cur]
}

func (it *sliceDocIdSetIterator) NextDoc() (int, error) {
	it.cur++
	if it.cur >= len(it.docs) {
		return search.NO_MORE_DOCS, nil
	}
	return it.docs[it.cur], nil
}

func (it *sliceDocIdSetIterator) Advance(target int) (int, error) {
	for it.cur+1 < len(it.docs) && it.docs[it.cur+1] < target {
		it.cur++
	}
	return it.NextDoc()
}

func (it *sliceDocIdSetIterator) Cost() int64 { return int64(len(it.docs)) }

func (it *sliceDocIdSetIterator) DocIDRunEnd() int { return it.DocID() + 1 }

// Compile-time check.
var _ search.DocIdSetIterator = (*sliceDocIdSetIterator)(nil)
