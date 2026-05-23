// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/intervals/CachingMatchesIterator.java

package intervals

import (
	"github.com/FlavioCFOliveira/Gocene/search"
)

// CachingMatchesIterator wraps an IntervalMatchesIterator and caches the
// sub-match positions and offsets of the current match so that they can be
// reported multiple times (needed during minimization).
//
// Mirrors org.apache.lucene.queries.intervals.CachingMatchesIterator.
//
// Deviations from Java:
//   - FilterMatchesIterator base replaced by direct delegation.
type CachingMatchesIterator struct {
	in             IntervalMatchesIterator
	posAndOffsets  []int   // packed: [start, end, startOffset, endOffset] * count
	matchingQueries []search.Query
	count          int
}

// NewCachingMatchesIterator wraps an IntervalMatchesIterator.
func NewCachingMatchesIterator(in IntervalMatchesIterator) *CachingMatchesIterator {
	return &CachingMatchesIterator{
		in:             in,
		posAndOffsets:  make([]int, 4*4),
		matchingQueries: make([]search.Query, 4),
	}
}

// Cache snapshots the current sub-match positions and offsets.
func (c *CachingMatchesIterator) Cache() error {
	c.count = 0
	subs, err := c.in.GetSubMatches()
	if err != nil {
		return err
	}
	if subs == nil {
		c.ensureCapacity(1)
		c.posAndOffsets[0] = c.in.StartPosition()
		c.posAndOffsets[1] = c.in.EndPosition()
		so, err := c.in.StartOffset()
		if err != nil {
			return err
		}
		eo, err := c.in.EndOffset()
		if err != nil {
			return err
		}
		c.posAndOffsets[2] = so
		c.posAndOffsets[3] = eo
		c.matchingQueries[0] = c.in.GetQuery()
		c.count = 1
	} else {
		for {
			ok, err := subs.Next()
			if err != nil {
				return err
			}
			if !ok {
				break
			}
			c.ensureCapacity(c.count + 1)
			c.posAndOffsets[c.count*4] = subs.StartPosition()
			c.posAndOffsets[c.count*4+1] = subs.EndPosition()
			so, err := subs.StartOffset()
			if err != nil {
				return err
			}
			eo, err := subs.EndOffset()
			if err != nil {
				return err
			}
			c.posAndOffsets[c.count*4+2] = so
			c.posAndOffsets[c.count*4+3] = eo
			c.matchingQueries[c.count] = subs.GetQuery()
			c.count++
		}
	}
	return nil
}

func (c *CachingMatchesIterator) ensureCapacity(needed int) {
	for needed*4 > len(c.posAndOffsets) {
		newBuf := make([]int, len(c.posAndOffsets)*2)
		copy(newBuf, c.posAndOffsets)
		c.posAndOffsets = newBuf
		newQ := make([]search.Query, len(c.matchingQueries)*2)
		copy(newQ, c.matchingQueries)
		c.matchingQueries = newQ
	}
}

// Next advances the inner iterator.
func (c *CachingMatchesIterator) Next() (bool, error) { return c.in.Next() }

// StartPosition returns the start position.
func (c *CachingMatchesIterator) StartPosition() int { return c.in.StartPosition() }

// EndPosition returns the end position.
func (c *CachingMatchesIterator) EndPosition() int { return c.in.EndPosition() }

// StartOffset returns the cached start offset of the first sub-match.
func (c *CachingMatchesIterator) StartOffset() (int, error) {
	if c.count == 0 {
		return -1, nil
	}
	return c.posAndOffsets[2], nil
}

// EndOffset returns the cached end offset of the last sub-match.
func (c *CachingMatchesIterator) EndOffset() (int, error) {
	if c.count == 0 {
		return -1, nil
	}
	return c.posAndOffsets[(c.count-1)*4+3], nil
}

// GetSubMatches returns an iterator over the cached sub-matches.
func (c *CachingMatchesIterator) GetSubMatches() (search.MatchesIterator, error) {
	return &cachedSubMatchesIterator{parent: c, upto: -1}, nil
}

// GetQuery returns the query for the first cached match.
func (c *CachingMatchesIterator) GetQuery() search.Query {
	if c.count == 0 {
		return nil
	}
	return c.matchingQueries[0]
}

// Gaps delegates to inner.
func (c *CachingMatchesIterator) Gaps() int { return c.in.Gaps() }

// Width delegates to inner.
func (c *CachingMatchesIterator) Width() int { return c.in.Width() }

// cachedSubMatchesIterator replays cached sub-match data.
type cachedSubMatchesIterator struct {
	parent *CachingMatchesIterator
	upto   int
}

func (s *cachedSubMatchesIterator) Next() (bool, error) {
	s.upto++
	return s.upto < s.parent.count, nil
}
func (s *cachedSubMatchesIterator) StartPosition() int { return s.parent.posAndOffsets[s.upto*4] }
func (s *cachedSubMatchesIterator) EndPosition() int   { return s.parent.posAndOffsets[s.upto*4+1] }
func (s *cachedSubMatchesIterator) StartOffset() (int, error) {
	return s.parent.posAndOffsets[s.upto*4+2], nil
}
func (s *cachedSubMatchesIterator) EndOffset() (int, error) {
	return s.parent.posAndOffsets[s.upto*4+3], nil
}
func (s *cachedSubMatchesIterator) GetSubMatches() (search.MatchesIterator, error) { return nil, nil }
func (s *cachedSubMatchesIterator) GetQuery() search.Query {
	if s.upto >= 0 && s.upto < s.parent.count {
		return s.parent.matchingQueries[s.upto]
	}
	return nil
}
