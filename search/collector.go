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
type BaseSimpleCollector struct {
	// Put fields if needed
}

// GetLeafCollector creates a new LeafCollector to collect the given context.
func (s *BaseSimpleCollector) GetLeafCollector(context *index.LeafReaderContext) (LeafCollector, error) {
	if err := s.DoSetNextReader(context); err != nil {
		return nil, err
	}
	return s
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
	return ScoreMode_NONE
}

// Note: Collect(doc int) must be implemented by the actual collector.

