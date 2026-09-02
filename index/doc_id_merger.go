// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"sort"
)

// DocIDMerger merges several Sub iterators into a single stream of
// (sub, mappedDocID) pairs. Mirrors org.apache.lucene.index.DocIDMerger
// from Apache Lucene 10.4.0.
//
// Gocene models the abstract class as an interface plus two concrete
// implementations (sorted-merge and concat). The mergeStateAware variants
// that participate in merge operations land alongside MergeState in
// Phase 7/Phase 8.
type DocIDMerger interface {
	// Reset positions every Sub at the beginning of its document stream.
	Reset() error

	// Next returns the next Sub whose current mappedDocID is the smallest
	// across the active Subs, or nil when all Subs are exhausted.
	Next() (DocIDMergerSub, error)
}

// DocIDMergerSub is the per-segment participant in a DocIDMerger. Mirrors
// DocIDMerger.Sub from Lucene 10.4.0.
type DocIDMergerSub interface {
	// MappedDocID returns the docID currently mapped through the segment's
	// DocMap (or the raw docID when no mapping is applied).
	MappedDocID() int

	// NextDoc advances the underlying iterator and returns the next raw docID
	// (pre-mapping) — NO_MORE_DOCS when exhausted.
	NextDoc() (int, error)

	// NextMappedDoc advances and returns the next mapped docID, skipping
	// raw docs the DocMap reports as deleted (sentinel value -1).
	NextMappedDoc() (int, error)
}

// NewDocIDMerger builds a DocIDMerger over subs. When indexIsSorted is true,
// a sorted-merge implementation is used (priority queue keyed on
// MappedDocID); otherwise a concat implementation iterates the Subs in input
// order. maxCount is a hint and is currently unused by Gocene (kept for API
// parity with Lucene).
func NewDocIDMerger(subs []DocIDMergerSub, maxCount int, indexIsSorted bool) (DocIDMerger, error) {
	_ = maxCount
	if indexIsSorted {
		return newSortedDocIDMerger(subs)
	}
	return &concatDocIDMerger{subs: subs}, nil
}

// concatDocIDMerger walks the Subs strictly in input order, exhausting each
// before moving to the next. Used when the index is not sorted.
type concatDocIDMerger struct {
	subs    []DocIDMergerSub
	current int
}

func (c *concatDocIDMerger) Reset() error {
	c.current = 0
	return nil
}

func (c *concatDocIDMerger) Next() (DocIDMergerSub, error) {
	for c.current < len(c.subs) {
		sub := c.subs[c.current]
		doc, err := sub.NextMappedDoc()
		if err != nil {
			return nil, err
		}
		if doc != NO_MORE_DOCS {
			return sub, nil
		}
		c.current++
	}
	return nil, nil
}

// sortedDocIDMerger walks the Subs in mapped-docID order via repeated
// selection. Each call to Next() advances the chosen sub once and then
// re-sorts the active list by MappedDocID. For Gocene's expected segment
// counts (single digits) this is faster than a heap-based priority queue
// and avoids per-call allocation.
type sortedDocIDMerger struct {
	subs   []DocIDMergerSub
	active []DocIDMergerSub
	primed bool
}

func newSortedDocIDMerger(subs []DocIDMergerSub) (*sortedDocIDMerger, error) {
	m := &sortedDocIDMerger{subs: subs}
	if err := m.Reset(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *sortedDocIDMerger) Reset() error {
	// Build the active list referencing all supplied subs; do not advance —
	// Next() handles the first-call priming so that callers see every doc.
	m.active = append(m.active[:0], m.subs...)
	m.primed = false
	return nil
}

func (m *sortedDocIDMerger) Next() (DocIDMergerSub, error) {
	if !m.primed {
		// First call: advance every sub once so MappedDocID is valid.
		alive := m.active[:0]
		for _, sub := range m.active {
			doc, err := sub.NextMappedDoc()
			if err != nil {
				return nil, err
			}
			if doc != NO_MORE_DOCS {
				alive = append(alive, sub)
			}
		}
		m.active = alive
		m.primed = true
	} else if len(m.active) > 0 {
		// Subsequent calls: advance the previously-returned head; if it
		// is exhausted, drop it; otherwise leave it in the list.
		head := m.active[0]
		doc, err := head.NextMappedDoc()
		if err != nil {
			return nil, err
		}
		if doc == NO_MORE_DOCS {
			m.active = m.active[1:]
		}
	}
	if len(m.active) == 0 {
		return nil, nil
	}
	// Re-sort by current MappedDocID so the next head is the lowest.
	sort.Slice(m.active, func(i, j int) bool {
		return m.active[i].MappedDocID() < m.active[j].MappedDocID()
	})
	return m.active[0], nil
}
