// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

package hnsw

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"sync/atomic"
	"time"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// HnswGraphBuilder constants. These mirror the Java public statics
// verbatim and are exposed so callers — most notably the codec layer —
// can default to the same hyper-parameters as Lucene.
const (
	// DefaultMaxConn is the default number of maximum connections per
	// node. Mirrors Lucene's HnswGraphBuilder.DEFAULT_MAX_CONN.
	DefaultMaxConn = 16

	// DefaultBeamWidth is the default size of the candidate queue
	// maintained while searching during graph construction. Mirrors
	// HnswGraphBuilder.DEFAULT_BEAM_WIDTH.
	DefaultBeamWidth = 100

	// HnswComponent is the InfoStream component tag used for HNSW
	// build messages. Mirrors HnswGraphBuilder.HNSW_COMPONENT.
	HnswComponent = "HNSW"

	// defaultRandSeed is the default random seed used when none is
	// supplied. Mirrors HnswGraphBuilder.DEFAULT_RAND_SEED.
	defaultRandSeed = 42

	// maxBulkScoreNodes caps the size of the scratch buffer used by
	// diversityCheck for bulk-scoring an existing neighbour list. The
	// constant mirrors the package-private MAX_BULK_SCORE_NODES from
	// Lucene. Keeping the value identical preserves the call shape and
	// the scoring chunk boundaries.
	maxBulkScoreNodes = 8
)

// RandSeed is the package-level random seed published for testing.
// The Lucene reference exposes the field as a public mutable static
// to let tests run construction with a fixed seed (the @SuppressWarnings
// annotation on the Java side acknowledges this is intentional).
//
// The Gocene port retains the same shape — including the global —
// because test parity hinges on it: a Lucene-style "set the seed, run
// the deterministic test" idiom would have to grow a per-test plumbing
// path otherwise. The variable is read once per builder, at
// construction time, when the caller passes -1 for seed.
//
//nolint:gochecknoglobals // mirrors Lucene's public static randSeed.
var RandSeed int64 = defaultRandSeed

// HnswGraphBuilder builds an [OnHeapHnswGraph] connecting node ordinals
// by their learnt edges. It is the Go port of
// org.apache.lucene.util.hnsw.HnswGraphBuilder (Lucene 10.4.0).
//
// Thread-safety contract (mirrors the Java reference):
//   - A single HnswGraphBuilder instance is not safe for concurrent
//     use. The intermediate buffers (bulk score scratch, scratch
//     NeighborArrays, the GraphBuilderKnnCollectors, the random
//     generator) are tied to the receiver and would race under
//     concurrent insertion.
//   - However, multiple HnswGraphBuilder instances may write into the
//     same [OnHeapHnswGraph] when the graph size is known in advance
//     (e.g. during a merge). The graph's per-node entry CAS, the
//     atomic counters on Size / MaxNodeID, and the per-NeighborArray
//     mutation discipline keep concurrent ordinal-partitioned writes
//     correct.
//
// Construction occurs in five passes:
//  1. Pick a random level via the geometric distribution governed by ml.
//  2. Pre-allocate per-level slots up to the chosen level.
//  3. Promote the node as entry point if the graph is empty.
//  4. From top to nodeLevel, do a single-hop greedy descent finding the
//     best entry point at each level.
//  5. From nodeLevel to 0, do a beam search seeded at the prior level's
//     best entry, collect candidates, then link with the diversity
//     heuristic — outgoing first, then incoming.
type HnswGraphBuilder struct {
	// m is the configured max number of connections per node on upper
	// levels (level 0 carries 2*m). The level-0 doubling mirrors the
	// HNSW paper's recommendation and Lucene's parameterisation.
	m int

	// ml is the normalisation factor for the geometric level
	// distribution, computed once at construction as 1/ln(M) (or 1
	// when M == 1, which is degenerate but legal). Mirrors Lucene.
	ml float64

	// bulkScoreNodes / bulkScores are the small fixed buffers used by
	// diversityCheck. They are receiver-scoped and sized to
	// maxBulkScoreNodes so the diversityCheck call path never
	// allocates.
	bulkScoreNodes []int
	bulkScores     []float32

	// random is the SplittableRandom counterpart in Go: a PCG-based
	// rand.Rand seeded deterministically from the user-supplied
	// long. The same seed reproduces the same level sequence within a
	// single goroutine.
	random *rand.Rand

	// scorer is the in-place RandomVectorScorer that addGraphNode
	// updates with the inserting node's ordinal before each search.
	scorer UpdateableRandomVectorScorer

	// graphSearcher drives the per-level descent (FindBestEntryPoint
	// equivalent) and the per-level beam search.
	graphSearcher *HnswGraphSearcher

	// Candidate collectors. entryCandidates is used on levels strictly
	// above the inserting node's level (where Lucene takes only the
	// single best). beamCandidates is the wide collector used for
	// regular levels. beamCandidates0 is the smaller level-0
	// collector reserved for the MergingHnswGraphBuilder path —
	// activated when AddGraphNodeWithEntryPoints is invoked with a
	// non-empty eps0 set.
	entryCandidates *GraphBuilderKnnCollector
	beamCandidates  *GraphBuilderKnnCollector
	beamCandidates0 *GraphBuilderKnnCollector

	// hnsw is the in-progress graph the builder is writing into. The
	// field is exported via [HnswGraphBuilder.GetGraph].
	hnsw *OnHeapHnswGraph

	// hnswLock is the optional striped lock that guards concurrent
	// mutation of the graph's neighbour arrays. When nil the builder
	// uses the single-thread fast path (no locking, matching Java's
	// `hnswLock == null` branch). When non-nil — set by
	// [HnswConcurrentMergeBuilder] for its worker builders —
	// addDiverseNeighbors takes a per-(level, node) write lock around
	// each neighbour update and the merge-aware searcher takes a read
	// lock around each graphSeek.
	hnswLock *HnswLock

	// infoStream is the diagnostic sink for HNSW build messages.
	infoStream util.InfoStream

	// frozen flips to true on the first call to GetCompletedGraph or
	// finish. Subsequent Add / Build attempts return an error,
	// mirroring Java's IllegalStateException.
	frozen atomic.Bool
}

// NewHnswGraphBuilder constructs a builder with M, beamWidth, seed
// and an unknown graph size. Equivalent to Lucene's
// HnswGraphBuilder.create(scorerSupplier, M, beamWidth, seed).
func NewHnswGraphBuilder(
	scorerSupplier RandomVectorScorerSupplier, m, beamWidth int, seed int64,
) (*HnswGraphBuilder, error) {
	return NewHnswGraphBuilderWithGraphSize(scorerSupplier, m, beamWidth, seed, -1)
}

// NewHnswGraphBuilderWithGraphSize constructs a builder with an
// upfront upper bound on the graph size. Pass -1 when the size is
// unknown. Equivalent to Lucene's HnswGraphBuilder.create with a
// graphSize override.
func NewHnswGraphBuilderWithGraphSize(
	scorerSupplier RandomVectorScorerSupplier, m, beamWidth int, seed int64, graphSize int,
) (*HnswGraphBuilder, error) {
	hnsw := NewOnHeapHnswGraph(m, graphSize)
	return newHnswGraphBuilderWithGraph(scorerSupplier, m, beamWidth, seed, hnsw)
}

// NewHnswGraphBuilderFromGraph constructs a builder that writes into
// the supplied OnHeapHnswGraph; M is taken from graph.MaxConn().
// Mirrors Lucene's three-arg HnswGraphBuilder constructor.
func NewHnswGraphBuilderFromGraph(
	scorerSupplier RandomVectorScorerSupplier, beamWidth int, seed int64, graph *OnHeapHnswGraph,
) (*HnswGraphBuilder, error) {
	if graph == nil {
		return nil, errors.New("hnsw: NewHnswGraphBuilderFromGraph: graph must not be nil")
	}
	return newHnswGraphBuilderWithGraph(scorerSupplier, graph.MaxConn(), beamWidth, seed, graph)
}

// newHnswGraphBuilderWithLock is the constructor variant used by
// [HnswConcurrentMergeBuilder] to plug in the striped HnswLock and a
// pre-built [HnswGraphSearcher] whose seek/next policies serialise
// reads against concurrent neighbour updates. The lock and searcher
// arguments may both be nil, in which case the call is equivalent to
// [newHnswGraphBuilderWithGraph].
//
// Exposed unexported because every legitimate caller lives inside this
// package; the public surface keeps the single-thread constructors.
func newHnswGraphBuilderWithLock(
	scorerSupplier RandomVectorScorerSupplier,
	m, beamWidth int,
	seed int64,
	graph *OnHeapHnswGraph,
	hnswLock *HnswLock,
	searcher *HnswGraphSearcher,
) (*HnswGraphBuilder, error) {
	b, err := newHnswGraphBuilderWithGraph(scorerSupplier, m, beamWidth, seed, graph)
	if err != nil {
		return nil, err
	}
	b.hnswLock = hnswLock
	if searcher != nil {
		b.graphSearcher = searcher
	}
	return b, nil
}

// newHnswGraphBuilderWithGraph is the unexported workhorse the public
// constructors funnel through. It validates the hyper-parameters,
// pulls the scorer from the supplier, sizes the searcher's scratch
// state, and constructs the candidate collectors.
func newHnswGraphBuilderWithGraph(
	scorerSupplier RandomVectorScorerSupplier,
	m, beamWidth int,
	seed int64,
	graph *OnHeapHnswGraph,
) (*HnswGraphBuilder, error) {
	if m <= 0 {
		return nil, errors.New("hnsw: M (max connections) must be positive")
	}
	if beamWidth <= 0 {
		return nil, errors.New("hnsw: beamWidth must be positive")
	}
	if scorerSupplier == nil {
		return nil, errors.New("hnsw: scorer supplier must not be null")
	}
	scorer, err := scorerSupplier.Scorer()
	if err != nil {
		return nil, fmt.Errorf("hnsw: scorer supplier: %w", err)
	}

	// The Java reference instantiates the searcher inline:
	//   new HnswGraphSearcher(new NeighborQueue(beamWidth, true),
	//                          new FixedBitSet(hnsw.size()))
	// hnsw.size() at this point is 0 (no nodes added yet) — the
	// constructor receives an empty FixedBitSet. Go's
	// util.NewFixedBitSet rejects length <= 0, so we seed with a
	// length-1 bitset; prepareScratchState grows it on the first
	// SearchLevel call.
	bitsetSize := graph.Size()
	if bitsetSize < 1 {
		bitsetSize = 1
	}
	visited, err := util.NewFixedBitSet(bitsetSize)
	if err != nil {
		return nil, fmt.Errorf("hnsw: visited bitset: %w", err)
	}
	searcher := NewHnswGraphSearcher(
		NewNeighborQueue(beamWidth, true),
		visited,
	)

	// ml = 1/ln(M); M == 1 is degenerate (ln(1) == 0) so Lucene falls
	// back to 1. The geometric distribution becomes -log(U)/ln(M),
	// drawing levels with probability decreasing exponentially.
	var ml float64
	if m == 1 {
		ml = 1
	} else {
		ml = 1.0 / math.Log(float64(m))
	}

	// The Java reference uses SplittableRandom (a streaming-friendly,
	// 64-bit-stride generator with a fixed seed contract). Go's
	// math/rand/v2 PCG is the closest available stdlib equivalent;
	// the level sequences will not byte-match Lucene's, but they
	// share the same statistical properties — a divergence the build
	// task documents in the rmp completion notes.
	r := rand.New(rand.NewPCG(uint64(seed), uint64(seed^0x6364136223846793)))

	beam0Cap := beamWidth / 2
	if m*3 < beam0Cap {
		beam0Cap = m * 3
	}
	if beam0Cap < 1 {
		// k must be > 0 (NeighborQueue rejects k <= 0). Degenerate
		// configurations (e.g. M == 1, beamWidth == 1) are valid in
		// Lucene; default to 1 so the builder remains usable.
		beam0Cap = 1
	}

	b := &HnswGraphBuilder{
		m:               m,
		ml:              ml,
		bulkScoreNodes:  make([]int, maxBulkScoreNodes),
		bulkScores:      make([]float32, maxBulkScoreNodes),
		random:          r,
		scorer:          scorer,
		graphSearcher:   searcher,
		entryCandidates: NewGraphBuilderKnnCollector(1),
		beamCandidates:  NewGraphBuilderKnnCollector(beamWidth),
		beamCandidates0: NewGraphBuilderKnnCollector(beam0Cap),
		hnsw:            graph,
		infoStream:      util.DefaultInfoStream(),
	}
	return b, nil
}

// Build adds nodes 0..maxOrd (exclusive) to the graph and returns the
// completed graph. Mirrors Java's public OnHeapHnswGraph build(int).
func (b *HnswGraphBuilder) Build(maxOrd int) (*OnHeapHnswGraph, error) {
	if b.frozen.Load() {
		return nil, errors.New("hnsw: builder is frozen and cannot be updated")
	}
	if b.infoStream.IsEnabled(HnswComponent) {
		b.infoStream.Message(HnswComponent,
			fmt.Sprintf("build graph from %d vectors", maxOrd))
	}
	if err := b.addVectors(0, maxOrd); err != nil {
		return nil, err
	}
	return b.GetCompletedGraph()
}

// SetInfoStream installs a diagnostic sink. Subsequent build messages
// will be routed through this stream. Mirrors Java's
// public void setInfoStream(InfoStream).
func (b *HnswGraphBuilder) SetInfoStream(stream util.InfoStream) {
	if stream == nil {
		stream = util.DefaultInfoStream()
	}
	b.infoStream = stream
}

// GetCompletedGraph returns the final graph, freezing the builder if
// not already frozen. Subsequent AddGraphNode calls return an error.
// Mirrors Java's public OnHeapHnswGraph getCompletedGraph().
func (b *HnswGraphBuilder) GetCompletedGraph() (*OnHeapHnswGraph, error) {
	if !b.frozen.Load() {
		if err := b.finish(); err != nil {
			return nil, err
		}
	}
	return b.GetGraph(), nil
}

// GetGraph returns the in-progress graph. Mirrors Java's
// public OnHeapHnswGraph getGraph().
func (b *HnswGraphBuilder) GetGraph() *OnHeapHnswGraph { return b.hnsw }

// AddGraphNode inserts node into the graph. Mirrors Java's
// public void addGraphNode(int).
func (b *HnswGraphBuilder) AddGraphNode(node int) error {
	if err := b.scorer.SetScoringOrdinal(node); err != nil {
		return err
	}
	return b.addGraphNodeInternal(node, b.scorer, nil)
}

// AddGraphNodeWithEntryPoints inserts node into the graph, seeding the
// level-0 search with the supplied entry points. Mirrors Java's
// public void addGraphNode(int, IntHashSet).
func (b *HnswGraphBuilder) AddGraphNodeWithEntryPoints(node int, eps0 map[int]struct{}) error {
	if err := b.scorer.SetScoringOrdinal(node); err != nil {
		return err
	}
	return b.addGraphNodeInternal(node, b.scorer, eps0)
}

// addVectors inserts the half-open range [minOrd, maxOrd). Mirrors
// Java's protected void addVectors(int, int).
func (b *HnswGraphBuilder) addVectors(minOrd, maxOrd int) error {
	if b.frozen.Load() {
		return errors.New("hnsw: builder is frozen and cannot be updated")
	}
	start := time.Now()
	t := start
	if b.infoStream.IsEnabled(HnswComponent) {
		b.infoStream.Message(HnswComponent,
			fmt.Sprintf("addVectors [%d %d)", minOrd, maxOrd))
	}
	for node := minOrd; node < maxOrd; node++ {
		if err := b.AddGraphNode(node); err != nil {
			return err
		}
		if node%10000 == 0 && b.infoStream.IsEnabled(HnswComponent) {
			t = b.printGraphBuildStatus(node, start, t)
		}
	}
	return nil
}

// printGraphBuildStatus emits a per-batch progress message and returns
// the current timestamp so callers can use it as the next baseline.
// Mirrors Java's private long printGraphBuildStatus(int, long, long).
func (b *HnswGraphBuilder) printGraphBuildStatus(node int, start, t time.Time) time.Time {
	now := time.Now()
	b.infoStream.Message(HnswComponent,
		fmt.Sprintf("built %d in %d/%d ms",
			node, now.Sub(t).Milliseconds(), now.Sub(start).Milliseconds()))
	return now
}

// addGraphNodeInternal is the workhorse for AddGraphNode. The Java
// reference's javadoc lays out the algorithm in four steps:
//
//  1. Pick a level for the node; pre-allocate it on every level [0,
//     nodeLevel] (without making any connections yet).
//  2. If the graph is empty, promote the node as entry and return.
//  3. Otherwise, descend the graph from the current top level: for
//     levels above nodeLevel use a single-best-entry search; for the
//     remaining levels run a wide beam search and pop into a scratch
//     NeighborArray.
//  4. Walk the scratch arrays bottom-up, linking each pair with the
//     diversity heuristic (outgoing first, then incoming).
//  5. If the node's level exceeds the graph's current top level, try
//     to promote it as the new entry; otherwise repeat the wide-beam
//     loop on the newly-introduced levels and try again.
//
// eps0 is the optional MergingHnswGraphBuilder seed: when non-empty
// the level-0 beam search is seeded with these ordinals instead of the
// entry node, and the smaller beamCandidates0 collector is used.
func (b *HnswGraphBuilder) addGraphNodeInternal(
	node int, scorer UpdateableRandomVectorScorer, eps0 map[int]struct{},
) error {
	if b.frozen.Load() {
		return errors.New("hnsw: builder is already frozen")
	}

	nodeLevel := getRandomGraphLevel(b.ml, b.random)
	// First pre-allocate every level slot the new node will occupy.
	for level := nodeLevel; level >= 0; level-- {
		b.hnsw.AddNode(level, node)
	}

	// If the graph had no entry, promote the freshly added node and
	// return — no connections yet. The CAS guards against another
	// builder beating us to the punch.
	if b.hnsw.TrySetNewEntryNode(node, nodeLevel) {
		return nil
	}

	lowestUnsetLevel := 0
	for {
		curMaxLevelPlus1, err := b.hnsw.NumLevels()
		if err != nil {
			return err
		}
		curMaxLevel := curMaxLevelPlus1 - 1

		entry, err := b.hnsw.EntryNode()
		if err != nil {
			return err
		}
		eps := []int{entry}

		// Single-best-entry descent on levels strictly above nodeLevel.
		candidates := b.entryCandidates
		for level := curMaxLevel; level > nodeLevel; level-- {
			candidates.Clear()
			if err := b.graphSearcher.SearchLevel(candidates, scorer, level, eps, b.hnsw, nil); err != nil {
				return err
			}
			eps[0] = candidates.PopNode()
		}

		// Wide-beam search on every level the new node belongs to.
		minLevel := nodeLevel
		if curMaxLevel < minLevel {
			minLevel = curMaxLevel
		}
		scratchLen := minLevel - lowestUnsetLevel + 1
		scratchPerLevel := make([]*NeighborArray, scratchLen)
		for i := scratchLen - 1; i >= 0; i-- {
			level := i + lowestUnsetLevel
			candidates = b.beamCandidates
			if level == 0 && len(eps0) > 0 {
				eps = mapKeysToSlice(eps0)
				candidates = b.beamCandidates0
			}
			candidates.Clear()
			if err := b.graphSearcher.SearchLevel(candidates, scorer, level, eps, b.hnsw, nil); err != nil {
				return err
			}
			eps = candidates.PopUntilNearestKNodes()
			// The scratch NeighborArray is ascending-order (worst
			// score first) — popToScratch fills it from the heap top,
			// which is the worst-kept entry, downward. The scratch
			// capacity follows Java verbatim: max(k, M+1).
			scratchCap := candidates.K()
			if b.m+1 > scratchCap {
				scratchCap = b.m + 1
			}
			scratchPerLevel[i] = NewNeighborArray(scratchCap, false)
			popToScratch(candidates, scratchPerLevel[i])
		}

		// Bottom-up connection pass.
		for i := 0; i < scratchLen; i++ {
			if err := b.addDiverseNeighbors(
				i+lowestUnsetLevel, node, scratchPerLevel[i], scorer, false,
			); err != nil {
				return err
			}
		}
		lowestUnsetLevel += scratchLen
		expected := minLevel + 1
		if lowestUnsetLevel != expected {
			return fmt.Errorf(
				"hnsw: invariant violation: lowestUnsetLevel=%d expected=%d",
				lowestUnsetLevel, expected)
		}
		if lowestUnsetLevel == nodeLevel+1 {
			return nil
		}
		// Java assert: lowestUnsetLevel == curMaxLevel + 1 && nodeLevel
		// > curMaxLevel — the new node needs to be promoted to the
		// graph's entry on a level above the current top.
		if b.hnsw.TryPromoteNewEntryNode(node, nodeLevel, curMaxLevel) {
			return nil
		}
		// Lost the promotion race. Verify the graph's max level has
		// changed; otherwise we are looking at an impossible state.
		levels, err := b.hnsw.NumLevels()
		if err != nil {
			return err
		}
		if levels == curMaxLevel+1 {
			return fmt.Errorf(
				"hnsw: unable to promote node %d at level %d as entry node — "+
					"max graph level %d unchanged",
				node, nodeLevel, curMaxLevel)
		}
	}
}

// addDiverseNeighbors links node and its candidates on the supplied
// level using the diversity heuristic. The candidate scores were
// gathered by the surrounding beam search; this method picks at most
// maxConnOnLevel of them (filtered for diversity), records them on the
// new node, and then folds the new node back into each selected
// candidate's neighbour list, again applying the diversity heuristic.
//
// isLinkRepair is the flag used by the (currently dormant)
// connectComponents path to switch into a duplicate-aware addition
// mode; it is always false for regular insertion.
//
// Mirrors Java's void addDiverseNeighbors(int, int, NeighborArray,
// UpdateableRandomVectorScorer, boolean).
func (b *HnswGraphBuilder) addDiverseNeighbors(
	level, node int,
	candidates *NeighborArray,
	scorer UpdateableRandomVectorScorer,
	isLinkRepair bool,
) error {
	neighbors := b.hnsw.GetNeighbors(level, node)
	maxConnOnLevel := b.m
	if level == 0 {
		maxConnOnLevel = b.m * 2
	}
	mask, err := b.selectAndLinkDiverse(node, neighbors, candidates, maxConnOnLevel, scorer, isLinkRepair)
	if err != nil {
		return err
	}

	// Re-fold the new node into each selected candidate's neighbour
	// list. When hnswLock is non-nil (concurrent merge path) we take
	// the per-(level, nbr) write lock around the update; otherwise we
	// follow the unlocked fast path, matching Java verbatim.
	//
	// NOTE (mirrors the Java comment): we deliberately read nbr from
	// the local `candidates` / `mask` and not from the new node's
	// `neighbors` array — once an incoming link is added, the new node
	// becomes discoverable by concurrent searches and its neighbour
	// array may be mutated underneath us.
	for i := 0; i < candidates.Size(); i++ {
		if !mask[i] {
			continue
		}
		nbr := candidates.Nodes()[i]
		if b.hnswLock != nil {
			release := b.hnswLock.WriteLock(level, nbr)
			err := b.updateNeighbor(
				b.hnsw.GetNeighbors(level, nbr),
				node,
				candidates.GetScore(i),
				nbr,
				scorer,
				isLinkRepair,
			)
			release()
			if err != nil {
				return err
			}
		} else {
			if err := b.updateNeighbor(
				b.hnsw.GetNeighbors(level, nbr),
				node,
				candidates.GetScore(i),
				nbr,
				scorer,
				isLinkRepair,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

// updateNeighbor splices node into nbrsOfNbr with the diversity
// heuristic. The Java `isLinkRepair` branch dedups against the
// existing neighbour list — used only by connectComponents, which the
// upstream reference has disabled (see Lucene #14214). Mirrors Java's
// private void updateNeighbor(...).
func (b *HnswGraphBuilder) updateNeighbor(
	nbrsOfNbr *NeighborArray, node int, score float32, nbr int,
	scorer UpdateableRandomVectorScorer, isLinkRepair bool,
) error {
	if isLinkRepair {
		for j := 0; j < nbrsOfNbr.Size(); j++ {
			if nbrsOfNbr.Nodes()[j] == node {
				return nil
			}
		}
	}
	return nbrsOfNbr.AddAndEnsureDiversity(node, score, nbr, scorer)
}

// selectAndLinkDiverse iterates candidates from best to worst,
// admitting each one if it satisfies the diversity heuristic. Returns
// a mask aligned with candidates.Nodes() indicating which entries were
// retained. Mirrors Java's private boolean[] selectAndLinkDiverse(...).
//
// The mask is allocated per call (Java allocates a boolean[] of the
// same size). Hot-path callers can recycle the returned slice if
// needed; the current code does not, mirroring Java.
func (b *HnswGraphBuilder) selectAndLinkDiverse(
	node int,
	neighbors, candidates *NeighborArray,
	maxConnOnLevel int,
	scorer UpdateableRandomVectorScorer,
	isLinkRepair bool,
) ([]bool, error) {
	mask := make([]bool, candidates.Size())
	for i := candidates.Size() - 1; neighbors.Size() < maxConnOnLevel && i >= 0; i-- {
		cNode := candidates.Nodes()[i]
		if node == cNode {
			continue
		}
		cScore := candidates.GetScore(i)
		// Java asserts cNode <= hnsw.maxNodeId(); the Go port relies
		// on the same invariant — if it ever broke, GetNeighbors
		// would panic up the call stack.
		if err := scorer.SetScoringOrdinal(cNode); err != nil {
			return nil, err
		}
		diverse, err := b.diversityCheck(cScore, neighbors, scorer)
		if err != nil {
			return nil, err
		}
		if !diverse {
			continue
		}
		mask[i] = true
		if isLinkRepair {
			neighbors.AddOutOfOrder(cNode, cScore)
		} else {
			neighbors.AddInOrder(cNode, cScore)
		}
	}
	return mask, nil
}

// popToScratch drains candidates into scratch in worst-to-best order.
// scratch must be configured with ScoresDescOrder == false (i.e.
// ascending scores) because [NeighborArray.AddInOrder] enforces a
// monotonic sort. Mirrors Java's static void popToScratch(...).
func popToScratch(candidates *GraphBuilderKnnCollector, scratch *NeighborArray) {
	scratch.Clear()
	candidateCount := candidates.Size()
	for i := 0; i < candidateCount; i++ {
		maxSimilarity := candidates.MinimumScore()
		scratch.AddInOrder(candidates.PopNode(), maxSimilarity)
	}
}

// diversityCheck reports whether a new candidate (score) is diverse
// relative to the current neighbour list. The Java implementation
// scores the existing neighbours in chunks of up to maxBulkScoreNodes
// and short-circuits as soon as any neighbour scores higher than the
// new candidate (i.e. the candidate is closer to an existing
// neighbour than to the inserting node). Mirrors Java verbatim,
// including the chunk arithmetic.
func (b *HnswGraphBuilder) diversityCheck(
	score float32, neighbors *NeighborArray, scorer RandomVectorScorer,
) (bool, error) {
	bulkChunk := (neighbors.Size() + 1) / 2
	if bulkChunk > maxBulkScoreNodes {
		bulkChunk = maxBulkScoreNodes
	}
	if bulkChunk == 0 {
		return true, nil
	}
	for scored := 0; scored < neighbors.Size(); scored += bulkChunk {
		chunkSize := bulkChunk
		if neighbors.Size()-scored < chunkSize {
			chunkSize = neighbors.Size() - scored
		}
		copy(b.bulkScoreNodes[:chunkSize], neighbors.Nodes()[scored:scored+chunkSize])
		maxScore, err := scorer.BulkScore(
			b.bulkScoreNodes[:chunkSize], b.bulkScores[:chunkSize], chunkSize,
		)
		if err != nil {
			return false, err
		}
		if maxScore >= score {
			return false, nil
		}
	}
	return true, nil
}

// getRandomGraphLevel samples a level from the geometric distribution
// parameterised by ml. The randDouble == 0 retry guards against
// log(0) = -Inf, which the cast to int would turn into INT_MIN.
// Mirrors Java's private static int getRandomGraphLevel(double,
// SplittableRandom).
func getRandomGraphLevel(ml float64, r *rand.Rand) int {
	var u float64
	for {
		u = r.Float64()
		if u != 0.0 {
			break
		}
	}
	return int(-math.Log(u) * ml)
}

// finish marks the builder frozen. The Java reference's
// connectComponents pass is intentionally disabled upstream (see
// Lucene #14214); the Go port keeps the helper as dormant code with
// the same intent so callers can re-enable it locally if they want.
func (b *HnswGraphBuilder) finish() error {
	// TODO(rmp): connectComponents is dormant in Lucene 10.4.0; reactivate
	// when the upstream issue lands (https://github.com/apache/lucene/issues/14214).
	b.frozen.Store(true)
	return nil
}

// mapKeysToSlice returns the keys of m as a slice in unspecified
// order. Used to translate Java's IntHashSet.toArray() into Go's
// map-based eps0 representation. Mirrors the substitution used
// elsewhere in this package.
func mapKeysToSlice(m map[int]struct{}) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// Compile-time guard.
var _ HnswBuilder = (*HnswGraphBuilder)(nil)
