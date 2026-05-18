// Package search implements org.apache.lucene.sandbox.search.
package search

import (
	"sort"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// CombinedFieldQuery is the sandbox variant that scores documents across
// multiple fields combined into a virtual one. Mirrors
// org.apache.lucene.sandbox.search.CombinedFieldQuery.
type CombinedFieldQuery struct {
	Fields []string
	Term   string
}

// NewCombinedFieldQuery builds the query.
func NewCombinedFieldQuery(term string, fields ...string) *CombinedFieldQuery {
	return &CombinedFieldQuery{Term: term, Fields: append([]string(nil), fields...)}
}

// CoveringQuery accepts documents that match at least minMatch sub-queries.
// Mirrors org.apache.lucene.sandbox.search.CoveringQuery.
type CoveringQuery struct {
	SubQueries []search.Query
	MinMatch   int
}

// NewCoveringQuery builds the query.
func NewCoveringQuery(minMatch int, subQueries ...search.Query) *CoveringQuery {
	if minMatch < 1 {
		minMatch = 1
	}
	return &CoveringQuery{SubQueries: append([]search.Query(nil), subQueries...), MinMatch: minMatch}
}

// MultiRangeQuery is the base for sandbox multi-range queries.
type MultiRangeQuery struct {
	Field string
}

// NewMultiRangeQuery builds the base.
func NewMultiRangeQuery(field string) *MultiRangeQuery { return &MultiRangeQuery{Field: field} }

// DocValuesMultiRangeQuery is the DocValues-backed variant.
type DocValuesMultiRangeQuery struct {
	*MultiRangeQuery
	Ranges [][2]int64
}

// NewDocValuesMultiRangeQuery builds the query.
func NewDocValuesMultiRangeQuery(field string, ranges [][2]int64) *DocValuesMultiRangeQuery {
	clone := make([][2]int64, len(ranges))
	copy(clone, ranges)
	return &DocValuesMultiRangeQuery{MultiRangeQuery: NewMultiRangeQuery(field), Ranges: clone}
}

// SortedNumericDocValuesMultiRangeQuery is the sorted-numeric variant.
type SortedNumericDocValuesMultiRangeQuery struct {
	*DocValuesMultiRangeQuery
}

// NewSortedNumericDocValuesMultiRangeQuery builds the query.
func NewSortedNumericDocValuesMultiRangeQuery(field string, ranges [][2]int64) *SortedNumericDocValuesMultiRangeQuery {
	return &SortedNumericDocValuesMultiRangeQuery{DocValuesMultiRangeQuery: NewDocValuesMultiRangeQuery(field, ranges)}
}

// SortedSetDocValuesMultiRangeQuery is the sorted-set variant.
type SortedSetDocValuesMultiRangeQuery struct {
	Field  string
	Terms  [][2][]byte
}

// NewSortedSetDocValuesMultiRangeQuery builds the query.
func NewSortedSetDocValuesMultiRangeQuery(field string, terms [][2][]byte) *SortedSetDocValuesMultiRangeQuery {
	clone := make([][2][]byte, len(terms))
	for i, t := range terms {
		clone[i] = [2][]byte{append([]byte(nil), t[0]...), append([]byte(nil), t[1]...)}
	}
	return &SortedSetDocValuesMultiRangeQuery{Field: field, Terms: clone}
}

// LargeNumHitsTopDocsCollector keeps the top-N hits when N is very large.
type LargeNumHitsTopDocsCollector struct {
	NumHits int
	hits    []Hit
}

// Hit is the (doc, score) tuple this collector records.
type Hit struct {
	Doc   int
	Score float32
}

// NewLargeNumHitsTopDocsCollector builds the collector.
func NewLargeNumHitsTopDocsCollector(numHits int) *LargeNumHitsTopDocsCollector {
	if numHits < 1 {
		numHits = 1
	}
	return &LargeNumHitsTopDocsCollector{NumHits: numHits}
}

// Collect records a hit and keeps the top-N by descending score.
func (c *LargeNumHitsTopDocsCollector) Collect(doc int, score float32) {
	c.hits = append(c.hits, Hit{Doc: doc, Score: score})
	sort.SliceStable(c.hits, func(i, j int) bool { return c.hits[i].Score > c.hits[j].Score })
	if len(c.hits) > c.NumHits {
		c.hits = c.hits[:c.NumHits]
	}
}

// Hits returns a copy of the accumulated hits.
func (c *LargeNumHitsTopDocsCollector) Hits() []Hit {
	out := make([]Hit, len(c.hits))
	copy(out, c.hits)
	return out
}

// PhraseWildcardQuery is the phrase query whose terms may include wildcards.
// Mirrors org.apache.lucene.sandbox.search.PhraseWildcardQuery.
type PhraseWildcardQuery struct {
	Field string
	Terms []string
}

// NewPhraseWildcardQuery builds the query.
func NewPhraseWildcardQuery(field string, terms ...string) *PhraseWildcardQuery {
	return &PhraseWildcardQuery{Field: field, Terms: append([]string(nil), terms...)}
}

// TermAutomatonQuery is the automaton-backed multi-term phrase query.
type TermAutomatonQuery struct {
	Field string
}

// NewTermAutomatonQuery builds the query.
func NewTermAutomatonQuery(field string) *TermAutomatonQuery {
	return &TermAutomatonQuery{Field: field}
}

// TokenStreamToTermAutomatonQuery is the helper that builds a
// TermAutomatonQuery from a token stream. Mirrors
// org.apache.lucene.sandbox.search.TokenStreamToTermAutomatonQuery.
type TokenStreamToTermAutomatonQuery struct{}

// Build returns a TermAutomatonQuery for field; concrete automaton building
// is delegated to the caller.
func (TokenStreamToTermAutomatonQuery) Build(field string) *TermAutomatonQuery {
	return NewTermAutomatonQuery(field)
}

// QueryProfilerTimingType enumerates the timer buckets the profiler tracks.
type QueryProfilerTimingType int

const (
	// TimingBuild covers query build time.
	TimingBuild QueryProfilerTimingType = iota
	// TimingScore covers scoring time.
	TimingScore
	// TimingMatch covers matching time.
	TimingMatch
)

// QueryLeafProfilerBreakdown records timing per timer type on a leaf.
type QueryLeafProfilerBreakdown struct {
	Timings map[QueryProfilerTimingType]int64
}

// NewQueryLeafProfilerBreakdown builds the breakdown.
func NewQueryLeafProfilerBreakdown() *QueryLeafProfilerBreakdown {
	return &QueryLeafProfilerBreakdown{Timings: make(map[QueryProfilerTimingType]int64)}
}

// Add records nanoseconds for kind.
func (b *QueryLeafProfilerBreakdown) Add(kind QueryProfilerTimingType, nanos int64) {
	b.Timings[kind] += nanos
}

// QueryProfilerResult aggregates per-leaf timings for one query.
type QueryProfilerResult struct {
	QueryName string
	Leaves    []*QueryLeafProfilerBreakdown
}

// NewQueryProfilerResult builds the result.
func NewQueryProfilerResult(name string) *QueryProfilerResult {
	return &QueryProfilerResult{QueryName: name}
}

// AggregatedQueryLeafProfilerResult merges multiple leaf breakdowns into one
// summary entry.
type AggregatedQueryLeafProfilerResult struct {
	Totals map[QueryProfilerTimingType]int64
}

// NewAggregatedQueryLeafProfilerResult builds the helper.
func NewAggregatedQueryLeafProfilerResult() *AggregatedQueryLeafProfilerResult {
	return &AggregatedQueryLeafProfilerResult{Totals: make(map[QueryProfilerTimingType]int64)}
}

// Add merges a leaf breakdown.
func (a *AggregatedQueryLeafProfilerResult) Add(b *QueryLeafProfilerBreakdown) {
	for kind, nanos := range b.Timings {
		a.Totals[kind] += nanos
	}
}

// ProfilerCollector is the collector used while profiling.
type ProfilerCollector struct {
	Result *QueryProfilerResult
}

// NewProfilerCollector builds the collector for the named query.
func NewProfilerCollector(name string) *ProfilerCollector {
	return &ProfilerCollector{Result: NewQueryProfilerResult(name)}
}

// ProfilerCollectorManager builds ProfilerCollectors per leaf.
type ProfilerCollectorManager struct {
	QueryName string
}

// NewProfilerCollectorManager builds the manager.
func NewProfilerCollectorManager(name string) *ProfilerCollectorManager {
	return &ProfilerCollectorManager{QueryName: name}
}

// NewCollector returns a fresh ProfilerCollector.
func (m *ProfilerCollectorManager) NewCollector() *ProfilerCollector {
	return NewProfilerCollector(m.QueryName)
}

// ProfilerCollectorResult bundles the per-leaf breakdowns into a final result.
type ProfilerCollectorResult struct {
	Result *QueryProfilerResult
}

// NewProfilerCollectorResult builds the helper.
func NewProfilerCollectorResult(r *QueryProfilerResult) *ProfilerCollectorResult {
	return &ProfilerCollectorResult{Result: r}
}

// QueryProfilerIndexSearcher wraps a search.IndexSearcher with profiling hooks.
type QueryProfilerIndexSearcher struct {
	Searcher *search.IndexSearcher
	Result   *QueryProfilerResult
}

// NewQueryProfilerIndexSearcher builds the wrapper.
func NewQueryProfilerIndexSearcher(s *search.IndexSearcher) *QueryProfilerIndexSearcher {
	return &QueryProfilerIndexSearcher{Searcher: s, Result: NewQueryProfilerResult("query")}
}
