// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

// DiversifyingNearestChildrenKnnCollectorManager produces per-segment KNN
// collectors that enforce "at most one child per parent" diversification.
// Mirrors org.apache.lucene.search.join.DiversifyingNearestChildrenKnnCollectorManager.
//
// The Go port surfaces the factory contract: callers supply the parents
// BitSetProducer and a top-K budget; NewCollector constructs a fresh
// per-segment collector.
type DiversifyingNearestChildrenKnnCollectorManager struct {
	k       int
	parents BitSetProducer
}

// NewDiversifyingNearestChildrenKnnCollectorManager builds the manager.
func NewDiversifyingNearestChildrenKnnCollectorManager(k int, parents BitSetProducer) *DiversifyingNearestChildrenKnnCollectorManager {
	if k < 1 {
		k = 1
	}
	return &DiversifyingNearestChildrenKnnCollectorManager{k: k, parents: parents}
}

// GetK returns the top-K budget.
func (m *DiversifyingNearestChildrenKnnCollectorManager) GetK() int { return m.k }

// GetParentsFilter returns the parents BitSetProducer.
func (m *DiversifyingNearestChildrenKnnCollectorManager) GetParentsFilter() BitSetProducer {
	return m.parents
}

// DiversifyingChildKnnCollector is a per-segment collector that keeps the
// top-K (childDoc, score) pairs with the diversification invariant that no
// parent contributes more than one child.
type DiversifyingChildKnnCollector struct {
	k         int
	parentToChild map[int]int
	parentScore   map[int]float32
}

// NewCollector returns a fresh DiversifyingChildKnnCollector.
func (m *DiversifyingNearestChildrenKnnCollectorManager) NewCollector() *DiversifyingChildKnnCollector {
	return &DiversifyingChildKnnCollector{
		k:             m.k,
		parentToChild: make(map[int]int),
		parentScore:   make(map[int]float32),
	}
}

// Collect records a (parentDoc, childDoc, score) tuple, replacing the parent's
// pinned child when score is higher.
func (c *DiversifyingChildKnnCollector) Collect(parentDoc, childDoc int, score float32) {
	if cur, ok := c.parentScore[parentDoc]; ok {
		if score <= cur {
			return
		}
	}
	c.parentToChild[parentDoc] = childDoc
	c.parentScore[parentDoc] = score
}

// Results returns the collected (childDoc, score) pairs.
func (c *DiversifyingChildKnnCollector) Results() (children []int, scores []float32) {
	children = make([]int, 0, len(c.parentToChild))
	scores = make([]float32, 0, len(c.parentToChild))
	for parent, child := range c.parentToChild {
		children = append(children, child)
		scores = append(scores, c.parentScore[parent])
	}
	return children, scores
}
