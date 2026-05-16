// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hnsw

import (
	"errors"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// graphBackedKnnVectorsReader is the test stub for KnnVectorsReader
// used by the IncrementalHnswGraphMerger tests. It pairs an
// OnHeapHnswGraph with a KnnVectorValues view of the same vector
// count; the (field, graph) lookup ignores field name (every field
// resolves to the same pair). This is intentional: the merger only
// exercises a single field name per instance.
type graphBackedKnnVectorsReader struct {
	graph  HnswGraph
	values KnnVectorValues
	// hnswErr forces HnswGraph to return this error (used to verify
	// the merger surfaces errors instead of swallowing them).
	hnswErr error
	// valuesErr forces GetFloatVectorValues to return this error.
	valuesErr error
}

func (g *graphBackedKnnVectorsReader) HnswGraph(field string) (HnswGraph, error) {
	if g.hnswErr != nil {
		return nil, g.hnswErr
	}
	return g.graph, nil
}

func (g *graphBackedKnnVectorsReader) GetFloatVectorValues(field string) (KnnVectorValues, error) {
	if g.valuesErr != nil {
		return nil, g.valuesErr
	}
	return g.values, nil
}

// identityDocMap returns the identity DocMap used by tests that do
// not exercise inter-segment doc remapping.
type identityDocMap struct{}

func (identityDocMap) Get(docID int) int { return docID }

// fixedBitsDocs is a tiny util.Bits implementation backed by a
// boolean slice. Used by the AddReader tests to filter live docs.
type fixedBitsDocs struct {
	bits []bool
}

func (b *fixedBitsDocs) Get(index int) bool {
	if index < 0 || index >= len(b.bits) {
		return false
	}
	return b.bits[index]
}

func (b *fixedBitsDocs) Length() int { return len(b.bits) }

// TestNewIncrementalHnswGraphMerger_RejectsBadParams verifies the
// constructor surfaces invalid parameters as explicit errors.
func TestNewIncrementalHnswGraphMerger_RejectsBadParams(t *testing.T) {
	sup := newBuilderScorerSupplier([]float32{0.0})

	cases := []struct {
		name      string
		fieldName string
		sup       RandomVectorScorerSupplier
		m         int
		beamWidth int
	}{
		{"empty fieldName", "", sup, 4, 10},
		{"nil supplier", "vec", nil, 4, 10},
		{"zero m", "vec", sup, 0, 10},
		{"negative m", "vec", sup, -1, 10},
		{"zero beamWidth", "vec", sup, 4, 0},
		{"negative beamWidth", "vec", sup, 4, -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewIncrementalHnswGraphMerger(
				tc.fieldName, tc.sup, tc.m, tc.beamWidth,
			); err == nil {
				t.Fatalf("want error, got nil")
			}
		})
	}
}

// TestNewIncrementalHnswGraphMerger_Accessors verifies the
// getter shape and that the carrier fields are stored verbatim.
func TestNewIncrementalHnswGraphMerger_Accessors(t *testing.T) {
	sup := newBuilderScorerSupplier([]float32{0.0})
	im, err := NewIncrementalHnswGraphMerger("vec", sup, 4, 10)
	if err != nil {
		t.Fatalf("NewIncrementalHnswGraphMerger: %v", err)
	}
	if im.FieldName() != "vec" {
		t.Errorf("FieldName=%q, want %q", im.FieldName(), "vec")
	}
	if im.ScorerSupplier() != sup {
		t.Errorf("ScorerSupplier(): pointer mismatch")
	}
	if im.M() != 4 {
		t.Errorf("M=%d, want 4", im.M())
	}
	if im.BeamWidth() != 10 {
		t.Errorf("BeamWidth=%d, want 10", im.BeamWidth())
	}
}

// TestIncrementalHnswGraphMerger_AddReader_NilReader verifies that
// AddReader handles a nil reader gracefully — numReaders is
// incremented but the readers list stays untouched.
func TestIncrementalHnswGraphMerger_AddReader_NilReader(t *testing.T) {
	sup := newBuilderScorerSupplier([]float32{0.0})
	im, err := NewIncrementalHnswGraphMerger("vec", sup, 4, 10)
	if err != nil {
		t.Fatalf("NewIncrementalHnswGraphMerger: %v", err)
	}
	if _, err := im.AddReader(nil, nil, nil); err != nil {
		t.Fatalf("AddReader(nil): %v", err)
	}
	if im.numReaders != 1 {
		t.Errorf("numReaders=%d, want 1", im.numReaders)
	}
	if len(im.graphReaders) != 0 {
		t.Errorf("graphReaders len=%d, want 0", len(im.graphReaders))
	}
	if im.largestGraphReader != nil {
		t.Errorf("largestGraphReader: want nil")
	}
}

// TestIncrementalHnswGraphMerger_AddReader_SkipsEmptyGraph verifies
// that a reader returning a nil or empty graph bumps numReaders but
// does not populate graphReaders or largestGraphReader.
func TestIncrementalHnswGraphMerger_AddReader_SkipsEmptyGraph(t *testing.T) {
	sup := newBuilderScorerSupplier([]float32{0.0})
	im, err := NewIncrementalHnswGraphMerger("vec", sup, 4, 10)
	if err != nil {
		t.Fatalf("NewIncrementalHnswGraphMerger: %v", err)
	}

	// Reader 1: nil graph.
	r1 := &graphBackedKnnVectorsReader{graph: nil}
	if _, err := im.AddReader(r1, identityDocMap{}, nil); err != nil {
		t.Fatalf("AddReader r1: %v", err)
	}
	// Reader 2: empty graph (Empty() returns Size==0).
	r2 := &graphBackedKnnVectorsReader{graph: Empty()}
	if _, err := im.AddReader(r2, identityDocMap{}, nil); err != nil {
		t.Fatalf("AddReader r2: %v", err)
	}
	if im.numReaders != 2 {
		t.Errorf("numReaders=%d, want 2", im.numReaders)
	}
	if len(im.graphReaders) != 0 {
		t.Errorf("graphReaders len=%d, want 0", len(im.graphReaders))
	}
	if im.largestGraphReader != nil {
		t.Errorf("largestGraphReader: want nil")
	}
}

// TestIncrementalHnswGraphMerger_AddReader_ErrorPropagation verifies
// that HnswGraph and GetFloatVectorValues errors surface through
// AddReader rather than being silently swallowed.
func TestIncrementalHnswGraphMerger_AddReader_ErrorPropagation(t *testing.T) {
	sup := newBuilderScorerSupplier([]float32{0.0})

	t.Run("HnswGraph error", func(t *testing.T) {
		im, err := NewIncrementalHnswGraphMerger("vec", sup, 4, 10)
		if err != nil {
			t.Fatalf("NewIncrementalHnswGraphMerger: %v", err)
		}
		reader := &graphBackedKnnVectorsReader{hnswErr: errors.New("boom")}
		_, err = im.AddReader(reader, identityDocMap{}, nil)
		if err == nil {
			t.Fatalf("want error, got nil")
		}
	})

	t.Run("GetFloatVectorValues error", func(t *testing.T) {
		const n = 4
		coords := linspace(n, 0, 3)
		src := buildSourceGraph(t, coords, 4, 10, 1)

		im, err := NewIncrementalHnswGraphMerger("vec", sup, 4, 10)
		if err != nil {
			t.Fatalf("NewIncrementalHnswGraphMerger: %v", err)
		}
		reader := &graphBackedKnnVectorsReader{
			graph:     src,
			valuesErr: errors.New("vector failure"),
		}
		_, err = im.AddReader(reader, identityDocMap{}, nil)
		if err == nil {
			t.Fatalf("want error, got nil")
		}
	})

	t.Run("nil values surfaces error", func(t *testing.T) {
		const n = 4
		coords := linspace(n, 0, 3)
		src := buildSourceGraph(t, coords, 4, 10, 1)

		im, err := NewIncrementalHnswGraphMerger("vec", sup, 4, 10)
		if err != nil {
			t.Fatalf("NewIncrementalHnswGraphMerger: %v", err)
		}
		reader := &graphBackedKnnVectorsReader{graph: src, values: nil}
		_, err = im.AddReader(reader, identityDocMap{}, nil)
		if err == nil {
			t.Fatalf("want error, got nil")
		}
	})
}

// TestIncrementalHnswGraphMerger_AddReader_NoDeletes verifies that a
// reader with zero deletions ends up both in graphReaders and as
// largestGraphReader, and that AddReader returns the receiver for
// chaining.
func TestIncrementalHnswGraphMerger_AddReader_NoDeletes(t *testing.T) {
	const n = 6
	coords := linspace(n, 0, 5)
	sup := newBuilderScorerSupplier(coords)
	src := buildSourceGraph(t, coords, 4, 10, 1)

	im, err := NewIncrementalHnswGraphMerger("vec", sup, 4, 10)
	if err != nil {
		t.Fatalf("NewIncrementalHnswGraphMerger: %v", err)
	}
	reader := &graphBackedKnnVectorsReader{
		graph:  src,
		values: &stubKnnVectorValues{dim: 1, n: n},
	}
	got, err := im.AddReader(reader, identityDocMap{}, nil)
	if err != nil {
		t.Fatalf("AddReader: %v", err)
	}
	if got != im {
		t.Errorf("AddReader: receiver chaining lost (got != im)")
	}
	if im.numReaders != 1 {
		t.Errorf("numReaders=%d, want 1", im.numReaders)
	}
	if len(im.graphReaders) != 1 {
		t.Errorf("graphReaders len=%d, want 1", len(im.graphReaders))
	}
	if im.largestGraphReader == nil {
		t.Errorf("largestGraphReader: want non-nil")
	} else if im.largestGraphReader.graphSize != n {
		t.Errorf("largestGraphReader.graphSize=%d, want %d",
			im.largestGraphReader.graphSize, n)
	}
}

// TestIncrementalHnswGraphMerger_AddReader_SomeDeletes verifies that
// a reader with deletes under the threshold still becomes
// largestGraphReader but does NOT join graphReaders (a graph with
// deletions cannot be folded in via the standard merge path until
// remapping support lands).
func TestIncrementalHnswGraphMerger_AddReader_SomeDeletes(t *testing.T) {
	const n = 10
	coords := linspace(n, 0, 9)
	sup := newBuilderScorerSupplier(coords)
	src := buildSourceGraph(t, coords, 4, 10, 1)

	// Two deletes out of 10 = 20% deletion (below the 40 threshold).
	live := make([]bool, n)
	for i := range live {
		live[i] = true
	}
	live[3] = false
	live[7] = false

	im, err := NewIncrementalHnswGraphMerger("vec", sup, 4, 10)
	if err != nil {
		t.Fatalf("NewIncrementalHnswGraphMerger: %v", err)
	}
	reader := &graphBackedKnnVectorsReader{
		graph:  src,
		values: &stubKnnVectorValues{dim: 1, n: n},
	}
	if _, err := im.AddReader(reader, identityDocMap{}, &fixedBitsDocs{bits: live}); err != nil {
		t.Fatalf("AddReader: %v", err)
	}
	if im.largestGraphReader == nil {
		t.Errorf("largestGraphReader: want non-nil under threshold")
	}
	if len(im.graphReaders) != 0 {
		t.Errorf("graphReaders len=%d, want 0 (deletes block list inclusion)",
			len(im.graphReaders))
	}
}

// TestIncrementalHnswGraphMerger_AddReader_AboveDeleteThreshold
// verifies that a reader with deletion percentage strictly above
// deletePctThreshold ends up neither in graphReaders nor as
// largestGraphReader.
func TestIncrementalHnswGraphMerger_AddReader_AboveDeleteThreshold(t *testing.T) {
	const n = 10
	coords := linspace(n, 0, 9)
	sup := newBuilderScorerSupplier(coords)
	src := buildSourceGraph(t, coords, 4, 10, 1)

	// 5 deletes out of 10 = 50% deletion (above the 40 threshold).
	live := make([]bool, n)
	for i := 0; i < 5; i++ {
		live[i] = true
	}

	im, err := NewIncrementalHnswGraphMerger("vec", sup, 4, 10)
	if err != nil {
		t.Fatalf("NewIncrementalHnswGraphMerger: %v", err)
	}
	reader := &graphBackedKnnVectorsReader{
		graph:  src,
		values: &stubKnnVectorValues{dim: 1, n: n},
	}
	if _, err := im.AddReader(reader, identityDocMap{}, &fixedBitsDocs{bits: live}); err != nil {
		t.Fatalf("AddReader: %v", err)
	}
	if im.largestGraphReader != nil {
		t.Errorf("largestGraphReader: want nil above threshold, got non-nil")
	}
	if len(im.graphReaders) != 0 {
		t.Errorf("graphReaders len=%d, want 0", len(im.graphReaders))
	}
}

// TestIncrementalHnswGraphMerger_AddReader_PicksLargest verifies the
// largest-graph selection logic over multiple readers. Three
// readers are added with sizes 4, 8, and 6 respectively; the
// largest (8) must win.
func TestIncrementalHnswGraphMerger_AddReader_PicksLargest(t *testing.T) {
	coordsBig := linspace(8, 0, 7)
	coordsMid := linspace(6, 0, 5)
	coordsSm := linspace(4, 0, 3)
	allCoords := append(append([]float32{}, coordsBig...), coordsMid...)
	allCoords = append(allCoords, coordsSm...)
	sup := newBuilderScorerSupplier(allCoords)

	srcBig := buildSourceGraph(t, coordsBig, 4, 10, 1)
	srcMid := buildSourceGraph(t, coordsMid, 4, 10, 2)
	srcSm := buildSourceGraph(t, coordsSm, 4, 10, 3)

	im, err := NewIncrementalHnswGraphMerger("vec", sup, 4, 10)
	if err != nil {
		t.Fatalf("NewIncrementalHnswGraphMerger: %v", err)
	}
	rSm := &graphBackedKnnVectorsReader{
		graph: srcSm, values: &stubKnnVectorValues{dim: 1, n: 4},
	}
	rBig := &graphBackedKnnVectorsReader{
		graph: srcBig, values: &stubKnnVectorValues{dim: 1, n: 8},
	}
	rMid := &graphBackedKnnVectorsReader{
		graph: srcMid, values: &stubKnnVectorValues{dim: 1, n: 6},
	}
	for _, r := range []*graphBackedKnnVectorsReader{rSm, rBig, rMid} {
		if _, err := im.AddReader(r, identityDocMap{}, nil); err != nil {
			t.Fatalf("AddReader: %v", err)
		}
	}
	if im.numReaders != 3 {
		t.Errorf("numReaders=%d, want 3", im.numReaders)
	}
	if len(im.graphReaders) != 3 {
		t.Errorf("graphReaders len=%d, want 3", len(im.graphReaders))
	}
	if im.largestGraphReader == nil || im.largestGraphReader.graphSize != 8 {
		var got int
		if im.largestGraphReader != nil {
			got = im.largestGraphReader.graphSize
		}
		t.Errorf("largestGraphReader.graphSize=%d, want 8", got)
	}
}

// TestIncrementalHnswGraphMerger_Merge_NoReaders verifies that
// Merge with no AddReader calls falls through to a plain
// HnswGraphBuilder (no seed). The result is the same graph the
// regular HnswGraphBuilder would produce for the same scorer.
func TestIncrementalHnswGraphMerger_Merge_NoReaders(t *testing.T) {
	const n = 8
	coords := linspace(n, 0, 5)
	sup := newBuilderScorerSupplier(coords)

	im, err := NewIncrementalHnswGraphMerger("vec", sup, 4, 10)
	if err != nil {
		t.Fatalf("NewIncrementalHnswGraphMerger: %v", err)
	}
	merged := &stubKnnVectorValues{dim: 1, n: n}
	dst, err := im.Merge(merged, util.DefaultInfoStream(), n)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if dst == nil {
		t.Fatalf("Merge: nil graph")
	}
	if dst.Size() != n {
		t.Errorf("dst.Size=%d, want %d", dst.Size(), n)
	}
	// Plain Build path must yield a rooted graph.
	rooted, err := IsRooted(dst)
	if err != nil {
		t.Fatalf("IsRooted: %v", err)
	}
	if !rooted {
		t.Errorf("dst not rooted after no-reader merge")
	}
}

// TestIncrementalHnswGraphMerger_Merge_NoEligibleReaders verifies
// that Merge with readers that produced no graph (empty or nil)
// also falls through to the plain builder path.
func TestIncrementalHnswGraphMerger_Merge_NoEligibleReaders(t *testing.T) {
	const n = 6
	coords := linspace(n, 0, 4)
	sup := newBuilderScorerSupplier(coords)

	im, err := NewIncrementalHnswGraphMerger("vec", sup, 4, 10)
	if err != nil {
		t.Fatalf("NewIncrementalHnswGraphMerger: %v", err)
	}
	// Add two readers that resolve to nil/empty graphs.
	for _, g := range []HnswGraph{nil, Empty()} {
		r := &graphBackedKnnVectorsReader{graph: g}
		if _, err := im.AddReader(r, identityDocMap{}, nil); err != nil {
			t.Fatalf("AddReader: %v", err)
		}
	}
	if im.largestGraphReader != nil {
		t.Fatalf("largestGraphReader: want nil, got %+v", im.largestGraphReader)
	}
	merged := &stubKnnVectorValues{dim: 1, n: n}
	dst, err := im.Merge(merged, util.DefaultInfoStream(), n)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if dst == nil {
		t.Fatalf("Merge: nil graph")
	}
	if dst.Size() != n {
		t.Errorf("dst.Size=%d, want %d", dst.Size(), n)
	}
}

// TestIncrementalHnswGraphMerger_Merge_SingleReader_FullCoverage
// verifies that a single zero-delete reader produces a merged
// graph whose level-0 contains every reader ordinal. Mirrors the
// Java "graphReaders.size() == numReaders" path: the
// initializedNodes bitset is omitted and the final sweep is
// skipped, so dst.Size() reflects only the seed reader's ordinals.
func TestIncrementalHnswGraphMerger_Merge_SingleReader_FullCoverage(t *testing.T) {
	const seedN = 6
	coords := linspace(seedN, 0, 5)
	sup := newBuilderScorerSupplier(coords)
	src := buildSourceGraph(t, coords, 4, 10, 1)

	im, err := NewIncrementalHnswGraphMerger("vec", sup, 4, 10)
	if err != nil {
		t.Fatalf("NewIncrementalHnswGraphMerger: %v", err)
	}
	reader := &graphBackedKnnVectorsReader{
		graph:  src,
		values: &stubKnnVectorValues{dim: 1, n: seedN},
	}
	if _, err := im.AddReader(reader, identityDocMap{}, nil); err != nil {
		t.Fatalf("AddReader: %v", err)
	}

	merged := &stubKnnVectorValues{dim: 1, n: seedN}
	dst, err := im.Merge(merged, util.DefaultInfoStream(), seedN)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if dst == nil {
		t.Fatalf("Merge: nil graph")
	}
	if dst.Size() != seedN {
		t.Errorf("dst.Size=%d, want %d", dst.Size(), seedN)
	}
	for node := 0; node < seedN; node++ {
		if !dst.NodeExistAtLevel(0, node) {
			t.Errorf("node %d missing from dst level 0", node)
		}
	}
}

// TestIncrementalHnswGraphMerger_Merge_MultipleReaders verifies the
// happy path with two readers whose ordinal columns cover [0, n)
// and [n, 2n) respectively. Each reader's vector values iterate
// per-segment doc ids (identity); the cross-segment shift is done
// by the per-reader DocMap. The merged view exposes total doc ids
// 0..total-1, of which only n..2n-1 will be claimed by reader B
// and 0..n-1 by reader A.
func TestIncrementalHnswGraphMerger_Merge_MultipleReaders(t *testing.T) {
	const n = 6
	const total = 2 * n
	coords := linspace(total, 0, 9)
	sup := newBuilderScorerSupplier(coords)

	srcA := buildSourceGraph(t, coords[:n], 4, 10, 1)
	srcB := buildSourceGraph(t, coords[n:], 4, 10, 2)

	im, err := NewIncrementalHnswGraphMerger("vec", sup, 4, 10)
	if err != nil {
		t.Fatalf("NewIncrementalHnswGraphMerger: %v", err)
	}

	// Reader A: per-segment doc ids 0..n-1 (identity values iter)
	// shifted by 0 in the merged segment.
	rA := &graphBackedKnnVectorsReader{
		graph:  srcA,
		values: &stubKnnVectorValues{dim: 1, n: n},
	}
	if _, err := im.AddReader(rA, identityDocMap{}, nil); err != nil {
		t.Fatalf("AddReader rA: %v", err)
	}

	// Reader B: per-segment doc ids 0..n-1 shifted by n in the
	// merged segment via shiftingDocMap.
	rB := &graphBackedKnnVectorsReader{
		graph:  srcB,
		values: &stubKnnVectorValues{dim: 1, n: n},
	}
	if _, err := im.AddReader(rB, shiftingDocMap{offset: n}, nil); err != nil {
		t.Fatalf("AddReader rB: %v", err)
	}

	// The merged view exposes every doc id 0..total-1 with ordinal i
	// at doc i (identity mapping).
	merged := &stubKnnVectorValues{dim: 1, n: total}
	dst, err := im.Merge(merged, util.DefaultInfoStream(), total)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if dst == nil {
		t.Fatalf("Merge: nil graph")
	}
	// graphReaders.size() == numReaders (2 == 2) so no final sweep
	// runs; dst.Size() == total because both readers cover the full
	// merged ordinal range.
	if dst.Size() != total {
		t.Errorf("dst.Size=%d, want %d", dst.Size(), total)
	}
	for node := 0; node < total; node++ {
		if !dst.NodeExistAtLevel(0, node) {
			t.Errorf("node %d missing from dst level 0", node)
		}
	}
	rooted, err := IsRooted(dst)
	if err != nil {
		t.Fatalf("IsRooted: %v", err)
	}
	if !rooted {
		t.Errorf("merged graph not rooted")
	}
}

// TestIncrementalHnswGraphMerger_Merge_SeedPickedByLargest verifies
// that when the readers are added in non-decreasing order of size,
// createBuilder reorders graphReaders so the largest reader sits
// at index 0 (seed position) before handing off to
// MergingHnswGraphBuilder. We assert by inspecting graphReaders
// after Merge.
func TestIncrementalHnswGraphMerger_Merge_SeedPickedByLargest(t *testing.T) {
	coordsSm := linspace(4, 0, 3)
	coordsBig := linspace(8, 4, 11)
	allCoords := append([]float32{}, coordsSm...)
	allCoords = append(allCoords, coordsBig...)
	sup := newBuilderScorerSupplier(allCoords)
	srcSm := buildSourceGraph(t, coordsSm, 4, 10, 1)
	srcBig := buildSourceGraph(t, coordsBig, 4, 10, 2)

	im, err := NewIncrementalHnswGraphMerger("vec", sup, 4, 10)
	if err != nil {
		t.Fatalf("NewIncrementalHnswGraphMerger: %v", err)
	}
	// Add the small one first, big one second — the natural order is
	// reverse of what createBuilder wants.
	rSm := &graphBackedKnnVectorsReader{
		graph: srcSm, values: &stubKnnVectorValues{dim: 1, n: 4},
	}
	rBig := &graphBackedKnnVectorsReader{
		graph:  srcBig,
		values: &stubKnnVectorValues{dim: 1, n: 8},
	}
	if _, err := im.AddReader(rSm, identityDocMap{}, nil); err != nil {
		t.Fatalf("AddReader rSm: %v", err)
	}
	if _, err := im.AddReader(rBig, shiftingDocMap{offset: 4}, nil); err != nil {
		t.Fatalf("AddReader rBig: %v", err)
	}

	const total = 12
	merged := &stubKnnVectorValues{dim: 1, n: total}
	if _, err := im.Merge(merged, util.DefaultInfoStream(), total); err != nil {
		t.Fatalf("Merge: %v", err)
	}
	// Post-Merge, graphReaders must be sorted descending by size:
	// big (8) first, small (4) second.
	if len(im.graphReaders) != 2 {
		t.Fatalf("graphReaders len=%d, want 2", len(im.graphReaders))
	}
	if im.graphReaders[0].graphSize != 8 {
		t.Errorf("graphReaders[0].graphSize=%d, want 8 (largest first)",
			im.graphReaders[0].graphSize)
	}
	if im.graphReaders[1].graphSize != 4 {
		t.Errorf("graphReaders[1].graphSize=%d, want 4",
			im.graphReaders[1].graphSize)
	}
}

// TestIncrementalHnswGraphMerger_Merge_LargestWithDeletesIsPrepended
// verifies the "largest with deletes" rescue path of createBuilder.
// The largest graph has 20% deletes — accepted as
// largestGraphReader but excluded from graphReaders. createBuilder
// must prepend it to graphReaders so it still seeds the merge.
func TestIncrementalHnswGraphMerger_Merge_LargestWithDeletesIsPrepended(t *testing.T) {
	const big = 10
	const sm = 4
	const total = 14
	coordsBig := linspace(big, 0, 9)
	coordsSm := linspace(sm, 10, 13)
	coords := append([]float32{}, coordsBig...)
	coords = append(coords, coordsSm...)
	sup := newBuilderScorerSupplier(coords)
	srcBig := buildSourceGraph(t, coordsBig, 4, 10, 1)
	srcSm := buildSourceGraph(t, coordsSm, 4, 10, 2)

	im, err := NewIncrementalHnswGraphMerger("vec", sup, 4, 10)
	if err != nil {
		t.Fatalf("NewIncrementalHnswGraphMerger: %v", err)
	}

	// Big reader has 20% deletes (2 of 10) — under threshold but
	// excluded from graphReaders.
	live := make([]bool, big)
	for i := range live {
		live[i] = true
	}
	live[3] = false
	live[7] = false
	rBig := &graphBackedKnnVectorsReader{
		graph: srcBig, values: &stubKnnVectorValues{dim: 1, n: big},
	}
	if _, err := im.AddReader(rBig, identityDocMap{}, &fixedBitsDocs{bits: live}); err != nil {
		t.Fatalf("AddReader rBig: %v", err)
	}
	// Small reader has zero deletes — goes into graphReaders.
	rSm := &graphBackedKnnVectorsReader{
		graph:  srcSm,
		values: &stubKnnVectorValues{dim: 1, n: sm},
	}
	if _, err := im.AddReader(rSm, shiftingDocMap{offset: big}, nil); err != nil {
		t.Fatalf("AddReader rSm: %v", err)
	}

	// Pre-Merge sanity: largest carries the big reader, but
	// graphReaders only carries the small reader.
	if im.largestGraphReader == nil || im.largestGraphReader.graphSize != big {
		t.Fatalf("pre-Merge largestGraphReader: want size %d", big)
	}
	if len(im.graphReaders) != 1 || im.graphReaders[0].graphSize != sm {
		t.Fatalf("pre-Merge graphReaders: want [small], got %d entries", len(im.graphReaders))
	}

	merged := &stubKnnVectorValues{dim: 1, n: total}
	if _, err := im.Merge(merged, util.DefaultInfoStream(), total); err != nil {
		t.Fatalf("Merge: %v", err)
	}
	// Post-Merge: graphReaders must include both, with the big
	// reader prepended at index 0.
	if len(im.graphReaders) != 2 {
		t.Fatalf("post-Merge graphReaders len=%d, want 2", len(im.graphReaders))
	}
	if im.graphReaders[0].graphSize != big {
		t.Errorf("post-Merge graphReaders[0].graphSize=%d, want %d (prepended)",
			im.graphReaders[0].graphSize, big)
	}
	if im.graphReaders[1].graphSize != sm {
		t.Errorf("post-Merge graphReaders[1].graphSize=%d, want %d",
			im.graphReaders[1].graphSize, sm)
	}
}

// TestIncrementalHnswGraphMerger_HnswGraphMergerInterface confirms
// the type satisfies HnswGraphMerger at the interface level so the
// codec layer can hold the merger through that abstraction.
func TestIncrementalHnswGraphMerger_HnswGraphMergerInterface(t *testing.T) {
	const n = 4
	coords := linspace(n, 0, 3)
	sup := newBuilderScorerSupplier(coords)

	var iface HnswGraphMerger
	im, err := NewIncrementalHnswGraphMerger("vec", sup, 4, 10)
	if err != nil {
		t.Fatalf("NewIncrementalHnswGraphMerger: %v", err)
	}
	iface = im

	got, err := iface.AddReader(nil, nil, nil)
	if err != nil {
		t.Fatalf("AddReader: %v", err)
	}
	if got == nil {
		t.Errorf("AddReader through interface returned nil")
	}

	merged := &stubKnnVectorValues{dim: 1, n: n}
	graph, err := iface.Merge(merged, util.DefaultInfoStream(), n)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if graph == nil {
		t.Errorf("Merge through interface returned nil")
	}
}

// shiftingDocMap is a DocMap that translates per-segment doc ids
// into merged doc ids by adding a fixed offset. Used by the
// multi-reader merge test to simulate two segments whose docs end
// up in disjoint id ranges of the merged segment.
type shiftingDocMap struct {
	offset int
}

func (s shiftingDocMap) Get(docID int) int { return docID + s.offset }
