// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package facets

import (
	"errors"
	"sync/atomic"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// stubLeafReader is a minimal LeafReaderInterface used to drive the
// accumulator resolver-hook tests. It returns a fixed MaxDoc and panics on
// any operation the tests do not exercise.
type stubLeafReader struct {
	index.LeafReaderInterface
	maxDoc int
}

func (s *stubLeafReader) MaxDoc() int   { return s.maxDoc }
func (s *stubLeafReader) NumDocs() int  { return s.maxDoc }
func (s *stubLeafReader) DocCount() int { return s.maxDoc }

// stubBits implements the facets.Bits interface for filtering matches.
type stubBits struct {
	set    map[int]struct{}
	length int
}

func newStubBits(length int, matches ...int) *stubBits {
	b := &stubBits{set: make(map[int]struct{}, len(matches)), length: length}
	for _, m := range matches {
		b.set[m] = struct{}{}
	}
	return b
}

func (b *stubBits) Get(i int) bool {
	_, ok := b.set[i]
	return ok
}
func (b *stubBits) Length() int { return b.length }

// matchingDocsWithReader constructs a MatchingDocs whose GetLeafReader()
// returns the supplied reader. The accumulator only needs the leaf reader, so
// we wrap it in a standalone LeafReaderContext with no parent.
func matchingDocsWithReader(reader index.LeafReaderInterface, bits Bits, total int) *MatchingDocs {
	ctx := index.NewLeafReaderContext(reader, nil, 0, 0)
	return &MatchingDocs{Context: ctx, Bits: bits, TotalHits: total}
}

// --- BaseFacetsAccumulator.matchesPath --------------------------------------

func TestBaseFacetsAccumulatorMatchesPathMultiComponent(t *testing.T) {
	acc := NewBaseFacetsAccumulator()

	cases := []struct {
		name  string
		label string
		path  []string
		want  bool
	}{
		{"empty path matches anything", "a/b/c", nil, true},
		{"exact two component match", "a/b", []string{"a", "b"}, true},
		{"prefix match accepts descendant label", "a/b/c", []string{"a", "b"}, true},
		{"first component differs", "x/b", []string{"a", "b"}, false},
		{"path longer than label", "a", []string{"a", "b"}, false},
		{"empty label only matches empty path", "", nil, true},
		{"empty label rejected by non-empty path", "", []string{"a"}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := acc.matchesPath(tc.label, tc.path)
			if got != tc.want {
				t.Fatalf("matchesPath(%q, %v) = %v, want %v", tc.label, tc.path, got, tc.want)
			}
		})
	}
}

// --- TaxonomyFacetsAccumulator.accumulateFromSegment ------------------------

func TestTaxonomyFacetsAccumulatorResolverDrivesCounts(t *testing.T) {
	reader := NewTaxonomyReader()
	tw := NewTaxonomyWriterWithReader(reader)
	a, _ := tw.AddCategory("dim/a")
	b, _ := tw.AddCategory("dim/b")
	if err := tw.Commit(); err != nil {
		t.Fatalf("commit failed: %v", err)
	}

	acc, err := NewTaxonomyFacetsAccumulator(reader, NewFacetsConfig())
	if err != nil {
		t.Fatalf("new accumulator: %v", err)
	}

	resolved := map[int][]int{
		0: {a},
		1: {a, b},
		2: {b},
	}
	acc.SetOrdinalsResolver(func(_ *MatchingDocs, doc int) ([]int, error) {
		return resolved[doc], nil
	})

	leaf := &stubLeafReader{maxDoc: 3}
	md := matchingDocsWithReader(leaf, nil, 3)

	if err := acc.AccumulateFromMatchingDocs([]*MatchingDocs{md}); err != nil {
		t.Fatalf("accumulate: %v", err)
	}

	if got := acc.GetCount(a); got != 2 {
		t.Errorf("count(a) = %d, want 2", got)
	}
	if got := acc.GetCount(b); got != 2 {
		t.Errorf("count(b) = %d, want 2", got)
	}
}

func TestTaxonomyFacetsAccumulatorRespectsBits(t *testing.T) {
	reader := NewTaxonomyReader()
	tw := NewTaxonomyWriterWithReader(reader)
	a, _ := tw.AddCategory("dim/a")
	_ = tw.Commit()

	acc, _ := NewTaxonomyFacetsAccumulator(reader, NewFacetsConfig())
	acc.SetOrdinalsResolver(func(_ *MatchingDocs, _ int) ([]int, error) {
		return []int{a}, nil
	})

	leaf := &stubLeafReader{maxDoc: 4}
	bits := newStubBits(4, 1, 3)
	md := matchingDocsWithReader(leaf, bits, 2)

	if err := acc.AccumulateFromMatchingDocs([]*MatchingDocs{md}); err != nil {
		t.Fatalf("accumulate: %v", err)
	}

	if got := acc.GetCount(a); got != 2 {
		t.Errorf("count(a) = %d, want 2 (only docs 1 and 3 match bits)", got)
	}
}

func TestTaxonomyFacetsAccumulatorPropagatesResolverError(t *testing.T) {
	reader := NewTaxonomyReader()
	acc, _ := NewTaxonomyFacetsAccumulator(reader, NewFacetsConfig())
	want := errors.New("resolver boom")
	acc.SetOrdinalsResolver(func(_ *MatchingDocs, _ int) ([]int, error) {
		return nil, want
	})

	leaf := &stubLeafReader{maxDoc: 1}
	md := matchingDocsWithReader(leaf, nil, 1)
	err := acc.AccumulateFromMatchingDocs([]*MatchingDocs{md})
	if err == nil || !errors.Is(err, want) {
		t.Fatalf("expected wrapped resolver error, got %v", err)
	}
}

func TestTaxonomyFacetsAccumulatorNoResolverIsNoOp(t *testing.T) {
	reader := NewTaxonomyReader()
	acc, _ := NewTaxonomyFacetsAccumulator(reader, NewFacetsConfig())
	leaf := &stubLeafReader{maxDoc: 5}
	md := matchingDocsWithReader(leaf, nil, 5)
	if err := acc.AccumulateFromMatchingDocs([]*MatchingDocs{md}); err != nil {
		t.Fatalf("accumulate: %v", err)
	}
}

// --- ConcurrentFacetsAccumulator.accumulateFromSegment ----------------------

func TestConcurrentFacetsAccumulatorResolverDrivesCounts(t *testing.T) {
	acc, err := NewConcurrentFacetsAccumulator(NewFacetsConfig())
	if err != nil {
		t.Fatalf("new accumulator: %v", err)
	}

	ord1 := 1
	ord2 := 2
	var calls atomic.Int64
	acc.SetOrdinalsResolver(func(_ *MatchingDocs, doc int) ([]int, error) {
		calls.Add(1)
		if doc%2 == 0 {
			return []int{ord1}, nil
		}
		return []int{ord1, ord2}, nil
	})

	leaf := &stubLeafReader{maxDoc: 4}
	md := matchingDocsWithReader(leaf, nil, 4)

	if err := acc.AccumulateFromMatchingDocs([]*MatchingDocs{md}); err != nil {
		t.Fatalf("accumulate: %v", err)
	}

	if got := calls.Load(); got != 4 {
		t.Errorf("resolver call count = %d, want 4", got)
	}
	if got := acc.GetCount(ord1); got != 4 {
		t.Errorf("count(ord1) = %d, want 4", got)
	}
	if got := acc.GetCount(ord2); got != 2 {
		t.Errorf("count(ord2) = %d, want 2", got)
	}
}

func TestConcurrentFacetsAccumulatorNoResolverIsNoOp(t *testing.T) {
	acc, _ := NewConcurrentFacetsAccumulator(NewFacetsConfig())
	leaf := &stubLeafReader{maxDoc: 5}
	md := matchingDocsWithReader(leaf, nil, 5)
	if err := acc.AccumulateFromMatchingDocs([]*MatchingDocs{md}); err != nil {
		t.Fatalf("accumulate: %v", err)
	}
	if !acc.IsEmpty() {
		t.Error("accumulator should be empty without resolver")
	}
}

// --- RandomSamplingFacetsAccumulator.accumulateFromSampledDoc ---------------

func TestRandomSamplingFacetsAccumulatorResolverDrivesCounts(t *testing.T) {
	acc, err := NewRandomSamplingFacetsAccumulator(NewFacetsConfig(), 1.0)
	if err != nil {
		t.Fatalf("new accumulator: %v", err)
	}
	acc.SetMinSampleSize(1)

	const ord = 7
	acc.SetOrdinalsResolver(func(_ *MatchingDocs, _ int) ([]int, error) {
		return []int{ord}, nil
	})

	leaf := &stubLeafReader{maxDoc: 3}
	md := matchingDocsWithReader(leaf, nil, 3)
	if err := acc.AccumulateFromMatchingDocs([]*MatchingDocs{md}); err != nil {
		t.Fatalf("accumulate: %v", err)
	}

	if got := acc.GetCount(ord); got != 3 {
		t.Errorf("count(ord) = %d, want 3", got)
	}
}

// --- TaxonomyReaderFactory.Open + Manager.MaybeRefresh ----------------------

func TestTaxonomyReaderFactoryLoaderPopulatesReader(t *testing.T) {
	f := NewTaxonomyReaderFactory(nil)
	f.SetLoader(func(_ *index.IndexReader, dst *TaxonomyReader) error {
		dst.ordinals["dim/a"] = 1
		dst.paths[1] = "dim/a"
		dst.nextOrdinal = 2
		return nil
	})

	r, err := f.Open()
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if got := r.GetOrdinal("dim/a"); got != 1 {
		t.Errorf("ordinal(dim/a) = %d, want 1", got)
	}
}

func TestTaxonomyReaderFactoryNoLoaderReturnsEmpty(t *testing.T) {
	f := NewTaxonomyReaderFactory(nil)
	r, err := f.Open()
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if r.GetSize() != 0 {
		t.Errorf("size = %d, want 0", r.GetSize())
	}
}

func TestTaxonomyReaderFactoryLoaderError(t *testing.T) {
	want := errors.New("loader boom")
	f := NewTaxonomyReaderFactory(nil)
	f.SetLoader(func(_ *index.IndexReader, _ *TaxonomyReader) error { return want })

	if _, err := f.Open(); err == nil || !errors.Is(err, want) {
		t.Fatalf("expected wrapped loader error, got %v", err)
	}
}

func TestTaxonomyReaderManagerRefresherInvoked(t *testing.T) {
	r := NewTaxonomyReader()
	m := NewTaxonomyReaderManager(r)
	var calls atomic.Int64
	m.SetRefresher(func(_ *TaxonomyReader) error {
		calls.Add(1)
		return nil
	})
	if err := m.MaybeRefresh(); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("refresher invocations = %d, want 1", got)
	}
}

func TestTaxonomyReaderManagerNoRefresherIsNoOp(t *testing.T) {
	m := NewTaxonomyReaderManager(NewTaxonomyReader())
	if err := m.MaybeRefresh(); err != nil {
		t.Fatalf("refresh: %v", err)
	}
}

// --- TaxonomyWriter.Commit with IndexCommitter ------------------------------

func TestTaxonomyWriterCommitInvokesIndexCommitter(t *testing.T) {
	w := NewTaxonomyWriter()
	if _, err := w.AddCategory("dim/a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	var captured TaxonomyWriterSnapshot
	w.SetIndexCommitter(func(s TaxonomyWriterSnapshot) error {
		captured = s
		return nil
	})

	if err := w.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if captured.Ordinals["dim/a"] == 0 {
		t.Errorf("snapshot did not include dim/a")
	}
	if captured.NextOrdinal < 2 {
		t.Errorf("snapshot nextOrdinal = %d, want >= 2", captured.NextOrdinal)
	}
}

func TestTaxonomyWriterCommitCommitterErrorPropagates(t *testing.T) {
	w := NewTaxonomyWriter()
	want := errors.New("commit boom")
	w.SetIndexCommitter(func(_ TaxonomyWriterSnapshot) error { return want })
	if err := w.Commit(); err == nil || !errors.Is(err, want) {
		t.Fatalf("expected wrapped committer error, got %v", err)
	}
}

func TestTaxonomyWriterCommitNoCommitterIsBackwardCompatible(t *testing.T) {
	w := NewTaxonomyWriter()
	if _, err := w.AddCategory("dim/a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := w.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
}
