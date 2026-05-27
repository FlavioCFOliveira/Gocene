// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package testutil_test

import (
	"reflect"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/index/testutil"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// emptyDoc is a no-field [index.Document] used by the tests. Field
// content does not matter for the call-sequence determinism check;
// only the call stream against the IndexWriter does.
type emptyDoc struct{}

func (emptyDoc) GetFields() []interface{} { return nil }

func newRIW(t *testing.T, seed int64, cfg testutil.Config) (*testutil.RandomIndexWriter, store.Directory) {
	t.Helper()
	dir := store.NewByteBuffersDirectory()
	iwCfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := index.NewIndexWriter(dir, iwCfg)
	if err != nil {
		t.Fatalf("NewIndexWriter: %v", err)
	}
	return testutil.NewWithConfig(w, seed, cfg), dir
}

// TestRandomIndexWriter_SeededDeterminism is the core AC #3 check:
// two RandomIndexWriter instances driven through the same input
// sequence with the same seed must produce identical call logs.
func TestRandomIndexWriter_SeededDeterminism(t *testing.T) {
	t.Parallel()

	cfg := testutil.Config{
		CommitProbability:          0.5,
		ForceMergeProbability:      0.1,
		MaxNumSegmentsOnForceMerge: 1,
	}

	run := func() []string {
		w, _ := newRIW(t, 12345, cfg)
		defer w.Close()
		for i := 0; i < 25; i++ {
			if err := w.AddDocument(emptyDoc{}); err != nil {
				t.Fatalf("AddDocument: %v", err)
			}
		}
		return w.CallLog()
	}

	a := run()
	b := run()
	if !reflect.DeepEqual(a, b) {
		t.Errorf("seeded determinism: call logs differ\n a=%v\n b=%v", a, b)
	}
	// Sanity: the dice-driven probabilities should have triggered
	// at least one random Commit at p=0.5 over 25 rolls.
	foundCommit := false
	for _, c := range a {
		if c == "Commit(random)" {
			foundCommit = true
			break
		}
	}
	if !foundCommit {
		t.Errorf("expected at least one random Commit at p=0.5 over 25 rolls; log=%v", a)
	}
}

// TestRandomIndexWriter_DifferentSeedsDiverge verifies that
// different seeds produce different call sequences (when the dice
// have any room to roll).
func TestRandomIndexWriter_DifferentSeedsDiverge(t *testing.T) {
	t.Parallel()

	cfg := testutil.Config{
		CommitProbability:          0.5,
		ForceMergeProbability:      0.3,
		MaxNumSegmentsOnForceMerge: 1,
	}

	run := func(seed int64) []string {
		w, _ := newRIW(t, seed, cfg)
		defer w.Close()
		for i := 0; i < 30; i++ {
			if err := w.AddDocument(emptyDoc{}); err != nil {
				t.Fatalf("AddDocument: %v", err)
			}
		}
		return w.CallLog()
	}

	a := run(1)
	b := run(2)
	if reflect.DeepEqual(a, b) {
		t.Errorf("different seeds produced identical call logs; seed coverage broken")
	}
}

// TestRandomIndexWriter_CommitSemantics verifies that an explicit
// Commit is forwarded verbatim (recorded as "Commit" without the
// "(random)" suffix) and that the underlying writer observes it.
func TestRandomIndexWriter_CommitSemantics(t *testing.T) {
	t.Parallel()

	// Disable random interleaving so we can isolate the explicit
	// Commit semantics.
	cfg := testutil.Config{
		CommitProbability:          0.0001, // effectively off
		ForceMergeProbability:      0.0001,
		MaxNumSegmentsOnForceMerge: 1,
	}
	w, dir := newRIW(t, 7, cfg)
	defer dir.Close()
	defer w.Close()

	for i := 0; i < 3; i++ {
		if err := w.AddDocument(emptyDoc{}); err != nil {
			t.Fatalf("AddDocument %d: %v", i, err)
		}
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Verify the call log contains the explicit Commit, not the
	// random-flavoured one.
	log := w.CallLog()
	foundExplicit := false
	for _, c := range log {
		if c == "Commit" {
			foundExplicit = true
		}
	}
	if !foundExplicit {
		t.Errorf("expected explicit Commit in call log, got %v", log)
	}
}

// TestRandomIndexWriter_ForceMergeToSingleSegment verifies that
// ForceMerge(1) collapses the index to a single segment.
func TestRandomIndexWriter_ForceMergeToSingleSegment(t *testing.T) {
	t.Parallel()

	cfg := testutil.Config{
		CommitProbability:          0.0001,
		ForceMergeProbability:      0.0001,
		MaxNumSegmentsOnForceMerge: 1,
	}
	w, dir := newRIW(t, 99, cfg)
	defer dir.Close()
	defer w.Close()

	// Add docs and commit between batches to force multiple segments.
	for batch := 0; batch < 3; batch++ {
		for i := 0; i < 4; i++ {
			if err := w.AddDocument(emptyDoc{}); err != nil {
				t.Fatalf("AddDocument: %v", err)
			}
		}
		if err := w.Commit(); err != nil {
			t.Fatalf("Commit: %v", err)
		}
	}
	preSegments := w.Writer().GetSegmentCount()

	if err := w.ForceMerge(1); err != nil {
		t.Fatalf("ForceMerge(1): %v", err)
	}
	postSegments := w.Writer().GetSegmentCount()

	if postSegments > 1 {
		t.Errorf("ForceMerge(1): segment count post=%d (pre=%d), want <= 1", postSegments, preSegments)
	}
}

// TestRandomIndexWriter_CloseIdempotent verifies that calling Close
// twice does not panic and returns nil on the second call.
func TestRandomIndexWriter_CloseIdempotent(t *testing.T) {
	t.Parallel()

	w, dir := newRIW(t, 1, testutil.DefaultConfig())
	defer dir.Close()

	if err := w.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Errorf("second Close: got %v, want nil", err)
	}
	if !w.IsClosed() {
		t.Errorf("IsClosed: got false, want true")
	}
}

// TestRandomIndexWriter_OpsAfterCloseFail verifies that mutating
// operations after Close return an error rather than panicking.
func TestRandomIndexWriter_OpsAfterCloseFail(t *testing.T) {
	t.Parallel()

	w, dir := newRIW(t, 1, testutil.DefaultConfig())
	defer dir.Close()

	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := w.AddDocument(emptyDoc{}); err == nil {
		t.Errorf("AddDocument after Close: expected error, got nil")
	}
	if err := w.Commit(); err == nil {
		t.Errorf("Commit after Close: expected error, got nil")
	}
	if err := w.ForceMerge(1); err == nil {
		t.Errorf("ForceMerge after Close: expected error, got nil")
	}
}

// TestRandomIndexWriter_DefaultConfig verifies that DefaultConfig
// returns the documented Lucene-faithful defaults.
func TestRandomIndexWriter_DefaultConfig(t *testing.T) {
	t.Parallel()

	c := testutil.DefaultConfig()
	if c.CommitProbability < 0.05 || c.CommitProbability > 0.07 {
		t.Errorf("CommitProbability default: got %g, want ~0.06", c.CommitProbability)
	}
	if c.ForceMergeProbability < 0.005 || c.ForceMergeProbability > 0.015 {
		t.Errorf("ForceMergeProbability default: got %g, want ~0.01", c.ForceMergeProbability)
	}
	if c.MaxNumSegmentsOnForceMerge != 1 {
		t.Errorf("MaxNumSegmentsOnForceMerge default: got %d, want 1", c.MaxNumSegmentsOnForceMerge)
	}
}

// TestRandomIndexWriter_OpenConvenience verifies the Open
// constructor builds and wraps an IndexWriter from a Directory +
// IndexWriterConfig.
func TestRandomIndexWriter_OpenConvenience(t *testing.T) {
	t.Parallel()

	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	cfg := index.NewIndexWriterConfig(analysis.NewWhitespaceAnalyzer())
	w, err := testutil.Open(dir, cfg, 42)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := w.AddDocument(emptyDoc{}); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}
