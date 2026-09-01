// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"fmt"
	"math"
	"sort"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TooManyClauses is thrown when a query exceeds the permitted number of clauses.
type TooManyClauses struct {
	MaxClauseCount int
}

func (e *TooManyClauses) Error() string {
	return fmt.Sprintf("maxClauseCount is set to %d", e.MaxClauseCount)
}

// TooManyNestedClauses is thrown when a query exceeds the permitted number of nested clauses.
type TooManyNestedClauses struct {
	TooManyClauses
}

func (e *TooManyNestedClauses) Error() string {
	return fmt.Sprintf("Query contains too many nested clauses; maxClauseCount is set to %d", e.MaxClauseCount)
}

// IndexSearcher searches an index.
// It is safe for concurrent use.
type IndexSearcher struct {
	reader index.IndexReaderInterface

	// similarity is the Similarity used to score matching documents.
	similarity Similarity

	// readerContext and leafContexts are cached from the reader.
	readerContext index.IndexReaderContext
	leafContexts  []index.LeafReaderContext

	// leafSlices caches the concurrent search partitions.
	leafSlices []LeafSlice

	// taskExecutor handles concurrent execution across slices.
	taskExecutor *TaskExecutor

	// queryTimeout limits search time.
	queryTimeout index.QueryTimeout

	// partialResult is set if a search hit the timeout.
	partialResult bool

	mu sync.RWMutex
}

var (
	// MaxClauseCount is the maximum number of clauses permitted.
	MaxClauseCount = 1024

	// DefaultSimilarity is the default Similarity instance.
	DefaultSimilarity = NewBM25Similarity()
)

// NewIndexSearcher creates a new IndexSearcher.
func NewIndexSearcher(reader index.IndexReaderInterface) *IndexSearcher {
	return NewIndexSearcherWithExecutor(reader, nil)
}

// NewIndexSearcherWithExecutor creates a searcher using a specific executor for concurrency.
func NewIndexSearcherWithExecutor(reader index.IndexReaderInterface, executor Executor) *IndexSearcher {
	ctx := reader.GetContext()
	leaves := ctx.Leaves()

	s := &IndexSearcher{
		reader:        reader,
		similarity:    DefaultSimilarity,
		readerContext: ctx,
		leafContexts:  leaves,
		taskExecutor:  NewTaskExecutor(executor),
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

// GetReader returns the underlying IndexReader of this searcher.
func (s *IndexSearcher) GetReader() index.IndexReaderInterface {
	return s.reader
}

// SetSimilarity sets the Similarity used to score matching documents.
func (s *IndexSearcher) SetSimilarity(similarity Similarity) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if similarity != nil {
		s.similarity = similarity
	}
}

// GetSimilarity returns the Similarity used to score matching documents.
func (s *IndexSearcher) GetSimilarity() Similarity {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.similarity == nil {
		return DefaultSimilarity
	}
	return s.similarity
}

// MaxClauseCount returns the maximum number of clauses permitted.
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

// GetSlices returns the leaf slices used for concurrent searching.
func (s *IndexSearcher) GetSlices() []LeafSlice {
	s.mu.RLock()
	res := s.leafSlices
	s.mu.RUnlock()

	if res == nil {
		s.mu.Lock()
		// Double-checked locking
		if s.leafSlices == nil {
			s.leafSlices = s.computeSlices()
		}
		res = s.leafSlices
		s.mu.Unlock()
	}
	return res
}

func (s *IndexSearcher) computeSlices() []LeafSlice {
	res := slices(s.leafContexts, 250_000, 5, false)
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

// Count counts how many documents match the given query.
func (s *IndexSearcher) Count(query Query) (int, error) {
	// Rewrite the query as a ConstantScoreQuery to avoid computing scores.
	q := NewConstantScoreQuery(query)
	rewritten, err := s.Rewrite(q)
	if err != nil {
		return 0, err
	}

	// Optimization for two-clause pure disjunctions
	innerQuery := rewritten
	if csq, ok := rewritten.(*ConstantScoreQuery); ok {
		innerQuery = csq.Query
	}

	if bq, ok := innerQuery.(*BooleanQuery); ok && !s.reader.HasDeletions() && bq.IsTwoClausePureDisjunctionWithTerms() {
		queries := bq.RewriteTwoClauseDisjunctionWithTermsForCount(s)
		count1, err1 := s.Count(queries[0])
		count2, err2 := s.Count(queries[1])
		if err1 != nil || err2 != nil {
			return 0, fmt.Errorf("error counting disjunction clauses")
		}
		if count1 == 0 || count2 == 0 {
			return int(math.Max(float64(count1), float64(count2))), nil
		} else if float64(int(math.Min(float64(count1), float64(count2))))/float64(int(math.Max(float64(count1), float64(count2)))) < 0.1 {
			count3, err3 := s.Count(queries[2])
			if err3 != nil {
				return 0, err3
			}
			return count1 + count2 - count3, nil
		}
	}

	collectorManager := NewTotalHitCountCollectorManager(s.GetSlices())
	firstCollector := collectorManager.NewCollector()
	weight, err := s.CreateWeight(rewritten, firstCollector.ScoreMode(), 1.0)
	if err != nil {
		return 0, err
	}

	result := s.searchWeight(weight, collectorManager, firstCollector)
	return result.(int), nil
}

// Search finds the top n hits for query.
func (s *IndexSearcher) Search(query Query, n int) (*TopDocs, error) {
	return s.SearchAfter(nil, query, n)
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

	manager := NewTopScoreDocCollectorManager(cappedNumHits, after, 1000)
	return s.searchQuery(query, manager)
}

// SearchWithSort finds the top n hits for query, sorted by the given sort.
func (s *IndexSearcher) SearchWithSort(query Query, n int, sort *Sort, doDocScores bool) (*TopFieldDocs, error) {
	return s.SearchWithSortAfter(nil, query, n, sort, doDocScores)
}

// SearchWithSortAfter finds the top n hits for query, sorted by sort, after a previous result.
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

	rewrittenSort := sort.Rewrite(s)
	manager := NewTopFieldCollectorManager(rewrittenSort, cappedNumHits, after, 1000)

	topDocs := s.searchQuery(query, manager)
	if topDocs == nil {
		return nil, fmt.Errorf("search failed to produce results")
	}

	tfDocs := topDocs.(*TopFieldDocs)
	if doDocScores {
		PopulateScores(tfDocs.ScoreDocs, s, query)
	}
	return tfDocs, nil
}

// Search searches the index using the given collector.
func (s *IndexSearcher) Search(query Query, collector Collector) error {
	rewritten, err := s.Rewrite(query)
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

// SearchWithCollector is a convenience wrapper around Search.
func (s *IndexSearcher) SearchWithCollector(query Query, collector Collector) error {
	return s.Search(query, collector)
}

// SearchWithCollectorManager searches the index using a CollectorManager to parallelize execution.
// It mirrors org.apache.lucene.search.IndexSearcher.search(Query, CollectorManager).
func SearchWithCollectorManager[C Collector, T any](s *IndexSearcher, query Query, manager CollectorManager[C, T]) (T, error) {
	var zero T
	firstCollector, err := manager.NewCollector()
	if err != nil {
		return zero, err
	}

	rewritten, err := s.Rewrite(query, firstCollector.ScoreMode().NeedsScores())
	if err != nil {
		return zero, err
	}

	weight, err := s.CreateWeight(rewritten, firstCollector.ScoreMode(), 1.0)
	if err != nil {
		return zero, err
	}

	slices := s.GetSlices()
	if len(slices) == 0 {
		return manager.Reduce([]C{firstCollector})
	}

	collectors := make([]C, len(slices))
	collectors[0] = firstCollector
	scoreMode := firstCollector.ScoreMode()

	for i := 1; i < len(slices); i++ {
		c, err := manager.NewCollector()
		if err != nil {
			return zero, err
		}
		if c.ScoreMode() != scoreMode {
			return zero, fmt.Errorf("CollectorManager does not always produce collectors with the same score mode")
		}
		collectors[i] = c
	}

	tasks := make([]func() Collector, len(slices))
	for i := 0; i < len(slices); i++ {
		partitions := slices[i].Partitions
		c := collectors[i]
		tasks[i] = func() Collector {
			s.searchPartitions(partitions, weight, c)
			return c
		}
	}

	results := s.taskExecutor.InvokeAll(tasks)
	// results is []Collector, we need to convert it to []C.
	typedResults := make([]C, len(results))
	for i, r := range results {
		typedResults[i] = r.(C)
	}

	return manager.Reduce(typedResults)
}

func (s *IndexSearcher) searchPartitions(partitions []LeafReaderContextPartition, weight Weight, collector Collector) {
	collector.SetWeight(weight)

	for _, partition := range partitions {
		s.searchLeaf(partition.Ctx, partition.MinDocId, partition.MaxDocId, weight, collector)
	}
}

func (s *IndexSearcher) searchLeaf(ctx index.LeafReaderContext, minDocId, maxDocId int, weight Weight, collector Collector) {
	leafCollector, err := collector.GetLeafCollector(ctx)
	if err != nil {
		if IsCollectionTerminated(err) {
			return
		}
		panic(err)
	}

	scorerSupplier := weight.ScorerSupplier(ctx)
	if scorerSupplier != nil {
		scorerSupplier.SetTopLevelScoringClause()
		scorer := scorerSupplier.BulkScorer()

		if s.queryTimeout != nil {
			scorer = NewTimeLimitingBulkScorer(scorer, s.queryTimeout)
		}

		var liveDocs util.Bits
		if lr, ok := ctx.Reader().(interface{ GetLiveDocs() util.Bits }); ok {
			liveDocs = lr.GetLiveDocs()
		}

		for {
			doc, err := scorer.NextDoc()
			if err != nil {
				panic(err)
			}
			if doc == NO_MORE_DOCS {
				break
			}
			if liveDocs != nil && !liveDocs.Get(doc) {
				continue
			}

			err = leafCollector.Collect(doc)
			if err != nil {
				if IsCollectionTerminated(err) {
					break
				}
				panic(err)
			}
		}
	}
	leafCollector.Finish()
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

func (s *IndexSearcher) Rewrite(original Query, needsScores bool) (Query, error) {
	if needsScores {
		return s.Rewrite(original)
	}
	return s.Rewrite(NewConstantScoreQuery(original))
}

func getNumClausesCheckVisitor() *numClausesCheckVisitor {
	return &numClausesCheckVisitor{}
}

type numClausesCheckVisitor struct {
	numClauses int
}

func (v *numClausesCheckVisitor) GetSubVisitor(occur Occur, parent Query) QueryVisitor {
	return v
}

func (v *numClausesCheckVisitor) VisitLeaf(query Query) {
	if v.numClauses > MaxClauseCount {
		panic(&TooManyNestedClauses{TooManyClauses{MaxClauseCount: MaxClauseCount}})
	}
	v.numClauses++
}

func (v *numClausesCheckVisitor) ConsumeTerms(query Query, terms ...Term) {
	if v.numClauses > MaxClauseCount {
		panic(&TooManyNestedClauses{TooManyClauses{MaxClauseCount: MaxClauseCount}})
	}
	v.numClauses++
}

func (v *numClausesCheckVisitor) ConsumeTermsMatching(query Query, field string, automaton func() ByteRunAutomaton) {
	if v.numClauses > MaxClauseCount {
		panic(&TooManyNestedClauses{TooManyClauses{MaxClauseCount: MaxClauseCount}})
	}
	v.numClauses++
}

// CreateWeight builds the Weight for the given query.
func (s *IndexSearcher) CreateWeight(query Query, scoreMode ScoreMode, boost float32) (Weight, error) {
	weight, err := query.CreateWeight(s, scoreMode, boost)
	if err != nil {
		return nil, err
	}
	return weight, nil
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

func (s *IndexSearcher) explainWeight(weight Weight, doc int) (Explanation, error) {
	docBase := 0
	for ord, sr := range s.reader.GetSegmentReaders() {
		maxDoc := sr.MaxDoc()
		if doc >= docBase && doc < docBase+maxDoc {
			return s.explainLeaf(sr, ord, docBase, weight, doc-docBase, doc)
		}
		docBase += maxDoc
	}
	return NoMatchExplanation(fmt.Sprintf("Document %d is out of range", doc)), nil
}

func (s *IndexSearcher) explainLeaf(reader index.IndexReaderInterface, ord, docBase int, weight Weight, leafDoc, globalDoc int) (Explanation, error) {
	if lr, ok := reader.(interface{ GetLiveDocs() util.Bits }); ok {
		if liveDocs := lr.GetLiveDocs(); liveDocs != nil && !liveDocs.Get(leafDoc) {
			return NoMatchExplanation(fmt.Sprintf("Document %d is deleted", globalDoc)), nil
		}
	}
	ctx := index.NewLeafReaderContext(reader, nil, ord, docBase)
	return weight.Explain(ctx, leafDoc)
}

// TermStatistics returns statistics for a term.
func (s *IndexSearcher) TermStatistics(term Term, docFreq int, totalTermFreq int64) TermStatistics {
	return NewTermStatistics(term.Bytes(), docFreq, totalTermFreq)
}

// CollectionStatistics returns statistics for a field.
func (s *IndexSearcher) CollectionStatistics(field string) (*CollectionStatistics, error) {
	var docCount int64
	var sumTotalTermFreq int64
	var sumDocFreq int64

	for _, leaf := range s.leafContexts {
		terms := GetTerms(leaf.Reader(), field)
		docCount += int64(terms.DocCount())
		sumTotalTermFreq += terms.SumTotalTermFreq()
		sumDocFreq += int64(terms.SumDocFreq())
	}

	if docCount == 0 {
		return nil, nil
	}

	return NewCollectionStatistics(field, s.reader.MaxDoc(), docCount, sumTotalTermFreq, sumDocFreq), nil
}

// Doc returns the stored fields for a document.
func (s *IndexSearcher) Doc(docID int) (*document.Document, error) {
	docBase := 0
	for _, sr := range s.reader.GetSegmentReaders() {
		maxDoc := sr.MaxDoc()
		if docID >= docBase && docID < docBase+maxDoc {
			return s.docFromSegment(sr, docID-docBase)
		}
		docBase += maxDoc
	}
	return nil, nil
}

func (s *IndexSearcher) docFromSegment(sr *index.SegmentReader, docID int) (*document.Document, error) {
	storedFields, err := sr.StoredFields()
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

func (v *DocumentVisitor) StringField(field, value string) {
	sf, _ := document.NewStoredField(field, value)
	v.doc.Add(sf)
}

func (v *DocumentVisitor) BinaryField(field string, value []byte) {
	sf, _ := document.NewStoredFieldFromBytes(field, value)
	v.doc.Add(sf)
}

func (v *DocumentVisitor) IntField(field string, value int) {
	sf, _ := document.NewStoredFieldFromInt(field, value)
	v.doc.Add(sf)
}

func (v *DocumentVisitor) LongField(field string, value int64) {
	sf, _ := document.NewStoredFieldFromInt64(field, value)
	v.doc.Add(sf)
}

func (v *DocumentVisitor) FloatField(field string, value float32) {
	sf, _ := document.NewStoredFieldFromFloat64(field, float64(value))
	v.doc.Add(sf)
}

func (v *DocumentVisitor) DoubleField(field string, value float64) {
	sf, _ := document.NewStoredFieldFromFloat64(field, value)
	v.doc.Add(sf)
}

func (v *DocumentVisitor) Document() *document.Document {
	return v.doc
}

// LeafSlice holds a subset of leaf contexts to be executed in a single thread.
type LeafSlice struct {
	Partitions []LeafReaderContextPartition
}

func entireSegments(contexts []index.LeafReaderContext) LeafSlice {
	parts := make([]LeafReaderContextPartition, len(contexts))
	for i, ctx := range contexts {
		parts[i] = NewLeafReaderContextPartitionForEntireSegment(ctx)
	}
	return LeafSlice{Partitions: parts}
}

func slices(leaves []index.LeafReaderContext, maxDocsPerSlice, maxSegmentsPerSlice int, allowSegmentPartitions bool) []LeafSlice {
	sortedLeaves := make([]index.LeafReaderContext, len(leaves))
	copy(sortedLeaves, leaves)

	sort.Slice(sortedLeaves, func(i, j int) bool {
		return sortedLeaves[i].Reader().MaxDoc() > sortedLeaves[j].Reader().MaxDoc()
	})

	if allowSegmentPartitions {
		return slicesWithSegmentPartitions(maxDocsPerSlice, maxSegmentsPerSlice, sortedLeaves)
	}

	var groupedLeaves [][]index.LeafReaderContext
	var docSum int
	var group []index.LeafReaderContext

	for _, ctx := range sortedLeaves {
		if ctx.Reader().MaxDoc() > maxDocsPerSlice {
			group = nil
			groupedLeaves = append(groupedLeaves, []index.LeafReaderContext{ctx})
		} else {
			if group == nil {
				group = []index.LeafReaderContext{ctx}
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

func slicesWithSegmentPartitions(maxDocsPerSlice, maxSegmentsPerSlice int, sortedLeaves []index.LeafReaderContext) []LeafSlice {
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

// LeafReaderContextPartition targets a specific doc id range of a LeafReaderContext.
type LeafReaderContextPartition struct {
	MinDocId int
	MaxDocId int
	Ctx      index.LeafReaderContext
}

func NewLeafReaderContextPartitionForEntireSegment(ctx index.LeafReaderContext) LeafReaderContextPartition {
	return LeafReaderContextPartition{
		MinDocId: 0,
		MaxDocId: NO_MORE_DOCS,
		Ctx:      ctx,
	}
}

func NewLeafReaderContextPartitionFromAndTo(ctx index.LeafReaderContext, minDocId, maxDocId int) LeafReaderContextPartition {
	return LeafReaderContextPartition{
		MinDocId: minDocId,
		MaxDocId: maxDocId,
		Ctx:      ctx,
	}
}

// TaskExecutor handles concurrent execution of tasks.
type TaskExecutor struct {
	executor Executor
}

func NewTaskExecutor(executor Executor) *TaskExecutor {
	return &TaskExecutor{executor: executor}
}

func (te *TaskExecutor) InvokeAll(tasks []func() Collector) []Collector {
	if te.executor == nil {
		results := make([]Collector, len(tasks))
		for i, task := range tasks {
			results[i] = task()
		}
		return results
	}

	results := make([]Collector, len(tasks))
	var wg sync.WaitGroup
	wg.Add(len(tasks))

	for i, task := range tasks {
		go func(idx int, t func() Collector) {
			defer wg.Done()
			results[idx] = t()
		}(i, task)
	}
	wg.Wait()
	return results
}

// Executor is an interface for executing tasks concurrently.
type Executor interface {
	Execute(runnable func())
}
