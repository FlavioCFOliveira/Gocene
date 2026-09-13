// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// Collector is used to gather raw results from a search, and implement sorting
// or custom result filtering, collation, etc.
//
// Lucene's core collectors are derived from Collector and SimpleCollector.
// Likely your application can use one of these classes, or subclass
// TopDocsCollector, instead of implementing Collector directly.
type Collector interface {
	// GetLeafCollector creates a new LeafCollector to collect the given context.
	GetLeafCollector(context *index.LeafReaderContext) (LeafCollector, error)

	// ScoreMode indicates what features are required from the scorer.
	ScoreMode() ScoreMode

	// SetWeight sets the Weight that will be used to produce scorers that will feed
	// LeafCollectors. This is typically useful to have access to Weight.Count from
	// Collector.GetLeafCollector.
	SetWeight(weight Weight)
}

// BaseCollector carries the default methods of the interface
// org.apache.lucene.search.Collector (Lucene 10.5.0): setWeight(Weight).
//
// Java's getLeafCollector(LeafReaderContext) and scoreMode() have no default
// body and are therefore not provided here: the embedder must supply them.
type BaseCollector struct{}

// SetWeight mirrors the default body of Collector.setWeight(Weight), which is
// empty.
func (c *BaseCollector) SetWeight(weight Weight) {}

// FilterCollector is a Collector delegator.
type FilterCollector struct {
	In Collector
}

func (f *FilterCollector) GetLeafCollector(context *index.LeafReaderContext) (LeafCollector, error) {
	return f.In.GetLeafCollector(context)
}

func (f *FilterCollector) SetWeight(weight Weight) {
	f.In.SetWeight(weight)
}

func (f *FilterCollector) ScoreMode() ScoreMode {
	return f.In.ScoreMode()
}

// LeafCollector collects documents from a single segment.
type LeafCollector interface {
	// SetScorer is called before successive calls to Collect.
	SetScorer(scorer Scorable) error

	// Collect is called once for every document matching a query.
	Collect(doc int) error

	// CollectRange collects a range of doc IDs.
	CollectRange(min, max int) error

	// CollectStream bulk-collects doc IDs from a DocIdStream.
	CollectStream(stream DocIdStream) error

	// CompetitiveIterator optionally returns an iterator over competitive documents.
	CompetitiveIterator() (DocIdSetIterator, error)

	// Finish is called once the leaf has finished collecting.
	Finish() error
}

// FilterLeafCollector is a LeafCollector delegator.
type FilterLeafCollector struct {
	In LeafCollector
}

func (f *FilterLeafCollector) SetScorer(scorer Scorable) error {
	return f.In.SetScorer(scorer)
}

func (f *FilterLeafCollector) Collect(doc int) error {
	return f.In.Collect(doc)
}

func (f *FilterLeafCollector) CollectRange(min, max int) error {
	return f.In.CollectRange(min, max)
}

func (f *FilterLeafCollector) CollectStream(stream DocIdStream) error {
	return f.In.CollectStream(stream)
}

func (f *FilterLeafCollector) CompetitiveIterator() (DocIdSetIterator, error) {
	return f.In.CompetitiveIterator()
}

func (f *FilterLeafCollector) Finish() error {
	return f.In.Finish()
}

// SimpleCollector is a base Collector implementation that is used to collect all contexts.
type SimpleCollector interface {
	Collector
	LeafCollector

	// DoSetNextReader is called before collecting context.
	DoSetNextReader(context *index.LeafReaderContext) error
}

// BaseSimpleCollector provides a default implementation of SimpleCollector.
//
// Mirrors the abstract class org.apache.lucene.search.SimpleCollector, which
// implements both Collector and LeafCollector and whose getLeafCollector
// returns `this`. Go embedding cannot reach the embedder, so the concrete
// collector registers itself in Outer, the idiom the package already uses for
// Java's "return this" bases.
type BaseSimpleCollector struct {
	// Outer is the concrete SimpleCollector that embeds this base. It renders
	// Java's `this` in getLeafCollector.
	Outer LeafCollector
}

// GetLeafCollector creates a new LeafCollector to collect the given context.
//
// Mirrors `public final LeafCollector getLeafCollector(LeafReaderContext)`.
func (s *BaseSimpleCollector) GetLeafCollector(context *index.LeafReaderContext) (LeafCollector, error) {
	if err := s.DoSetNextReader(context); err != nil {
		return nil, err
	}
	return s.Outer, nil
}

// DoSetNextReader is called before collecting context.
func (s *BaseSimpleCollector) DoSetNextReader(context *index.LeafReaderContext) error {
	return nil
}

// SetScorer is called before successive calls to Collect.
func (s *BaseSimpleCollector) SetScorer(scorer Scorable) error {
	return nil
}

// SetWeight provides a default empty implementation of Collector.SetWeight.
func (s *BaseSimpleCollector) SetWeight(weight Weight) {
	// Default: do nothing.
}

// ScoreMode returns the score mode. BaseSimpleCollector defaults to ScoreMode_NONE.
func (s *BaseSimpleCollector) ScoreMode() ScoreMode {
	return COMPLETE_NO_SCORES
}

// Note: Collect(doc int) must be implemented by the actual collector.

// BaseLeafCollector carries the default methods of the interface
// org.apache.lucene.search.LeafCollector (Lucene 10.5.0): competitiveIterator()
// and finish().
//
// Java's setScorer(Scorable) and collect(int) have no default body and are
// therefore not provided here: the embedder must supply them.
//
// LeafCollector's other two defaults, collectRange(int, int) and
// collect(DocIdStream), both dispatch back to the abstract collect(int), which
// an embedded Go struct cannot reach; they are therefore rendered as the free
// functions DefaultCollectRange and DefaultCollectStream below, which take the
// concrete collector explicitly. This mirrors the idiom the port already uses
// for such self-dispatching defaults (see DefaultNextDocsAndScores in
// scorer.go). An implementation with a cheaper bulk path — as several Java ones
// have — simply declares its own method instead of calling these.
type BaseLeafCollector struct{}

// NewBaseLeafCollector creates a BaseLeafCollector.
func NewBaseLeafCollector() *BaseLeafCollector {
	return &BaseLeafCollector{}
}

// CompetitiveIterator mirrors the default body of
// LeafCollector.competitiveIterator(), which returns null.
func (c *BaseLeafCollector) CompetitiveIterator() (DocIdSetIterator, error) {
	return nil, nil
}

// Finish mirrors the default body of LeafCollector.finish(), which is empty.
func (c *BaseLeafCollector) Finish() error {
	return nil
}

// DefaultCollectRange is the body of LeafCollector.collectRange(int, int) in
// Apache Lucene 10.5.0: collect(new RangeDocIdStream(min, max)).
func DefaultCollectRange(lc LeafCollector, min, max int) error {
	return lc.CollectStream(NewRangeDocIdStream(min, max))
}

// DefaultCollectStream is the body of LeafCollector.collect(DocIdStream) in
// Apache Lucene 10.5.0: stream.forEach(this::collect).
func DefaultCollectStream(lc LeafCollector, stream DocIdStream) error {
	return stream.ForEach(lc.Collect)
}
