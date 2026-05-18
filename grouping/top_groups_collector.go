// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

// TopGroupsCollector is a single-pass grouping collector that retains the
// top-N hits per group keyed by an arbitrary comparator. Mirrors
// org.apache.lucene.search.grouping.TopGroupsCollector.
type TopGroupsCollector[T comparable] struct {
	docsPerGroup int
	compare      func(a, b float64) bool
	hits         map[T][]hitDoc
}

type hitDoc struct {
	docID int
	score float64
}

// NewTopGroupsCollector builds a collector that retains up to docsPerGroup
// docs per group, ordered by compare(a, b) which returns true when a should
// rank ahead of b. Use a descending comparator (a > b) for highest-score-wins.
func NewTopGroupsCollector[T comparable](docsPerGroup int, compare func(a, b float64) bool) *TopGroupsCollector[T] {
	if docsPerGroup < 1 {
		docsPerGroup = 1
	}
	if compare == nil {
		compare = func(a, b float64) bool { return a > b }
	}
	return &TopGroupsCollector[T]{
		docsPerGroup: docsPerGroup,
		compare:      compare,
		hits:         make(map[T][]hitDoc),
	}
}

// Collect records a (group, docID, score) tuple. When the per-group queue is
// full and score does not rank ahead of the weakest entry the tuple is dropped.
func (c *TopGroupsCollector[T]) Collect(group T, docID int, score float64) {
	cur := c.hits[group]
	if len(cur) < c.docsPerGroup {
		cur = append(cur, hitDoc{docID: docID, score: score})
		c.hits[group] = cur
		return
	}
	worst := 0
	for i := 1; i < len(cur); i++ {
		if c.compare(cur[worst].score, cur[i].score) {
			worst = i
		}
	}
	if c.compare(score, cur[worst].score) {
		cur[worst] = hitDoc{docID: docID, score: score}
		c.hits[group] = cur
	}
}

// GetDocsAndScores returns the per-group docs and scores in collection order.
func (c *TopGroupsCollector[T]) GetDocsAndScores(group T) ([]int, []float64) {
	src := c.hits[group]
	docs := make([]int, len(src))
	scores := make([]float64, len(src))
	for i, h := range src {
		docs[i] = h.docID
		scores[i] = h.score
	}
	return docs, scores
}

// GroupCount returns the number of groups holding at least one document.
func (c *TopGroupsCollector[T]) GroupCount() int { return len(c.hits) }
