// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"bytes"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// br is a tiny helper that wraps a literal byte slice into a BytesRef
// covering the whole input. Tests use it to keep assertions terse.
func br(b []byte) *util.BytesRef {
	return &util.BytesRef{Bytes: b, Offset: 0, Length: len(b)}
}

// TestBinaryDocValuesFieldUpdates_BasicAdd asserts the happy path:
// add three updates in arbitrary order, finish, and verify the
// iterator returns them sorted by doc id with their bytes intact.
func TestBinaryDocValuesFieldUpdates_BasicAdd(t *testing.T) {
	b, err := NewBinaryDocValuesFieldUpdates(7, "f", 100)
	if err != nil {
		t.Fatalf("NewBinaryDocValuesFieldUpdates: %v", err)
	}
	for _, in := range []struct {
		doc int
		val []byte
	}{
		{42, []byte("alpha")},
		{3, []byte("bravo")},
		{17, []byte("charlie")},
	} {
		if err := b.AddBinary(in.doc, br(in.val)); err != nil {
			t.Fatalf("AddBinary doc=%d: %v", in.doc, err)
		}
	}
	if got := b.SizeLocked(); got != 3 {
		t.Fatalf("Size after 3 adds: got %d want 3", got)
	}
	if !b.Any() {
		t.Fatal("Any() = false after adds")
	}
	if err := b.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if !b.GetFinished() {
		t.Fatal("GetFinished() = false after Finish")
	}
	it := b.Iterator()
	wantDocs := []int{3, 17, 42}
	wantVals := [][]byte{[]byte("bravo"), []byte("charlie"), []byte("alpha")}
	for i, wd := range wantDocs {
		got := it.NextDoc()
		if got != wd {
			t.Fatalf("NextDoc[%d]: got %d want %d", i, got, wd)
		}
		if !it.HasValue() {
			t.Fatalf("HasValue[%d]: got false", i)
		}
		v := it.BinaryValue()
		actual := v.Bytes[v.Offset : v.Offset+v.Length]
		if !bytes.Equal(actual, wantVals[i]) {
			t.Fatalf("BinaryValue[%d]: got %q want %q", i, actual, wantVals[i])
		}
		if got := it.DelGen(); got != 7 {
			t.Fatalf("DelGen[%d]: got %d want 7", i, got)
		}
	}
	if got := it.NextDoc(); got != util.NO_MORE_DOCS {
		t.Fatalf("NextDoc after exhaustion: got %d want NO_MORE_DOCS", got)
	}
}

// TestBinaryDocValuesFieldUpdates_LastWriteWins asserts that two
// updates to the same doc collapse to the most-recently added value,
// per Lucene's stable-sort guarantee.
func TestBinaryDocValuesFieldUpdates_LastWriteWins(t *testing.T) {
	b, err := NewBinaryDocValuesFieldUpdates(0, "f", 10)
	if err != nil {
		t.Fatalf("NewBinaryDocValuesFieldUpdates: %v", err)
	}
	if err := b.AddBinary(5, br([]byte("first"))); err != nil {
		t.Fatalf("AddBinary first: %v", err)
	}
	if err := b.AddBinary(5, br([]byte("second"))); err != nil {
		t.Fatalf("AddBinary second: %v", err)
	}
	if err := b.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	it := b.Iterator()
	if got := it.NextDoc(); got != 5 {
		t.Fatalf("NextDoc: got %d want 5", got)
	}
	v := it.BinaryValue()
	if got := string(v.Bytes[v.Offset : v.Offset+v.Length]); got != "second" {
		t.Fatalf("BinaryValue: got %q want %q", got, "second")
	}
	if got := it.NextDoc(); got != util.NO_MORE_DOCS {
		t.Fatalf("NextDoc after exhaustion: got %d want NO_MORE_DOCS", got)
	}
}

// TestBinaryDocValuesFieldUpdates_ResetClearsValue asserts that
// Reset records a no-value entry that the iterator surfaces via
// HasValue=false.
func TestBinaryDocValuesFieldUpdates_ResetClearsValue(t *testing.T) {
	b, err := NewBinaryDocValuesFieldUpdates(3, "f", 10)
	if err != nil {
		t.Fatalf("NewBinaryDocValuesFieldUpdates: %v", err)
	}
	if err := b.AddBinary(2, br([]byte("keep"))); err != nil {
		t.Fatalf("AddBinary: %v", err)
	}
	if err := b.Reset(8); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	if err := b.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	it := b.Iterator()
	if got := it.NextDoc(); got != 2 {
		t.Fatalf("NextDoc[0]: got %d want 2", got)
	}
	if !it.HasValue() {
		t.Fatal("HasValue[0]: got false")
	}
	if got := it.NextDoc(); got != 8 {
		t.Fatalf("NextDoc[1]: got %d want 8", got)
	}
	if it.HasValue() {
		t.Fatal("HasValue[1]: got true, want false after Reset")
	}
}

// TestBinaryDocValuesFieldUpdates_AddAfterFinish asserts the Java
// IllegalStateException contract: further mutation after Finish is
// rejected with an error.
func TestBinaryDocValuesFieldUpdates_AddAfterFinish(t *testing.T) {
	b, err := NewBinaryDocValuesFieldUpdates(0, "f", 10)
	if err != nil {
		t.Fatalf("NewBinaryDocValuesFieldUpdates: %v", err)
	}
	if err := b.AddBinary(1, br([]byte("x"))); err != nil {
		t.Fatalf("AddBinary: %v", err)
	}
	if err := b.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if err := b.AddBinary(2, br([]byte("y"))); err == nil {
		t.Fatal("AddBinary after Finish: got nil error, want failure")
	}
	if err := b.Finish(); err == nil {
		t.Fatal("Finish after Finish: got nil error, want failure")
	}
}

// TestBinaryDocValuesFieldUpdates_IteratorBeforeFinishPanics asserts
// the Java ensureFinished() contract.
func TestBinaryDocValuesFieldUpdates_IteratorBeforeFinishPanics(t *testing.T) {
	b, err := NewBinaryDocValuesFieldUpdates(0, "f", 10)
	if err != nil {
		t.Fatalf("NewBinaryDocValuesFieldUpdates: %v", err)
	}
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Iterator before Finish: did not panic")
		}
	}()
	_ = b.Iterator()
}

// TestBinaryDocValuesFieldUpdates_OutOfRangeDoc asserts the maxDoc
// guard in addInternal.
func TestBinaryDocValuesFieldUpdates_OutOfRangeDoc(t *testing.T) {
	b, err := NewBinaryDocValuesFieldUpdates(0, "f", 5)
	if err != nil {
		t.Fatalf("NewBinaryDocValuesFieldUpdates: %v", err)
	}
	if err := b.AddBinary(5, br([]byte("x"))); err == nil {
		t.Fatal("AddBinary(5, ...) with maxDoc=5: got nil error, want failure")
	}
	if err := b.AddBinary(-1, br([]byte("x"))); err == nil {
		t.Fatal("AddBinary(-1, ...): got nil error, want failure")
	}
}

// TestBinaryDocValuesFieldUpdates_MergedIteratorPicksLargestDelGen
// verifies the merge-sort tie-break: when two packets cover the same
// doc, the value from the larger delGen wins.
func TestBinaryDocValuesFieldUpdates_MergedIteratorPicksLargestDelGen(t *testing.T) {
	older, err := NewBinaryDocValuesFieldUpdates(1, "f", 10)
	if err != nil {
		t.Fatalf("NewBinaryDocValuesFieldUpdates older: %v", err)
	}
	if err := older.AddBinary(2, br([]byte("OLD"))); err != nil {
		t.Fatalf("older.AddBinary: %v", err)
	}
	if err := older.AddBinary(5, br([]byte("only-old"))); err != nil {
		t.Fatalf("older.AddBinary: %v", err)
	}
	if err := older.Finish(); err != nil {
		t.Fatalf("older.Finish: %v", err)
	}

	newer, err := NewBinaryDocValuesFieldUpdates(2, "f", 10)
	if err != nil {
		t.Fatalf("NewBinaryDocValuesFieldUpdates newer: %v", err)
	}
	if err := newer.AddBinary(2, br([]byte("NEW"))); err != nil {
		t.Fatalf("newer.AddBinary: %v", err)
	}
	if err := newer.AddBinary(7, br([]byte("only-new"))); err != nil {
		t.Fatalf("newer.AddBinary: %v", err)
	}
	if err := newer.Finish(); err != nil {
		t.Fatalf("newer.Finish: %v", err)
	}

	merged := MergedDocValuesFieldUpdatesIterator([]DocValuesFieldUpdatesIterator{
		older.Iterator(),
		newer.Iterator(),
	})
	if merged == nil {
		t.Fatal("MergedDocValuesFieldUpdatesIterator: got nil, want iterator")
	}
	wantDocs := []int{2, 5, 7}
	wantVals := []string{"NEW", "only-old", "only-new"}
	for i, wd := range wantDocs {
		got := merged.NextDoc()
		if got != wd {
			t.Fatalf("NextDoc[%d]: got %d want %d", i, got, wd)
		}
		v := merged.BinaryValue()
		if got := string(v.Bytes[v.Offset : v.Offset+v.Length]); got != wantVals[i] {
			t.Fatalf("BinaryValue[%d]: got %q want %q", i, got, wantVals[i])
		}
	}
	if got := merged.NextDoc(); got != util.NO_MORE_DOCS {
		t.Fatalf("NextDoc after exhaustion: got %d want NO_MORE_DOCS", got)
	}
}

// TestBinaryDocValuesFieldUpdates_MergedIteratorSingleSubReturnsSub
// asserts the one-input fast path returns the input verbatim, per
// the Java {@code if (subs.length == 1) return subs[0];} shortcut.
func TestBinaryDocValuesFieldUpdates_MergedIteratorSingleSubReturnsSub(t *testing.T) {
	b, err := NewBinaryDocValuesFieldUpdates(0, "f", 10)
	if err != nil {
		t.Fatalf("NewBinaryDocValuesFieldUpdates: %v", err)
	}
	if err := b.AddBinary(0, br([]byte("only"))); err != nil {
		t.Fatalf("AddBinary: %v", err)
	}
	if err := b.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	sub := b.Iterator()
	merged := MergedDocValuesFieldUpdatesIterator([]DocValuesFieldUpdatesIterator{sub})
	// Same reference: Go interface equality compares the underlying
	// concrete value, which both wrap the same iterator.
	if merged != sub {
		t.Fatal("MergedDocValuesFieldUpdatesIterator with one sub: did not return the sub")
	}
}

// TestBinaryDocValuesFieldUpdates_MergedIteratorEmptyReturnsNil
// asserts the Java {@code if (queue.size() == 0) return null;}
// shortcut.
func TestBinaryDocValuesFieldUpdates_MergedIteratorEmptyReturnsNil(t *testing.T) {
	if got := MergedDocValuesFieldUpdatesIterator(nil); got != nil {
		t.Fatalf("nil subs: got %v want nil", got)
	}
	a, err := NewBinaryDocValuesFieldUpdates(0, "f", 10)
	if err != nil {
		t.Fatalf("NewBinaryDocValuesFieldUpdates: %v", err)
	}
	if err := a.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	b, err := NewBinaryDocValuesFieldUpdates(1, "f", 10)
	if err != nil {
		t.Fatalf("NewBinaryDocValuesFieldUpdates: %v", err)
	}
	if err := b.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	got := MergedDocValuesFieldUpdatesIterator([]DocValuesFieldUpdatesIterator{a.Iterator(), b.Iterator()})
	if got != nil {
		t.Fatalf("empty packets: got %v want nil", got)
	}
}

// TestBinaryDocValuesFieldUpdates_LongValuePanics asserts the Java
// {@code BinaryDocValuesFieldUpdates.Iterator#longValue()} contract.
func TestBinaryDocValuesFieldUpdates_LongValuePanics(t *testing.T) {
	b, err := NewBinaryDocValuesFieldUpdates(0, "f", 10)
	if err != nil {
		t.Fatalf("NewBinaryDocValuesFieldUpdates: %v", err)
	}
	if err := b.AddBinary(0, br([]byte("x"))); err != nil {
		t.Fatalf("AddBinary: %v", err)
	}
	if err := b.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	it := b.Iterator()
	if got := it.NextDoc(); got != 0 {
		t.Fatalf("NextDoc: got %d want 0", got)
	}
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("LongValue: did not panic")
		}
	}()
	_ = it.LongValue()
}

// TestBinaryDocValuesFieldUpdates_AddLongUnsupported asserts the Java
// {@code BinaryDocValuesFieldUpdates#add(int, long)} contract.
func TestBinaryDocValuesFieldUpdates_AddLongUnsupported(t *testing.T) {
	b, err := NewBinaryDocValuesFieldUpdates(0, "f", 10)
	if err != nil {
		t.Fatalf("NewBinaryDocValuesFieldUpdates: %v", err)
	}
	if err := b.AddLong(0, 42); err == nil {
		t.Fatal("AddLong: got nil error, want failure")
	}
}

// TestBinaryDocValuesFieldUpdates_TypeAndField asserts the trivial
// accessors round-trip the constructor parameters.
func TestBinaryDocValuesFieldUpdates_TypeAndField(t *testing.T) {
	b, err := NewBinaryDocValuesFieldUpdates(99, "my_field", 1)
	if err != nil {
		t.Fatalf("NewBinaryDocValuesFieldUpdates: %v", err)
	}
	if got := b.Field(); got != "my_field" {
		t.Errorf("Field: got %q want %q", got, "my_field")
	}
	if got := b.Type(); got != DocValuesTypeBinary {
		t.Errorf("Type: got %v want %v", got, DocValuesTypeBinary)
	}
	if got := b.DelGen(); got != 99 {
		t.Errorf("DelGen: got %d want 99", got)
	}
	if got := b.MaxDoc(); got != 1 {
		t.Errorf("MaxDoc: got %d want 1", got)
	}
}

// TestBinaryDocValuesFieldUpdates_RamBytesUsedNonZero asserts the
// approximate accounting returns a positive figure once the packet
// has content.
func TestBinaryDocValuesFieldUpdates_RamBytesUsedNonZero(t *testing.T) {
	b, err := NewBinaryDocValuesFieldUpdates(0, "f", 1024)
	if err != nil {
		t.Fatalf("NewBinaryDocValuesFieldUpdates: %v", err)
	}
	if err := b.AddBinary(0, br([]byte("hello world"))); err != nil {
		t.Fatalf("AddBinary: %v", err)
	}
	if got := b.RamBytesUsed(); got <= 0 {
		t.Fatalf("RamBytesUsed: got %d want > 0", got)
	}
}

// TestBinaryDocValuesFieldUpdates_GrowAcrossPages exercises the
// PagedGrowableWriter.Grow path by pushing well past the initial
// single-page allocation, forcing multiple grow rounds. With a
// page size of 1024 this guarantees a page boundary is crossed.
func TestBinaryDocValuesFieldUpdates_GrowAcrossPages(t *testing.T) {
	const n = 3_000
	b, err := NewBinaryDocValuesFieldUpdates(0, "f", n)
	if err != nil {
		t.Fatalf("NewBinaryDocValuesFieldUpdates: %v", err)
	}
	for i := 0; i < n; i++ {
		if err := b.AddBinary(i, br([]byte{byte(i), byte(i >> 8)})); err != nil {
			t.Fatalf("AddBinary[%d]: %v", i, err)
		}
	}
	if err := b.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	it := b.Iterator()
	for i := 0; i < n; i++ {
		got := it.NextDoc()
		if got != i {
			t.Fatalf("NextDoc[%d]: got %d", i, got)
		}
		v := it.BinaryValue()
		want := []byte{byte(i), byte(i >> 8)}
		if !bytes.Equal(v.Bytes[v.Offset:v.Offset+v.Length], want) {
			t.Fatalf("BinaryValue[%d]: got %v want %v", i, v.Bytes[v.Offset:v.Offset+v.Length], want)
		}
	}
	if got := it.NextDoc(); got != util.NO_MORE_DOCS {
		t.Fatalf("NextDoc after exhaustion: got %d want NO_MORE_DOCS", got)
	}
}

// TestBinaryDocValuesFieldUpdates_AddFromIterator asserts the
// AddFromIterator helper copies the current binary value of the
// source iterator into the new packet.
func TestBinaryDocValuesFieldUpdates_AddFromIterator(t *testing.T) {
	src, err := NewBinaryDocValuesFieldUpdates(0, "f", 10)
	if err != nil {
		t.Fatalf("NewBinaryDocValuesFieldUpdates src: %v", err)
	}
	if err := src.AddBinary(3, br([]byte("from-src"))); err != nil {
		t.Fatalf("src.AddBinary: %v", err)
	}
	if err := src.Finish(); err != nil {
		t.Fatalf("src.Finish: %v", err)
	}
	srcIt := src.Iterator()
	if got := srcIt.NextDoc(); got != 3 {
		t.Fatalf("src.NextDoc: got %d want 3", got)
	}

	dst, err := NewBinaryDocValuesFieldUpdates(1, "f", 10)
	if err != nil {
		t.Fatalf("NewBinaryDocValuesFieldUpdates dst: %v", err)
	}
	if err := dst.AddFromIterator(5, srcIt); err != nil {
		t.Fatalf("AddFromIterator: %v", err)
	}
	if err := dst.Finish(); err != nil {
		t.Fatalf("dst.Finish: %v", err)
	}
	dstIt := dst.Iterator()
	if got := dstIt.NextDoc(); got != 5 {
		t.Fatalf("dst.NextDoc: got %d want 5", got)
	}
	v := dstIt.BinaryValue()
	if got := string(v.Bytes[v.Offset : v.Offset+v.Length]); got != "from-src" {
		t.Fatalf("dst.BinaryValue: got %q want %q", got, "from-src")
	}
}
