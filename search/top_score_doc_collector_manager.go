// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import "errors"

// TopScoreDocCollectorManager creates TopScoreDocCollector instances and
// reduces their per-segment results into a single TopDocs.
//
// Mirrors org.apache.lucene.search.TopScoreDocCollectorManager.
type TopScoreDocCollectorManager struct {
	numHits            int
	after              *ScoreDoc
	totalHitsThreshold int
}

// NewTopScoreDocCollectorManager builds a manager with the given parameters.
// numHits must be > 0 and totalHitsThreshold must be >= 0.
func NewTopScoreDocCollectorManager(numHits int, after *ScoreDoc, totalHitsThreshold int) (*TopScoreDocCollectorManager, error) {
	if numHits <= 0 {
		return nil, errors.New("TopScoreDocCollectorManager: numHits must be > 0")
	}
	if totalHitsThreshold < 0 {
		return nil, errors.New("TopScoreDocCollectorManager: totalHitsThreshold must be >= 0")
	}
	return &TopScoreDocCollectorManager{
		numHits:            numHits,
		after:              after,
		totalHitsThreshold: totalHitsThreshold,
	}, nil
}

// NewCollector creates a fresh TopScoreDocCollector.
func (m *TopScoreDocCollectorManager) NewCollector() (*TopScoreDocCollector, error) {
	if m.after != nil {
		return NewTopScoreDocCollectorWithAfter(m.numHits, m.after), nil
	}
	return NewTopScoreDocCollector(m.numHits), nil
}

// Reduce merges the per-segment results into a single TopDocs.
func (m *TopScoreDocCollectorManager) Reduce(collectors []*TopScoreDocCollector) (*TopDocs, error) {
	if len(collectors) == 0 {
		return NewTopDocs(NewTotalHits(0, EQUAL_TO), nil), nil
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
		return NewTopDocs(NewTotalHits(0, EQUAL_TO), nil), nil
	}
	return merged, nil
}
