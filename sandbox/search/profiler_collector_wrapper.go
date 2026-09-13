// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.sandbox.search.ProfilerCollectorWrapper.
package search

import (
	"time"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// ProfilerCollectorWrapper wraps a search.Collector and measures the total
// nanoseconds spent calling it. ScoreMode, GetLeafCollector, LeafCollector.Collect,
// and LeafCollector.SetScorer are all timed.
//
// Mirrors org.apache.lucene.sandbox.search.ProfilerCollectorWrapper.
type ProfilerCollectorWrapper struct {
	in   search.Collector
	time int64 // accumulated nanoseconds
}

// NewProfilerCollectorWrapper wraps in with profiling instrumentation.
func NewProfilerCollectorWrapper(in search.Collector) *ProfilerCollectorWrapper {
	return &ProfilerCollectorWrapper{in: in}
}

// ScoreMode times the delegation to the inner collector.
func (c *ProfilerCollectorWrapper) ScoreMode() search.ScoreMode {
	start := time.Now().UnixNano()
	mode := c.in.ScoreMode()
	elapsed := time.Now().UnixNano() - start
	if elapsed < 1 {
		elapsed = 1
	}
	c.time += elapsed
	return mode
}

// GetLeafCollector times the creation of the inner leaf collector and returns
// a profilerLeafCollectorWrapper that times Collect and SetScorer.
func (c *ProfilerCollectorWrapper) GetLeafCollector(context *index.LeafReaderContext) (search.LeafCollector, error) {
	start := time.Now().UnixNano()
	inner, err := c.in.GetLeafCollector(context)
	elapsed := time.Now().UnixNano() - start
	if elapsed < 1 {
		elapsed = 1
	}
	c.time += elapsed
	if err != nil {
		return nil, err
	}
	return &profilerLeafCollectorWrapper{BaseLeafCollector: search.NewBaseLeafCollector(), in: inner, parent: c}, nil
}

// SetWeight delegates to the wrapped collector.
//
// Java: ProfilerCollectorWrapper extends FilterCollector, which delegates
// setWeight to the wrapped collector.
func (c *ProfilerCollectorWrapper) SetWeight(weight search.Weight) {
	c.in.SetWeight(weight)
}

// GetTime returns the total nanoseconds spent in this collector so far.
func (c *ProfilerCollectorWrapper) GetTime() int64 {
	return c.time
}

var _ search.Collector = (*ProfilerCollectorWrapper)(nil)

// profilerLeafCollectorWrapper wraps a search.LeafCollector, timing each
// Collect and SetScorer call and accumulating into the parent's time counter.
type profilerLeafCollectorWrapper struct {
	// Java's inner class extends FilterLeafCollector, which supplies
	// competitiveIterator() and finish().
	*search.BaseLeafCollector

	in     search.LeafCollector
	parent *ProfilerCollectorWrapper
}

// Collect times the call and delegates to the inner leaf collector.
func (lc *profilerLeafCollectorWrapper) Collect(doc int) error {
	start := time.Now().UnixNano()
	err := lc.in.Collect(doc)
	elapsed := time.Now().UnixNano() - start
	if elapsed < 1 {
		elapsed = 1
	}
	lc.parent.time += elapsed
	return err
}

// CollectRange carries the LeafCollector default, which dispatches back to
// Collect; Java's inner class extends FilterLeafCollector and does not
// override it.
func (lc *profilerLeafCollectorWrapper) CollectRange(min, max int) error {
	return search.DefaultCollectRange(lc, min, max)
}

// CollectStream carries the LeafCollector default, which dispatches back to
// Collect.
func (lc *profilerLeafCollectorWrapper) CollectStream(stream search.DocIdStream) error {
	return search.DefaultCollectStream(lc, stream)
}

// SetScorer times the call and delegates to the inner leaf collector.
func (lc *profilerLeafCollectorWrapper) SetScorer(scorer search.Scorable) error {
	start := time.Now().UnixNano()
	err := lc.in.SetScorer(scorer)
	elapsed := time.Now().UnixNano() - start
	if elapsed < 1 {
		elapsed = 1
	}
	lc.parent.time += elapsed
	return err
}

var _ search.LeafCollector = (*profilerLeafCollectorWrapper)(nil)
