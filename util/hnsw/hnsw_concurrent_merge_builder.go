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
	"sync"
	"sync/atomic"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// hnswConcurrentMergeDefaultBatchSize is the number of vectors a worker
// claims and processes sequentially per batch. Mirrors the
// DEFAULT_BATCH_SIZE = 2048 constant in the Java reference.
const hnswConcurrentMergeDefaultBatchSize = 2048

// HnswConcurrentMergeBuilder is a parallel-graph builder used during
// segment merging. Multiple worker goroutines each drive their own
// underlying [HnswGraphBuilder], but they all write into the same
// [OnHeapHnswGraph] and coordinate through a single [HnswLock] and a
// shared atomic work cursor.
//
// Port of org.apache.lucene.util.hnsw.HnswConcurrentMergeBuilder
// (Lucene 10.4.0). The Java reference dispatches workers through
// org.apache.lucene.search.TaskExecutor; the Go port uses standard-
// library goroutines coordinated by a sync.WaitGroup, a buffered chan
// struct{} bounded semaphore of capacity numWorker, and an error
// channel.
//
// Thread-safety contract:
//   - One concurrent merge builder may be used to drive a single Build
//     call. Build must not be invoked twice on the same receiver — the
//     second invocation returns an error, mirroring Java's
//     IllegalStateException("graph has already been built").
//   - The receiver is otherwise safe for concurrent use across worker
//     goroutines because every shared piece of mutable state is either
//     atomic (workProgress, frozen) or protected by the striped
//     HnswLock.
//   - The single-instance HnswGraphBuilder underneath each worker is
//     NOT safe for concurrent use; the workers respect that contract
//     because they each own one and there is a 1:1 mapping between
//     workers and worker builders.
//
// Determinism caveat (mirrors Lucene's documented behaviour): even with
// a fixed seed the concurrent build may produce different graphs across
// runs, because goroutine scheduling determines which worker claims
// which batch of node ordinals. Java has the same caveat — the
// SplittableRandom seed reproduces per-worker level sequences, but the
// inter-worker interleaving is not deterministic. Callers that require
// byte-for-byte reproducible output must use numWorker == 1 or fall
// back to [MergingHnswGraphBuilder].
type HnswConcurrentMergeBuilder struct {
	// workers holds the per-goroutine builders. Index i corresponds to
	// worker i; workers[0] is privileged in that it owns the graph
	// finish step (see GetCompletedGraph) — mirrors the Java reference's
	// finish() implementation.
	workers []*hnswConcurrentMergeWorker

	// hnswLock is the striped read/write lock shared by every worker.
	// Workers acquire writes around neighbour-array mutations and reads
	// around graphSeek copies — see addDiverseNeighbors in
	// hnsw_graph_builder.go and the seek/next policy functions in
	// installMergeSearcherPolicies below.
	hnswLock *HnswLock

	// numWorker is the configured worker count. The semaphore inside
	// Build is sized from this value so callers can rely on never seeing
	// more than numWorker goroutines running concurrently.
	numWorker int

	// infoStream is the diagnostic sink for HNSW build messages.
	// SetInfoStream propagates it to every underlying worker builder so
	// merge-time messages are attributed correctly.
	infoStream util.InfoStream

	// frozen flips to true on the first Build / GetCompletedGraph; a
	// second Build attempt returns an error. Atomic because Build /
	// GetCompletedGraph can be called from different goroutines.
	frozen atomic.Bool
}

// NewHnswConcurrentMergeBuilder constructs a concurrent merge builder
// that drives numWorker goroutines, each writing into hnsw.
//
// Parameters mirror the Java constructor:
//   - scorerSupplier supplies the vector scorer; numWorker copies are
//     made through scorerSupplier.Copy() so each worker has an
//     independent scorer (the Java reference takes scorerSupplier.copy()
//     per worker for the same reason).
//   - m, beamWidth, seed are the standard HNSW hyper-parameters,
//     forwarded to each worker's underlying HnswGraphBuilder. The seed
//     is shared across workers — Java uses HnswGraphBuilder.randSeed
//     verbatim.
//   - hnsw is the destination graph; it must be writeable and must
//     have at least as many slots as the maximum ordinal Build will
//     receive.
//   - initializedNodes is an optional bitset of ordinals that have
//     already been initialised (e.g. via [InitGraph]). Workers skip any
//     ordinal whose bit is set; pass nil when every ordinal is fresh.
//
// The constructor returns an error if numWorker <= 0, if any supplier
// copy fails, or if the underlying HnswGraphBuilder constructor
// rejects the parameters (M <= 0, beamWidth <= 0, etc.).
func NewHnswConcurrentMergeBuilder(
	scorerSupplier RandomVectorScorerSupplier,
	numWorker, m, beamWidth int,
	seed int64,
	hnsw *OnHeapHnswGraph,
	initializedNodes util.BitSet,
) (*HnswConcurrentMergeBuilder, error) {
	if scorerSupplier == nil {
		return nil, errors.New("hnsw: NewHnswConcurrentMergeBuilder: scorer supplier must not be nil")
	}
	if hnsw == nil {
		return nil, errors.New("hnsw: NewHnswConcurrentMergeBuilder: graph must not be nil")
	}
	if numWorker <= 0 {
		return nil, fmt.Errorf("hnsw: NewHnswConcurrentMergeBuilder: numWorker must be > 0 (got %d)", numWorker)
	}

	lock := NewHnswLock()
	workers := make([]*hnswConcurrentMergeWorker, numWorker)
	progress := new(atomic.Int64)

	for i := 0; i < numWorker; i++ {
		workerSupplier, err := scorerSupplier.Copy()
		if err != nil {
			return nil, fmt.Errorf("hnsw: NewHnswConcurrentMergeBuilder: supplier.Copy worker %d: %w", i, err)
		}
		w, err := newHnswConcurrentMergeWorker(
			workerSupplier, m, beamWidth, seed, hnsw, lock, initializedNodes, progress,
		)
		if err != nil {
			return nil, fmt.Errorf("hnsw: NewHnswConcurrentMergeBuilder: worker %d: %w", i, err)
		}
		workers[i] = w
	}

	return &HnswConcurrentMergeBuilder{
		workers:    workers,
		hnswLock:   lock,
		numWorker:  numWorker,
		infoStream: util.DefaultInfoStream(),
	}, nil
}

// Build runs every worker concurrently until each one observes the
// shared atomic cursor catch up with maxOrd, then returns the completed
// graph. Mirrors Java's OnHeapHnswGraph build(int).
//
// Build is single-shot: a second invocation on the same receiver
// returns an error, matching Java's IllegalStateException semantics.
//
// Concurrency: spawns exactly numWorker goroutines. The buffered
// semaphore caps the in-flight goroutine count, which is the same as
// numWorker by construction; the semaphore is kept regardless because
// it makes the contract explicit and because future variants (e.g.
// dynamic worker pools) can grow without changing the surrounding
// loop. The buffered error channel preserves the first error and
// signals every worker to drain its remaining batches without
// re-entering the slow path.
func (b *HnswConcurrentMergeBuilder) Build(maxOrd int) (*OnHeapHnswGraph, error) {
	if b.frozen.Load() {
		return nil, errors.New("hnsw: HnswConcurrentMergeBuilder is frozen and cannot be updated")
	}
	if b.infoStream.IsEnabled(HnswComponent) {
		b.infoStream.Message(HnswComponent,
			fmt.Sprintf("build graph from %d vectors, with %d workers", maxOrd, b.numWorker))
	}

	// errCh holds the first error seen by any worker; subsequent errors
	// drop on the floor because the channel is size 1. After a worker
	// hits an error it stops claiming new batches via the shared cancel
	// flag (workProgress is set high enough to skip the maxOrd guard).
	errCh := make(chan error, 1)
	semaphore := make(chan struct{}, b.numWorker)
	var wg sync.WaitGroup
	var cancelled atomic.Bool

	for i := 0; i < b.numWorker; i++ {
		semaphore <- struct{}{}
		wg.Add(1)
		w := b.workers[i]
		go func() {
			defer wg.Done()
			defer func() { <-semaphore }()
			if err := w.run(maxOrd, &cancelled); err != nil {
				cancelled.Store(true)
				select {
				case errCh <- err:
				default:
					// Another worker beat us to the punch; drop.
				}
			}
		}()
	}
	wg.Wait()
	close(errCh)
	if err, ok := <-errCh; ok && err != nil {
		return nil, err
	}

	return b.GetCompletedGraph()
}

// AddGraphNode is unsupported on the concurrent merge builder: there is
// no single-thread insertion entry point. Mirrors Java's
// UnsupportedOperationException("This builder is for merge only").
func (b *HnswConcurrentMergeBuilder) AddGraphNode(_ int) error {
	return errors.New("hnsw: HnswConcurrentMergeBuilder is for merge only; AddGraphNode is unsupported")
}

// AddGraphNodeWithEntryPoints is unsupported on the concurrent merge
// builder. Mirrors Java verbatim.
func (b *HnswConcurrentMergeBuilder) AddGraphNodeWithEntryPoints(_ int, _ map[int]struct{}) error {
	return errors.New("hnsw: HnswConcurrentMergeBuilder is for merge only; AddGraphNodeWithEntryPoints is unsupported")
}

// SetInfoStream installs the diagnostic sink on the receiver and on
// every underlying worker so per-worker progress messages are routed
// through the same stream.
func (b *HnswConcurrentMergeBuilder) SetInfoStream(stream util.InfoStream) {
	if stream == nil {
		stream = util.DefaultInfoStream()
	}
	b.infoStream = stream
	for _, w := range b.workers {
		w.SetInfoStream(stream)
	}
}

// GetCompletedGraph returns the finished graph, freezing the builder if
// not already frozen. Calling Build implies a call to
// GetCompletedGraph at the end; calling GetCompletedGraph again is
// safe and idempotent — it returns the same graph.
//
// Mirrors Java's OnHeapHnswGraph getCompletedGraph().
func (b *HnswConcurrentMergeBuilder) GetCompletedGraph() (*OnHeapHnswGraph, error) {
	if !b.frozen.Load() {
		// Java only calls finish() through workers[0] — every worker's
		// finish() is a no-op except the entry-point promotion check on
		// the first one. The Go port preserves the asymmetry.
		if err := b.workers[0].finish(); err != nil {
			return nil, err
		}
		b.frozen.Store(true)
	}
	return b.GetGraph(), nil
}

// GetGraph returns the in-progress graph the workers share. All workers
// write into the same OnHeapHnswGraph by design, so workers[0].GetGraph
// is authoritative.
func (b *HnswConcurrentMergeBuilder) GetGraph() *OnHeapHnswGraph {
	return b.workers[0].GetGraph()
}

// SetBatchSize overrides the per-worker batch size on every worker. The
// Java reference exposes this as a package-private setter intended for
// tests; the Go port exposes the same hook, capitalised to be callable
// from package-external tests if they ever live there. The default is
// hnswConcurrentMergeDefaultBatchSize (2048).
func (b *HnswConcurrentMergeBuilder) SetBatchSize(newSize int) {
	if newSize <= 0 {
		return
	}
	for _, w := range b.workers {
		w.batchSize = newSize
	}
}

// Lock returns the striped HnswLock shared by every worker. Exposed for
// tests that want to assert lock acquisition behaviour without going
// through the full Build path.
func (b *HnswConcurrentMergeBuilder) Lock() *HnswLock {
	return b.hnswLock
}

// Compile-time guard.
var _ HnswBuilder = (*HnswConcurrentMergeBuilder)(nil)

// hnswConcurrentMergeWorker is the inner builder driving one worker
// goroutine. It embeds *HnswGraphBuilder and adds the work-claim cursor,
// the optional initializedNodes skip set, and the per-worker batch
// size. Mirrors the package-private ConcurrentMergeWorker class in the
// Java reference.
type hnswConcurrentMergeWorker struct {
	*HnswGraphBuilder

	// workProgress is the shared atomic cursor every worker pulls from
	// to claim its next batch. It points at the lowest ordinal not yet
	// claimed; getAndAdd returns the previous value (the batch start).
	workProgress *atomic.Int64

	// initializedNodes is the optional skip set. When set, ordinals
	// whose bit is true short-circuit AddGraphNode — they are already
	// in the graph and should not be re-inserted. Mirrors Java's
	// BitSet initializedNodes.
	initializedNodes util.BitSet

	// batchSize is the per-worker batch size. The default is
	// hnswConcurrentMergeDefaultBatchSize; tests can override via
	// HnswConcurrentMergeBuilder.SetBatchSize.
	batchSize int
}

// newHnswConcurrentMergeWorker constructs one worker. It builds the
// underlying HnswGraphBuilder with the shared HnswLock and installs the
// merge-aware seek/next policies on its searcher — both are required so
// the worker can safely traverse a graph that other workers are mutating
// concurrently.
func newHnswConcurrentMergeWorker(
	scorerSupplier RandomVectorScorerSupplier,
	m, beamWidth int,
	seed int64,
	hnsw *OnHeapHnswGraph,
	lock *HnswLock,
	initializedNodes util.BitSet,
	progress *atomic.Int64,
) (*hnswConcurrentMergeWorker, error) {
	// Build the searcher first. We need to wire the merge-aware seek /
	// next policies *after* the HnswGraphBuilder constructor builds the
	// searcher's scratch state — easiest done by constructing the
	// searcher here and passing it through newHnswGraphBuilderWithLock.
	bitsetSize := hnsw.Size()
	if bitsetSize < 1 {
		bitsetSize = 1
	}
	visited, err := util.NewFixedBitSet(bitsetSize)
	if err != nil {
		return nil, fmt.Errorf("hnsw: worker visited bitset: %w", err)
	}
	searcher := NewHnswGraphSearcher(NewNeighborQueue(beamWidth, true), visited)
	installMergeSearcherPolicies(searcher, lock)

	builder, err := newHnswGraphBuilderWithLock(
		scorerSupplier, m, beamWidth, seed, hnsw, lock, searcher,
	)
	if err != nil {
		return nil, err
	}

	return &hnswConcurrentMergeWorker{
		HnswGraphBuilder: builder,
		workProgress:     progress,
		initializedNodes: initializedNodes,
		batchSize:        hnswConcurrentMergeDefaultBatchSize,
	}, nil
}

// run claims batches of ordinals from the shared cursor and inserts
// each ordinal in [start, end) through the underlying HnswGraphBuilder.
// It returns the first error from any insertion; the surrounding
// goroutine drains the function and reports the error up to Build.
//
// cancelled is the shared cancellation flag; the worker stops claiming
// new batches as soon as it observes the flag set, ensuring no extra
// work runs after the first error.
//
// Mirrors Java's private void run(int maxOrd).
func (w *hnswConcurrentMergeWorker) run(maxOrd int, cancelled *atomic.Bool) error {
	for {
		if cancelled.Load() {
			return nil
		}
		start := w.getStartPos(maxOrd)
		if start == -1 {
			return nil
		}
		end := start + w.batchSize
		if end > maxOrd {
			end = maxOrd
		}
		if err := w.addBatch(start, end, cancelled); err != nil {
			return err
		}
	}
}

// getStartPos atomically reserves the next batch by advancing the
// shared cursor. Returns -1 once the cursor has moved past maxOrd —
// i.e. there is no more work to claim. Mirrors Java's getStartPos.
func (w *hnswConcurrentMergeWorker) getStartPos(maxOrd int) int {
	prev := w.workProgress.Add(int64(w.batchSize)) - int64(w.batchSize)
	if prev < int64(maxOrd) {
		return int(prev)
	}
	return -1
}

// addBatch inserts every ordinal in [start, end), respecting the
// initializedNodes skip set. Mirrors Java's super.addVectors plus the
// override that consults initializedNodes inside addGraphNode.
//
// The Java reference reuses the inherited addVectors helper. The Go
// port re-implements the loop here to honour the early-cancellation
// signal and the per-ordinal skip set without duplicating the public
// AddGraphNode call shape.
func (w *hnswConcurrentMergeWorker) addBatch(start, end int, cancelled *atomic.Bool) error {
	for node := start; node < end; node++ {
		if cancelled.Load() {
			return nil
		}
		if w.initializedNodes != nil && node < w.initializedNodes.Length() && w.initializedNodes.Get(node) {
			continue
		}
		if err := w.HnswGraphBuilder.AddGraphNode(node); err != nil {
			return err
		}
	}
	return nil
}

// SetInfoStream forwards the stream to the embedded HnswGraphBuilder.
// It is a thin wrapper kept on the worker so the concurrent builder's
// SetInfoStream loop can call a uniform method on every worker without
// caring about the embedding.
func (w *hnswConcurrentMergeWorker) SetInfoStream(stream util.InfoStream) {
	w.HnswGraphBuilder.SetInfoStream(stream)
}

// mergeSeekState is the per-searcher state used by the merge-aware
// graphSeek policy. It is allocated lazily by installMergeSearcherPolicies
// and stored on the searcher's userData. Each worker has its own
// mergeSeekState so the buffers never cross goroutines.
type mergeSeekState struct {
	lock       *HnswLock
	nodeBuffer []int
	upto       int
	size       int
}

// installMergeSearcherPolicies wires the merge-aware seek/next
// functions onto searcher. graphSeek acquires the read lock for
// (level, target), copies the neighbour list into the per-searcher
// nodeBuffer, releases the lock, and resets the cursor. graphNextNeighbor
// walks the buffer one step at a time and returns NO_MORE_DOCS at the
// end.
//
// Mirrors Java's MergeSearcher.graphSeek / graphNextNeighbor — the read
// lock is held only for the duration of the array copy, so worker
// progress does not stall while a concurrent writer waits on the
// stripe.
func installMergeSearcherPolicies(searcher *HnswGraphSearcher, lock *HnswLock) {
	state := &mergeSeekState{lock: lock}
	searcher.seek = func(_ *HnswGraphSearcher, graph HnswGraph, level, target int) error {
		oh, ok := graph.(*OnHeapHnswGraph)
		if !ok {
			return fmt.Errorf(
				"hnsw: HnswConcurrentMergeBuilder requires *OnHeapHnswGraph, got %T", graph)
		}
		release := state.lock.ReadLock(level, target)
		nbr := oh.GetNeighbors(level, target)
		size := nbr.Size()
		if cap(state.nodeBuffer) < size {
			state.nodeBuffer = make([]int, size)
		} else {
			state.nodeBuffer = state.nodeBuffer[:size]
		}
		copy(state.nodeBuffer[:size], nbr.Nodes()[:size])
		release()
		state.size = size
		state.upto = -1
		return nil
	}
	searcher.next = func(_ *HnswGraphSearcher, _ HnswGraph) (int, error) {
		state.upto++
		if state.upto < state.size {
			return state.nodeBuffer[state.upto], nil
		}
		return util.NO_MORE_DOCS, nil
	}
	searcher.userData = state
}
