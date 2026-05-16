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

	"github.com/FlavioCFOliveira/Gocene/util"
)

// deletePctThreshold is the maximum acceptable deletion percentage
// for a graph to be considered as the base graph during the
// incremental merge. Graphs with deletion percentages above this
// threshold are not used for initialisation as they may have
// degraded connectivity. Mirrors Lucene's DELETE_PCT_THRESHOLD
// (instance field on IncrementalHnswGraphMerger, effectively a
// per-class constant).
//
// A value of 40 means that if more than 40% of the graph's original
// vectors have been deleted, the graph will not be selected as the
// base.
const deletePctThreshold = 40

// IncrementalHnswGraphMerger merges multiple HNSW graphs in a single
// thread by reusing the connectivity of the largest input graph as
// the base for the merged segment and incrementally folding the
// remaining vectors in. Port of
// org.apache.lucene.util.hnsw.IncrementalHnswGraphMerger (Lucene
// 10.4.0).
//
// Two divergences from the Java reference are intentional:
//
//   - FieldInfo: the Java type carries the field name plus the
//     vector encoding (BYTE / FLOAT32). The Gocene port has not yet
//     received its FieldInfo port, so the merger takes the field
//     name as a string and assumes FLOAT32 encoding through
//     [KnnVectorsReader.GetFloatVectorValues]. Re-introducing the
//     byte-encoding switch is deferred to the codec sprint together
//     with the FieldInfo port.
//   - PerFieldKnnVectorsFormat.FieldsReader unwrapping: the Java
//     reference probes the reader for the per-field wrapper via
//     instanceof and pulls the underlying field reader. Gocene's
//     [KnnVectorsReader] stub already takes the field name as a
//     parameter, so the unwrapping is the reader's own
//     responsibility. The merger does not attempt to peek inside.
//
// Thread-safety: an IncrementalHnswGraphMerger is not safe for
// concurrent use. AddReader mutates the receiver's reader list.
// Merge is a single-shot operation.
type IncrementalHnswGraphMerger struct {
	// fieldName is the name of the field being merged. Java carries
	// this inside the FieldInfo carrier; the Gocene port takes it as
	// a plain string until the FieldInfo type lands.
	fieldName string

	// scorerSupplier supplies the in-place vector scorer used by the
	// underlying HnswBuilder. Mirrors Java's field of the same name.
	scorerSupplier RandomVectorScorerSupplier

	// m is the max number of connections per node on upper levels.
	// Mirrors Java's M.
	m int

	// beamWidth is the candidate queue size used while searching
	// during graph construction. Mirrors Java's beamWidth.
	beamWidth int

	// graphReaders holds the readers whose source graphs have zero
	// deletions and are therefore safe to fold into the merge. The
	// list is ordered in AddReader call order and re-ordered into
	// descending size by createBuilder. Mirrors Java's
	// protected List<GraphReader> graphReaders.
	graphReaders []*graphReader

	// largestGraphReader tracks the best candidate seen so far for
	// the base graph: the graph with the largest live-vector count
	// whose deletion percentage is at or under deletePctThreshold.
	// The pointer may reference an entry that is also in
	// graphReaders (when the candidate had zero deletes) or one that
	// is not (when the candidate had some deletes but stayed under
	// the threshold). Mirrors Java's protected GraphReader
	// largestGraphReader.
	largestGraphReader *graphReader

	// numReaders is the total number of AddReader calls observed,
	// counting readers that produced no graph and readers that
	// produced one. The createBuilder bitset gating depends on
	// (graphReaders.size() == numReaders). Mirrors Java's private
	// int numReaders.
	numReaders int
}

// graphReader is the per-reader carrier used during the merge. It
// captures the upstream reader, the docMap that translates per-
// segment doc ids into merged doc ids, and the source graph size
// (live vector count is not retained — only graphSize for the
// largest-graph comparison and the ordinal map sizing). Mirrors
// Java's protected record GraphReader(KnnVectorsReader, DocMap, int).
type graphReader struct {
	reader     KnnVectorsReader
	initDocMap DocMap
	graphSize  int
}

// NewIncrementalHnswGraphMerger constructs an IncrementalHnswGraphMerger
// for the given field. Mirrors Java's two-argument constructor (the
// Java reference also takes a FieldInfo; the Gocene port substitutes
// the field name).
//
// scorerSupplier supplies the underlying vector scorer; the merger
// retains the supplier and forwards it to the constructed
// HnswBuilder. m and beamWidth tune the destination graph's
// hyper-parameters.
//
// Returns an error when any parameter is invalid (empty field name,
// nil supplier, non-positive m or beamWidth).
func NewIncrementalHnswGraphMerger(
	fieldName string,
	scorerSupplier RandomVectorScorerSupplier,
	m, beamWidth int,
) (*IncrementalHnswGraphMerger, error) {
	if fieldName == "" {
		return nil, errors.New("hnsw: NewIncrementalHnswGraphMerger: fieldName must not be empty")
	}
	if scorerSupplier == nil {
		return nil, errors.New("hnsw: NewIncrementalHnswGraphMerger: scorerSupplier must not be nil")
	}
	if m <= 0 {
		return nil, errors.New("hnsw: NewIncrementalHnswGraphMerger: m must be positive")
	}
	if beamWidth <= 0 {
		return nil, errors.New("hnsw: NewIncrementalHnswGraphMerger: beamWidth must be positive")
	}
	return &IncrementalHnswGraphMerger{
		fieldName:      fieldName,
		scorerSupplier: scorerSupplier,
		m:              m,
		beamWidth:      beamWidth,
	}, nil
}

// FieldName returns the field name carried by this merger. It is
// exposed for tests and the codec layer; subclasses (the Java
// equivalent of) read fieldInfo.name in several places.
func (im *IncrementalHnswGraphMerger) FieldName() string { return im.fieldName }

// ScorerSupplier returns the scorer supplier carried by this
// merger. Used by subclasses (the Java pattern) and exposed here for
// tests and for the codec layer.
func (im *IncrementalHnswGraphMerger) ScorerSupplier() RandomVectorScorerSupplier {
	return im.scorerSupplier
}

// M returns the configured max-connections hyper-parameter.
func (im *IncrementalHnswGraphMerger) M() int { return im.m }

// BeamWidth returns the configured beam-width hyper-parameter.
func (im *IncrementalHnswGraphMerger) BeamWidth() int { return im.beamWidth }

// AddReader records a reader to merge from. Mirrors Java's
// AddReader verbatim, modulo the byte / float vector encoding
// switch (FLOAT32 only — see the type doc for the rationale).
//
// A reader is admitted as a graph reader only when:
//
//   - It can produce a non-empty HnswGraph for the field
//     (the [KnnVectorsReader.HnswGraph] cast equivalent).
//   - Its live-vector count divided by graph size yields a deletion
//     percentage at or under [deletePctThreshold].
//   - It has zero deletions (i.e. live-vector count equals graph
//     size). Readers with some deletions may still be tracked as
//     largestGraphReader candidates but are not added to the
//     graphReaders list.
//
// Returns the receiver, mirroring Java's "return this" chaining
// idiom that the HnswGraphMerger interface declares.
func (im *IncrementalHnswGraphMerger) AddReader(
	reader KnnVectorsReader, docMap DocMap, liveDocs util.Bits,
) (HnswGraphMerger, error) {
	im.numReaders++

	if reader == nil {
		return im, nil
	}

	graph, err := reader.HnswGraph(im.fieldName)
	if err != nil {
		return nil, fmt.Errorf("hnsw: incremental: HnswGraph(%q): %w", im.fieldName, err)
	}
	// Java: if (!(reader instanceof HnswGraphProvider)) return this;
	// followed by: if (graph == null || graph.size() == 0) return this;
	// Both checks collapse to "graph absent or empty" in the Gocene
	// stub interface — a reader that cannot produce a graph returns
	// nil here.
	if graph == nil || graph.Size() == 0 {
		return im, nil
	}

	// FLOAT32-encoding only for now. See the type doc for the
	// byte-encoding TODO.
	knnVectorValues, err := reader.GetFloatVectorValues(im.fieldName)
	if err != nil {
		return nil, fmt.Errorf("hnsw: incremental: GetFloatVectorValues(%q): %w",
			im.fieldName, err)
	}
	if knnVectorValues == nil {
		// No vector values means we cannot count live vectors. Java
		// would NPE here, which signals a contract violation; the
		// Go port surfaces it as an explicit error.
		return nil, fmt.Errorf("hnsw: incremental: nil vector values for field %q",
			im.fieldName)
	}

	candidateVectorCount, err := countLiveVectors(liveDocs, knnVectorValues)
	if err != nil {
		return nil, fmt.Errorf("hnsw: incremental: countLiveVectors: %w", err)
	}
	graphSize := graph.Size()

	gr := &graphReader{
		reader:     reader,
		initDocMap: docMap,
		graphSize:  graphSize,
	}

	// deletePct rounded down toward zero, matching Java's integer
	// division semantics. graphSize is non-zero (checked above) so
	// the division is safe.
	deletePct := ((graphSize - candidateVectorCount) * 100) / graphSize

	if deletePct <= deletePctThreshold &&
		(im.largestGraphReader == nil ||
			candidateVectorCount > im.largestGraphReader.graphSize) {
		im.largestGraphReader = gr
	}

	// Only zero-delete graphs join graphReaders. A graph with some
	// deletions may still be the largest seed but cannot be folded
	// in via the standard merge path (its ordinals would need
	// remapping that the current Gocene port does not yet support).
	if candidateVectorCount == graphSize {
		im.graphReaders = append(im.graphReaders, gr)
	}

	return im, nil
}

// Merge produces the merged on-heap graph from the recorded readers
// and the merged vector values view. Mirrors Java's three-argument
// merge(KnnVectorValues, InfoStream, int).
func (im *IncrementalHnswGraphMerger) Merge(
	mergedVectorValues KnnVectorValues, infoStream util.InfoStream, maxOrd int,
) (*OnHeapHnswGraph, error) {
	builder, err := im.createBuilder(mergedVectorValues, maxOrd)
	if err != nil {
		return nil, err
	}
	if infoStream != nil {
		builder.SetInfoStream(infoStream)
	}
	return builder.Build(maxOrd)
}

// createBuilder constructs the HnswBuilder used by Merge. Mirrors
// Java's protected HnswBuilder createBuilder(KnnVectorValues, int).
//
// The decision tree:
//
//   - When no reader produced a graph, fall through to a plain
//     [HnswGraphBuilder] that builds from scratch.
//   - Otherwise sort the recorded graphs by descending size
//     (with the largest-graph seed inserted at index 0 when it was
//     not already in the list), compute the new-to-old ordinal map
//     for each reader, and hand the result to
//     [MergingHnswGraphBuilder] for the incremental fold-in.
//
// The bitset gating mirrors Java verbatim: when graphReaders covers
// every reader the caller registered (i.e. no reader was rejected
// for having deletions), the initializedNodes bitset is omitted —
// the resulting MergingHnswGraphBuilder assumes every ordinal is
// covered by one of the input graphs. When at least one reader was
// rejected, the bitset is allocated and populated by
// [IncrementalHnswGraphMerger.getNewOrdMapping] so the final sweep
// catches the gap.
func (im *IncrementalHnswGraphMerger) createBuilder(
	mergedVectorValues KnnVectorValues, maxOrd int,
) (HnswBuilder, error) {
	if im.largestGraphReader == nil {
		return NewHnswGraphBuilder(
			im.scorerSupplier, im.m, im.beamWidth, RandSeed,
		)
	}

	// Java:
	//   if (!graphReaders.contains(largestGraphReader)) {
	//     graphReaders.addFirst(largestGraphReader);
	//   } else {
	//     graphReaders.sort(Comparator.comparingInt(GraphReader::graphSize).reversed());
	//   }
	// The else branch sorts on graphSize descending. The if branch
	// is the rescue path for a largest reader that itself had some
	// deletions — it was not eligible for the graphReaders list at
	// AddReader time, but we still want it at the head of the merge
	// queue here. The Go port mirrors both arms verbatim.
	if !containsGraphReader(im.graphReaders, im.largestGraphReader) {
		im.graphReaders = prependGraphReader(im.graphReaders, im.largestGraphReader)
	} else {
		sort.SliceStable(im.graphReaders, func(i, j int) bool {
			return im.graphReaders[i].graphSize > im.graphReaders[j].graphSize
		})
	}

	var initializedNodes util.BitSet
	if len(im.graphReaders) != im.numReaders {
		bs, err := util.NewFixedBitSet(maxOrd)
		if err != nil {
			return nil, fmt.Errorf("hnsw: incremental: initializedNodes bitset: %w", err)
		}
		initializedNodes = bs
	}

	ordMaps, err := im.getNewOrdMapping(mergedVectorValues, initializedNodes)
	if err != nil {
		return nil, fmt.Errorf("hnsw: incremental: ord mapping: %w", err)
	}

	graphs := make([]HnswGraph, len(im.graphReaders))
	for i, gr := range im.graphReaders {
		g, err := gr.reader.HnswGraph(im.fieldName)
		if err != nil {
			return nil, fmt.Errorf("hnsw: incremental: HnswGraph[%d]: %w", i, err)
		}
		if g == nil || g.Size() == 0 {
			return nil, fmt.Errorf("hnsw: incremental: graph[%d] should not be empty", i)
		}
		graphs[i] = g
	}

	return NewMergingHnswGraphBuilderFromGraphs(
		im.scorerSupplier,
		im.m,
		im.beamWidth,
		RandSeed,
		graphs,
		ordMaps,
		maxOrd,
		initializedNodes,
	)
}

// getNewOrdMapping computes, for each recorded graph, the
// translation from a source ordinal to its merged-segment ordinal.
// Mirrors Java's protected final int[][] getNewOrdMapping verbatim,
// modulo the byte / float vector encoding switch (FLOAT32 only).
//
// The algorithm is two-phase:
//
//  1. For every source reader, walk the per-segment vector values
//     iterator and build a (newDocId -> oldOrdinal) map. newDocId
//     is obtained by feeding the per-segment docId through the
//     reader's initDocMap.
//  2. Walk the merged vector values iterator. For each (newDocId,
//     newOrdinal) pair, locate the source map that contains
//     newDocId; the corresponding oldOrdinal then maps to
//     newOrdinal in that reader's column of the output. When
//     initializedNodes is non-nil, the newOrdinal is also flagged
//     so the downstream MergingHnswGraphBuilder knows which
//     ordinals were covered.
//
// The first match in iteration order wins (Java's `break`); the
// graphReaders sorted-by-descending-size order ensures the largest
// graph claims each shared docId.
func (im *IncrementalHnswGraphMerger) getNewOrdMapping(
	mergedVectorValues KnnVectorValues, initializedNodes util.BitSet,
) ([][]int, error) {
	numGraphs := len(im.graphReaders)
	newDocIdToOldOrdinals := make([]map[int]int, numGraphs)
	oldToNewOrdinalMap := make([][]int, numGraphs)

	for i, gr := range im.graphReaders {
		values, err := gr.reader.GetFloatVectorValues(im.fieldName)
		if err != nil {
			return nil, fmt.Errorf("hnsw: incremental: getNewOrdMapping graph %d: %w", i, err)
		}
		if values == nil {
			return nil, fmt.Errorf("hnsw: incremental: getNewOrdMapping graph %d: nil values", i)
		}
		it := values.Iterator()
		// Pre-size the map to graphSize: every source ordinal will
		// be inserted into the map exactly once. Java passes
		// graphReaders.get(i).graphSize to IntIntHashMap.
		newDocIdToOldOrdinals[i] = make(map[int]int, gr.graphSize)
		docMap := gr.initDocMap

		for {
			docId, err := it.NextDoc()
			if err != nil {
				return nil, fmt.Errorf("hnsw: incremental: source iter graph %d: %w", i, err)
			}
			if docId == util.NO_MORE_DOCS {
				break
			}
			var newDocId int
			if docMap != nil {
				newDocId = docMap.Get(docId)
			} else {
				newDocId = docId
			}
			newDocIdToOldOrdinals[i][newDocId] = it.Index()
		}

		oldToNewOrdinalMap[i] = make([]int, gr.graphSize)
		for j := range oldToNewOrdinalMap[i] {
			oldToNewOrdinalMap[i][j] = -1
		}
	}

	mergedIter := mergedVectorValues.Iterator()
	for {
		docId, err := mergedIter.NextDoc()
		if err != nil {
			return nil, fmt.Errorf("hnsw: incremental: merged iter: %w", err)
		}
		if docId == util.NO_MORE_DOCS {
			break
		}
		newOrd := mergedIter.Index()
		for i := 0; i < numGraphs; i++ {
			oldOrd, ok := newDocIdToOldOrdinals[i][docId]
			if !ok {
				continue
			}
			oldToNewOrdinalMap[i][oldOrd] = newOrd
			if initializedNodes != nil {
				initializedNodes.Set(newOrd)
			}
			break
		}
	}
	return oldToNewOrdinalMap, nil
}

// countLiveVectors returns the number of live vectors in the given
// KnnVectorValues view. When liveDocs is nil every vector counts;
// otherwise the iterator is walked and each docId is consulted
// against the supplied bits. Mirrors Java's static private
// countLiveVectors helper verbatim.
func countLiveVectors(liveDocs util.Bits, knnVectorValues KnnVectorValues) (int, error) {
	if liveDocs == nil {
		return knnVectorValues.Size(), nil
	}
	it := knnVectorValues.Iterator()
	count := 0
	for {
		doc, err := it.NextDoc()
		if err != nil {
			return 0, fmt.Errorf("hnsw: incremental: countLiveVectors next: %w", err)
		}
		if doc == util.NO_MORE_DOCS {
			break
		}
		if doc >= 0 && doc < liveDocs.Length() && liveDocs.Get(doc) {
			count++
		}
	}
	return count, nil
}

// containsGraphReader reports whether s contains target as an exact
// pointer match. The list is small (one entry per source segment)
// so a linear scan dominates over the cost of a map lookup.
func containsGraphReader(s []*graphReader, target *graphReader) bool {
	for _, gr := range s {
		if gr == target {
			return true
		}
	}
	return false
}

// prependGraphReader returns a new slice with target inserted at
// index 0. The Java side uses ArrayList.addFirst which mutates the
// list in place; the Go port returns a fresh header to keep the
// assignment site explicit. The single allocation is acceptable
// here — prependGraphReader runs at most once per Merge.
func prependGraphReader(s []*graphReader, target *graphReader) []*graphReader {
	out := make([]*graphReader, 0, len(s)+1)
	out = append(out, target)
	out = append(out, s...)
	return out
}

// Compile-time guard: IncrementalHnswGraphMerger satisfies
// HnswGraphMerger.
var _ HnswGraphMerger = (*IncrementalHnswGraphMerger)(nil)
