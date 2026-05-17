// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "errors"

// TopFieldCollectorManager creates TopFieldCollector instances and reduces
// their per-segment results into a single TopFieldDocs.
//
// Mirrors org.apache.lucene.search.TopFieldCollectorManager. A fresh manager
// should be created per search because the Lucene equivalent maintains
// internal state across NewCollector calls.
type TopFieldCollectorManager struct {
	sort               *Sort
	numHits            int
	after              *ScoreDoc
	totalHitsThreshold int
}

// NewTopFieldCollectorManager constructs a TopFieldCollectorManager.
// numHits must be > 0, totalHitsThreshold must be >= 0, and sort must contain
// at least one SortField.
func NewTopFieldCollectorManager(sort *Sort, numHits int, after *ScoreDoc, totalHitsThreshold int) (*TopFieldCollectorManager, error) {
	if sort == nil || len(sort.Fields) == 0 {
		return nil, errors.New("TopFieldCollectorManager: sort with at least one field is required")
	}
	if numHits <= 0 {
		return nil, errors.New("TopFieldCollectorManager: numHits must be > 0")
	}
	if totalHitsThreshold < 0 {
		return nil, errors.New("TopFieldCollectorManager: totalHitsThreshold must be >= 0")
	}
	return &TopFieldCollectorManager{
		sort:               sort,
		numHits:            numHits,
		after:              after,
		totalHitsThreshold: totalHitsThreshold,
	}, nil
}

// NewTopFieldCollectorManagerSimple is the convenience two-argument form,
// matching the Lucene constructor that drops the after marker.
func NewTopFieldCollectorManagerSimple(sort *Sort, numHits, totalHitsThreshold int) (*TopFieldCollectorManager, error) {
	return NewTopFieldCollectorManager(sort, numHits, nil, totalHitsThreshold)
}

// NewCollector creates a fresh TopFieldCollector.
func (m *TopFieldCollectorManager) NewCollector() (*TopFieldCollector, error) {
	return NewTopFieldCollector(m.numHits, m.sort), nil
}

// Reduce merges the per-segment TopFieldCollector results into a single
// TopFieldDocs. The total hit count is summed and the score docs are merged
// using TopDocs.Merge semantics.
func (m *TopFieldCollectorManager) Reduce(collectors []*TopFieldCollector) (*TopFieldDocs, error) {
	if len(collectors) == 0 {
		empty := NewTotalHits(0, EQUAL_TO)
		return NewTopFieldDocs(empty, nil, m.sort.Fields), nil
	}
	parts := make([]*TopDocs, 0, len(collectors))
	for _, c := range collectors {
		td := c.TopDocs()
		if td != nil {
			parts = append(parts, td)
		}
	}
	merged, err := MergeWithStart(0, m.numHits, parts)
	if err != nil {
		return nil, err
	}
	if merged == nil {
		empty := NewTotalHits(0, EQUAL_TO)
		return NewTopFieldDocs(empty, nil, m.sort.Fields), nil
	}
	return NewTopFieldDocs(merged.TotalHits, merged.ScoreDocs, m.sort.Fields), nil
}
