// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestStopwordAnalyzerBase_Empty verifies that the no-stopwords constructor
// yields an empty set, never nil.
func TestStopwordAnalyzerBase_Empty(t *testing.T) {
	base := NewEmptyStopwordAnalyzerBase()
	if base.Stopwords() == nil {
		t.Fatal("Stopwords() returned nil for empty constructor")
	}
	if !base.Stopwords().IsEmpty() {
		t.Fatalf("expected empty stop-word set, got size=%d", base.Stopwords().Size())
	}
	// GetStopwordSet must be the same view.
	if base.GetStopwordSet() != base.Stopwords() {
		t.Error("GetStopwordSet() and Stopwords() should return the same instance")
	}
}

// TestStopwordAnalyzerBase_DefensiveCopy verifies that the constructor takes
// a defensive copy of the input set so later mutations are invisible.
func TestStopwordAnalyzerBase_DefensiveCopy(t *testing.T) {
	src := NewCharArraySetFromStrings(true, "the", "a", "an")
	base := NewStopwordAnalyzerBase(src)

	if !base.Stopwords().ContainsString("the") {
		t.Fatal("expected 'the' in stop-word set")
	}

	// Mutating the original must NOT affect the analyzer's set.
	src.Add("foo")
	if base.Stopwords().ContainsString("foo") {
		t.Error("StopwordAnalyzerBase did not defensively copy the input set")
	}
}

// TestStopwordAnalyzerBase_Unmodifiable verifies that the returned set
// panics on mutation, matching Lucene's CharArraySet.unmodifiableSet wrapper.
func TestStopwordAnalyzerBase_Unmodifiable(t *testing.T) {
	src := NewCharArraySetFromStrings(true, "the")
	base := NewStopwordAnalyzerBase(src)

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when mutating an unmodifiable stop-word set")
		}
	}()
	base.Stopwords().Add("new")
}

// TestLoadStopwordSetFromReader verifies the Reader-based loader strips
// blank lines and keeps one word per line.
func TestLoadStopwordSetFromReader(t *testing.T) {
	data := "the\na\n\nan\n  in  \n"
	set, err := LoadStopwordSetFromReader(strings.NewReader(data))
	if err != nil {
		t.Fatalf("LoadStopwordSetFromReader: %v", err)
	}
	want := []string{"the", "a", "an", "in"}
	for _, w := range want {
		if !set.ContainsString(w) {
			t.Errorf("expected %q in loaded stop-word set", w)
		}
	}
	if set.Size() != len(want) {
		t.Errorf("expected %d entries, got %d", len(want), set.Size())
	}
}

// TestLoadStopwordSetFromPath verifies the path-based loader.
func TestLoadStopwordSetFromPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stop.txt")
	if err := os.WriteFile(path, []byte("the\na\nan\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	set, err := LoadStopwordSetFromPath(path)
	if err != nil {
		t.Fatalf("LoadStopwordSetFromPath: %v", err)
	}
	if set.Size() != 3 || !set.ContainsString("the") || !set.ContainsString("a") || !set.ContainsString("an") {
		t.Errorf("unexpected set contents, size=%d items=%v", set.Size(), set.Items())
	}
}
