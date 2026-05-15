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
	"sort"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// MergingHnswGraphBuilder is the Go port of
// org.apache.lucene.util.hnsw.MergingHnswGraphBuilder (Lucene 10.4.0).
// It is used during segment merging to consolidate multiple HNSW
// graphs into a single graph by reusing the connectivity of the
// largest input graph and folding the remaining smaller graphs in
// with the "join set" heuristic described by Lucene.
//
// The algorithm:
//
//  1. Pre-condition (handled by the caller of the factory): the
//     graphs slice is sorted descending by size and the first entry
//     becomes the seed for the destination graph via
//     [InitGraph].
//  2. For every remaining graph gS:
//     a. Compute its join set j via [ComputeJoinSet]. j is a small
//     subset of nodes whose insertion best "covers" gS.
//     b. Insert the join-set nodes through the default AddGraphNode
//     path so they receive a full search-and-link pass.
//     c. For every other node u in gS, build an entry-point set eps
//     by unioning:
//     - u's neighbours v in gS (after ordinal remap to gL),
//     restricted to neighbours that are already part of gL
//     (either v is in j, or v < u so the prior iteration
//     already inserted it);
//     - those neighbours' neighbours in gL (level 0).
//     Then insert u via AddGraphNodeWithEntryPoints(eps). The
//     smaller beamCandidates0 collector is used at level 0,
//     mirroring Lucene's beamWidth = M*3 cap for merge-path
//     searches.
//  3. After every input graph is folded in, if initializedNodes is
//     not nil, walk [0, maxOrd) and insert any ordinal that was not
//     yet initialised. This catches ordinals that did not appear in
//     any input graph (e.g. newly-added documents in the merged
//     segment).
//
// Java extends HnswGraphBuilder; the Go port uses an embedded
// *HnswGraphBuilder plus a shadowed [MergingHnswGraphBuilder.Build].
// Only Build is shadowed because MergingHnswGraphBuilder does not
// override AddGraphNode itself — the merge path uses the inherited
// AddGraphNode / AddGraphNodeWithEntryPoints implementations verbatim.
//
// Thread-safety contract mirrors HnswGraphBuilder: a single instance
// is not safe for concurrent use.
type MergingHnswGraphBuilder struct {
	// HnswGraphBuilder is embedded so the new type inherits every
	// helper, including GetGraph / GetCompletedGraph / SetInfoStream
	// / AddGraphNode / AddGraphNodeWithEntryPoints / addDiverseNeighbors.
	// Build is shadowed below.
	*HnswGraphBuilder

	// graphs holds the input HNSW graphs to merge, sorted in
	// descending size order. graphs[0] was already consumed to seed
	// the destination graph via InitGraph and is not re-traversed
	// during Build; only graphs[1:] are folded in.
	graphs []HnswGraph

	// ordMaps[i] maps a node ordinal in graphs[i] to its ordinal in
	// the destination graph. ordMaps and graphs are parallel slices
	// of equal length.
	ordMaps [][]int

	// initializedNodes flags which destination ordinals were
	// initialised through one of the input graphs (either by the
	// initial InitGraph copy or by the per-graph updateGraph pass).
	// When non-nil, Build performs a final sweep over [0, maxOrd)
	// and inserts any ordinal that was not yet initialised. When
	// nil, the caller is asserting that every ordinal in
	// [0, maxOrd) is covered by one of the input graphs.
	initializedNodes util.BitSet
}

// NewMergingHnswGraphBuilderFromGraphs is the Go counterpart of
// Lucene's static MergingHnswGraphBuilder.fromGraphs factory. It
// constructs a builder seeded with the connectivity of graphs[0]
// (via [InitGraph]) and prepared to fold graphs[1:] in during
// [MergingHnswGraphBuilder.Build].
//
// Parameters mirror the Java reference:
//
//   - scorerSupplier: supplies the in-place vector scorer used by
//     the underlying builder.
//   - m: max number of connections per node (graph fan-out).
//   - beamWidth: candidate queue size used while searching during
//     graph construction.
//   - seed: PCG seed for level assignment / promotion decisions.
//   - graphs: the input HNSW graphs to merge. Must be sorted
//     descending by size; graphs[0] is the seed. Must not be empty
//     or contain nil entries.
//   - ordMaps: parallel ordinal maps. ordMaps[i][n] is the
//     destination ordinal for node n in graphs[i]. Must match
//     graphs in length.
//   - totalNumberOfVectors: upper bound on the destination graph
//     size; passed verbatim to InitGraph for slot pre-allocation.
//   - initializedNodes: optional bitset flagging ordinals already
//     present in the destination graph after the InitGraph seed.
//     May be nil — Build then assumes every ordinal in
//     [0, totalNumberOfVectors) is covered.
//
// Returns an error when any pre-condition is violated or when the
// InitGraph seed fails.
func NewMergingHnswGraphBuilderFromGraphs(
	scorerSupplier RandomVectorScorerSupplier,
	m, beamWidth int,
	seed int64,
	graphs []HnswGraph,
	ordMaps [][]int,
	totalNumberOfVectors int,
	initializedNodes util.BitSet,
) (*MergingHnswGraphBuilder, error) {
	if len(graphs) == 0 {
		return nil, errors.New("hnsw: NewMergingHnswGraphBuilderFromGraphs: graphs must not be empty")
	}
	if len(graphs) != len(ordMaps) {
		return nil, fmt.Errorf(
			"hnsw: NewMergingHnswGraphBuilderFromGraphs: graphs/ordMaps length mismatch: %d vs %d",
			len(graphs), len(ordMaps))
	}
	for i, g := range graphs {
		if g == nil {
			return nil, fmt.Errorf("hnsw: NewMergingHnswGraphBuilderFromGraphs: graphs[%d] is nil", i)
		}
		if ordMaps[i] == nil {
			return nil, fmt.Errorf("hnsw: NewMergingHnswGraphBuilderFromGraphs: ordMaps[%d] is nil", i)
		}
	}

	// Seed the destination graph with the structure of graphs[0]
	// through InitGraph. Mirrors the Java line:
	//   OnHeapHnswGraph graph = InitializedHnswGraphBuilder.initGraph(
	//       graphs[0], ordMaps[0], totalNumberOfVectors, beamWidth,
	//       scorerSupplier);
	dest, err := InitGraph(graphs[0], ordMaps[0], totalNumberOfVectors, beamWidth, scorerSupplier)
	if err != nil {
		return nil, fmt.Errorf("hnsw: merging: seed graph: %w", err)
	}

	// Build the underlying HnswGraphBuilder that writes into the
	// seed graph. M is taken explicitly (Java passes M alongside
	// beamWidth) so the merge path can use a different fan-out
	// from the seed graph if the caller chose to.
	base, err := newHnswGraphBuilderWithGraph(scorerSupplier, m, beamWidth, seed, dest)
	if err != nil {
		return nil, fmt.Errorf("hnsw: merging: builder: %w", err)
	}

	return &MergingHnswGraphBuilder{
		HnswGraphBuilder: base,
		graphs:           graphs,
		ordMaps:          ordMaps,
		initializedNodes: initializedNodes,
	}, nil
}

// Build folds every remaining input graph into the destination
// graph and returns the completed result. Mirrors Java's
// MergingHnswGraphBuilder.build(int maxOrd) verbatim, including
// the post-fold initializedNodes sweep.
//
// The body intentionally duplicates the info-stream prelude rather
// than delegating to the embedded HnswGraphBuilder.Build because
// the latter would call the embedded addVectors, which iterates
// over the entire [0, maxOrd) range. The merge path needs the
// updateGraph pass first and then a sparse sweep over only the
// uninitialised ordinals — both done here.
func (b *MergingHnswGraphBuilder) Build(maxOrd int) (*OnHeapHnswGraph, error) {
	if b.frozen.Load() {
		return nil, errors.New("hnsw: builder is frozen and cannot be updated")
	}
	if b.infoStream.IsEnabled(HnswComponent) {
		b.infoStream.Message(HnswComponent, b.formatBuildMessage(maxOrd))
	}

	// Fold graphs[1:] in. graphs[0] was already imported through
	// the InitGraph seed at construction time.
	for i := 1; i < len(b.graphs); i++ {
		if err := b.updateGraph(b.graphs[i], b.ordMaps[i]); err != nil {
			return nil, fmt.Errorf("hnsw: merging: updateGraph[%d]: %w", i, err)
		}
	}

	// Final sweep: every ordinal not yet initialised must be
	// inserted through the regular AddGraphNode path. This catches
	// new documents that were not part of any input graph.
	//
	// TODO(rmp): optimise to iterate only over unset bits in
	// initializedNodes (Lucene leaves the same TODO in place).
	if b.initializedNodes != nil {
		for node := 0; node < maxOrd; node++ {
			if b.initializedNodes.Get(node) {
				continue
			}
			if err := b.HnswGraphBuilder.AddGraphNode(node); err != nil {
				return nil, fmt.Errorf("hnsw: merging: final add node %d: %w", node, err)
			}
		}
	}

	return b.GetCompletedGraph()
}

// updateGraph merges the smaller graph gS into the current larger
// destination graph. Ordinals in gS are translated through ordMapS
// before being written into the destination.
//
// The pass operates in two phases. First, the join set j is
// inserted through the unmodified AddGraphNode path so the
// inserted nodes receive a full search-and-link treatment. Second,
// for every remaining node u in gS, the entry-point set eps is
// formed by unioning u's neighbours in gS with those neighbours'
// neighbours in the destination graph, and u is inserted through
// AddGraphNodeWithEntryPoints so the level-0 beam is seeded with
// eps and clipped to the smaller beamCandidates0 collector.
//
// Mirrors Java's private MergingHnswGraphBuilder.updateGraph
// verbatim, including the (v < u || j.contains(v)) gate that
// preserves Lucene's incremental ordering: a neighbour v is a
// valid seed for u only when it is already present in the
// destination graph at the time u is inserted.
func (b *MergingHnswGraphBuilder) updateGraph(gS HnswGraph, ordMapS []int) error {
	size := gS.Size()

	j, err := ComputeJoinSet(gS)
	if err != nil {
		return fmt.Errorf("hnsw: merging: compute join set: %w", err)
	}

	// Phase 1: insert join-set nodes through the regular path.
	// Lucene materialises j.toArray() and sorts ascending to keep
	// the insertion order deterministic across runs; the Go port
	// does the same by extracting keys and sorting.
	jNodes := make([]int, 0, len(j))
	for node := range j {
		jNodes = append(jNodes, node)
	}
	sort.Ints(jNodes)
	for _, node := range jNodes {
		if err := b.checkOrdMap(ordMapS, node); err != nil {
			return err
		}
		if err := b.HnswGraphBuilder.AddGraphNode(ordMapS[node]); err != nil {
			return fmt.Errorf("hnsw: merging: add join-set node %d (dst %d): %w",
				node, ordMapS[node], err)
		}
	}

	// Phase 2: for every node u outside j, form eps and insert.
	for u := 0; u < size; u++ {
		if _, inJoin := j[u]; inJoin {
			continue
		}
		eps := make(map[int]struct{})

		if err := gS.SeekLevel(0, u); err != nil {
			return fmt.Errorf("hnsw: merging: seek gS level 0 node %d: %w", u, err)
		}
		for {
			v, err := gS.NextNeighbor()
			if err != nil {
				return fmt.Errorf("hnsw: merging: gS next neighbor for %d: %w", u, err)
			}
			if v == util.NO_MORE_DOCS {
				break
			}
			// Only consider v if it is already part of gL: either v
			// is in the join set, or v < u (the prior iteration of
			// this loop already inserted it). Otherwise v has not
			// yet been added to gL and would not be a valid entry
			// point.
			_, vInJoin := j[v]
			if v >= u && !vInJoin {
				continue
			}
			if err := b.checkOrdMap(ordMapS, v); err != nil {
				return err
			}
			newv := ordMapS[v]
			eps[newv] = struct{}{}

			// Fan out: add newv's neighbours in gL (level 0) to eps.
			// The destination graph's SeekLevel / NextNeighbor pair
			// mirrors the Java hnsw.seek / hnsw.nextNeighbor usage
			// verbatim — operating on the embedded
			// HnswGraphBuilder's destination graph.
			if err := b.hnsw.SeekLevel(0, newv); err != nil {
				return fmt.Errorf("hnsw: merging: seek gL level 0 node %d: %w", newv, err)
			}
			for {
				friend, err := b.hnsw.NextNeighbor()
				if err != nil {
					return fmt.Errorf("hnsw: merging: gL next neighbor for %d: %w", newv, err)
				}
				if friend == util.NO_MORE_DOCS {
					break
				}
				eps[friend] = struct{}{}
			}
		}

		if err := b.checkOrdMap(ordMapS, u); err != nil {
			return err
		}
		if err := b.HnswGraphBuilder.AddGraphNodeWithEntryPoints(ordMapS[u], eps); err != nil {
			return fmt.Errorf("hnsw: merging: add non-join node %d (dst %d): %w",
				u, ordMapS[u], err)
		}
	}
	return nil
}

// checkOrdMap validates that idx is a valid index into ordMapS and
// that the mapped destination ordinal is non-negative. The
// merge pipeline expects every node in the input graphs to map to a
// surviving destination ordinal; -1 entries would only appear if the
// caller mishandled a deletion-aware merge upstream, in which case we
// surface a typed error rather than panic in AddGraphNode.
func (b *MergingHnswGraphBuilder) checkOrdMap(ordMapS []int, idx int) error {
	if idx < 0 || idx >= len(ordMapS) {
		return fmt.Errorf("hnsw: merging: ordinal %d out of ordMap bounds [0,%d)",
			idx, len(ordMapS))
	}
	if ordMapS[idx] < 0 {
		return fmt.Errorf("hnsw: merging: ordinal %d maps to negative destination %d",
			idx, ordMapS[idx])
	}
	return nil
}

// formatBuildMessage builds the info-stream prelude string matching
// Lucene's "build graph from merging N graphs of K vectors, graph
// sizes:S1 S2 …". The trailing space after every size mirrors the
// Java string concatenation verbatim.
func (b *MergingHnswGraphBuilder) formatBuildMessage(maxOrd int) string {
	var sb strings.Builder
	sb.WriteString("build graph from merging ")
	fmt.Fprintf(&sb, "%d graphs of %d vectors, graph sizes:", len(b.graphs), maxOrd)
	for _, g := range b.graphs {
		fmt.Fprintf(&sb, "%d ", g.Size())
	}
	return sb.String()
}

// Compile-time guard: MergingHnswGraphBuilder satisfies HnswBuilder.
var _ HnswBuilder = (*MergingHnswGraphBuilder)(nil)
