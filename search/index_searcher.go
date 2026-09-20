// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// TooManyClauses is thrown when an attempt is made to add more than
// MaxClauseCount clauses. This typically happens if a PrefixQuery,
// FuzzyQuery, WildcardQuery, or TermRangeQuery is expanded to many terms during search.
type TooManyClauses struct {
	maxClauseCount int
}

func (e *TooManyClauses) Error() string {
	return fmt.Sprintf("maxClauseCount is set to %d", e.maxClauseCount)
}

func (e *TooManyClauses) GetMaxClauseCount() int {
	return e.maxClauseCount
}

// NewTooManyClauses mirrors the no-argument constructor
// IndexSearcher.TooManyClauses(), whose body captures getMaxClauseCount().
func NewTooManyClauses() *TooManyClauses {
	return &TooManyClauses{maxClauseCount: GetMaxClauseCount()}
}

// TooManyNestedClauses is thrown when a client attempts to execute a Query that has more than
// MaxClauseCount total clauses cumulatively in all of its children.
type TooManyNestedClauses struct {
	TooManyClauses
}

func NewTooManyNestedClauses() *TooManyNestedClauses {
	return &TooManyNestedClauses{
		TooManyClauses: TooManyClauses{
			maxClauseCount: MaxClauseCount,
		},
	}
}

func (e *TooManyNestedClauses) Error() string {
	return fmt.Sprintf("Query contains too many nested clauses; maxClauseCount is set to %d", e.maxClauseCount)
}

var (
	// MaxClauseCount is the maximum number of clauses permitted, 1024 by default.
	MaxClauseCount = 1024

	// DefaultSimilarity is the default Similarity instance.
	DefaultSimilarity = NewLuceneBM25Similarity()

	defaultQueryCache         QueryCache
	defaultQueryCachingPolicy QueryCachingPolicy
)

func init() {
	const maxCachedQueries = 1000
	// min of 32MB or 5% of the heap size.
	// For Go, we use a fixed 32MB default to match the lower bound of Lucene's logic.
	const maxRamBytesUsed = 1 << 25
	defaultQueryCache = NewLRUQueryCache(maxCachedQueries, int64(maxRamBytesUsed))
	defaultQueryCachingPolicy = NewUsageTrackingQueryCachingPolicy()
}

const (
	// TOTAL_HITS_THRESHOLD is the default threshold for accurate hit counting.
	TOTAL_HITS_THRESHOLD = 1000

	// MAX_DOCS_PER_SLICE is the threshold for index slice allocation logic.
	MAX_DOCS_PER_SLICE = 250_000

	// MAX_SEGMENTS_PER_SLICE is the threshold for index slice allocation logic.
	MAX_SEGMENTS_PER_SLICE = 5
)

// IndexSearcher implements search over a single IndexReader.
// It is completely thread safe.
type IndexSearcher struct {
	reader index.IndexReaderInterface

	// readerContext and leafContexts are cached from the reader.
	readerContext index.IndexReaderContext
	leafContexts  []*index.LeafReaderContext

	// leafSlices caches the concurrent search partitions.
	leafSlices []LeafSlice

	// taskExecutor handles concurrent execution across slices.
	taskExecutor *TaskExecutor

	// similarity is the Similarity implementation used by this searcher.
	similarity Similarity

	// queryCache is the cache used when scores are not needed.
	queryCache QueryCache

	// queryCachingPolicy is the policy used for query caching.
	queryCachingPolicy QueryCachingPolicy

	// queryTimeout limits search time.
	queryTimeout index.QueryTimeout

	// partialResult is set if any search hit the timeout.
	partialResult bool

	mu sync.RWMutex
}

// GetDefaultSimilarity returns the default Similarity instance.
func GetDefaultSimilarity() Similarity {
	return DefaultSimilarity
}

// GetIndexReader returns the IndexReader associated with this searcher.
func (s *IndexSearcher) GetIndexReader() index.IndexReaderInterface {
	return s.reader
}

// GetLeafContexts returns leaf contexts associated with this searcher.
func (s *IndexSearcher) GetLeafContexts() []*index.LeafReaderContext {
	return s.leafContexts
}

// GetDefaultQueryCache returns the default QueryCache.
func GetDefaultQueryCache() QueryCache {
	return defaultQueryCache
}

// SetDefaultQueryCache sets the default QueryCache instance.
func SetDefaultQueryCache(qc QueryCache) {
	defaultQueryCache = qc
}

// GetDefaultQueryCachingPolicy returns the default QueryCachingPolicy.
func GetDefaultQueryCachingPolicy() QueryCachingPolicy {
	return defaultQueryCachingPolicy
}

// SetDefaultQueryCachingPolicy sets the default QueryCachingPolicy instance.
func SetDefaultQueryCachingPolicy(qcp QueryCachingPolicy) {
	defaultQueryCachingPolicy = qcp
}

// NewIndexSearcher creates a searcher searching the provided index.
func NewIndexSearcher(r index.IndexReaderInterface) *IndexSearcher {
	return NewIndexSearcherWithExecutor(r, nil)
}

// NewIndexSearcherWithExecutor creates a searcher searching the provided index,
// using the provided Executor to run searches for each segment separately.
func NewIndexSearcherWithExecutor(r index.IndexReaderInterface, executor Executor) *IndexSearcher {
	ctx, err := r.GetContext()
	if err != nil {
		panic(err)
	}
	leaves, err := r.Leaves()
	if err != nil {
		panic(err)
	}

	s := &IndexSearcher{
		reader:             r,
		similarity:         DefaultSimilarity,
		readerContext:      ctx,
		leafContexts:       leaves,
		taskExecutor:       NewTaskExecutor(executorDispatch(executor)),
		queryCache:         defaultQueryCache,
		queryCachingPolicy: defaultQueryCachingPolicy,
	}

	if executor == nil {
		if len(leaves) == 0 {
			s.leafSlices = []LeafSlice{}
		} else {
			s.leafSlices = []LeafSlice{entireSegments(leaves)}
		}
	}

	return s
}

// GetMaxClauseCount returns the maximum number of clauses permitted.
func GetMaxClauseCount() int {
	return MaxClauseCount
}

// SetMaxClauseCount sets the maximum number of clauses permitted per Query.
func SetMaxClauseCount(value int) {
	if value < 1 {
		panic("maxClauseCount must be >= 1")
	}
	MaxClauseCount = value
}

// SetQueryCache sets the QueryCache to use when scores are not needed.
func (s *IndexSearcher) SetQueryCache(qc QueryCache) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queryCache = qc
}

// GetQueryCache returns the query cache of this IndexSearcher.
func (s *IndexSearcher) GetQueryCache() QueryCache {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.queryCache
}

// SetQueryCachingPolicy sets the QueryCachingPolicy to use for query caching.
func (s *IndexSearcher) SetQueryCachingPolicy(qcp QueryCachingPolicy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if qcp == nil {
		panic("queryCachingPolicy cannot be nil")
	}
	s.queryCachingPolicy = qcp
}

// GetQueryCachingPolicy returns the query caching policy of this IndexSearcher.
func (s *IndexSearcher) GetQueryCachingPolicy() QueryCachingPolicy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.queryCachingPolicy
}

// slices creates an array of leaf slices each holding a subset of the given leaves.
func (s *IndexSearcher) slices(leaves []*index.LeafReaderContext) []LeafSlice {
	return Slices(leaves, MAX_DOCS_PER_SLICE, MAX_SEGMENTS_PER_SLICE, false)
}

// Slices segregates LeafReaderContexts amongst multiple slices.
func Slices(leaves []*index.LeafReaderContext, maxDocsPerSlice, maxSegmentsPerSlice int, allowSegmentPartitions bool) []LeafSlice {
	sortedLeaves := make([]*index.LeafReaderContext, len(leaves))
	copy(sortedLeaves, leaves)

	sort.Slice(sortedLeaves, func(i, j int) bool {
		return sortedLeaves[i].Reader().MaxDoc() > sortedLeaves[j].Reader().MaxDoc()
	})

	if allowSegmentPartitions {
		return slicesWithSegmentPartitions(maxDocsPerSlice, maxSegmentsPerSlice, sortedLeaves)
	}

	var groupedLeaves [][]*index.LeafReaderContext
	var docSum int
	var group []*index.LeafReaderContext

	for _, ctx := range sortedLeaves {
		if ctx.Reader().MaxDoc() > maxDocsPerSlice {
			group = nil
			groupedLeaves = append(groupedLeaves, []*index.LeafReaderContext{ctx})
		} else {
			if group == nil {
				group = []*index.LeafReaderContext{ctx}
				groupedLeaves = append(groupedLeaves, group)
			} else {
				group = append(group, ctx)
			}

			docSum += ctx.Reader().MaxDoc()
			if len(group) >= maxSegmentsPerSlice || docSum > maxDocsPerSlice {
				group = nil
				docSum = 0
			}
		}
	}

	res := make([]LeafSlice, len(groupedLeaves))
	for i, group := range groupedLeaves {
		res[i] = entireSegments(group)
	}
	return res
}

func slicesWithSegmentPartitions(maxDocsPerSlice, maxSegmentsPerSlice int, sortedLeaves []*index.LeafReaderContext) []LeafSlice {
	var groupedLeafPartitions [][]LeafReaderContextPartition
	currentSliceNumDocs := 0
	var group []LeafReaderContextPartition

	for _, ctx := range sortedLeaves {
		maxDoc := ctx.Reader().MaxDoc()
		if maxDoc > maxDocsPerSlice {
			group = nil
			numSlices := int(math.Min(5, math.Ceil(float64(maxDoc)/float64(maxDocsPerSlice))))
			numDocs := maxDoc / numSlices
			minDocId := 0
			maxDocId := numDocs
			for i := 0; i < numSlices-1; i++ {
				groupedLeafPartitions = append(groupedLeafPartitions, []LeafReaderContextPartition{
					NewLeafReaderContextPartitionFromAndTo(ctx, minDocId, maxDocId),
				})
				minDocId = maxDocId
				maxDocId += numDocs
			}
			groupedLeafPartitions = append(groupedLeafPartitions, []LeafReaderContextPartition{
				NewLeafReaderContextPartitionFromAndTo(ctx, minDocId, maxDoc),
			})
		} else {
			if group == nil {
				group = []LeafReaderContextPartition{}
				groupedLeafPartitions = append(groupedLeafPartitions, group)
			}
			group = append(group, NewLeafReaderContextPartitionForEntireSegment(ctx))
			currentSliceNumDocs += maxDoc
			if len(group) >= maxSegmentsPerSlice || currentSliceNumDocs > maxDocsPerSlice {
				group = nil
				currentSliceNumDocs = 0
			}
		}
	}

	res := make([]LeafSlice, len(groupedLeafPartitions))
	for i, group := range groupedLeafPartitions {
		res[i] = LeafSlice{Partitions: group}
	}
	return res
}

// StoredFields returns a StoredFields reader for the stored fields of this index.
func (s *IndexSearcher) StoredFields() (index.StoredFields, error) {
	return s.reader.StoredFields()
}

// SetSimilarity sets the Similarity implementation used by this IndexSearcher.
func (s *IndexSearcher) SetSimilarity(similarity Similarity) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.similarity = similarity
}

// GetSimilarity returns the Similarity to use to compute scores.
func (s *IndexSearcher) GetSimilarity() Similarity {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.similarity == nil {
		return DefaultSimilarity
	}
	return s.similarity
}

// Count counts how many documents match the given query.
func (s *IndexSearcher) Count(query Query) (int, error) {
	// CSQ.rewrite may simplify the query -- don't need scores
	q := NewConstantScoreQuery(query)
	rewritten, err := s.Rewrite(q)
	if err != nil {
		return 0, err
	}

	// Unwrap CSQ to check for optimizations on the inner query
	innerQuery := rewritten
	if csq, ok := rewritten.(*ConstantScoreQuery); ok {
		innerQuery = csq.GetQuery()
	}

	// Check if two clause disjunction optimization applies
	if bq, ok := innerQuery.(*BooleanQuery); ok && !s.reader.HasDeletions() && bq.IsTwoClausePureDisjunctionWithTerms() {
		queries, err := bq.RewriteTwoClauseDisjunctionWithTermsForCount(s)
		if err != nil {
			return 0, err
		}
		countTerm1, err := s.Count(queries[0])
		if err != nil {
			return 0, err
		}
		countTerm2, err := s.Count(queries[1])
		if err != nil {
			return 0, err
		}
		if countTerm1 == 0 || countTerm2 == 0 {
			return max(countTerm1, countTerm2), nil
			// Only apply optimization if the intersection is significantly smaller than the union
		} else if float64(min(countTerm1, countTerm2))/float64(max(countTerm1, countTerm2)) < 0.1 {
			countTerm3, err := s.Count(queries[2])
			if err != nil {
				return 0, err
			}
			return countTerm1 + countTerm2 - countTerm3, nil
		}
	}

	// Use the already-rewritten query directly, avoiding a redundant rewrite in search(query,
	// collector)
	//
	// PORT NOTE. Lucene passes getSlices() to the TotalHitCountCollectorManager
	// constructor so the manager can tell whether any leaf is partitioned;
	// Gocene's TotalHitCountCollectorManager does not yet carry that parameter.
	collectorManager := NewTotalHitCountCollectorManager()
	firstCollector, err := collectorManager.NewCollector()
	if err != nil {
		return 0, err
	}
	weight, err := s.CreateWeight(rewritten, firstCollector.ScoreMode(), 1.0)
	if err != nil {
		return 0, err
	}
	return searchWeightWithCollectorManager[*TotalHitCountCollector, int](s, weight, collectorManager, firstCollector)
}

// GetSlices returns the leaf slices used for concurrent searching.
func (s *IndexSearcher) GetSlices() []LeafSlice {
	s.mu.RLock()
	res := s.leafSlices
	s.mu.RUnlock()

	if res == nil {
		s.mu.Lock()
		if s.leafSlices == nil {
			s.leafSlices = s.computeAndCacheSlices()
		}
		res = s.leafSlices
		s.mu.Unlock()
	}
	return res
}

func (s *IndexSearcher) computeAndCacheSlices() []LeafSlice {
	res := s.slices(s.leafContexts)
	for _, slice := range res {
		if len(slice.Partitions) > 1 {
			s.enforceDistinctLeaves(slice)
		}
	}
	return res
}

func (s *IndexSearcher) enforceDistinctLeaves(slice LeafSlice) {
	distinctLeaves := make(map[*index.LeafReaderContext]struct{})
	for _, partition := range slice.Partitions {
		if _, exists := distinctLeaves[partition.Ctx]; exists {
			panic("The same slice targets multiple leaf partitions of the same leaf reader context")
		}
		distinctLeaves[partition.Ctx] = struct{}{}
	}
}

// SearchAfter finds the top n hits for query where all results are after a previous result.
func (s *IndexSearcher) SearchAfter(after *ScoreDoc, query Query, n int) (*TopDocs, error) {
	limit := s.reader.MaxDoc()
	if limit < 1 {
		limit = 1
	}
	if after != nil && after.Doc >= limit {
		return nil, fmt.Errorf("after.doc exceeds the number of documents in the reader: after.doc=%d limit=%d", after.Doc, limit)
	}

	cappedNumHits := n
	if cappedNumHits > limit {
		cappedNumHits = limit
	}

	manager, err := NewTopScoreDocCollectorManager(cappedNumHits, after, TOTAL_HITS_THRESHOLD)
	if err != nil {
		return nil, err
	}

	return SearchWithCollectorManager[*TopScoreDocCollector, *TopDocs](s, query, manager)
}

// GetTimeout returns the configured QueryTimeout for all searches.
func (s *IndexSearcher) GetTimeout() index.QueryTimeout {
	return s.queryTimeout
}

// SetTimeout sets a QueryTimeout for all searches.
func (s *IndexSearcher) SetTimeout(timeout index.QueryTimeout) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queryTimeout = timeout
}

// Search finds the top n hits for query.
func (s *IndexSearcher) Search(query Query, n int) (*TopDocs, error) {
	return s.SearchAfter(nil, query, n)
}

// SearchWithCollector searches the index using the given collector.
func (s *IndexSearcher) SearchWithCollector(query Query, collector Collector) error {
	rewritten, err := s.rewrite(query, collector.ScoreMode().NeedsScores())
	if err != nil {
		return err
	}
	weight, err := s.CreateWeight(rewritten, collector.ScoreMode(), 1.0)
	if err != nil {
		return err
	}
	collector.SetWeight(weight)
	for _, ctx := range s.leafContexts {
		if err := s.searchLeaf(ctx, 0, ctx.Reader().MaxDoc(), weight, collector); err != nil {
			return err
		}
	}
	return nil
}

// TimedOut returns true if any search hit the timeout.
func (s *IndexSearcher) TimedOut() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.partialResult
}

// SearchWithSort finds the top n hits for query, sorted by the given sort.
func (s *IndexSearcher) SearchWithSort(query Query, n int, sort *Sort, doDocScores bool) (*TopFieldDocs, error) {
	return s.SearchWithSortAfter(nil, query, n, sort, doDocScores)
}

// SearchWithSort finds the top n hits for query, sorted by sort.
func (s *IndexSearcher) SearchWithSortNoScores(query Query, n int, sort *Sort) (*TopFieldDocs, error) {
	return s.SearchWithSort(query, n, sort, false)
}

// SearchWithSortAfter finds the top n hits for query where all results are after a previous result,
// sorted by sort, allowing control over whether hit scores should be computed.
func (s *IndexSearcher) SearchWithSortAfter(after *FieldDoc, query Query, n int, sort *Sort, doDocScores bool) (*TopFieldDocs, error) {
	limit := s.reader.MaxDoc()
	if limit < 1 {
		limit = 1
	}
	if after != nil && after.Doc >= limit {
		return nil, fmt.Errorf("after.doc exceeds the number of documents in the reader: after.doc=%d limit=%d", after.Doc, limit)
	}

	cappedNumHits := n
	if cappedNumHits > limit {
		cappedNumHits = limit
	}

	rewrittenSort, err := sort.Rewrite(s)
	if err != nil {
		return nil, err
	}

	// PORT NOTE. Lucene's TopFieldCollectorManager takes the FieldDoc marker
	// itself; Gocene's constructor declares a *ScoreDoc, so only the ScoreDoc
	// half of the marker is forwarded.
	var afterScoreDoc *ScoreDoc
	if after != nil {
		afterScoreDoc = after.ScoreDoc
	}
	manager, err := NewTopFieldCollectorManager(rewrittenSort, cappedNumHits, afterScoreDoc, TOTAL_HITS_THRESHOLD)
	if err != nil {
		return nil, err
	}

	topDocs, err := SearchWithCollectorManager[*TopFieldCollector, *TopFieldDocs](s, query, manager)
	if err != nil {
		return nil, err
	}
	if doDocScores {
		if err := PopulateScores(topDocs.ScoreDocs, s, query); err != nil {
			return nil, err
		}
	}
	return topDocs, nil
}

// SearchWithCollectorManager is the lower-level search API: it searches all
// leaves using the given CollectorManager, using the searcher's Executor to
// parallelize execution of the collection over the configured slices.
//
// Mirrors `public <C extends Collector, T> T search(Query, CollectorManager<C, T>)`
// of Apache Lucene 10.5.0. Go has no generic methods, so the Java method is
// rendered as a free function taking the searcher as its first parameter.
func SearchWithCollectorManager[C Collector, T any](s *IndexSearcher, query Query, manager CollectorManager[C, T]) (T, error) {
	var zero T
	firstCollector, err := manager.NewCollector()
	if err != nil {
		return zero, err
	}

	rewritten, err := s.rewrite(query, firstCollector.ScoreMode().NeedsScores())
	if err != nil {
		return zero, err
	}

	weight, err := s.CreateWeight(rewritten, firstCollector.ScoreMode(), 1.0)
	if err != nil {
		return zero, err
	}

	return searchWeightWithCollectorManager(s, weight, manager, firstCollector)
}

// searchWeightWithCollectorManager mirrors the private
// `<C extends Collector, T> T search(Weight, CollectorManager<C, T>, C)` of
// Apache Lucene 10.5.0, rendered as a free function for the same reason as
// SearchWithCollectorManager above.
func searchWeightWithCollectorManager[C Collector, T any](
	s *IndexSearcher,
	weight Weight,
	manager CollectorManager[C, T],
	firstCollector C,
) (T, error) {
	var zero T
	leafSlices := s.GetSlices()
	if len(leafSlices) == 0 {
		// there are no segments, nothing to offload to the executor, but we do
		// need to call reduce to create some kind of empty result
		return manager.Reduce([]C{firstCollector})
	}

	collectors := make([]C, 0, len(leafSlices))
	collectors = append(collectors, firstCollector)
	scoreMode := firstCollector.ScoreMode()
	for i := 1; i < len(leafSlices); i++ {
		collector, err := manager.NewCollector()
		if err != nil {
			return zero, err
		}
		collectors = append(collectors, collector)
		if scoreMode != collector.ScoreMode() {
			return zero, fmt.Errorf("CollectorManager does not always produce collectors with the same score mode")
		}
	}

	listTasks := make([]Callable[C], 0, len(leafSlices))
	for i := 0; i < len(leafSlices); i++ {
		leaves := leafSlices[i].Partitions
		collector := collectors[i]
		listTasks = append(listTasks, func(context.Context) (C, error) {
			if err := s.searchPartitions(leaves, weight, collector); err != nil {
				var zeroC C
				return zeroC, err
			}
			return collector, nil
		})
	}

	results, err := InvokeAll(s.taskExecutor, context.Background(), listTasks)
	if err != nil {
		return zero, err
	}
	return manager.Reduce(results)
}

// searchPartitions mirrors
// `protected void search(LeafReaderContextPartition[], Weight, Collector)`.
func (s *IndexSearcher) searchPartitions(partitions []LeafReaderContextPartition, weight Weight, collector Collector) error {
	collector.SetWeight(weight)

	for _, partition := range partitions { // search each leaf partition
		if err := s.searchLeaf(partition.Ctx, partition.MinDocId, partition.MaxDocId, weight, collector); err != nil {
			return err
		}
	}
	return nil
}

// searchLeaf mirrors
// `protected void searchLeaf(LeafReaderContext, int, int, Weight, Collector)`
// of Apache Lucene 10.5.0.
//
// PORT NOTE. Java wraps the live docs in ScorerUtil.likelyLiveDocs, a JIT
// devirtualisation hint with no Go counterpart; the bits are used directly.
func (s *IndexSearcher) searchLeaf(ctx *index.LeafReaderContext, minDocId, maxDocId int, weight Weight, collector Collector) error {
	leafCollector, err := collector.GetLeafCollector(ctx)
	if err != nil {
		if IsCollectionTerminated(err) {
			// there is no doc of interest in this reader context
			// continue with the following leaf
			return nil
		}
		return err
	}
	scorerSupplier, err := weight.ScorerSupplier(ctx)
	if err != nil {
		return err
	}
	if scorerSupplier != nil {
		if err := scorerSupplier.SetTopLevelScoringClause(); err != nil {
			return err
		}
		scorer, err := scorerSupplier.BulkScorer()
		if err != nil {
			return err
		}
		var bulkScorer BulkScorer = scorer
		if s.queryTimeout != nil {
			bulkScorer = NewTimeLimitingBulkScorer(scorer, s.queryTimeout)
		}
		acceptDocs := ctx.LeafReader().GetLiveDocs()
		if _, err := bulkScorer.Score(leafCollector, acceptDocs, minDocId, maxDocId); err != nil {
			switch {
			case IsCollectionTerminated(err):
				// collection was terminated prematurely
				// continue with the following leaf
			case errors.Is(err, ErrTimeExceeded):
				s.mu.Lock()
				s.partialResult = true
				s.mu.Unlock()
			default:
				return err
			}
		}
	}
	// Note: this is called if collection ran successfully, including the above
	// special cases of CollectionTerminatedException and TimeExceededException,
	// but no other error.
	return leafCollector.Finish()
}

// Rewrite rewrites the query into primitive queries.
func (s *IndexSearcher) Rewrite(original Query) (Query, error) {
	query := original
	for {
		rewritten, err := query.Rewrite(s)
		if err != nil {
			return nil, err
		}
		if rewritten == query {
			break
		}
		query = rewritten
	}

	visitor := getNumClausesCheckVisitor()
	query.Visit(visitor)
	return query, nil
}

// rewrite mirrors the private overload
// IndexSearcher.rewrite(Query original, boolean needsScores) of Apache Lucene
// 10.5.0. Go has no overloading, so the private sibling keeps the Java name in
// its unexported form.
func (s *IndexSearcher) rewrite(original Query, needsScores bool) (Query, error) {
	if needsScores {
		return s.Rewrite(original)
	}
	// Take advantage of the few extra rewrite rules of ConstantScoreQuery.
	return s.Rewrite(NewConstantScoreQuery(original))
}

func getNumClausesCheckVisitor() *numClausesCheckVisitor {
	return &numClausesCheckVisitor{}
}

type numClausesCheckVisitor struct {
	numClauses int
}

// AcceptField mirrors the default body of QueryVisitor.acceptField(String),
// which Java's anonymous subclass inherits unchanged: `return true`.
func (v *numClausesCheckVisitor) AcceptField(field string) bool {
	return true
}

func (v *numClausesCheckVisitor) GetSubVisitor(occur Occur, parent Query) QueryVisitor {
	return v
}

func (v *numClausesCheckVisitor) VisitLeaf(query Query) {
	if v.numClauses > MaxClauseCount {
		panic(NewTooManyNestedClauses())
	}
	v.numClauses++
}

func (v *numClausesCheckVisitor) ConsumeTerms(query Query, terms ...*index.Term) {
	if v.numClauses > MaxClauseCount {
		panic(NewTooManyNestedClauses())
	}
	v.numClauses++
}

func (v *numClausesCheckVisitor) ConsumeTermsMatching(query Query, field string, automaton func() ByteRunAutomaton) {
	if v.numClauses > MaxClauseCount {
		panic(NewTooManyNestedClauses())
	}
	v.numClauses++
}

// Explain returns an Explanation that describes how doc scored against query.
func (s *IndexSearcher) Explain(query Query, doc int) (Explanation, error) {
	rewritten, err := s.Rewrite(query)
	if err != nil {
		return nil, err
	}
	weight, err := s.CreateWeight(rewritten, COMPLETE, 1.0)
	if err != nil {
		return nil, err
	}
	if weight == nil {
		return NoMatchExplanation("no matching weight"), nil
	}
	return s.explainWeight(weight, doc)
}

// explainWeight is the expert low-level implementation method: it returns an
// Explanation that describes how doc scored against weight.
//
// Mirrors `protected Explanation explain(Weight weight, int doc)` of Apache
// Lucene 10.5.0. Go has no overloading, so the protected sibling of
// Explain(Query, int) keeps the Java name in an unexported, disambiguated form.
func (s *IndexSearcher) explainWeight(weight Weight, doc int) (Explanation, error) {
	n := index.ReaderUtilSubIndexLeaves(doc, s.leafContexts)
	if n < 0 || n >= len(s.leafContexts) {
		return nil, fmt.Errorf("doc id %d is out of bounds", doc)
	}
	ctx := s.leafContexts[n]
	deBasedDoc := doc - ctx.DocBase
	liveDocs := ctx.LeafReader().GetLiveDocs()
	if liveDocs != nil && !liveDocs.Get(deBasedDoc) {
		return NoMatchExplanation(fmt.Sprintf("Document %d is deleted", doc)), nil
	}
	return weight.Explain(ctx, deBasedDoc)
}

// CreateWeight builds the Weight for the given query, potentially adding caching if possible and configured.
func (s *IndexSearcher) CreateWeight(query Query, scoreMode ScoreMode, boost float32) (Weight, error) {
	s.mu.RLock()
	qc := s.queryCache
	qcp := s.queryCachingPolicy
	s.mu.RUnlock()

	weight, err := query.CreateWeight(s, scoreMode, boost)
	if err != nil {
		return nil, err
	}
	if scoreMode.NeedsScores() == false && qc != nil {
		weight = qc.DoCache(weight, qcp)
	}
	return weight, nil
}

// GetTopReaderContext returns this searcher's top-level IndexReaderContext.
func (s *IndexSearcher) GetTopReaderContext() index.IndexReaderContext {
	return s.readerContext
}

// TermStatistics returns statistics for a term, and never nil.
//
// docFreq is the document frequency of the term; it must be greater than or
// equal to 1. totalTermFreq is the total term frequency.
//
// Mirrors IndexSearcher.termStatistics(Term, int, long). Java passes
// term.bytes() to the TermStatistics constructor; Gocene's TermStatistics
// carries the whole Term, so the term is forwarded unchanged.
func (s *IndexSearcher) TermStatistics(term *index.Term, docFreq int, totalTermFreq int64) TermStatistics {
	// This constructor will throw an exception if docFreq <= 0.
	return *NewTermStatistics(term, docFreq, totalTermFreq)
}

// CollectionStatistics returns statistics for a field.
func (s *IndexSearcher) CollectionStatistics(field string) (*CollectionStatistics, error) {
	var docCount int64
	var sumTotalTermFreq int64
	var sumDocFreq int64

	for _, leaf := range s.leafContexts {
		terms, err := index.GetTerms(leaf.LeafReader(), field)
		if err != nil {
			return nil, err
		}
		dc, err := terms.GetDocCount()
		if err != nil {
			return nil, err
		}
		sttf, err := terms.GetSumTotalTermFreq()
		if err != nil {
			return nil, err
		}
		sdf, err := terms.GetSumDocFreq()
		if err != nil {
			return nil, err
		}
		docCount += int64(dc)
		sumTotalTermFreq += sttf
		sumDocFreq += sdf
	}

	if docCount == 0 {
		return nil, nil
	}
	return NewCollectionStatistics(field, s.reader.MaxDoc(), int(docCount), sumTotalTermFreq, sumDocFreq), nil
}

// GetTaskExecutor returns the TaskExecutor that this searcher relies on.
func (s *IndexSearcher) GetTaskExecutor() *TaskExecutor {
	return s.taskExecutor
}

// Doc retrieves the stored fields of a document.
//
// PORT NOTE. Apache Lucene 10.5.0 has no IndexSearcher.doc(int): the caller
// pulls a StoredFields from searcher.storedFields() and calls document(docID)
// on it. Gocene keeps this sugar because several packages already depend on it;
// its body reproduces the technique of BaseCompositeReader.storedFields(),
// which resolves the leaf that owns docID and delegates to that leaf's own
// StoredFields.
func (s *IndexSearcher) Doc(docID int) (*document.Document, error) {
	for _, ctx := range s.leafContexts {
		leaf := ctx.LeafReader()
		if docID >= ctx.DocBase && docID < ctx.DocBase+leaf.MaxDoc() {
			return s.docFromLeaf(leaf, docID-ctx.DocBase)
		}
	}
	return nil, nil
}

func (s *IndexSearcher) docFromLeaf(leaf index.LeafReader, docID int) (*document.Document, error) {
	storedFields, err := leaf.StoredFields()
	if err != nil {
		return nil, err
	}

	visitor := NewDocumentVisitor()
	err = storedFields.Document(docID, visitor)
	if err != nil {
		return nil, err
	}

	return visitor.Document(), nil
}

func (s *IndexSearcher) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reader == nil {
		return nil
	}
	reader := s.reader
	s.reader = nil
	if closer, ok := interface{}(reader).(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}

// DocumentVisitor collects stored fields into a Document.
type DocumentVisitor struct {
	doc *document.Document
}

func NewDocumentVisitor() *DocumentVisitor {
	return &DocumentVisitor{
		doc: document.NewDocument(),
	}
}

// Document returns the Document assembled from the visited stored fields.
//
// Mirrors DocumentStoredFieldVisitor.getDocument().
func (v *DocumentVisitor) Document() *document.Document {
	return v.doc
}

// NeedsField accepts every stored field, mirroring the load-all
// DocumentStoredFieldVisitor that StoredFields.document(int) uses.
func (v *DocumentVisitor) NeedsField(*index.FieldInfo) (index.StoredFieldVisitorStatus, error) {
	return index.StoredFieldVisitorStatusYes, nil
}

func (v *DocumentVisitor) StringField(fieldInfo *index.FieldInfo, value string) error {
	sf, err := document.NewStoredField(fieldInfo.Name(), value)
	if err != nil {
		return err
	}
	v.doc.Add(sf)
	return nil
}

func (v *DocumentVisitor) BinaryField(fieldInfo *index.FieldInfo, value []byte) error {
	sf, err := document.NewStoredFieldFromBytes(fieldInfo.Name(), value)
	if err != nil {
		return err
	}
	v.doc.Add(sf)
	return nil
}

func (v *DocumentVisitor) IntField(fieldInfo *index.FieldInfo, value int) error {
	sf, err := document.NewStoredFieldFromInt(fieldInfo.Name(), value)
	if err != nil {
		return err
	}
	v.doc.Add(sf)
	return nil
}

func (v *DocumentVisitor) LongField(fieldInfo *index.FieldInfo, value int64) error {
	sf, err := document.NewStoredFieldFromInt64(fieldInfo.Name(), value)
	if err != nil {
		return err
	}
	v.doc.Add(sf)
	return nil
}

func (v *DocumentVisitor) FloatField(fieldInfo *index.FieldInfo, value float32) error {
	sf, err := document.NewStoredFieldFromFloat64(fieldInfo.Name(), float64(value))
	if err != nil {
		return err
	}
	v.doc.Add(sf)
	return nil
}

func (v *DocumentVisitor) DoubleField(fieldInfo *index.FieldInfo, value float64) error {
	sf, err := document.NewStoredFieldFromFloat64(fieldInfo.Name(), value)
	if err != nil {
		return err
	}
	v.doc.Add(sf)
	return nil
}

func (s *IndexSearcher) String() string {
	return fmt.Sprintf("IndexSearcher(%v; taskExecutor=%v)", s.reader, s.taskExecutor)
}

// LeafSlice holds a subset of leaf contexts to be executed in a single thread.
type LeafSlice struct {
	Partitions []LeafReaderContextPartition
}

func entireSegments(contexts []*index.LeafReaderContext) LeafSlice {
	parts := make([]LeafReaderContextPartition, len(contexts))
	for i, ctx := range contexts {
		parts[i] = NewLeafReaderContextPartitionForEntireSegment(ctx)
	}
	return LeafSlice{Partitions: parts}
}

// LeafReaderContextPartition targets a specific doc id range of a LeafReaderContext.
type LeafReaderContextPartition struct {
	MinDocId int
	MaxDocId int
	Ctx      *index.LeafReaderContext
}

func NewLeafReaderContextPartitionForEntireSegment(ctx *index.LeafReaderContext) LeafReaderContextPartition {
	return LeafReaderContextPartition{
		MinDocId: 0,
		MaxDocId: NO_MORE_DOCS,
		Ctx:      ctx,
	}
}

func NewLeafReaderContextPartitionFromAndTo(ctx *index.LeafReaderContext, minDocId, maxDocId int) LeafReaderContextPartition {
	return LeafReaderContextPartition{
		MinDocId: minDocId,
		MaxDocId: maxDocId,
		Ctx:      ctx,
	}
}

// Executor is an interface for executing tasks concurrently.
//
// It is the Go rendering of java.util.concurrent.Executor, the type
// IndexSearcher's constructor accepts; like the JDK interface it declares
// exactly one method.
type Executor interface {
	Execute(runnable func())
}

// executorDispatch adapts an Executor to the dispatcher TaskExecutor expects.
// A nil Executor yields a nil dispatcher, which runs every task on the caller
// goroutine, mirroring Java's IndexSearcher(reader, null) contract.
func executorDispatch(executor Executor) func(func()) {
	if executor == nil {
		return nil
	}
	return executor.Execute
}
