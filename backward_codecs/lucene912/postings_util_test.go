// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene912

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// openDir opens a temp SimpleFSDirectory.
func openDir(t *testing.T) *store.SimpleFSDirectory {
	t.Helper()
	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	return dir
}

// TestPostingsUtil_IntegerOverflow replicates the Java TestPostingsUtil test
// that checks for the integer overflow bug described in
// https://github.com/apache/lucene/issues/13373.
func TestPostingsUtil_IntegerOverflow(t *testing.T) {
	const delta = int64(1 << 30)

	sizes := []int{1, 2, BlockSize / 2, BlockSize}
	for _, size := range sizes {
		t.Run("", func(t *testing.T) {
			docDelta := make([]int64, size)
			freq := make([]int64, size)
			docDelta[0] = delta

			dir := openDir(t)
			defer dir.Close()
			ctx := store.IOContext{Context: store.ContextRead}

			out, err := dir.CreateOutput("test.dat", ctx)
			if err != nil {
				t.Fatalf("CreateOutput: %v", err)
			}
			if err := WriteVIntBlock(out, docDelta, freq, size, true); err != nil {
				t.Fatalf("WriteVIntBlock: %v", err)
			}
			if err := out.Close(); err != nil {
				t.Fatalf("Close output: %v", err)
			}

			docDelta2 := make([]int64, size)
			freq2 := make([]int64, size)
			in, err := dir.OpenInput("test.dat", ctx)
			if err != nil {
				t.Fatalf("OpenInput: %v", err)
			}
			defer in.Close()

			if err := ReadVIntBlock(in, docDelta2, freq2, size, true, true); err != nil {
				t.Fatalf("ReadVIntBlock: %v", err)
			}
			if docDelta2[0] != delta {
				t.Errorf("docDelta[0]: got %d, want %d", docDelta2[0], delta)
			}
		})
	}
}

// TestPostingsUtil_RoundTrip_WithoutFreqs verifies that documents can be
// written and read back correctly when there are no frequencies.
func TestPostingsUtil_RoundTrip_WithoutFreqs(t *testing.T) {
	dir := openDir(t)
	defer dir.Close()
	ctx := store.IOContext{Context: store.ContextRead}

	const n = 10
	docs := make([]int64, n)
	for i := range docs {
		docs[i] = int64(i * 7)
	}
	empty := make([]int64, n)

	out, err := dir.CreateOutput("nofreq.dat", ctx)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	if err := WriteVIntBlock(out, docs, empty, n, false); err != nil {
		t.Fatalf("WriteVIntBlock: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	in, err := dir.OpenInput("nofreq.dat", ctx)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	defer in.Close()

	got := make([]int64, n)
	gotFreq := make([]int64, n)
	// indexHasFreq=false: raw doc delta reads without freq decoding.
	if err := ReadVIntBlock(in, got, gotFreq, n, false, false); err != nil {
		t.Fatalf("ReadVIntBlock: %v", err)
	}
	for i, want := range docs {
		if got[i] != want {
			t.Errorf("[%d] got %d, want %d", i, got[i], want)
		}
	}
}

// TestPostingsUtil_RoundTrip_WithFreqs verifies doc+freq encoding round-trip.
func TestPostingsUtil_RoundTrip_WithFreqs(t *testing.T) {
	dir := openDir(t)
	defer dir.Close()
	ctx := store.IOContext{Context: store.ContextRead}

	const n = 15
	docs := make([]int64, n)
	freqs := make([]int64, n)
	for i := range docs {
		docs[i] = int64(i * 3)
		if i%2 == 0 {
			freqs[i] = 1
		} else {
			freqs[i] = int64(i + 2)
		}
	}

	// WriteVIntBlock modifies docBuffer in-place; work on a copy.
	docsCopy := make([]int64, n)
	copy(docsCopy, docs)

	out, err := dir.CreateOutput("withfreq.dat", ctx)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	if err := WriteVIntBlock(out, docsCopy, freqs, n, true); err != nil {
		t.Fatalf("WriteVIntBlock: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	in, err := dir.OpenInput("withfreq.dat", ctx)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	defer in.Close()

	gotDoc := make([]int64, n)
	gotFreq := make([]int64, n)
	if err := ReadVIntBlock(in, gotDoc, gotFreq, n, true, true); err != nil {
		t.Fatalf("ReadVIntBlock: %v", err)
	}
	for i := range docs {
		if gotDoc[i] != docs[i] {
			t.Errorf("[%d] doc: got %d, want %d", i, gotDoc[i], docs[i])
		}
		if gotFreq[i] != freqs[i] {
			t.Errorf("[%d] freq: got %d, want %d", i, gotFreq[i], freqs[i])
		}
	}
}

// TestPostingsUtil_IndexHasFreqButNotDecoded verifies that docBuffer is
// right-shifted when indexHasFreq=true but decodeFreq=false.
func TestPostingsUtil_IndexHasFreqButNotDecoded(t *testing.T) {
	dir := openDir(t)
	defer dir.Close()
	ctx := store.IOContext{Context: store.ContextRead}

	const n = 4
	docs := []int64{10, 20, 30, 40}
	freqs := []int64{1, 1, 1, 1}
	docsCopy := make([]int64, n)
	copy(docsCopy, docs)

	out, err := dir.CreateOutput("hasfq.dat", ctx)
	if err != nil {
		t.Fatalf("CreateOutput: %v", err)
	}
	if err := WriteVIntBlock(out, docsCopy, freqs, n, true); err != nil {
		t.Fatalf("WriteVIntBlock: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	in, err := dir.OpenInput("hasfq.dat", ctx)
	if err != nil {
		t.Fatalf("OpenInput: %v", err)
	}
	defer in.Close()

	gotDoc := make([]int64, n)
	gotFreq := make([]int64, n)
	// decodeFreq=false: freq bit is stripped from doc delta but not decoded.
	if err := ReadVIntBlock(in, gotDoc, gotFreq, n, true, false); err != nil {
		t.Fatalf("ReadVIntBlock: %v", err)
	}
	for i, want := range docs {
		if gotDoc[i] != want {
			t.Errorf("[%d] doc: got %d, want %d", i, gotDoc[i], want)
		}
	}
}
