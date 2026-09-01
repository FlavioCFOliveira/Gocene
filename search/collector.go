package search

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// Collector is used to gather raw results from a search.
type Collector interface {
	// GetLeafCollector creates a new LeafCollector to collect the given context.
	GetLeafCollector(context *index.LeafReaderContext) (LeafCollector, error)

	// ScoreMode indicates what features are required from the scorer.
	ScoreMode() ScoreMode

	// SetWeight sets the Weight that will be used to produce scorers.
	SetWeight(weight Weight)
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

func (s *BaseSimpleCollector) GetLeafCollector(context *index.LeafReaderContext) (LeafCollector, error) {
	if err := s.DoSetNextReader(context); err != nil {
		return nil, err
	}
	return s
}

func (s *BaseSimpleCollector) DoSetNextReader(context *index.LeafReaderContext) error {
	return nil
}

func (s *BaseSimpleCollector) SetScorer(scorer Scorable) error {
	return nil
}

// Note: Collect(doc int) must be implemented by the actual collector.

// CollectorManager is a manager of collectors.
type CollectorManager[C Collector, T any] interface {
	// NewCollector returns a new Collector.
	NewCollector() (C, error)

	// Reduce reduces the results of individual collectors into a meaningful result.
	Reduce(collectors []C) (T, error)
}
