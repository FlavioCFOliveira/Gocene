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
	"time"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// InitializedHnswGraphBuilder constants. They mirror the Java
// reference verbatim and are exposed so callers can reason about the
// repair / rebalance thresholds without rebuilding from source.
const (
	// disconnectedNodeFactor is the threshold factor for deciding
	// whether a node has retained "enough" neighbours after ordinal
	// remapping. A node is considered disconnected when its post-
	// remap neighbour count is strictly less than
	// (originalNeighbourCount * disconnectedNodeFactor).
	//
	// Mirrors Lucene's DISCONNECTED_NODE_FACTOR field. The field is
	// declared as an instance double in Java with no setter — i.e.
	// effectively a constant — so the Go port exposes it as a
	// package-level untyped constant.
	disconnectedNodeFactor = 0.85
)

// InitializedHnswGraphBuilder is the Go port of
// org.apache.lucene.util.hnsw.InitializedHnswGraphBuilder (Lucene
// 10.4.0). It creates a graph builder pre-populated with the
// structure of an existing HnswGraph and then allows further nodes to
// be added incrementally. The typical use case is merging HNSW
// graphs from multiple segments while reusing the connectivity of a
// pre-built initialiser.
//
// The builder performs three internal phases when seeded from a
// source graph:
//
//  1. Copy the graph structure with ordinal remapping. Nodes whose
//     new ordinal is -1 are skipped (the corresponding documents
//     were deleted in the merged segment).
//  2. If any deletions were observed, repair nodes that lost more
//     than (1 - disconnectedNodeFactor) of their original neighbours
//     by searching the level from the remaining neighbours.
//  3. If any deletions were observed, rebalance the hierarchy so the
//     expected exponential level-size decay is preserved after the
//     repair phase.
//
// Java extends HnswGraphBuilder; the Go port uses an embedded
// *HnswGraphBuilder plus shadowed [InitializedHnswGraphBuilder.Build]
// and [InitializedHnswGraphBuilder.AddGraphNode] methods. The shadow
// design is the pre-approved override strategy: the embedded base
// supplies every other method untouched, while the shadowed two
// entry-points enforce the "skip already-initialised ordinals"
// invariant. The cost is ~5 LOC of duplicated loop body in Build;
// the gain is total isolation of the override from the base.
//
// Thread-safety contract mirrors HnswGraphBuilder: a single instance
// is not safe for concurrent use; multiple instances may write into
// the same graph under the same ordinal-partitioning discipline as
// the base type.
type InitializedHnswGraphBuilder struct {
	// HnswGraphBuilder is embedded so the new type inherits every
	// helper, including GetGraph / GetCompletedGraph / SetInfoStream
	// / addDiverseNeighbors / scorer / hnsw / graphSearcher.
	// AddGraphNode and Build are shadowed below.
	*HnswGraphBuilder

	// initializedNodes flags which ordinals were imported from the
	// source graph. May be nil — when nil, AddGraphNode never skips,
	// matching the convenience-path initGraph in Lucene.
	initializedNodes util.BitSet

	// levelToNodes records the ordinals present at each level after
	// the initial copy. The slice is indexed by level; element 0
	// contains the level-0 nodes, element 1 the level-1 nodes, etc.
	// rebalanceGraph may grow this slice when promoting nodes into
	// previously-empty levels.
	levelToNodes [][]int

	// hasDeletes is set to true by copyGraphStructure when at least
	// one source ordinal mapped to -1. When true, the repair and
	// rebalance phases run; when false, the build skips them and
	// behaves like a straight structural copy.
	hasDeletes bool
}

// NewInitializedHnswGraphBuilderFromGraph constructs and pre-populates
// an InitializedHnswGraphBuilder from initializerGraph. Mirrors
// Lucene's static fromGraph factory method.
//
// scorerSupplier supplies the vector-similarity scorer used while
// repairing disconnected nodes. beamWidth tunes the search beam used
// throughout. seed seeds the PCG random used for rebalancing-time
// node promotions (see [HnswGraphBuilder]). newOrdMap maps each
// ordinal in initializerGraph to its destination ordinal in the
// merged graph; entries set to -1 indicate deleted documents.
// initializedNodes may be nil to skip the "already imported" guard.
// totalNumberOfVectors fixes the upper bound on the destination
// graph size, used for slot pre-allocation.
//
// Returns an error if any parameter is invalid or if the destination
// graph could not be initialised from the supplied source.
func NewInitializedHnswGraphBuilderFromGraph(
	scorerSupplier RandomVectorScorerSupplier,
	beamWidth int,
	seed int64,
	initializerGraph HnswGraph,
	newOrdMap []int,
	initializedNodes util.BitSet,
	totalNumberOfVectors int,
) (*InitializedHnswGraphBuilder, error) {
	if initializerGraph == nil {
		return nil, errors.New("hnsw: NewInitializedHnswGraphBuilderFromGraph: initializerGraph must not be nil")
	}
	if newOrdMap == nil {
		return nil, errors.New("hnsw: NewInitializedHnswGraphBuilderFromGraph: newOrdMap must not be nil")
	}
	// Mirror Java: new OnHeapHnswGraph(initializerGraph.maxConn(),
	// totalNumberOfVectors) — the destination graph adopts the
	// initializer's M and the merged segment's total vector count.
	dest := NewOnHeapHnswGraph(initializerGraph.MaxConn(), totalNumberOfVectors)
	base, err := NewHnswGraphBuilderFromGraph(scorerSupplier, beamWidth, seed, dest)
	if err != nil {
		return nil, fmt.Errorf("hnsw: initialised builder: %w", err)
	}
	b := &InitializedHnswGraphBuilder{
		HnswGraphBuilder: base,
		initializedNodes: initializedNodes,
	}
	if err := b.initializeFromGraph(initializerGraph, newOrdMap); err != nil {
		return nil, fmt.Errorf("hnsw: initialise from graph: %w", err)
	}
	return b, nil
}

// InitGraph is the convenience factory equivalent to Lucene's static
// initGraph helper: build a fully initialised on-heap graph from a
// source graph without tracking initialised nodes (i.e. without
// planning to add additional nodes incrementally).
//
// The returned graph is frozen and ready for read-only consumption.
// The function mirrors the Java reference's parameter order verbatim.
func InitGraph(
	initializerGraph HnswGraph,
	newOrdMap []int,
	totalNumberOfVectors int,
	beamWidth int,
	scorerSupplier RandomVectorScorerSupplier,
) (*OnHeapHnswGraph, error) {
	b, err := NewInitializedHnswGraphBuilderFromGraph(
		scorerSupplier,
		beamWidth,
		RandSeed,
		initializerGraph,
		newOrdMap,
		nil,
		totalNumberOfVectors,
	)
	if err != nil {
		return nil, err
	}
	return b.GetGraph(), nil
}

// AddGraphNode shadows [HnswGraphBuilder.AddGraphNode] to honour the
// initialised-nodes guard. When initializedNodes is non-nil and
// reports node as already imported, the call is a no-op — the
// embedded base would otherwise re-process the ordinal and pollute
// its existing neighbour list. Otherwise the call delegates to the
// embedded base.
//
// The shadowing is by-method on the outer type; Go has no virtual
// dispatch, so callers must hold a *InitializedHnswGraphBuilder (not
// an *HnswGraphBuilder) for the guard to take effect. The shadowed
// Build below uses the outer receiver throughout to preserve that
// guarantee inside this package.
func (b *InitializedHnswGraphBuilder) AddGraphNode(node int) error {
	if b.initializedNodes != nil && b.initializedNodes.Get(node) {
		return nil
	}
	return b.HnswGraphBuilder.AddGraphNode(node)
}

// Build shadows [HnswGraphBuilder.Build] so the per-node loop calls
// the shadowed [InitializedHnswGraphBuilder.AddGraphNode] above. The
// duplicated loop is the price of Go's lack of virtual dispatch: if
// we delegated to b.HnswGraphBuilder.Build the loop inside that
// method would call the embedded AddGraphNode, bypassing the guard.
//
// The body is structurally identical to the base Build + addVectors
// helper: emit the build-graph info-stream message, iterate over
// [0, maxOrd), and finally request the completed graph.
func (b *InitializedHnswGraphBuilder) Build(maxOrd int) (*OnHeapHnswGraph, error) {
	if b.frozen.Load() {
		return nil, errors.New("hnsw: builder is frozen and cannot be updated")
	}
	if b.infoStream.IsEnabled(HnswComponent) {
		b.infoStream.Message(HnswComponent,
			fmt.Sprintf("build graph from %d vectors", maxOrd))
	}
	if err := b.addVectorsShadow(0, maxOrd); err != nil {
		return nil, err
	}
	return b.GetCompletedGraph()
}

// AddGraphNodeWithEntryPoints shadows the embedded counterpart so
// the initialised-nodes guard is also enforced when the level-0
// beam is seeded with explicit entry points. Mirrors Java's
// inherited overload reaching addGraphNode under the override.
func (b *InitializedHnswGraphBuilder) AddGraphNodeWithEntryPoints(
	node int, eps0 map[int]struct{},
) error {
	if b.initializedNodes != nil && b.initializedNodes.Get(node) {
		return nil
	}
	return b.HnswGraphBuilder.AddGraphNodeWithEntryPoints(node, eps0)
}

// addVectorsShadow is the per-task duplicate of the base
// addVectors helper. The shadow routes every per-node call through
// the outer receiver so the InitializedHnswGraphBuilder.AddGraphNode
// guard fires. The progress-message cadence is the same constant
// (every 10 000 nodes) used by the base.
func (b *InitializedHnswGraphBuilder) addVectorsShadow(minOrd, maxOrd int) error {
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

// initializeFromGraph is the three-phase boot procedure executed by
// the factory: copy the source structure with ordinal remapping;
// repair disconnected nodes; rebalance the hierarchy. Phases 2 and 3
// only run when at least one source ordinal was deleted in the
// merge, mirroring the Lucene `if (hasDeletes)` guard verbatim.
func (b *InitializedHnswGraphBuilder) initializeFromGraph(
	initializerGraph HnswGraph, newOrdMap []int,
) error {
	b.hasDeletes = false
	disconnectedNodesByLevel, err := b.copyGraphStructure(initializerGraph, newOrdMap)
	if err != nil {
		return err
	}
	if b.hasDeletes {
		if err := b.repairDisconnectedNodes(disconnectedNodesByLevel, initializerGraph); err != nil {
			return err
		}
		if err := b.rebalanceGraph(); err != nil {
			return err
		}
	}
	return nil
}

// copyGraphStructure copies every (level, node, neighbour) triple
// from initializerGraph into the destination graph held by the
// embedded base. Ordinals are remapped through newOrdMap; -1 entries
// flag deleted source documents and trigger b.hasDeletes.
//
// The method also computes the per-level disconnected-node set: a
// node is disconnected when its post-remap neighbour count is
// strictly less than (originalNeighbourCount * disconnectedNodeFactor).
// The returned map keys by level the list of disconnected ordinals,
// which the repair phase consumes.
func (b *InitializedHnswGraphBuilder) copyGraphStructure(
	initializerGraph HnswGraph, newOrdMap []int,
) (map[int][]int, error) {
	numLevels, err := initializerGraph.NumLevels()
	if err != nil {
		return nil, err
	}
	b.levelToNodes = make([][]int, numLevels)
	disconnectedNodesByLevel := make(map[int][]int, numLevels)

	for level := numLevels - 1; level >= 0; level-- {
		b.levelToNodes[level] = nil
		var disconnected []int
		it, err := initializerGraph.GetNodesOnLevel(level)
		if err != nil {
			return nil, err
		}

		for it.HasNext() {
			oldOrd := it.NextInt()
			if oldOrd < 0 || oldOrd >= len(newOrdMap) {
				return nil, fmt.Errorf(
					"hnsw: copyGraphStructure: source ordinal %d out of newOrdMap bounds [0,%d)",
					oldOrd, len(newOrdMap))
			}
			newOrd := newOrdMap[oldOrd]

			// Skip deleted documents (mapped to -1) and record that
			// the merge dropped at least one source node — the repair
			// and rebalance phases gate on this flag.
			if newOrd == -1 {
				b.hasDeletes = true
				continue
			}

			b.hnsw.AddNode(level, newOrd)
			b.levelToNodes[level] = append(b.levelToNodes[level], newOrd)
			b.hnsw.TrySetNewEntryNode(newOrd, level)
			if err := b.scorer.SetScoringOrdinal(newOrd); err != nil {
				return nil, err
			}

			// Copy neighbours.
			newNeighbors := b.hnsw.GetNeighbors(level, newOrd)
			if err := initializerGraph.SeekLevel(level, oldOrd); err != nil {
				return nil, err
			}
			oldNeighbourCount := 0
			for {
				oldNeighbor, err := initializerGraph.NextNeighbor()
				if err != nil {
					return nil, err
				}
				if oldNeighbor == util.NO_MORE_DOCS {
					break
				}
				oldNeighbourCount++
				if oldNeighbor < 0 || oldNeighbor >= len(newOrdMap) {
					return nil, fmt.Errorf(
						"hnsw: copyGraphStructure: neighbour ordinal %d out of newOrdMap bounds [0,%d)",
						oldNeighbor, len(newOrdMap))
				}
				newNeighbor := newOrdMap[oldNeighbor]
				if newNeighbor != -1 {
					// Java uses Float.NaN as the placeholder score
					// here. NeighborArray.AddOutOfOrder stores the
					// score verbatim; later repair / rebalance
					// passes only read node ids, so NaN propagates
					// inertly through the structural copy.
					newNeighbors.AddOutOfOrder(newNeighbor, float32(math.NaN()))
				}
			}

			// A node is "disconnected" when its surviving neighbour
			// count is strictly under DISCONNECTED_NODE_FACTOR of the
			// original count. Mirrors Java's strict-less-than guard.
			if float64(newNeighbors.Size()) < float64(oldNeighbourCount)*disconnectedNodeFactor {
				disconnected = append(disconnected, newOrd)
			}
		}
		disconnectedNodesByLevel[level] = disconnected
	}
	return disconnectedNodesByLevel, nil
}

// repairDisconnectedNodes drives the per-level repair pass for
// nodes flagged in copyGraphStructure. Mirrors Lucene's top-down
// iteration order.
func (b *InitializedHnswGraphBuilder) repairDisconnectedNodes(
	disconnectedNodesByLevel map[int][]int, initializerGraph HnswGraph,
) error {
	numLevels, err := initializerGraph.NumLevels()
	if err != nil {
		return err
	}
	for level := numLevels - 1; level >= 0; level-- {
		if err := b.fixDisconnectedNodes(disconnectedNodesByLevel[level], level); err != nil {
			return err
		}
	}
	return nil
}

// fixDisconnectedNodes repairs the supplied ordinals at level by
// searching from their existing neighbours (when any remain) and
// folding the diverse new candidates into their neighbour list.
// Nodes with no neighbours at all fall through to a full
// addConnections re-search, since Lucene cannot seed a search
// without entry points.
func (b *InitializedHnswGraphBuilder) fixDisconnectedNodes(
	disconnectedNodes []int, level int,
) error {
	if len(disconnectedNodes) == 0 {
		return nil
	}
	beamWidth := b.beamCandidates.K()
	candidates := NewGraphBuilderKnnCollector(beamWidth)
	scratchArray := NewNeighborArray(beamWidth, false)

	for _, node := range disconnectedNodes {
		if err := b.scorer.SetScoringOrdinal(node); err != nil {
			return err
		}
		existingNeighbors := b.hnsw.GetNeighbors(level, node)

		if existingNeighbors.Size() > 0 {
			entryPoints := make([]int, existingNeighbors.Size())
			copy(entryPoints, existingNeighbors.Nodes()[:existingNeighbors.Size()])
			if err := b.graphSearcher.SearchLevel(
				candidates, b.scorer, level, entryPoints, b.hnsw, nil,
			); err != nil {
				return err
			}
			popToScratch(candidates, scratchArray)
			if err := b.addDiverseNeighbors(
				level, node, scratchArray, b.scorer, true,
			); err != nil {
				return err
			}
		} else {
			if err := b.addConnections(node, level); err != nil {
				return err
			}
		}

		// Reset scratch state for the next disconnected node.
		scratchArray.Clear()
		candidates.Clear()
	}
	return nil
}

// rebalanceGraph promotes nodes from lower levels into higher
// levels until each level holds roughly the HNSW-paper expected
// node count (size * (1/M)^level). The promotion is randomised so
// the resulting hierarchy resembles a freshly built graph and not
// a degenerate one biased by the deletion pattern. Mirrors
// Lucene's rebalanceGraph verbatim, including the iteration order
// (levelToNodes[level-1] in insertion order) and the 1/M promotion
// probability.
//
// The Lucene reference instantiates a fresh SplittableRandom with
// the implicit nanos seed; the Go port reuses the builder's PCG
// random for determinism with the rest of the build. This is a
// documented divergence: SplittableRandom byte streams are not
// reproducible in Go (see hnsw_graph_builder.go), so a per-method
// random would not yield Lucene parity anyway, while reusing the
// builder's random keeps the test suite deterministic.
func (b *InitializedHnswGraphBuilder) rebalanceGraph() error {
	size := b.hnsw.Size()
	invMaxConn := 1.0 / float64(b.m)

	for level := 1; ; level++ {
		maxNodesAtLevel := int(float64(size) * math.Pow(invMaxConn, float64(level)))
		if maxNodesAtLevel <= 0 {
			break
		}

		currentNodesAtLevel := 0

		if level >= len(b.levelToNodes) {
			b.levelToNodes = util.GrowExact(b.levelToNodes, level+1)
			b.levelToNodes[level] = nil
		} else {
			currentNodesAtLevel = len(b.levelToNodes[level])
		}

		if currentNodesAtLevel >= maxNodesAtLevel {
			continue
		}

		// Iterate the level-below node list in insertion order so the
		// promotion is reproducible for the same builder seed.
		below := b.levelToNodes[level-1]
		for _, node := range below {
			if currentNodesAtLevel >= maxNodesAtLevel {
				break
			}
			if b.random.Float64() >= invMaxConn {
				continue
			}
			if b.hnsw.NodeExistAtLevel(level, node) {
				continue
			}
			if err := b.scorer.SetScoringOrdinal(node); err != nil {
				return err
			}
			b.hnsw.AddNode(level, node)

			if currentNodesAtLevel == 0 {
				curLevels, err := b.hnsw.NumLevels()
				if err != nil {
					return err
				}
				b.hnsw.TryPromoteNewEntryNode(node, level, curLevels-1)
			} else {
				if err := b.addConnections(node, level); err != nil {
					return err
				}
			}

			b.levelToNodes[level] = append(b.levelToNodes[level], node)
			currentNodesAtLevel++
		}
	}
	return nil
}

// addConnections links node into the graph at targetLevel, mirroring
// Lucene's private addConnections helper. The path is the standard
// HNSW descent: greedy single-best-entry search down to targetLevel,
// then a wide beam search at targetLevel, then diversity-filtered
// neighbour linking through addDiverseNeighbors.
func (b *InitializedHnswGraphBuilder) addConnections(node, targetLevel int) error {
	beamWidth := b.beamCandidates.K()
	candidates := NewGraphBuilderKnnCollector(beamWidth)
	entry, err := b.hnsw.EntryNode()
	if err != nil {
		return err
	}
	eps := []int{entry}

	curLevels, err := b.hnsw.NumLevels()
	if err != nil {
		return err
	}
	for level := curLevels - 1; level > targetLevel; level-- {
		if err := b.graphSearcher.SearchLevel(candidates, b.scorer, level, eps, b.hnsw, nil); err != nil {
			return err
		}
		eps[0] = candidates.PopNode()
		candidates.Clear()
	}

	if err := b.graphSearcher.SearchLevel(
		candidates, b.scorer, targetLevel, eps, b.hnsw, nil,
	); err != nil {
		return err
	}

	scratchArray := NewNeighborArray(beamWidth, false)
	popToScratch(candidates, scratchArray)
	return b.addDiverseNeighbors(targetLevel, node, scratchArray, b.scorer, true)
}

// Compile-time guard.
var _ HnswBuilder = (*InitializedHnswGraphBuilder)(nil)
