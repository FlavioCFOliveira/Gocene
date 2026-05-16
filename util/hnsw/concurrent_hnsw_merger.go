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

	"github.com/FlavioCFOliveira/Gocene/util"
)

// ConcurrentHnswMerger merges multiple HNSW graphs concurrently. It is
// the parallel counterpart of [IncrementalHnswGraphMerger]: same reader
// admission and largest-graph selection rules, same FLOAT32-only stub
// surface, but the final build is dispatched to a
// [HnswConcurrentMergeBuilder] driving numWorker goroutines instead of
// a single-threaded MergingHnswGraphBuilder.
//
// Port of org.apache.lucene.util.hnsw.ConcurrentHnswMerger (Lucene
// 10.4.0). The Java reference extends IncrementalHnswGraphMerger and
// overrides only createBuilder; the Go port uses an embedded
// *IncrementalHnswGraphMerger plus a shadowed [Merge] method that
// dispatches to a local createBuilder. The shadowing is the
// pre-approved override strategy (see InitializedHnswGraphBuilder for
// the original pattern): Go has no virtual dispatch, so the parent's
// Merge -> createBuilder call cannot be intercepted from a derived
// type. Re-declaring Merge on the outer type routes the call to the
// outer createBuilder while leaving every other method (AddReader,
// FieldName, ScorerSupplier, M, BeamWidth) inherited verbatim.
//
// Divergences from the Java reference, all carried over from the
// embedded IncrementalHnswGraphMerger:
//
//   - FieldInfo: replaced by a plain field-name string until the
//     FieldInfo port lands. See [IncrementalHnswGraphMerger] for the
//     rationale.
//   - Vector encoding: FLOAT32 only; byte-encoded vectors are deferred
//     to the codec sprint.
//   - TaskExecutor: the Java reference dispatches workers through
//     org.apache.lucene.search.TaskExecutor. The Go port has no
//     TaskExecutor port; [HnswConcurrentMergeBuilder] manages its own
//     goroutines and the executor parameter is therefore absent.
//
// Thread-safety: a ConcurrentHnswMerger is not safe for concurrent use
// (AddReader mutates state). Merge is single-shot. The underlying
// concurrent build is itself thread-safe — see
// [HnswConcurrentMergeBuilder] for the worker-level invariants.
type ConcurrentHnswMerger struct {
	*IncrementalHnswGraphMerger

	// numWorker is the configured worker count forwarded to every
	// HnswConcurrentMergeBuilder constructed by createBuilder. Must be
	// strictly positive; the constructor rejects zero or negative
	// values, mirroring HnswConcurrentMergeBuilder's own guard.
	numWorker int
}

// NewConcurrentHnswMerger constructs a ConcurrentHnswMerger over the
// given field. Mirrors Java's five-argument constructor
// (fieldInfo, scorerSupplier, M, beamWidth, taskExecutor, numWorker)
// minus the TaskExecutor parameter — see the type doc for the
// rationale.
//
// numWorker fixes the goroutine count used during the concurrent
// build phase. The other parameters are forwarded to the embedded
// IncrementalHnswGraphMerger.
//
// Returns an error when any parameter is invalid (empty field name,
// nil supplier, non-positive m / beamWidth / numWorker).
func NewConcurrentHnswMerger(
	fieldName string,
	scorerSupplier RandomVectorScorerSupplier,
	m, beamWidth, numWorker int,
) (*ConcurrentHnswMerger, error) {
	if numWorker <= 0 {
		return nil, fmt.Errorf(
			"hnsw: NewConcurrentHnswMerger: numWorker must be positive (got %d)", numWorker)
	}
	parent, err := NewIncrementalHnswGraphMerger(fieldName, scorerSupplier, m, beamWidth)
	if err != nil {
		return nil, err
	}
	return &ConcurrentHnswMerger{
		IncrementalHnswGraphMerger: parent,
		numWorker:                  numWorker,
	}, nil
}

// NumWorker returns the configured worker count. Exposed for tests and
// the codec layer.
func (cm *ConcurrentHnswMerger) NumWorker() int { return cm.numWorker }

// AddReader records a reader to merge from. The body delegates to the
// embedded [IncrementalHnswGraphMerger.AddReader] for the actual
// admission logic; the shadow only exists to return the outer
// *ConcurrentHnswMerger instead of the inner pointer so that callers
// chaining `cm.AddReader(...).AddReader(...).Merge(...)` keep
// dispatching to the concurrent Merge below.
//
// Without this shadow, the inherited AddReader returns
// *IncrementalHnswGraphMerger, and a subsequent Merge call on that
// result would run the single-threaded parent merge — a quiet
// performance bug. The Java reference avoids the issue via virtual
// dispatch; Go has none, so the shadow is mandatory.
func (cm *ConcurrentHnswMerger) AddReader(
	reader KnnVectorsReader, docMap DocMap, liveDocs util.Bits,
) (HnswGraphMerger, error) {
	if _, err := cm.IncrementalHnswGraphMerger.AddReader(reader, docMap, liveDocs); err != nil {
		return nil, err
	}
	return cm, nil
}

// Merge produces the merged on-heap graph from the recorded readers
// and the merged vector values view. Shadows the embedded
// [IncrementalHnswGraphMerger.Merge] so the per-instance createBuilder
// resolves to the concurrent variant instead of the single-thread one.
//
// Mirrors Java's IncrementalHnswGraphMerger.merge (inherited by
// ConcurrentHnswMerger) verbatim — the only difference is the
// createBuilder return type.
func (cm *ConcurrentHnswMerger) Merge(
	mergedVectorValues KnnVectorValues, infoStream util.InfoStream, maxOrd int,
) (*OnHeapHnswGraph, error) {
	builder, err := cm.createBuilder(mergedVectorValues, maxOrd)
	if err != nil {
		return nil, err
	}
	if infoStream != nil {
		builder.SetInfoStream(infoStream)
	}
	return builder.Build(maxOrd)
}

// createBuilder constructs the HnswConcurrentMergeBuilder used by
// [ConcurrentHnswMerger.Merge]. Mirrors Java's protected HnswBuilder
// createBuilder(KnnVectorValues, int) override on ConcurrentHnswMerger.
//
// The decision tree:
//
//   - When no reader produced a graph (largestGraphReader is nil),
//     allocate a fresh OnHeapHnswGraph and let the concurrent builder
//     fill it from scratch.
//   - Otherwise read the largest-graph candidate, build the (single)
//     1D ordinal map for it, seed the destination graph via
//     [InitGraph], and let the concurrent builder fold in the
//     remaining ordinals.
//
// Unlike the parent createBuilder, the concurrent variant only seeds
// from a single source graph (the largest). The other readers are
// folded in implicitly by the concurrent build's per-ordinal pass —
// every ordinal not flagged in initializedNodes is inserted from
// scratch using the merged-segment vector values.
func (cm *ConcurrentHnswMerger) createBuilder(
	mergedVectorValues KnnVectorValues, maxOrd int,
) (HnswBuilder, error) {
	var (
		graph            *OnHeapHnswGraph
		initializedNodes util.BitSet
	)

	if cm.largestGraphReader == nil {
		graph = NewOnHeapHnswGraph(cm.m, maxOrd)
	} else {
		initReader := cm.largestGraphReader.reader
		initDocMap := cm.largestGraphReader.initDocMap
		initGraphSize := cm.largestGraphReader.graphSize

		initializerGraph, err := initReader.HnswGraph(cm.fieldName)
		if err != nil {
			return nil, fmt.Errorf(
				"hnsw: concurrent merger: HnswGraph(%q): %w", cm.fieldName, err)
		}

		// Mirrors Java: if (initializerGraph.size() == 0) ... else seed.
		// A nil graph is treated the same as an empty graph; the Java
		// reference would NPE on a nil cast, while the Go interface
		// surfaces it as a clean fallback to the fresh-graph path.
		if initializerGraph == nil || initializerGraph.Size() == 0 {
			graph = NewOnHeapHnswGraph(cm.m, maxOrd)
		} else {
			bs, err := util.NewFixedBitSet(maxOrd)
			if err != nil {
				return nil, fmt.Errorf(
					"hnsw: concurrent merger: initializedNodes bitset: %w", err)
			}
			initializedNodes = bs

			oldToNewOrdinalMap, err := concurrentMergerNewOrdMapping(
				cm.fieldName,
				initReader,
				initDocMap,
				initGraphSize,
				mergedVectorValues,
				initializedNodes,
			)
			if err != nil {
				return nil, fmt.Errorf("hnsw: concurrent merger: ord mapping: %w", err)
			}

			seeded, err := InitGraph(
				initializerGraph,
				oldToNewOrdinalMap,
				maxOrd,
				cm.beamWidth,
				cm.scorerSupplier,
			)
			if err != nil {
				return nil, fmt.Errorf("hnsw: concurrent merger: seed graph: %w", err)
			}
			graph = seeded
		}
	}

	builder, err := NewHnswConcurrentMergeBuilder(
		cm.scorerSupplier,
		cm.numWorker,
		cm.m,
		cm.beamWidth,
		RandSeed,
		graph,
		initializedNodes,
	)
	if err != nil {
		return nil, fmt.Errorf("hnsw: concurrent merger: build worker: %w", err)
	}
	return builder, nil
}

// concurrentMergerNewOrdMapping computes a 1D old-to-new ordinal map
// for the single seed reader. Mirrors Java's private static
// ConcurrentHnswMerger.getNewOrdMapping verbatim, modulo the byte /
// float vector encoding switch (FLOAT32 only — see the type doc).
//
// The algorithm is two-phase:
//
//  1. Walk the seed reader's per-segment vector values and build a
//     (newDocId -> oldOrdinal) map. newDocId is obtained by feeding
//     the per-segment docId through the reader's initDocMap; a
//     newDocId of -1 (deleted in merged segment) is skipped. Track
//     the maximum observed newDocId so phase 2 can stop early.
//  2. Walk the merged vector values iterator. For each (newDocId,
//     newOrdinal) pair, look up newDocId in the phase-1 map; on a
//     hit, record (oldOrdinal -> newOrdinal) in the output map and
//     flag the newOrdinal in initializedNodes. Phase 2 stops once
//     newDocId exceeds maxNewDocID.
//
// When no seed reader vector lives in the merged segment (maxNewDocID
// stays at -1), the function returns an empty slice — the
// initializerGraph carries no usable connectivity and the concurrent
// build will rebuild from scratch.
func concurrentMergerNewOrdMapping(
	fieldName string,
	initReader KnnVectorsReader,
	initDocMap DocMap,
	initGraphSize int,
	mergedVectorValues KnnVectorValues,
	initializedNodes util.BitSet,
) ([]int, error) {
	if initializedNodes == nil {
		return nil, errors.New(
			"hnsw: concurrentMergerNewOrdMapping: initializedNodes must not be nil")
	}
	if mergedVectorValues == nil {
		return nil, errors.New(
			"hnsw: concurrentMergerNewOrdMapping: mergedVectorValues must not be nil")
	}

	values, err := initReader.GetFloatVectorValues(fieldName)
	if err != nil {
		return nil, fmt.Errorf(
			"hnsw: concurrentMergerNewOrdMapping: GetFloatVectorValues(%q): %w",
			fieldName, err)
	}
	if values == nil {
		return nil, fmt.Errorf(
			"hnsw: concurrentMergerNewOrdMapping: nil vector values for field %q",
			fieldName)
	}

	// Pre-size to graphSize: every source ordinal will be inserted into
	// the map exactly once. Mirrors Java's IntIntHashMap(initGraphSize)
	// pre-allocation.
	newIDToOldOrdinal := make(map[int]int, initGraphSize)
	initIter := values.Iterator()
	maxNewDocID := -1
	for {
		docID, err := initIter.NextDoc()
		if err != nil {
			return nil, fmt.Errorf(
				"hnsw: concurrentMergerNewOrdMapping: init iter: %w", err)
		}
		if docID == util.NO_MORE_DOCS {
			break
		}
		var newID int
		if initDocMap != nil {
			newID = initDocMap.Get(docID)
		} else {
			newID = docID
		}
		if newID == -1 {
			continue
		}
		if newID > maxNewDocID {
			maxNewDocID = newID
		}
		// Java has an assert that newIdToOldOrdinal.containsKey(newId)
		// == false. The Go port keeps the same invariant implicitly:
		// duplicate keys would overwrite. Surface the violation as an
		// explicit error rather than a silent overwrite — the Java
		// assertion fires on the same path under -ea.
		if _, dup := newIDToOldOrdinal[newID]; dup {
			return nil, fmt.Errorf(
				"hnsw: concurrentMergerNewOrdMapping: duplicate newDocId %d in seed reader",
				newID)
		}
		newIDToOldOrdinal[newID] = initIter.Index()
	}

	if maxNewDocID == -1 {
		return []int{}, nil
	}

	oldToNewOrdinalMap := make([]int, initGraphSize)
	for i := range oldToNewOrdinalMap {
		oldToNewOrdinalMap[i] = -1
	}

	mergedIter := mergedVectorValues.Iterator()
	for {
		docID, err := mergedIter.NextDoc()
		if err != nil {
			return nil, fmt.Errorf(
				"hnsw: concurrentMergerNewOrdMapping: merged iter: %w", err)
		}
		// Mirrors Java's `newDocId <= maxNewDocID` upper-bound check;
		// NO_MORE_DOCS is INT32_MAX so the inequality short-circuits the
		// iteration cleanly when the merged iterator exhausts before
		// maxNewDocID is reached.
		if docID > maxNewDocID {
			break
		}
		oldOrd, ok := newIDToOldOrdinal[docID]
		if !ok {
			continue
		}
		newOrd := mergedIter.Index()
		initializedNodes.Set(newOrd)
		oldToNewOrdinalMap[oldOrd] = newOrd
	}

	return oldToNewOrdinalMap, nil
}

// Compile-time guard: ConcurrentHnswMerger satisfies HnswGraphMerger
// through the embedded IncrementalHnswGraphMerger plus the shadowed
// Merge above. The guard catches a future refactor that accidentally
// breaks the interface contract.
var _ HnswGraphMerger = (*ConcurrentHnswMerger)(nil)
