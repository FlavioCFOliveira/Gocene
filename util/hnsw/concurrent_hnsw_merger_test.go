// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hnsw

import (
	"runtime"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// TestNewConcurrentHnswMerger_RejectsBadParams verifies the constructor
// surfaces invalid parameters as explicit errors. The empty fieldName,
// nil supplier, and non-positive m / beamWidth cases are inherited from
// the embedded [NewIncrementalHnswGraphMerger]; the numWorker check
// is unique to the concurrent variant.
func TestNewConcurrentHnswMerger_RejectsBadParams(t *testing.T) {
	sup := newBuilderScorerSupplier([]float32{0.0})

	cases := []struct {
		name      string
		fieldName string
		sup       RandomVectorScorerSupplier
		m         int
		beamWidth int
		numWorker int
	}{
		{"empty fieldName", "", sup, 4, 10, 2},
		{"nil supplier", "vec", nil, 4, 10, 2},
		{"zero m", "vec", sup, 0, 10, 2},
		{"negative m", "vec", sup, -1, 10, 2},
		{"zero beamWidth", "vec", sup, 4, 0, 2},
		{"negative beamWidth", "vec", sup, 4, -1, 2},
		{"zero numWorker", "vec", sup, 4, 10, 0},
		{"negative numWorker", "vec", sup, 4, 10, -1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewConcurrentHnswMerger(
				tc.fieldName, tc.sup, tc.m, tc.beamWidth, tc.numWorker,
			); err == nil {
				t.Fatalf("want error, got nil")
			}
		})
	}
}

// TestNewConcurrentHnswMerger_Accessors verifies the constructor wires
// every parameter into the right slot and that the accessors return
// what was passed in. Inherited accessors (FieldName, ScorerSupplier,
// M, BeamWidth) come from the embedded IncrementalHnswGraphMerger.
func TestNewConcurrentHnswMerger_Accessors(t *testing.T) {
	sup := newBuilderScorerSupplier([]float32{0.0})
	cm, err := NewConcurrentHnswMerger("vec", sup, 4, 10, 3)
	if err != nil {
		t.Fatalf("NewConcurrentHnswMerger: %v", err)
	}
	if cm.FieldName() != "vec" {
		t.Errorf("FieldName=%q, want %q", cm.FieldName(), "vec")
	}
	if cm.ScorerSupplier() != sup {
		t.Errorf("ScorerSupplier: pointer mismatch")
	}
	if cm.M() != 4 {
		t.Errorf("M=%d, want 4", cm.M())
	}
	if cm.BeamWidth() != 10 {
		t.Errorf("BeamWidth=%d, want 10", cm.BeamWidth())
	}
	if cm.NumWorker() != 3 {
		t.Errorf("NumWorker=%d, want 3", cm.NumWorker())
	}
}

// TestConcurrentHnswMerger_AddReader_ReturnsOuter verifies the shadow:
// the outer pointer must be returned by AddReader so the chain keeps
// dispatching to the concurrent Merge. Without the shadow, the
// inherited AddReader returns *IncrementalHnswGraphMerger and a
// subsequent Merge call would silently run the single-threaded parent.
func TestConcurrentHnswMerger_AddReader_ReturnsOuter(t *testing.T) {
	sup := newBuilderScorerSupplier([]float32{0.0})
	cm, err := NewConcurrentHnswMerger("vec", sup, 4, 10, 2)
	if err != nil {
		t.Fatalf("NewConcurrentHnswMerger: %v", err)
	}
	got, err := cm.AddReader(nil, nil, nil)
	if err != nil {
		t.Fatalf("AddReader(nil): %v", err)
	}
	// Type assertion is the proof that the shadow fires: the
	// inherited method would return *IncrementalHnswGraphMerger, which
	// would not satisfy the *ConcurrentHnswMerger type assertion.
	if _, ok := got.(*ConcurrentHnswMerger); !ok {
		t.Fatalf("AddReader returned %T, want *ConcurrentHnswMerger", got)
	}
	// State inherited from the embedded parent must still update.
	if cm.numReaders != 1 {
		t.Errorf("numReaders=%d, want 1", cm.numReaders)
	}
}

// TestConcurrentHnswMerger_Merge_NoReaders verifies Merge with no
// AddReader calls falls through to a fresh OnHeapHnswGraph plus a
// concurrent build. The fresh-graph path mirrors the Java
// `largestGraphReader == null` branch and is the simplest end-to-end
// smoke test.
func TestConcurrentHnswMerger_Merge_NoReaders(t *testing.T) {
	const n = 8
	coords := linspace(n, 0, 5)
	sup := newBuilderScorerSupplier(coords)

	cm, err := NewConcurrentHnswMerger("vec", sup, 4, 10, 2)
	if err != nil {
		t.Fatalf("NewConcurrentHnswMerger: %v", err)
	}
	merged := &stubKnnVectorValues{dim: 1, n: n}
	dst, err := cm.Merge(merged, util.DefaultInfoStream(), n)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if dst == nil {
		t.Fatalf("Merge: nil graph")
	}
	if dst.Size() != n {
		t.Errorf("dst.Size=%d, want %d", dst.Size(), n)
	}
	rooted, err := IsRooted(dst)
	if err != nil {
		t.Fatalf("IsRooted: %v", err)
	}
	if !rooted {
		t.Errorf("dst not rooted after no-reader concurrent merge")
	}
}

// TestConcurrentHnswMerger_Merge_NoEligibleReaders verifies Merge with
// readers that produced no usable graph (nil or empty) also falls
// through to the fresh-graph path.
func TestConcurrentHnswMerger_Merge_NoEligibleReaders(t *testing.T) {
	const n = 6
	coords := linspace(n, 0, 4)
	sup := newBuilderScorerSupplier(coords)

	cm, err := NewConcurrentHnswMerger("vec", sup, 4, 10, 2)
	if err != nil {
		t.Fatalf("NewConcurrentHnswMerger: %v", err)
	}
	for _, g := range []HnswGraph{nil, Empty()} {
		r := &graphBackedKnnVectorsReader{graph: g}
		if _, err := cm.AddReader(r, identityDocMap{}, nil); err != nil {
			t.Fatalf("AddReader: %v", err)
		}
	}
	if cm.largestGraphReader != nil {
		t.Fatalf("largestGraphReader: want nil, got %+v", cm.largestGraphReader)
	}
	merged := &stubKnnVectorValues{dim: 1, n: n}
	dst, err := cm.Merge(merged, util.DefaultInfoStream(), n)
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

// TestConcurrentHnswMerger_Merge_SingleReader exercises the seeded
// concurrent build path: one zero-delete reader becomes
// largestGraphReader, the seed graph is built via InitGraph, and the
// concurrent build folds the rest in. The merged graph must contain
// every ordinal at level 0 and remain structurally valid.
func TestConcurrentHnswMerger_Merge_SingleReader(t *testing.T) {
	const seedN = 8
	coords := linspace(seedN, 0, 5)
	sup := newBuilderScorerSupplier(coords)
	src := buildSourceGraph(t, coords, 4, 10, 1)

	cm, err := NewConcurrentHnswMerger("vec", sup, 4, 10, 2)
	if err != nil {
		t.Fatalf("NewConcurrentHnswMerger: %v", err)
	}
	reader := &graphBackedKnnVectorsReader{
		graph:  src,
		values: &stubKnnVectorValues{dim: 1, n: seedN},
	}
	if _, err := cm.AddReader(reader, identityDocMap{}, nil); err != nil {
		t.Fatalf("AddReader: %v", err)
	}

	merged := &stubKnnVectorValues{dim: 1, n: seedN}
	dst, err := cm.Merge(merged, util.DefaultInfoStream(), seedN)
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

// TestConcurrentHnswMerger_Merge_MultipleReaders is the canonical
// happy-path test: two readers, each contributing half of the merged
// ordinal range, merged concurrently with two workers.
//
// The largest reader (here, both are the same size — Reader A is
// admitted first as largest by AddReader's strict-greater-than
// comparison) seeds the destination graph; the remaining ordinals are
// inserted in parallel by the concurrent builder. The output is
// asserted structurally because the concurrent build is not
// deterministic across runs (worker scheduling controls which ordinals
// each worker claims).
func TestConcurrentHnswMerger_Merge_MultipleReaders(t *testing.T) {
	if runtime.NumCPU() < 2 {
		t.Fatal("requires GOMAXPROCS >= 2 for meaningful concurrency")
	}
	const n = 8
	const total = 2 * n
	coords := linspace(total, 0, 11)
	sup := newBuilderScorerSupplier(coords)

	srcA := buildSourceGraph(t, coords[:n], 4, 10, 1)
	srcB := buildSourceGraph(t, coords[n:], 4, 10, 2)

	cm, err := NewConcurrentHnswMerger("vec", sup, 4, 10, 2)
	if err != nil {
		t.Fatalf("NewConcurrentHnswMerger: %v", err)
	}

	rA := &graphBackedKnnVectorsReader{
		graph:  srcA,
		values: &stubKnnVectorValues{dim: 1, n: n},
	}
	if _, err := cm.AddReader(rA, identityDocMap{}, nil); err != nil {
		t.Fatalf("AddReader rA: %v", err)
	}

	rB := &graphBackedKnnVectorsReader{
		graph:  srcB,
		values: &stubKnnVectorValues{dim: 1, n: n},
	}
	if _, err := cm.AddReader(rB, shiftingDocMap{offset: n}, nil); err != nil {
		t.Fatalf("AddReader rB: %v", err)
	}

	merged := &stubKnnVectorValues{dim: 1, n: total}
	dst, err := cm.Merge(merged, util.DefaultInfoStream(), total)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if dst == nil {
		t.Fatalf("Merge: nil graph")
	}
	if got := dst.MaxNodeID(); got != total-1 {
		t.Errorf("MaxNodeID=%d, want %d", got, total-1)
	}
	for node := 0; node < total; node++ {
		if !dst.NodeExistAtLevel(0, node) {
			t.Errorf("node %d missing from dst level 0", node)
		}
		nbrs := dst.GetNeighbors(0, node)
		for i := 0; i < nbrs.Size(); i++ {
			nbr := nbrs.Nodes()[i]
			if nbr == node {
				t.Errorf("self-loop on ordinal %d at level 0", node)
			}
			if nbr < 0 || nbr >= total {
				t.Errorf("ordinal %d neighbour %d out of range [0,%d)", node, nbr, total)
			}
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

// TestConcurrentHnswMerger_Merge_FourWorkers exercises the concurrent
// build with a larger graph and four workers, validating end-to-end
// behaviour under genuine parallelism. The graph is sized so every
// worker claims at least one batch — the test does not assert
// determinism (concurrent builds are not byte-for-byte reproducible
// across runs, see the HnswConcurrentMergeBuilder docs) and instead
// verifies structural correctness: every ordinal present at level 0,
// no self-loops, every neighbour in range.
func TestConcurrentHnswMerger_Merge_FourWorkers(t *testing.T) {
	if runtime.NumCPU() < 2 {
		t.Fatal("requires GOMAXPROCS >= 2 for meaningful concurrency")
	}
	const seedN = 32
	const total = 64
	coords := linspace(total, 0, 11)
	sup := newBuilderScorerSupplier(coords)
	src := buildSourceGraph(t, coords[:seedN], 8, 16, 1)

	cm, err := NewConcurrentHnswMerger("vec", sup, 8, 16, 4)
	if err != nil {
		t.Fatalf("NewConcurrentHnswMerger: %v", err)
	}
	reader := &graphBackedKnnVectorsReader{
		graph:  src,
		values: &stubKnnVectorValues{dim: 1, n: seedN},
	}
	if _, err := cm.AddReader(reader, identityDocMap{}, nil); err != nil {
		t.Fatalf("AddReader: %v", err)
	}

	merged := &stubKnnVectorValues{dim: 1, n: total}
	dst, err := cm.Merge(merged, util.DefaultInfoStream(), total)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if dst == nil {
		t.Fatalf("Merge: nil graph")
	}
	if got := dst.MaxNodeID(); got != total-1 {
		t.Errorf("MaxNodeID=%d, want %d", got, total-1)
	}
	for node := 0; node < total; node++ {
		if !dst.NodeExistAtLevel(0, node) {
			t.Errorf("node %d missing from dst level 0", node)
		}
		nbrs := dst.GetNeighbors(0, node)
		for i := 0; i < nbrs.Size(); i++ {
			nbr := nbrs.Nodes()[i]
			if nbr == node {
				t.Errorf("self-loop on ordinal %d at level 0", node)
			}
			if nbr < 0 || nbr >= total {
				t.Errorf("ordinal %d neighbour %d out of range [0,%d)", node, nbr, total)
			}
		}
	}
}

// TestConcurrentHnswMerger_HnswGraphMergerInterface confirms that
// ConcurrentHnswMerger satisfies HnswGraphMerger at the interface
// level so the codec layer can hold the merger through that
// abstraction. The shadowed AddReader and Merge must dispatch through
// the interface.
func TestConcurrentHnswMerger_HnswGraphMergerInterface(t *testing.T) {
	const n = 4
	coords := linspace(n, 0, 3)
	sup := newBuilderScorerSupplier(coords)

	var iface HnswGraphMerger
	cm, err := NewConcurrentHnswMerger("vec", sup, 4, 10, 2)
	if err != nil {
		t.Fatalf("NewConcurrentHnswMerger: %v", err)
	}
	iface = cm

	got, err := iface.AddReader(nil, nil, nil)
	if err != nil {
		t.Fatalf("AddReader: %v", err)
	}
	if got == nil {
		t.Errorf("AddReader through interface returned nil")
	}
	// The shadow guarantees the returned merger is the concurrent
	// variant, not the embedded parent. The contract matters because
	// downstream callers chain AddReader and then Merge.
	if _, ok := got.(*ConcurrentHnswMerger); !ok {
		t.Errorf("AddReader through interface returned %T, want *ConcurrentHnswMerger", got)
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

// TestConcurrentHnswMerger_Merge_NilInfoStream verifies the Merge code
// path handles a nil InfoStream gracefully — the Java reference passes
// the stream through unchecked and would NPE on a nil; the Go port
// must short-circuit the SetInfoStream call.
func TestConcurrentHnswMerger_Merge_NilInfoStream(t *testing.T) {
	const n = 6
	coords := linspace(n, 0, 4)
	sup := newBuilderScorerSupplier(coords)

	cm, err := NewConcurrentHnswMerger("vec", sup, 4, 10, 2)
	if err != nil {
		t.Fatalf("NewConcurrentHnswMerger: %v", err)
	}
	merged := &stubKnnVectorValues{dim: 1, n: n}
	dst, err := cm.Merge(merged, nil, n)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if dst == nil {
		t.Fatalf("Merge: nil graph with nil infoStream")
	}
}
