// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// HnswQueueSaturationCollector wraps a KnnCollector with patience-based early
// termination. It tracks how many consecutive visits failed to improve the
// minimum competitive similarity and signals saturation once a configurable
// patience threshold is reached.
//
// Mirrors org.apache.lucene.search.HnswQueueSaturationCollector. The wrapper
// surface lives here so consumers can program against it; concrete wiring to
// the codec-side HNSW search lives next to the codec.
type HnswQueueSaturationCollector struct {
	patience      int
	since         int
	lastMin       float32
	saturated     bool
	terminateHook func()
}

// NewHnswQueueSaturationCollector builds a collector with the given patience.
// terminate is invoked once the saturation predicate triggers; nil disables
// the callback.
func NewHnswQueueSaturationCollector(patience int, terminate func()) *HnswQueueSaturationCollector {
	if patience <= 0 {
		patience = 1
	}
	return &HnswQueueSaturationCollector{patience: patience, terminateHook: terminate}
}

// Observe records a new minimum competitive similarity. It returns true if
// the collector is now considered saturated.
func (c *HnswQueueSaturationCollector) Observe(minSim float32) bool {
	if minSim > c.lastMin {
		c.lastMin = minSim
		c.since = 0
		c.saturated = false
		return false
	}
	c.since++
	if c.since >= c.patience {
		if !c.saturated && c.terminateHook != nil {
			c.terminateHook()
		}
		c.saturated = true
		return true
	}
	return false
}

// Saturated reports whether the collector has triggered.
func (c *HnswQueueSaturationCollector) Saturated() bool { return c.saturated }

// Reset clears the observed state.
func (c *HnswQueueSaturationCollector) Reset() {
	c.since = 0
	c.lastMin = 0
	c.saturated = false
}
