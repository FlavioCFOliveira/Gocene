// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"bytes"
	"fmt"
	"math/rand"
	"sort"
	"testing"
)

// newTestTempDir constructs an os-backed TempDirectory and registers
// a cleanup that removes the underlying root on test completion.
func newTestTempDir(t *testing.T) TempDirectory {
	t.Helper()
	d, err := NewOSTempDirectory("gocene-offline-")
	if err != nil {
		t.Fatalf("NewOSTempDirectory: %v", err)
	}
	t.Cleanup(func() { _ = CleanupOSTempRoot(d) })
	return d
}

// TestOfflineSorter_Empty verifies the empty-input path produces an
// empty output without errors.
func TestOfflineSorter_Empty(t *testing.T) {
	dir := newTestTempDir(t)
	input, err := WriteEntries(dir, "in", nil)
	if err != nil {
		t.Fatalf("WriteEntries: %v", err)
	}
	s, err := NewOfflineSorter(dir, "test", WithBufferSize(OfflineSorterAbsoluteMinSortBufferSize))
	if err != nil {
		t.Fatalf("NewOfflineSorter: %v", err)
	}
	out, err := s.Sort(input)
	if err != nil {
		t.Fatalf("Sort: %v", err)
	}
	got, err := ReadEntries(dir, out, -1)
	if err != nil {
		t.Fatalf("ReadEntries: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty output, got %d entries", len(got))
	}
}

// TestOfflineSorter_Single verifies the single-entry round-trip.
func TestOfflineSorter_Single(t *testing.T) {
	dir := newTestTempDir(t)
	input, err := WriteEntries(dir, "in", [][]byte{[]byte("hello")})
	if err != nil {
		t.Fatalf("WriteEntries: %v", err)
	}
	s, err := NewOfflineSorter(dir, "test", WithBufferSize(OfflineSorterAbsoluteMinSortBufferSize))
	if err != nil {
		t.Fatalf("NewOfflineSorter: %v", err)
	}
	out, err := s.Sort(input)
	if err != nil {
		t.Fatalf("Sort: %v", err)
	}
	got, err := ReadEntries(dir, out, -1)
	if err != nil {
		t.Fatalf("ReadEntries: %v", err)
	}
	if len(got) != 1 || string(got[0]) != "hello" {
		t.Fatalf("got=%v", got)
	}
}

// TestOfflineSorter_InMemory verifies a fully in-memory sort path
// (one partition, no merge).
func TestOfflineSorter_InMemory(t *testing.T) {
	dir := newTestTempDir(t)
	entries := [][]byte{
		[]byte("zebra"), []byte("apple"), []byte("mango"),
		[]byte("banana"), []byte("cherry"),
	}
	input, err := WriteEntries(dir, "in", entries)
	if err != nil {
		t.Fatalf("WriteEntries: %v", err)
	}
	s, err := NewOfflineSorter(dir, "test", WithBufferSize(OfflineSorterAbsoluteMinSortBufferSize))
	if err != nil {
		t.Fatalf("NewOfflineSorter: %v", err)
	}
	out, err := s.Sort(input)
	if err != nil {
		t.Fatalf("Sort: %v", err)
	}
	got, err := ReadEntries(dir, out, -1)
	if err != nil {
		t.Fatalf("ReadEntries: %v", err)
	}
	want := []string{"apple", "banana", "cherry", "mango", "zebra"}
	if len(got) != len(want) {
		t.Fatalf("got=%v want=%v", got, want)
	}
	for i := range want {
		if string(got[i]) != want[i] {
			t.Fatalf("pos %d: got=%q want=%q", i, got[i], want[i])
		}
	}
}

// TestOfflineSorter_MultiPartition forces external merge by setting a
// tight buffer so each partition holds only a handful of entries.
func TestOfflineSorter_MultiPartition(t *testing.T) {
	dir := newTestTempDir(t)

	const N = 1000
	rnd := rand.New(rand.NewSource(42))
	entries := make([][]byte, N)
	for i := 0; i < N; i++ {
		entries[i] = []byte(fmt.Sprintf("%08d-payload", rnd.Intn(1_000_000)))
	}
	input, err := WriteEntries(dir, "in", entries)
	if err != nil {
		t.Fatalf("WriteEntries: %v", err)
	}

	// Tight buffer (smaller than 1 MB? must be at least the absolute
	// minimum). Use the absolute minimum so partitions are small.
	s, err := NewOfflineSorter(dir, "test",
		WithBufferSize(OfflineSorterAbsoluteMinSortBufferSize),
		WithMaxTempFiles(4),
	)
	if err != nil {
		t.Fatalf("NewOfflineSorter: %v", err)
	}
	out, err := s.Sort(input)
	if err != nil {
		t.Fatalf("Sort: %v", err)
	}
	got, err := ReadEntries(dir, out, -1)
	if err != nil {
		t.Fatalf("ReadEntries: %v", err)
	}
	if len(got) != N {
		t.Fatalf("len(got)=%d want %d", len(got), N)
	}
	want := make([][]byte, N)
	copy(want, entries)
	sort.Slice(want, func(i, j int) bool { return bytes.Compare(want[i], want[j]) < 0 })
	for i := range want {
		if !bytes.Equal(got[i], want[i]) {
			t.Fatalf("pos %d: got=%q want=%q", i, got[i], want[i])
		}
	}
}

// TestOfflineSorter_FixedLength exercises the fixed-width entry path
// (valueLength != -1). Mirrors the Java behaviour: the on-disk format
// remains length-prefixed; the valueLength only caps partition size
// and selects the in-memory storage layout.
func TestOfflineSorter_FixedLength(t *testing.T) {
	dir := newTestTempDir(t)
	entries := [][]byte{
		{0x05, 0x00, 0x00, 0x00},
		{0x02, 0x00, 0x00, 0x00},
		{0x09, 0x00, 0x00, 0x00},
		{0x01, 0x00, 0x00, 0x00},
	}
	input, err := WriteEntries(dir, "in", entries)
	if err != nil {
		t.Fatalf("WriteEntries: %v", err)
	}
	s, err := NewOfflineSorter(dir, "test",
		WithBufferSize(OfflineSorterAbsoluteMinSortBufferSize),
		WithFixedValueLength(4),
	)
	if err != nil {
		t.Fatalf("NewOfflineSorter: %v", err)
	}
	out, err := s.Sort(input)
	if err != nil {
		t.Fatalf("Sort: %v", err)
	}
	got, err := ReadEntries(dir, out, -1)
	if err != nil {
		t.Fatalf("ReadEntries: %v", err)
	}
	want := [][]byte{
		{0x01, 0x00, 0x00, 0x00},
		{0x02, 0x00, 0x00, 0x00},
		{0x05, 0x00, 0x00, 0x00},
		{0x09, 0x00, 0x00, 0x00},
	}
	if len(got) != len(want) {
		t.Fatalf("len(got)=%d want %d", len(got), len(want))
	}
	for i := range want {
		if !bytes.Equal(got[i], want[i]) {
			t.Fatalf("pos %d: got=%v want=%v", i, got[i], want[i])
		}
	}
}

// TestOfflineSorter_BufferTooSmall verifies the validation guard.
func TestOfflineSorter_BufferTooSmall(t *testing.T) {
	dir := newTestTempDir(t)
	_, err := NewOfflineSorter(dir, "test", WithBufferSize(1024))
	if err == nil {
		t.Fatalf("expected error for too-small buffer")
	}
}

// TestOfflineSorter_CustomComparator verifies the comparator override.
func TestOfflineSorter_CustomComparator(t *testing.T) {
	dir := newTestTempDir(t)
	entries := [][]byte{[]byte("aa"), []byte("c"), []byte("bbb")}
	input, err := WriteEntries(dir, "in", entries)
	if err != nil {
		t.Fatalf("WriteEntries: %v", err)
	}
	// Sort by length descending.
	cmp := func(a, b []byte) int { return len(b) - len(a) }
	s, err := NewOfflineSorter(dir, "test",
		WithBufferSize(OfflineSorterAbsoluteMinSortBufferSize),
		WithComparator(cmp),
	)
	if err != nil {
		t.Fatalf("NewOfflineSorter: %v", err)
	}
	out, err := s.Sort(input)
	if err != nil {
		t.Fatalf("Sort: %v", err)
	}
	got, err := ReadEntries(dir, out, -1)
	if err != nil {
		t.Fatalf("ReadEntries: %v", err)
	}
	want := []string{"bbb", "aa", "c"}
	for i := range want {
		if string(got[i]) != want[i] {
			t.Fatalf("pos %d: got=%q want=%q", i, got[i], want[i])
		}
	}
}

// TestSortInfo_String checks the diagnostic formatter.
func TestSortInfo_String(t *testing.T) {
	si := &SortInfo{BufferSize: OfflineSorterMB, LineCount: 10, TempMergeFiles: 2, MergeRounds: 1, TotalTimeMS: 1500, ReadTimeMS: 200}
	si.MergeTimeMS.Store(800)
	si.SortTimeMS.Store(400)
	s := si.String()
	if s == "" || !bytes.Contains([]byte(s), []byte("lines=10")) {
		t.Fatalf("unexpected String: %q", s)
	}
}

// TestTempDirectory_OSImpl exercises CreateTempFile/Open/Create/Remove/List.
func TestTempDirectory_OSImpl(t *testing.T) {
	d := newTestTempDir(t)
	a, err := d.CreateTempFile("a", ".tmp")
	if err != nil {
		t.Fatalf("CreateTempFile: %v", err)
	}
	b, err := d.CreateTempFile("b", ".tmp")
	if err != nil {
		t.Fatalf("CreateTempFile: %v", err)
	}
	if a == b {
		t.Fatalf("expected distinct names: %q", a)
	}
	list := d.List()
	if len(list) != 2 {
		t.Fatalf("List=%v", list)
	}
	if err := d.Remove(a); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if got := d.List(); len(got) != 1 {
		t.Fatalf("List after remove=%v", got)
	}
	// Removing again is not an error.
	if err := d.Remove(a); err != nil {
		t.Fatalf("Remove (missing): %v", err)
	}
}
