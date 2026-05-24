// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of
// org.apache.lucene.sandbox.facet.plain.histograms.HistogramCollectorManager.
package histograms

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/internal/hppc"
)

// defaultMaxBuckets mirrors HistogramCollectorManager.DEFAULT_MAX_BUCKETS.
const defaultMaxBuckets = 1024

// HistogramCollectorManager is a CollectorManager that computes a histogram
// of the distribution of the values of a numeric field.
//
// It takes a bucketWidth parameter and counts the number of documents that
// fall into intervals [0, bucketWidth), [bucketWidth, 2*bucketWidth), etc.
// The keys of the returned map identify these intervals as the quotient of
// the integer division by bucketWidth. A key equal to k maps to values in
// [k * bucketWidth, (k+1) * bucketWidth).
//
// Mirrors
// org.apache.lucene.sandbox.facet.plain.histograms.HistogramCollectorManager.
type HistogramCollectorManager struct {
	field       string
	bucketWidth int64
	maxBuckets  int
	// leafBulkCollected is shared across all collectors created by this
	// manager. It tracks which segment keys have already been processed
	// through the PointTree bulk-collection path so that concurrent
	// collectors in the same segment do not double-count.
	leafBulkCollected sync.Map
}

// NewHistogramCollectorManager creates a manager with the default maximum
// of 1024 buckets.
//
// bucketWidth must be at least 2.
//
// Mirrors HistogramCollectorManager(String, long).
func NewHistogramCollectorManager(field string, bucketWidth int64) (*HistogramCollectorManager, error) {
	return NewHistogramCollectorManagerWithMax(field, bucketWidth, defaultMaxBuckets)
}

// NewHistogramCollectorManagerWithMax creates a manager with an explicit
// bucket limit.
//
// bucketWidth must be at least 2. maxBuckets must be at least 1.
//
// Mirrors HistogramCollectorManager(String, long, int).
func NewHistogramCollectorManagerWithMax(field string, bucketWidth int64, maxBuckets int) (*HistogramCollectorManager, error) {
	if field == "" {
		return nil, fmt.Errorf("HistogramCollectorManager: field must not be empty")
	}
	if bucketWidth < 2 {
		return nil, fmt.Errorf("HistogramCollectorManager: bucketWidth must be at least 2, got %d", bucketWidth)
	}
	if maxBuckets < 1 {
		return nil, fmt.Errorf("HistogramCollectorManager: maxBuckets must be at least 1, got %d", maxBuckets)
	}
	return &HistogramCollectorManager{
		field:       field,
		bucketWidth: bucketWidth,
		maxBuckets:  maxBuckets,
	}, nil
}

// NewCollector allocates a new HistogramCollector that shares the
// leafBulkCollected map with this manager.
//
// Mirrors HistogramCollectorManager.newCollector().
func (m *HistogramCollectorManager) NewCollector() *HistogramCollector {
	return newHistogramCollector(m.field, m.bucketWidth, m.maxBuckets, &m.leafBulkCollected)
}

// Reduce merges the counts from all collectors into a single map.
//
// Mirrors HistogramCollectorManager.reduce(Collection<HistogramCollector>).
func (m *HistogramCollectorManager) Reduce(collectors []*HistogramCollector) (hppc.LongIntHashMap, error) {
	reduced := make(hppc.LongIntHashMap)
	for _, c := range collectors {
		for k, v := range c.GetCounts() {
			reduced[k] += v
			CheckMaxBuckets(len(reduced), m.maxBuckets)
		}
	}
	return reduced, nil
}

// Field returns the name of the field being histogrammed.
func (m *HistogramCollectorManager) Field() string { return m.field }

// BucketWidth returns the width of each histogram bucket.
func (m *HistogramCollectorManager) BucketWidth() int64 { return m.bucketWidth }

// MaxBuckets returns the maximum allowed number of buckets.
func (m *HistogramCollectorManager) MaxBuckets() int { return m.maxBuckets }
