// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// TotalHitCountCollectorManager creates TotalHitCountCollector instances and
// reduces their per-segment counts into a single total.
//
// Mirrors org.apache.lucene.search.TotalHitCountCollectorManager. Lucene
// optimizes this path with leaf-slice partitioning awareness; this port keeps
// the contract simple and sums per-collector counts.
type TotalHitCountCollectorManager struct{}

// NewTotalHitCountCollectorManager creates a default manager.
func NewTotalHitCountCollectorManager() *TotalHitCountCollectorManager {
	return &TotalHitCountCollectorManager{}
}

// NewCollector creates a fresh TotalHitCountCollector.
func (m *TotalHitCountCollectorManager) NewCollector() (*TotalHitCountCollector, error) {
	return NewTotalHitCountCollector(), nil
}

// Reduce sums the per-collector hit counts.
func (m *TotalHitCountCollectorManager) Reduce(collectors []*TotalHitCountCollector) (int, error) {
	total := 0
	for _, c := range collectors {
		total += c.GetTotalHits()
	}
	return total, nil
}
