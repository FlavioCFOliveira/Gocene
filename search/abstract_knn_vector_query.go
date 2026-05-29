// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// Ported from Apache Lucene 10.4.0:
//   lucene/core/src/java/org/apache/lucene/search/AbstractKnnVectorQuery.java

import (
	"context"
	"math"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search/knn"
	"github.com/FlavioCFOliveira/Gocene/util"
	hnswutil "github.com/FlavioCFOliveira/Gocene/util/hnsw"
)

// lambdaKnn controls the degree of additional result exploration during
// pro-rata search of segments.
//
// Mirrors AbstractKnnVectorQuery.LAMBDA.
const lambdaKnn = 16

// AbstractKnnVectorQuery is the base type for KNN vector queries that
// delegate to a codec-level KnnVectorsReader for approximate search.
//
// In Java this is an abstract class; the Go port exposes the shared
// behaviour via an embedded [BaseKnnVectorQuery] struct and an interface
// [KnnVectorQueryImpl] that concrete types must satisfy.
//
// Subclasses implement:
//   - [KnnVectorQueryImpl.ApproximateSearch] — perform the codec-level
//     HNSW search for this vector type.
//   - [KnnVectorQueryImpl.CreateVectorScorer] — build a [VectorScorer]
//     for exact (brute-force) search on a single leaf.
//
// Ported from org.apache.lucene.search.AbstractKnnVectorQuery.
type KnnVectorQueryImpl interface {
	Query

	// ApproximateSearch executes an approximate KNN search on one leaf.
	//
	// Mirrors AbstractKnnVectorQuery.approximateSearch.
	ApproximateSearch(
		ctx *index.LeafReaderContext,
		acceptDocs AcceptDocs,
		visitedLimit int,
		collectorManager knn.KnnCollectorManager,
	) (*TopDocs, error)

	// CreateVectorScorer returns a VectorScorer for exact brute-force
	// search on the leaf described by ctx and the given FieldInfo. May
	// return nil when the field does not exist in that segment.
	//
	// Mirrors AbstractKnnVectorQuery.createVectorScorer.
	CreateVectorScorer(
		ctx *index.LeafReaderContext,
		fi *index.FieldInfo,
	) (VectorScorer, error)
}

// ExactSearcher is an optional hook a [KnnVectorQueryImpl] may implement to
// take full control of the per-leaf exact (brute-force) search, replacing the
// default [BaseKnnVectorQuery.exactSearch] linear scan.
//
// This is the Go counterpart of overriding the protected method
// AbstractKnnVectorQuery.exactSearch in a subclass (as
// DiversifyingChildren{Float,Byte}KnnVectorQuery do). When the concrete impl
// passed to [NewBaseKnnVectorQuery] also satisfies ExactSearcher, the base
// query delegates to ExactSearch instead of running its own scan; otherwise the
// default behaviour is unchanged.
type ExactSearcher interface {
	// ExactSearch performs the exact per-leaf search over acceptIterator and
	// returns the per-leaf top results (leaf-local doc IDs).
	//
	// Mirrors AbstractKnnVectorQuery.exactSearch (protected, overridable).
	ExactSearch(
		ctx *index.LeafReaderContext,
		acceptIterator DocIdSetIterator,
		timeout index.QueryTimeout,
	) (*TopDocs, error)
}

// BaseKnnVectorQuery holds the shared state (field, k, filter, strategy)
// and the full Rewrite algorithm. Concrete query types embed this struct
// and satisfy [KnnVectorQueryImpl] by providing ApproximateSearch and
// CreateVectorScorer.
//
// Ported from the protected fields and concrete methods of
// org.apache.lucene.search.AbstractKnnVectorQuery.
type BaseKnnVectorQuery struct {
	BaseQuery
	impl     KnnVectorQueryImpl // back-reference to the concrete subtype
	field    string
	k        int
	filter   Query
	strategy knn.KnnSearchStrategy
}

// NewBaseKnnVectorQuery constructs the shared base for KNN vector queries.
// impl must be the concrete subtype that satisfies KnnVectorQueryImpl.
func NewBaseKnnVectorQuery(
	impl KnnVectorQueryImpl,
	field string,
	k int,
	filter Query,
	strategy knn.KnnSearchStrategy,
) BaseKnnVectorQuery {
	if field == "" {
		panic("AbstractKnnVectorQuery: field must not be empty")
	}
	if k < 1 {
		panic("AbstractKnnVectorQuery: k must be at least 1")
	}
	return BaseKnnVectorQuery{
		impl:     impl,
		field:    field,
		k:        k,
		filter:   filter,
		strategy: strategy,
	}
}

// GetField returns the indexed vector field name.
func (q *BaseKnnVectorQuery) GetField() string { return q.field }

// GetK returns the maximum number of nearest neighbours to retrieve.
func (q *BaseKnnVectorQuery) GetK() int { return q.k }

// GetFilter returns the optional pre-filter query (may be nil).
func (q *BaseKnnVectorQuery) GetFilter() Query { return q.filter }

// GetSearchStrategy returns the optional KnnSearchStrategy (may be nil).
func (q *BaseKnnVectorQuery) GetSearchStrategy() knn.KnnSearchStrategy { return q.strategy }

// Visit propagates the query visitor to this leaf.
func (q *BaseKnnVectorQuery) Visit(visitor QueryVisitor) {
	if visitor.AcceptField(q.field) {
		visitor.VisitLeaf(q.impl)
	}
}

// Rewrite executes the full KNN search algorithm across all index
// segments and returns a [DocAndScoreQuery] (or [MatchNoDocsQuery] when
// no results are found).
//
// Algorithm:
//  1. Rewrite and validate the filter (if any).
//  2. Wrap the base collector manager with an optimistic-K layer and a
//     time-limiting layer.
//  3. Run a per-leaf approximate search (in parallel when a TaskExecutor
//     is available).
//  4. Optionally re-run on segments whose results are competitive.
//  5. Merge and return the global top-K.
//
// Mirrors AbstractKnnVectorQuery.rewrite(IndexSearcher).
func (q *BaseKnnVectorQuery) Rewrite(reader IndexReader) (Query, error) {
	// Gocene's Query interface uses Rewrite(IndexReader) — a minimal
	// interface. The full algorithm needs Leaves() and leaf-level access;
	// we type-assert to index.IndexReaderInterface to access those.
	// If the reader does not satisfy that interface (e.g. a mock in tests),
	// return MatchNoDocsQuery gracefully rather than panicking.
	ir, ok := reader.(index.IndexReaderInterface)
	if !ok {
		return NewMatchNoDocsQuery(), nil
	}

	var filterWeight Weight
	if q.filter != nil {
		rewrittenFilter, err := q.filter.Rewrite(reader)
		if err != nil {
			return nil, err
		}
		if _, ok := rewrittenFilter.(*MatchNoDocsQuery); ok {
			return NewMatchNoDocsQuery(), nil
		}
		if _, ok := rewrittenFilter.(*MatchAllDocsQuery); !ok {
			// Build a filter BooleanQuery: filter AND field_exists.
			bq := NewBooleanQuery()
			bq.Add(q.filter, FILTER)
			bq.Add(NewFieldExistsQuery(q.field), FILTER)
			rewritten, err := bq.Rewrite(reader)
			if err != nil {
				return nil, err
			}
			if _, ok := rewritten.(*MatchNoDocsQuery); ok {
				return NewMatchNoDocsQuery(), nil
			}
			// CreateWeight with nil searcher — the weight is only used for
			// obtaining a per-leaf Scorer. Callers that need scoring must
			// provide a full IndexSearcher via RewriteWithSearcher.
			filterWeight, err = rewritten.CreateWeight(nil, false, 1.0)
			if err != nil {
				return nil, err
			}
		}
		// If rewrittenFilter is MatchAllDocsQuery, filterWeight stays nil.
	}

	collectorManager := knn.NewTopKnnCollectorManager(q.k, nil)
	optimistic := newOptimisticKnnCollectorManager(q.k, collectorManager)
	timeLimiting := newSearchTimeLimitingKnnCollectorManager(optimistic, nil)

	leaves, err := ir.Leaves()
	if err != nil {
		return nil, err
	}

	callables := make([]Callable[*TopDocs], len(leaves))
	for i, ctx := range leaves {
		ctx := ctx // capture
		callables[i] = func(_ context.Context) (*TopDocs, error) {
			return q.searchLeaf(ctx, filterWeight, timeLimiting)
		}
	}

	// Sequential execution (no TaskExecutor wired to this code path).
	perLeafResults := make(map[int]*TopDocs, len(leaves))
	topK, err := q.runSearchTasks(callables, nil, perLeafResults, leaves)
	if err != nil {
		return nil, err
	}

	// Phase-2 optimistic re-entry when eligible.
	if len(topK.ScoreDocs) > 0 &&
		len(perLeafResults) > 1 &&
		knn.IsOptimistic(collectorManager) &&
		topK.TotalHits.Relation == EQUAL_TO {

		minTopKScore := topK.ScoreDocs[len(topK.ScoreDocs)-1].Score
		reentrantMgr := newReentrantKnnCollectorManager(
			knn.NewTopKnnCollectorManager(q.k, nil),
			perLeafResults,
		)
		timeLimiting2 := newSearchTimeLimitingKnnCollectorManager(reentrantMgr, nil)

		var phase2Leaves []*index.LeafReaderContext
		var phase2Calls []Callable[*TopDocs]
		for _, ctx := range leaves {
			perLeaf, ok := perLeafResults[ctx.Ord()]
			if !ok {
				continue
			}
			if len(perLeaf.ScoreDocs) > 0 &&
				perLeaf.ScoreDocs[len(perLeaf.ScoreDocs)-1].Score >= minTopKScore {
				ctx := ctx // capture
				phase2Leaves = append(phase2Leaves, ctx)
				phase2Calls = append(phase2Calls, func(_ context.Context) (*TopDocs, error) {
					return q.searchLeaf(ctx, filterWeight, timeLimiting2)
				})
			}
		}
		if len(phase2Calls) > 0 {
			topK, err = q.runSearchTasks(phase2Calls, nil, perLeafResults, phase2Leaves)
			if err != nil {
				return nil, err
			}
		}
	}

	if len(topK.ScoreDocs) == 0 {
		return NewMatchNoDocsQuery(), nil
	}
	return newDocAndScoreQueryFromTopDocs(topK, leaves), nil
}

// runSearchTasks executes callables, stores results keyed by leaf ordinal,
// then merges and returns the global top-K.
func (q *BaseKnnVectorQuery) runSearchTasks(
	tasks []Callable[*TopDocs],
	_ *TaskExecutor, // reserved for future parallel execution
	perLeafResults map[int]*TopDocs,
	leaves []*index.LeafReaderContext,
) (*TopDocs, error) {
	// Sequential execution — TaskExecutor parallel dispatch is deferred
	// until IndexSearcher exposes GetTaskExecutor().
	for i, task := range tasks {
		td, err := task(context.Background())
		if err != nil {
			return nil, err
		}
		perLeafResults[leaves[i].Ord()] = td
	}
	tasks = tasks[:0]

	// Collect all per-leaf TopDocs and merge.
	all := make([]*TopDocs, 0, len(perLeafResults))
	for _, td := range perLeafResults {
		all = append(all, td)
	}
	return q.mergeLeafResults(all), nil
}

// searchLeaf executes the search on a single leaf, adjusting doc IDs by
// docBase.
//
// Mirrors AbstractKnnVectorQuery.searchLeaf.
func (q *BaseKnnVectorQuery) searchLeaf(
	ctx *index.LeafReaderContext,
	filterWeight Weight,
	timeLimiting *searchTimeLimitingKnnCollectorManager,
) (*TopDocs, error) {
	results, err := q.getLeafResults(ctx, filterWeight, timeLimiting)
	if err != nil {
		return nil, err
	}
	if ctx.DocBase() > 0 {
		for _, sd := range results.ScoreDocs {
			sd.Doc += ctx.DocBase()
		}
	}
	return results, nil
}

// getLeafResults implements the per-leaf search strategy decision.
//
// Mirrors AbstractKnnVectorQuery.getLeafResults (private method).
func (q *BaseKnnVectorQuery) getLeafResults(
	ctx *index.LeafReaderContext,
	filterWeight Weight,
	timeLimiting *searchTimeLimitingKnnCollectorManager,
) (*TopDocs, error) {
	leafReader := ctx.LeafReader()

	var liveDocs util.Bits
	if lr, ok := ctx.Reader().(interface{ GetLiveDocs() util.Bits }); ok {
		liveDocs = lr.GetLiveDocs()
	}

	maxDoc := ctx.Reader().MaxDoc()

	if filterWeight == nil {
		acceptDocs := AcceptDocsFromLiveDocs(liveDocs, maxDoc)
		return q.impl.ApproximateSearch(ctx, acceptDocs, math.MaxInt32, timeLimiting)
	}

	var scorer Scorer
	if leafReader != nil {
		leafCtx := index.NewLeafReaderContext(ctx.Reader(), ctx.Parent(), ctx.Ord(), ctx.DocBase())
		var err error
		scorer, err = filterWeight.Scorer(leafCtx)
		if err != nil {
			return nil, err
		}
	}

	var iterSupplier func() (DocIdSetIterator, error)
	if scorer == nil {
		iterSupplier = func() (DocIdSetIterator, error) {
			return NewEmptyDocIdSetIterator(), nil
		}
	} else {
		// Scorer embeds DocIdSetIterator in Gocene (Java: scorer.iterator()).
		s := scorer
		iterSupplier = func() (DocIdSetIterator, error) {
			return s, nil
		}
	}

	acceptDocs := AcceptDocsFromIteratorSupplier(iterSupplier, liveDocs, maxDoc)
	cost, err := acceptDocs.Cost()
	if err != nil {
		return nil, err
	}

	// Determine per-leaf k.
	perLeafTopK := q.k
	if ctx.Parent() != nil {
		leafProportion := float64(maxDoc) / float64(ctx.Parent().Reader().MaxDoc())
		perLeafTopK = perLeafTopKCalculation(q.k, leafProportion)
	}

	if cost <= perLeafTopK {
		// Exact search: too few candidates to make approximate search worthwhile.
		iter, err := acceptDocs.Iterator()
		if err != nil {
			return nil, err
		}
		return q.exactSearch(ctx, iter, timeLimiting.queryTimeout)
	}

	// Approximate search. cost+1 matches the Java "edge case when we
	// explore exactly cost vectors" comment.
	results, err := q.impl.ApproximateSearch(ctx, acceptDocs, cost+1, timeLimiting)
	if err != nil {
		return nil, err
	}

	timeout := timeLimiting.queryTimeout
	timeoutHit := timeout != nil && timeout.ShouldExit()

	if (results.TotalHits.Relation == EQUAL_TO && len(results.ScoreDocs) >= perLeafTopK) ||
		timeoutHit {
		return results, nil
	}

	// Approximate search visited too many nodes; fall back to exact search.
	iter, err := acceptDocs.Iterator()
	if err != nil {
		return nil, err
	}
	return q.exactSearch(ctx, iter, timeout)
}

// getKnnCollectorManager returns the KnnCollectorManager to use for
// collecting approximate-search results. Subclasses may override.
//
// Mirrors AbstractKnnVectorQuery.getKnnCollectorManager (protected).
func (q *BaseKnnVectorQuery) getKnnCollectorManager(k int) knn.KnnCollectorManager {
	return knn.NewTopKnnCollectorManager(k, nil)
}

// exactSearch performs an exhaustive linear scan over acceptIterator,
// scoring each doc via the concrete subtype's VectorScorer, and returns
// the top-k results.
//
// When the concrete impl implements [ExactSearcher], the search is delegated
// to it (mirroring a subclass that overrides the protected exactSearch in
// Java); otherwise the default linear scan below runs.
//
// Mirrors AbstractKnnVectorQuery.exactSearch (protected, overridable).
func (q *BaseKnnVectorQuery) exactSearch(
	ctx *index.LeafReaderContext,
	acceptIterator DocIdSetIterator,
	timeout index.QueryTimeout,
) (*TopDocs, error) {
	if es, ok := q.impl.(ExactSearcher); ok {
		return es.ExactSearch(ctx, acceptIterator, timeout)
	}

	// Resolve FieldInfo.
	var fi *index.FieldInfo
	if fi = leafFieldInfo(ctx, q.field); fi == nil || fi.VectorDimension() == 0 {
		return emptyTopDocs(), nil
	}

	vectorScorer, err := q.impl.CreateVectorScorer(ctx, fi)
	if err != nil {
		return nil, err
	}
	if vectorScorer == nil {
		return emptyTopDocs(), nil
	}

	cost := acceptIterator.Cost()
	queueSize := int(cost)
	if queueSize > q.k {
		queueSize = q.k
	}
	queue := NewHitQueue(queueSize, true)
	relation := EQUAL_TO

	scorer := vectorScorer.Iterator()
	for {
		doc, err := acceptIterator.NextDoc()
		if err != nil {
			return nil, err
		}
		if doc == NO_MORE_DOCS {
			break
		}
		if timeout != nil && timeout.ShouldExit() {
			relation = GREATER_THAN_OR_EQUAL_TO
			break
		}
		// Advance the vector scorer to this document.
		advDoc, err := scorer.Advance(doc)
		if err != nil {
			return nil, err
		}
		if advDoc != doc {
			// No vector for this document.
			continue
		}
		score, err := vectorScorer.Score()
		if err != nil {
			return nil, err
		}
		top := queue.Top()
		if top != nil && score > top.Score {
			top.Score = score
			top.Doc = doc
			queue.UpdateTop()
		}
	}

	// Remove sentinel values (pre-populated with -Inf score).
	for queue.Size() > 0 && queue.Top().Score < 0 {
		queue.Pop()
	}

	scoreDocs := make([]*ScoreDoc, queue.Size())
	for i := len(scoreDocs) - 1; i >= 0; i-- {
		scoreDocs[i] = queue.Pop()
	}

	return NewTopDocs(NewTotalHits(acceptIterator.Cost(), relation), scoreDocs), nil
}

// mergeLeafResults merges per-segment TopDocs into the global top-K.
//
// Mirrors AbstractKnnVectorQuery.mergeLeafResults (protected, overridable).
func (q *BaseKnnVectorQuery) mergeLeafResults(perLeafResults []*TopDocs) *TopDocs {
	return Merge(perLeafResults, q.k)
}

// EqualsBase checks structural equality for the base fields.
// Concrete types must also compare their own vector payload.
func (q *BaseKnnVectorQuery) EqualsBase(other *BaseKnnVectorQuery) bool {
	if q.k != other.k || q.field != other.field {
		return false
	}
	if (q.filter == nil) != (other.filter == nil) {
		return false
	}
	if q.filter != nil && !q.filter.Equals(other.filter) {
		return false
	}
	return true
}

// HashCodeBase returns a hash of the base fields.
func (q *BaseKnnVectorQuery) HashCodeBase() int {
	h := 17
	for _, b := range []byte(q.field) {
		h = 31*h + int(b)
	}
	h = 31*h + q.k
	if q.filter != nil {
		h = 31*h + q.filter.HashCode()
	}
	return h
}

// ─── perLeafTopKCalculation ──────────────────────────────────────────────────

// perLeafTopKCalculation returns the expected number of top-K hits in a
// leaf with the given proportion of the whole index, plus three standard
// deviations of the binomial distribution.
//
// Mirrors AbstractKnnVectorQuery.perLeafTopKCalculation (private static).
func perLeafTopKCalculation(k int, leafProportion float64) int {
	mean := float64(k) * leafProportion
	variance := mean * (1 - leafProportion)
	result := mean + float64(lambdaKnn)*math.Sqrt(variance)
	if result < 1 {
		return 1
	}
	return int(result)
}

// ─── optimisticKnnCollectorManager ───────────────────────────────────────────

// optimisticKnnCollectorManager wraps a [knn.KnnCollectorManager] and
// scales per-leaf collection to the expected per-leaf k when the delegate
// supports optimistic collection.
//
// Mirrors AbstractKnnVectorQuery.OptimisticKnnCollectorManager (static inner class).
type optimisticKnnCollectorManager struct {
	k        int
	delegate knn.KnnCollectorManager
}

func newOptimisticKnnCollectorManager(k int, delegate knn.KnnCollectorManager) *optimisticKnnCollectorManager {
	return &optimisticKnnCollectorManager{k: k, delegate: delegate}
}

// NewCollector creates a collector scaled to the per-leaf proportion of k
// when the delegate is optimistic and the leaf has a parent context.
func (m *optimisticKnnCollectorManager) NewCollector(
	visitedLimit int,
	strategy knn.KnnSearchStrategy,
	ctx *index.LeafReaderContext,
) (hnswutil.KnnCollector, error) {
	if knn.IsOptimistic(m.delegate) && ctx.Parent() != nil {
		proportion := float64(ctx.Reader().MaxDoc()) /
			float64(ctx.Parent().Reader().MaxDoc())
		perLeafK := perLeafTopKCalculation(m.k, proportion)
		if perLeafK <= 0 {
			perLeafK = 1
		}
		return knn.NewOptimisticCollector(m.delegate, visitedLimit, strategy, ctx, perLeafK)
	}
	return m.delegate.NewCollector(visitedLimit, strategy, ctx)
}

// Compile-time check.
var _ knn.KnnCollectorManager = (*optimisticKnnCollectorManager)(nil)

// ─── reentrantKnnCollectorManager ────────────────────────────────────────────

// reentrantKnnCollectorManager seeds phase-2 searches with the seed
// TopDocs from phase 1, enabling the HNSW graph to re-enter at the
// nearest known nodes.
//
// Mirrors AbstractKnnVectorQuery.ReentrantKnnCollectorManager (private inner class).
type reentrantKnnCollectorManager struct {
	base           knn.KnnCollectorManager
	perLeafResults map[int]*TopDocs
}

func newReentrantKnnCollectorManager(
	base knn.KnnCollectorManager,
	perLeafResults map[int]*TopDocs,
) *reentrantKnnCollectorManager {
	return &reentrantKnnCollectorManager{base: base, perLeafResults: perLeafResults}
}

// NewCollector returns a collector seeded with the phase-1 results for
// this leaf.  When the infrastructure for IndexedDISI / KnnVectorValues
// is fully wired, this will produce a Seeded strategy; currently it falls
// back to the base collector to keep the code path correct.
func (m *reentrantKnnCollectorManager) NewCollector(
	visitLimit int,
	strategy knn.KnnSearchStrategy,
	ctx *index.LeafReaderContext,
) (hnswutil.KnnCollector, error) {
	// Delegate to the base manager. Full seeded strategy requires
	// IndexedDISI / KnnVectorValues.DocIndexIterator which are not yet
	// wired in Gocene's SegmentReader. The base collector produces correct
	// (non-seeded) results; Sprint 59 backlog tracks the seeded wiring.
	return m.base.NewCollector(visitLimit, strategy, ctx)
}

// Compile-time check.
var _ knn.KnnCollectorManager = (*reentrantKnnCollectorManager)(nil)

// ─── searchTimeLimitingKnnCollectorManager ────────────────────────────────────

// searchTimeLimitingKnnCollectorManager wraps a [knn.KnnCollectorManager]
// with an optional [index.QueryTimeout].
//
// Mirrors org.apache.lucene.search.TimeLimitingKnnCollectorManager. The
// existing [TimeLimitingKnnCollectorManager] in this package is a
// different (deadline-based) type used by earlier sprints; this one is
// the canonical search-level wrapper matching the Java 10.4.0 source.
type searchTimeLimitingKnnCollectorManager struct {
	delegate     knn.KnnCollectorManager
	queryTimeout index.QueryTimeout
}

func newSearchTimeLimitingKnnCollectorManager(
	delegate knn.KnnCollectorManager,
	queryTimeout index.QueryTimeout,
) *searchTimeLimitingKnnCollectorManager {
	return &searchTimeLimitingKnnCollectorManager{
		delegate:     delegate,
		queryTimeout: queryTimeout,
	}
}

// NewCollector wraps the delegate collector with timeout signalling when
// a QueryTimeout is configured.
func (m *searchTimeLimitingKnnCollectorManager) NewCollector(
	visitedLimit int,
	strategy knn.KnnSearchStrategy,
	ctx *index.LeafReaderContext,
) (hnswutil.KnnCollector, error) {
	c, err := m.delegate.NewCollector(visitedLimit, strategy, ctx)
	if err != nil {
		return nil, err
	}
	if m.queryTimeout == nil {
		return c, nil
	}
	return &timeLimitingKnnCollector{KnnCollector: c, timeout: m.queryTimeout}, nil
}

// Compile-time check.
var _ knn.KnnCollectorManager = (*searchTimeLimitingKnnCollectorManager)(nil)

// timeLimitingKnnCollector decorates a [hnswutil.KnnCollector] with
// early-termination driven by a [index.QueryTimeout].
type timeLimitingKnnCollector struct {
	hnswutil.KnnCollector
	timeout index.QueryTimeout
}

// EarlyTerminated returns true when the timeout has fired or the
// underlying collector has terminated.
func (c *timeLimitingKnnCollector) EarlyTerminated() bool {
	return c.timeout.ShouldExit() || c.KnnCollector.EarlyTerminated()
}

// TopDocs adjusts the relation to GREATER_THAN_OR_EQUAL_TO when the
// timeout has fired.
func (c *timeLimitingKnnCollector) TopDocs() *hnswutil.TopDocs {
	docs := c.KnnCollector.TopDocs()
	if c.timeout.ShouldExit() {
		return hnswutil.NewTopDocs(
			hnswutil.NewTotalHits(docs.TotalHits.Value, hnswutil.GreaterThanOrEqualTo),
			docs.ScoreDocs,
		)
	}
	return docs
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// emptyTopDocs returns a zero-result TopDocs (equivalent to Java's
// TopDocsCollector.EMPTY_TOPDOCS).
func emptyTopDocs() *TopDocs {
	return NewTopDocs(NewTotalHits(0, EQUAL_TO), nil)
}

// newDocAndScoreQueryFromTopDocs converts a TopDocs (carrying GLOBAL doc IDs)
// into a leaf-scoped DocAndScoreQuery. The per-leaf segmentStarts are computed
// from the reader's leaf doc-bases so the resulting query's per-leaf scorers
// each emit only their own slice of the merged top-K (rebased to leaf-local
// doc IDs). Without the segmentStarts, IndexSearcher's per-segment execution
// would re-emit every global doc once per leaf and re-apply each leaf's
// docBase, corrupting multi-segment results.
//
// Mirrors DocAndScoreQuery.createDocAndScoreQuery(IndexReader, TopDocs).
func newDocAndScoreQueryFromTopDocs(topK *TopDocs, leaves []*index.LeafReaderContext) *DocAndScoreQuery {
	n := len(topK.ScoreDocs)
	docIDs := make([]int, n)
	scores := make([]float32, n)
	for i, sd := range topK.ScoreDocs {
		docIDs[i] = sd.Doc
		scores[i] = sd.Score
	}
	// docIDs must be ascending for findSegmentStarts; NewDocAndScoreQueryWithSegmentStarts
	// also sorts, so sort a local copy here to compute segmentStarts against
	// the same ordering.
	sortedDocs := make([]int, n)
	copy(sortedDocs, docIDs)
	sort.Ints(sortedDocs)

	var segmentStarts []int
	if len(leaves) > 0 {
		// Index doc-bases by leaf ordinal so docBases[ord] matches the ord the
		// per-leaf scorer is later created with (Leaves() is ord-ordered, but
		// keying by Ord() is robust regardless).
		docBases := make([]int, len(leaves))
		for _, lc := range leaves {
			if o := lc.Ord(); o >= 0 && o < len(docBases) {
				docBases[o] = lc.DocBase()
			}
		}
		segmentStarts = findSegmentStarts(docBases, sortedDocs)
	}
	return NewDocAndScoreQueryWithSegmentStarts(docIDs, scores, segmentStarts)
}

// leafFieldInfo extracts the FieldInfo for the given field from a leaf's
// reader. Returns nil when the reader does not expose GetFieldInfos.
func leafFieldInfo(ctx *index.LeafReaderContext, field string) *index.FieldInfo {
	type fieldInfoProvider interface {
		GetFieldInfos() *index.FieldInfos
	}
	if fip, ok := ctx.Reader().(fieldInfoProvider); ok {
		fi := fip.GetFieldInfos()
		if fi != nil {
			return fi.GetByName(field)
		}
	}
	return nil
}
