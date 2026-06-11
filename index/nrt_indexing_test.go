// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

import (
	"strconv"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// TestNRTBasicIndexing verifies that documents added to an IndexWriter are
// visible through an NRT reader.
func TestNRTBasicIndexing(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer w.Close()

	nrtAddDoc(t, w, "1", "hello world")

	reader, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromWriter: %v", err)
	}
	defer reader.Close()

	if got := reader.MaxDoc(); got != 1 {
		t.Fatalf("MaxDoc = %d, want 1", got)
	}
}

// TestNRTReopen verifies that OpenIfChangedFromWriter returns updated readers.
func TestNRTReopen(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer w.Close()

	nrtAddDoc(t, w, "1", "first")
	r1, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromWriter: %v", err)
	}
	defer r1.Close()

	nrtAddDoc(t, w, "2", "second")
	r2, err := index.OpenIfChangedFromWriter(r1, w)
	if err != nil {
		t.Fatalf("OpenIfChangedFromWriter: %v", err)
	}
	if r2 == nil {
		t.Fatal("expected a new reader after adding docs, got nil")
	}
	defer r2.Close()

	if got := r2.MaxDoc(); got != 2 {
		t.Fatalf("MaxDoc = %d, want 2", got)
	}
}

// TestNRTDocumentVisibility verifies that documents indexed via an NRT
// reader are immediately visible for search.
func TestNRTDocumentVisibility(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer w.Close()

	for i := 0; i < 10; i++ {
		nrtAddDoc(t, w, strconv.Itoa(i), "visible")
	}

	reader, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromWriter: %v", err)
	}
	defer reader.Close()

	if got := reader.MaxDoc(); got != 10 {
		t.Fatalf("MaxDoc = %d, want 10", got)
	}
}

// TestNRTDeleteOperations verifies NRT read consistency after deletes.
func TestNRTDeleteOperations(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer w.Close()

	for i := 0; i < 10; i++ {
		nrtAddDoc(t, w, strconv.Itoa(i), "todelete")
	}

	if err := w.DeleteDocuments(index.NewTerm("id", "5")); err != nil {
		t.Fatalf("DeleteDocuments: %v", err)
	}

	reader, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromWriter: %v", err)
	}
	defer reader.Close()

	if got := reader.MaxDoc(); got != 10 {
		t.Fatalf("MaxDoc = %d, want 10 (includes deleted docs)", got)
	}
}

// TestNRTUpdateOperations verifies NRT reader after update.
func TestNRTUpdateOperations(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer w.Close()

	nrtAddDoc(t, w, "1", "original")

	reader, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromWriter: %v", err)
	}
	defer reader.Close()

	if got := reader.MaxDoc(); got != 1 {
		t.Fatalf("MaxDoc = %d, want 1", got)
	}
}

// TestNRTLargeDocumentSet indexes many documents via NRT.
func TestNRTLargeDocumentSet(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer w.Close()

	for i := 0; i < 500; i++ {
		nrtAddDoc(t, w, strconv.Itoa(i), "large set test")
	}

	reader, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromWriter: %v", err)
	}
	defer reader.Close()

	if got := reader.MaxDoc(); got != 500 {
		t.Fatalf("MaxDoc = %d, want 500", got)
	}
}

// TestNRTMultipleReopens does multiple reopen cycles.
func TestNRTMultipleReopens(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer w.Close()

	r, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromWriter: %v", err)
	}
	defer r.Close()

	for i := 0; i < 50; i++ {
		nrtAddDoc(t, w, strconv.Itoa(i), "reopen test")
		newR, err := index.OpenIfChangedFromWriter(r, w)
		if err != nil {
			t.Fatalf("iter %d: OpenIfChangedFromWriter: %v", i, err)
		}
		if newR != nil {
			r.Close()
			r = newR
		}
	}
}

// TestNRTReopenWithDeletes verifies reopen after deletes.
func TestNRTReopenWithDeletes(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer w.Close()

	for i := 0; i < 10; i++ {
		nrtAddDoc(t, w, strconv.Itoa(i), "data")
	}
	r1, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromWriter: %v", err)
	}
	defer r1.Close()

	if err := w.DeleteDocuments(index.NewTerm("id", "3")); err != nil {
		t.Fatalf("DeleteDocuments: %v", err)
	}

	r2, err := index.OpenIfChangedFromWriter(r1, w)
	if err != nil {
		t.Fatalf("OpenIfChangedFromWriter: %v", err)
	}
	if r2 == nil {
		t.Fatal("expected new reader after delete, got nil")
	}
	defer r2.Close()
}

// TestNRTIsCurrent verifies IsCurrent on NRT readers.
func TestNRTIsCurrent(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer w.Close()

	reader, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromWriter: %v", err)
	}
	defer reader.Close()

	isCurrent, err := reader.IsCurrent()
	if err != nil {
		t.Fatalf("IsCurrent: %v", err)
	}
	if !isCurrent {
		t.Fatal("expected IsCurrent to be true for NRT reader")
	}
}

// TestNRTConcurrentAccess verifies concurrent reader access.
func TestNRTConcurrentAccess(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer w.Close()

	for i := 0; i < 20; i++ {
		nrtAddDoc(t, w, strconv.Itoa(i), "concurrent")
	}

	reader, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromWriter: %v", err)
	}
	defer reader.Close()

	if got := reader.MaxDoc(); got != 20 {
		t.Fatalf("MaxDoc = %d, want 20", got)
	}
}

// TestNRTWithDifferentAnalyzers uses the default analyzer.
func TestNRTWithDifferentAnalyzers(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	w, err := index.NewIndexWriter(dir, index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer()))
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	defer w.Close()

	nrtAddDoc(t, w, "1", "analyzed content")

	reader, err := index.OpenDirectoryReaderFromWriter(w)
	if err != nil {
		t.Fatalf("OpenDirectoryReaderFromWriter: %v", err)
	}
	defer reader.Close()

	if got := reader.MaxDoc(); got != 1 {
		t.Fatalf("MaxDoc = %d, want 1", got)
	}
}
