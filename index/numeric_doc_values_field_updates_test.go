// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestNumericDocValuesFieldUpdates_BasicAdd asserts the happy path:
// add three updates in arbitrary order, finish, and verify the
// iterator returns them sorted by doc id with their values intact.
func TestNumericDocValuesFieldUpdates_BasicAdd(t *testing.T) {
	n, err := NewNumericDocValuesFieldUpdates(7, "f", 100)
	if err != nil {
		t.Fatalf("NewNumericDocValuesFieldUpdates: %v", err)
	}
	for _, in := range []struct {
		doc int
		val int64
	}{
		{42, 100},
		{3, 200},
		{17, 300},
	} {
		if err := n.AddLong(in.doc, in.val); err != nil {
			t.Fatalf("AddLong doc=%d: %v", in.doc, err)
		}
	}
	if got := n.SizeLocked(); got != 3 {
		t.Fatalf("Size after 3 adds: got %d want 3", got)
	}
	if !n.Any() {
		t.Fatal("Any() = false after adds")
	}
	if err := n.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if !n.GetFinished() {
		t.Fatal("GetFinished() = false after Finish")
	}
	it := n.Iterator()
	wantDocs := []int{3, 17, 42}
	wantVals := []int64{200, 300, 100}
	for i, wd := range wantDocs {
		got := it.NextDoc()
		if got != wd {
			t.Fatalf("NextDoc[%d]: got %d want %d", i, got, wd)
		}
		if !it.HasValue() {
			t.Fatalf("HasValue[%d]: got false", i)
		}
		if got := it.LongValue(); got != wantVals[i] {
			t.Fatalf("LongValue[%d]: got %d want %d", i, got, wantVals[i])
		}
		if got := it.DelGen(); got != 7 {
			t.Fatalf("DelGen[%d]: got %d want 7", i, got)
		}
	}
	if got := it.NextDoc(); got != util.NO_MORE_DOCS {
		t.Fatalf("NextDoc after exhaustion: got %d want NO_MORE_DOCS", got)
	}
}

// TestNumericDocValuesFieldUpdates_NegativeValues asserts values that
// are not rebased to zero (the unbounded constructor uses minValue 0)
// round-trip correctly, including negative longs.
func TestNumericDocValuesFieldUpdates_NegativeValues(t *testing.T) {
	n, err := NewNumericDocValuesFieldUpdates(0, "f", 10)
	if err != nil {
		t.Fatalf("NewNumericDocValuesFieldUpdates: %v", err)
	}
	if err := n.AddLong(1, -42); err != nil {
		t.Fatalf("AddLong: %v", err)
	}
	if err := n.AddLong(2, 1<<40); err != nil {
		t.Fatalf("AddLong: %v", err)
	}
	if err := n.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	it := n.Iterator()
	if got := it.NextDoc(); got != 1 {
		t.Fatalf("NextDoc[0]: got %d want 1", got)
	}
	if got := it.LongValue(); got != -42 {
		t.Fatalf("LongValue[0]: got %d want -42", got)
	}
	if got := it.NextDoc(); got != 2 {
		t.Fatalf("NextDoc[1]: got %d want 2", got)
	}
	if got := it.LongValue(); got != 1<<40 {
		t.Fatalf("LongValue[1]: got %d want %d", got, int64(1<<40))
	}
}

// TestNumericDocValuesFieldUpdates_Bounded asserts the bounded
// constructor rebases stored values by minValue and exposes the
// original values through the iterator.
func TestNumericDocValuesFieldUpdates_Bounded(t *testing.T) {
	n, err := NewNumericDocValuesFieldUpdatesBounded(0, "f", 1000, 2000, 10)
	if err != nil {
		t.Fatalf("NewNumericDocValuesFieldUpdatesBounded: %v", err)
	}
	if err := n.AddLong(1, 1500); err != nil {
		t.Fatalf("AddLong: %v", err)
	}
	if err := n.AddLong(4, 1000); err != nil {
		t.Fatalf("AddLong: %v", err)
	}
	if err := n.AddLong(8, 2000); err != nil {
		t.Fatalf("AddLong: %v", err)
	}
	if err := n.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	it := n.Iterator()
	wantDocs := []int{1, 4, 8}
	wantVals := []int64{1500, 1000, 2000}
	for i, wd := range wantDocs {
		if got := it.NextDoc(); got != wd {
			t.Fatalf("NextDoc[%d]: got %d want %d", i, got, wd)
		}
		if got := it.LongValue(); got != wantVals[i] {
			t.Fatalf("LongValue[%d]: got %d want %d", i, got, wantVals[i])
		}
	}
}

// TestNumericDocValuesFieldUpdates_BoundedRejectsBadRange asserts the
// minValue <= maxValue guard.
func TestNumericDocValuesFieldUpdates_BoundedRejectsBadRange(t *testing.T) {
	if _, err := NewNumericDocValuesFieldUpdatesBounded(0, "f", 10, 5, 10); err == nil {
		t.Fatal("NewNumericDocValuesFieldUpdatesBounded with minValue > maxValue: got nil error, want failure")
	}
}

// TestNumericDocValuesFieldUpdates_LastWriteWins asserts that two
// updates to the same doc collapse to the most-recently added value,
// per Lucene's stable-sort guarantee.
func TestNumericDocValuesFieldUpdates_LastWriteWins(t *testing.T) {
	n, err := NewNumericDocValuesFieldUpdates(0, "f", 10)
	if err != nil {
		t.Fatalf("NewNumericDocValuesFieldUpdates: %v", err)
	}
	if err := n.AddLong(5, 111); err != nil {
		t.Fatalf("AddLong first: %v", err)
	}
	if err := n.AddLong(5, 222); err != nil {
		t.Fatalf("AddLong second: %v", err)
	}
	if err := n.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	it := n.Iterator()
	if got := it.NextDoc(); got != 5 {
		t.Fatalf("NextDoc: got %d want 5", got)
	}
	if got := it.LongValue(); got != 222 {
		t.Fatalf("LongValue: got %d want 222", got)
	}
	if got := it.NextDoc(); got != util.NO_MORE_DOCS {
		t.Fatalf("NextDoc after exhaustion: got %d want NO_MORE_DOCS", got)
	}
}

// TestNumericDocValuesFieldUpdates_ResetClearsValue asserts that Reset
// records a no-value entry that the iterator surfaces via
// HasValue=false.
func TestNumericDocValuesFieldUpdates_ResetClearsValue(t *testing.T) {
	n, err := NewNumericDocValuesFieldUpdates(3, "f", 10)
	if err != nil {
		t.Fatalf("NewNumericDocValuesFieldUpdates: %v", err)
	}
	if err := n.AddLong(2, 99); err != nil {
		t.Fatalf("AddLong: %v", err)
	}
	if err := n.Reset(8); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	if err := n.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	it := n.Iterator()
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

// TestNumericDocValuesFieldUpdates_AddAfterFinish asserts the Java
// IllegalStateException contract: further mutation after Finish is
// rejected with an error.
func TestNumericDocValuesFieldUpdates_AddAfterFinish(t *testing.T) {
	n, err := NewNumericDocValuesFieldUpdates(0, "f", 10)
	if err != nil {
		t.Fatalf("NewNumericDocValuesFieldUpdates: %v", err)
	}
	if err := n.AddLong(1, 1); err != nil {
		t.Fatalf("AddLong: %v", err)
	}
	if err := n.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if err := n.AddLong(2, 2); err == nil {
		t.Fatal("AddLong after Finish: got nil error, want failure")
	}
	if err := n.Finish(); err == nil {
		t.Fatal("Finish after Finish: got nil error, want failure")
	}
}

// TestNumericDocValuesFieldUpdates_IteratorBeforeFinishPanics asserts
// the Java ensureFinished() contract.
func TestNumericDocValuesFieldUpdates_IteratorBeforeFinishPanics(t *testing.T) {
	n, err := NewNumericDocValuesFieldUpdates(0, "f", 10)
	if err != nil {
		t.Fatalf("NewNumericDocValuesFieldUpdates: %v", err)
	}
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Iterator before Finish: did not panic")
		}
	}()
	_ = n.Iterator()
}

// TestNumericDocValuesFieldUpdates_OutOfRangeDoc asserts the maxDoc
// guard in addInternal.
func TestNumericDocValuesFieldUpdates_OutOfRangeDoc(t *testing.T) {
	n, err := NewNumericDocValuesFieldUpdates(0, "f", 5)
	if err != nil {
		t.Fatalf("NewNumericDocValuesFieldUpdates: %v", err)
	}
	if err := n.AddLong(5, 1); err == nil {
		t.Fatal("AddLong(5, ...) with maxDoc=5: got nil error, want failure")
	}
	if err := n.AddLong(-1, 1); err == nil {
		t.Fatal("AddLong(-1, ...): got nil error, want failure")
	}
}

// TestNumericDocValuesFieldUpdates_BinaryValuePanics asserts the Java
// {@code NumericDocValuesFieldUpdates.Iterator#binaryValue()} contract.
func TestNumericDocValuesFieldUpdates_BinaryValuePanics(t *testing.T) {
	n, err := NewNumericDocValuesFieldUpdates(0, "f", 10)
	if err != nil {
		t.Fatalf("NewNumericDocValuesFieldUpdates: %v", err)
	}
	if err := n.AddLong(0, 1); err != nil {
		t.Fatalf("AddLong: %v", err)
	}
	if err := n.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	it := n.Iterator()
	if got := it.NextDoc(); got != 0 {
		t.Fatalf("NextDoc: got %d want 0", got)
	}
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("BinaryValue: did not panic")
		}
	}()
	_ = it.BinaryValue()
}

// TestNumericDocValuesFieldUpdates_AddBinaryUnsupported asserts the
// Java {@code NumericDocValuesFieldUpdates#add(int, BytesRef)} contract.
func TestNumericDocValuesFieldUpdates_AddBinaryUnsupported(t *testing.T) {
	n, err := NewNumericDocValuesFieldUpdates(0, "f", 10)
	if err != nil {
		t.Fatalf("NewNumericDocValuesFieldUpdates: %v", err)
	}
	if err := n.AddBinary(0, &util.BytesRef{Bytes: []byte("x"), Length: 1}); err == nil {
		t.Fatal("AddBinary: got nil error, want failure")
	}
}

// TestNumericDocValuesFieldUpdates_TypeAndField asserts the trivial
// accessors round-trip the constructor parameters.
func TestNumericDocValuesFieldUpdates_TypeAndField(t *testing.T) {
	n, err := NewNumericDocValuesFieldUpdates(99, "my_field", 1)
	if err != nil {
		t.Fatalf("NewNumericDocValuesFieldUpdates: %v", err)
	}
	if got := n.Field(); got != "my_field" {
		t.Errorf("Field: got %q want %q", got, "my_field")
	}
	if got := n.Type(); got != DocValuesTypeNumeric {
		t.Errorf("Type: got %v want %v", got, DocValuesTypeNumeric)
	}
	if got := n.DelGen(); got != 99 {
		t.Errorf("DelGen: got %d want 99", got)
	}
	if got := n.MaxDoc(); got != 1 {
		t.Errorf("MaxDoc: got %d want 1", got)
	}
}

// TestNumericDocValuesFieldUpdates_RamBytesUsedNonZero asserts the
// approximate accounting returns a positive figure once the packet has
// content.
func TestNumericDocValuesFieldUpdates_RamBytesUsedNonZero(t *testing.T) {
	n, err := NewNumericDocValuesFieldUpdates(0, "f", 1024)
	if err != nil {
		t.Fatalf("NewNumericDocValuesFieldUpdates: %v", err)
	}
	if err := n.AddLong(0, 123456); err != nil {
		t.Fatalf("AddLong: %v", err)
	}
	if got := n.RamBytesUsed(); got <= 0 {
		t.Fatalf("RamBytesUsed: got %d want > 0", got)
	}
}

// TestNumericDocValuesFieldUpdates_GrowAcrossPages exercises the
// PagedGrowableWriter.Grow path by pushing well past the initial
// single-page allocation, forcing multiple grow rounds. With a page
// size of 1024 this guarantees a page boundary is crossed.
func TestNumericDocValuesFieldUpdates_GrowAcrossPages(t *testing.T) {
	const count = 3_000
	n, err := NewNumericDocValuesFieldUpdates(0, "f", count)
	if err != nil {
		t.Fatalf("NewNumericDocValuesFieldUpdates: %v", err)
	}
	for i := 0; i < count; i++ {
		if err := n.AddLong(i, int64(i)*7); err != nil {
			t.Fatalf("AddLong[%d]: %v", i, err)
		}
	}
	if err := n.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	it := n.Iterator()
	for i := 0; i < count; i++ {
		got := it.NextDoc()
		if got != i {
			t.Fatalf("NextDoc[%d]: got %d", i, got)
		}
		if v := it.LongValue(); v != int64(i)*7 {
			t.Fatalf("LongValue[%d]: got %d want %d", i, v, int64(i)*7)
		}
	}
	if got := it.NextDoc(); got != util.NO_MORE_DOCS {
		t.Fatalf("NextDoc after exhaustion: got %d want NO_MORE_DOCS", got)
	}
}

// TestNumericDocValuesFieldUpdates_AddFromIterator asserts the
// AddFromIterator helper copies the current long value of the source
// iterator into the new packet.
func TestNumericDocValuesFieldUpdates_AddFromIterator(t *testing.T) {
	src, err := NewNumericDocValuesFieldUpdates(0, "f", 10)
	if err != nil {
		t.Fatalf("NewNumericDocValuesFieldUpdates src: %v", err)
	}
	if err := src.AddLong(3, 555); err != nil {
		t.Fatalf("src.AddLong: %v", err)
	}
	if err := src.Finish(); err != nil {
		t.Fatalf("src.Finish: %v", err)
	}
	srcIt := src.Iterator()
	if got := srcIt.NextDoc(); got != 3 {
		t.Fatalf("src.NextDoc: got %d want 3", got)
	}

	dst, err := NewNumericDocValuesFieldUpdates(1, "f", 10)
	if err != nil {
		t.Fatalf("NewNumericDocValuesFieldUpdates dst: %v", err)
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
	if got := dstIt.LongValue(); got != 555 {
		t.Fatalf("dst.LongValue: got %d want 555", got)
	}
}

// TestNumericDocValuesFieldUpdates_MergedIteratorPicksLargestDelGen
// verifies the merge-sort tie-break: when two packets cover the same
// doc, the value from the larger delGen wins.
func TestNumericDocValuesFieldUpdates_MergedIteratorPicksLargestDelGen(t *testing.T) {
	older, err := NewNumericDocValuesFieldUpdates(1, "f", 10)
	if err != nil {
		t.Fatalf("NewNumericDocValuesFieldUpdates older: %v", err)
	}
	if err := older.AddLong(2, 10); err != nil {
		t.Fatalf("older.AddLong: %v", err)
	}
	if err := older.AddLong(5, 50); err != nil {
		t.Fatalf("older.AddLong: %v", err)
	}
	if err := older.Finish(); err != nil {
		t.Fatalf("older.Finish: %v", err)
	}

	newer, err := NewNumericDocValuesFieldUpdates(2, "f", 10)
	if err != nil {
		t.Fatalf("NewNumericDocValuesFieldUpdates newer: %v", err)
	}
	if err := newer.AddLong(2, 999); err != nil {
		t.Fatalf("newer.AddLong: %v", err)
	}
	if err := newer.AddLong(7, 70); err != nil {
		t.Fatalf("newer.AddLong: %v", err)
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
	wantVals := []int64{999, 50, 70}
	for i, wd := range wantDocs {
		if got := merged.NextDoc(); got != wd {
			t.Fatalf("NextDoc[%d]: got %d want %d", i, got, wd)
		}
		if got := merged.LongValue(); got != wantVals[i] {
			t.Fatalf("LongValue[%d]: got %d want %d", i, got, wantVals[i])
		}
	}
	if got := merged.NextDoc(); got != util.NO_MORE_DOCS {
		t.Fatalf("NextDoc after exhaustion: got %d want NO_MORE_DOCS", got)
	}
}

// TestSingleValueNumericDocValuesFieldUpdates_BasicAdd asserts the
// single-value packet surfaces every added doc, in order, all carrying
// the shared value.
func TestSingleValueNumericDocValuesFieldUpdates_BasicAdd(t *testing.T) {
	s, err := NewSingleValueNumericDocValuesFieldUpdates(4, "f", 100, 777)
	if err != nil {
		t.Fatalf("NewSingleValueNumericDocValuesFieldUpdates: %v", err)
	}
	if got := s.LongValue(); got != 777 {
		t.Fatalf("LongValue: got %d want 777", got)
	}
	for _, doc := range []int{42, 3, 17} {
		if err := s.AddLong(doc, 777); err != nil {
			t.Fatalf("AddLong doc=%d: %v", doc, err)
		}
	}
	if !s.Any() {
		t.Fatal("Any() = false after adds")
	}
	it := s.Iterator()
	for _, wd := range []int{3, 17, 42} {
		if got := it.NextDoc(); got != wd {
			t.Fatalf("NextDoc: got %d want %d", got, wd)
		}
		if !it.HasValue() {
			t.Fatalf("HasValue at doc %d: got false", wd)
		}
		if got := it.LongValue(); got != 777 {
			t.Fatalf("LongValue at doc %d: got %d want 777", wd, got)
		}
		if got := it.DelGen(); got != 4 {
			t.Fatalf("DelGen at doc %d: got %d want 4", wd, got)
		}
	}
	if got := it.NextDoc(); got != util.NO_MORE_DOCS {
		t.Fatalf("NextDoc after exhaustion: got %d want NO_MORE_DOCS", got)
	}
}

// TestSingleValueNumericDocValuesFieldUpdates_WrongValueRejected
// asserts the Java {@code assert this.value == value} contract.
func TestSingleValueNumericDocValuesFieldUpdates_WrongValueRejected(t *testing.T) {
	s, err := NewSingleValueNumericDocValuesFieldUpdates(0, "f", 10, 5)
	if err != nil {
		t.Fatalf("NewSingleValueNumericDocValuesFieldUpdates: %v", err)
	}
	if err := s.AddLong(1, 6); err == nil {
		t.Fatal("AddLong with mismatched value: got nil error, want failure")
	}
}

// TestSingleValueNumericDocValuesFieldUpdates_Reset asserts Reset marks
// a doc as present but value-less, surfaced via HasValue=false.
func TestSingleValueNumericDocValuesFieldUpdates_Reset(t *testing.T) {
	s, err := NewSingleValueNumericDocValuesFieldUpdates(0, "f", 20, 9)
	if err != nil {
		t.Fatalf("NewSingleValueNumericDocValuesFieldUpdates: %v", err)
	}
	if err := s.AddLong(2, 9); err != nil {
		t.Fatalf("AddLong: %v", err)
	}
	if err := s.Reset(8); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	if !s.Any() {
		t.Fatal("Any() = false after reset")
	}
	it := s.Iterator()
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

// TestSingleValueNumericDocValuesFieldUpdates_ResetThenAddRestoresValue
// asserts an Add after a Reset on the same doc clears the no-value
// flag, mirroring the Java {@code hasNoValue.clear(doc)} in add.
func TestSingleValueNumericDocValuesFieldUpdates_ResetThenAddRestoresValue(t *testing.T) {
	s, err := NewSingleValueNumericDocValuesFieldUpdates(0, "f", 20, 9)
	if err != nil {
		t.Fatalf("NewSingleValueNumericDocValuesFieldUpdates: %v", err)
	}
	if err := s.Reset(5); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	if err := s.AddLong(5, 9); err != nil {
		t.Fatalf("AddLong: %v", err)
	}
	it := s.Iterator()
	if got := it.NextDoc(); got != 5 {
		t.Fatalf("NextDoc: got %d want 5", got)
	}
	if !it.HasValue() {
		t.Fatal("HasValue: got false, want true after Add re-cleared the no-value flag")
	}
}

// TestSingleValueNumericDocValuesFieldUpdates_OutOfRangeDoc asserts the
// maxDoc guard on AddLong and Reset.
func TestSingleValueNumericDocValuesFieldUpdates_OutOfRangeDoc(t *testing.T) {
	s, err := NewSingleValueNumericDocValuesFieldUpdates(0, "f", 5, 1)
	if err != nil {
		t.Fatalf("NewSingleValueNumericDocValuesFieldUpdates: %v", err)
	}
	if err := s.AddLong(5, 1); err == nil {
		t.Fatal("AddLong(5, ...) with maxDoc=5: got nil error, want failure")
	}
	if err := s.AddLong(-1, 1); err == nil {
		t.Fatal("AddLong(-1, ...): got nil error, want failure")
	}
	if err := s.Reset(5); err == nil {
		t.Fatal("Reset(5) with maxDoc=5: got nil error, want failure")
	}
}

// TestSingleValueNumericDocValuesFieldUpdates_Unsupported asserts the
// AddBinary and AddFromIterator unsupported-operation contracts.
func TestSingleValueNumericDocValuesFieldUpdates_Unsupported(t *testing.T) {
	s, err := NewSingleValueNumericDocValuesFieldUpdates(0, "f", 10, 1)
	if err != nil {
		t.Fatalf("NewSingleValueNumericDocValuesFieldUpdates: %v", err)
	}
	if err := s.AddBinary(0, &util.BytesRef{Bytes: []byte("x"), Length: 1}); err == nil {
		t.Fatal("AddBinary: got nil error, want failure")
	}
	if err := s.AddFromIterator(0, nil); err == nil {
		t.Fatal("AddFromIterator: got nil error, want failure")
	}
}

// TestSingleValueNumericDocValuesFieldUpdates_BinaryValuePanics asserts
// the iterator's binaryValue() unsupported contract.
func TestSingleValueNumericDocValuesFieldUpdates_BinaryValuePanics(t *testing.T) {
	s, err := NewSingleValueNumericDocValuesFieldUpdates(0, "f", 10, 1)
	if err != nil {
		t.Fatalf("NewSingleValueNumericDocValuesFieldUpdates: %v", err)
	}
	if err := s.AddLong(0, 1); err != nil {
		t.Fatalf("AddLong: %v", err)
	}
	it := s.Iterator()
	if got := it.NextDoc(); got != 0 {
		t.Fatalf("NextDoc: got %d want 0", got)
	}
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("BinaryValue: did not panic")
		}
	}()
	_ = it.BinaryValue()
}

// TestSingleValueNumericDocValuesFieldUpdates_RamBytesUsedNonZero
// asserts the approximate accounting returns a positive figure, and
// that recording a Reset (which allocates the hasNoValue bit set)
// increases the reported footprint.
func TestSingleValueNumericDocValuesFieldUpdates_RamBytesUsedNonZero(t *testing.T) {
	s, err := NewSingleValueNumericDocValuesFieldUpdates(0, "f", 4096, 1)
	if err != nil {
		t.Fatalf("NewSingleValueNumericDocValuesFieldUpdates: %v", err)
	}
	before := s.RamBytesUsed()
	if before <= 0 {
		t.Fatalf("RamBytesUsed before reset: got %d want > 0", before)
	}
	if err := s.Reset(10); err != nil {
		t.Fatalf("Reset: %v", err)
	}
	if after := s.RamBytesUsed(); after <= before {
		t.Fatalf("RamBytesUsed after reset: got %d want > %d", after, before)
	}
}

// TestSingleValueNumericDocValuesFieldUpdates_TypeAndField asserts the
// trivial accessors round-trip the constructor parameters.
func TestSingleValueNumericDocValuesFieldUpdates_TypeAndField(t *testing.T) {
	s, err := NewSingleValueNumericDocValuesFieldUpdates(99, "my_field", 1, 0)
	if err != nil {
		t.Fatalf("NewSingleValueNumericDocValuesFieldUpdates: %v", err)
	}
	if got := s.Field(); got != "my_field" {
		t.Errorf("Field: got %q want %q", got, "my_field")
	}
	if got := s.Type(); got != DocValuesTypeNumeric {
		t.Errorf("Type: got %v want %v", got, DocValuesTypeNumeric)
	}
	if got := s.DelGen(); got != 99 {
		t.Errorf("DelGen: got %d want 99", got)
	}
	if got := s.MaxDoc(); got != 1 {
		t.Errorf("MaxDoc: got %d want 1", got)
	}
}
