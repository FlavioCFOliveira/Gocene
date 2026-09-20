// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// QueryBitSetProducer is a BitSetProducer that wraps a query and caches matching BitSets per segment.
//
// This is the Go port of Lucene's org.apache.lucene.search.join.QueryBitSetProducer.
type QueryBitSetProducer struct {
	query search.Query
	mu    sync.Mutex
	cache map[interface{}]util.BitSet
}

var sentinelBitSet, _ = util.NewFixedBitSet(0)

// NewQueryBitSetProducer creates a new QueryBitSetProducer for the given query.
func NewQueryBitSetProducer(query search.Query) *QueryBitSetProducer {
	return &QueryBitSetProducer{
		query: query,
	}
}

// GetBitSet returns a BitSet of matching documents for the given context.
//
// This is the Go port of Lucene's QueryBitSetProducer.getBitSet. It creates
// an IndexSearcher over the leaf, rewrites the wrapped query, builds a Weight
// (with COMPLETE_NO_SCORES since BitSetProducer never needs scores), obtains a
// Scorer for the context, and iterates the scorer setting one bit per matching
// document.
//
// Results are cached per leaf reader core cache key so repeated calls for the
// same segment reuse the computed bitset.
func (p *QueryBitSetProducer) GetBitSet(context *index.LeafReaderContext) (util.BitSet, error) {
	reader := context.LeafReader()
	if reader == nil {
		return nil, nil
	}
	maxDoc := reader.MaxDoc()
	if p.query == nil || maxDoc == 0 {
		bs, _ := util.NewFixedBitSet(maxDoc)
		return bs, nil
	}

	// Attempt cache lookup using the leaf reader's core cache key.
	var cacheKey interface{}
	if ck, ok := reader.(interface{ GetCoreCacheKey() interface{} }); ok {
		cacheKey = ck.GetCoreCacheKey()
	}
	if cacheKey != nil {
		p.mu.Lock()
		if p.cache != nil {
			if cached, ok := p.cache[cacheKey]; ok {
				p.mu.Unlock()
				if cached == sentinelBitSet {
					return nil, nil
				}
				return cached, nil
			}
		}
		p.mu.Unlock()
	}

	// Build a searcher over the leaf reader.
	leafReader, ok := reader.(index.IndexReaderInterface)
	if !ok {
		bs, _ := util.NewFixedBitSet(maxDoc)
		return bs, nil
	}
	searcher := search.NewIndexSearcher(leafReader)

	// Rewrite + create a non-scoring Weight.
	rewritten, err := p.query.Rewrite(searcher)
	if err != nil {
		return nil, err
	}
	weight, err := rewritten.CreateWeight(searcher, search.ScoreModeCompleteNoScores, 1.0)
	if err != nil {
		return nil, err
	}
	if weight == nil {
		bs, _ := util.NewFixedBitSet(maxDoc)
		if cacheKey != nil {
			p.mu.Lock()
			if p.cache == nil {
				p.cache = make(map[interface{}]util.BitSet)
			}
			p.cache[cacheKey] = bs
			p.mu.Unlock()
		}
		return bs, nil
	}

	scorer, err := weight.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		if cacheKey != nil {
			p.mu.Lock()
			if p.cache == nil {
				p.cache = make(map[interface{}]util.BitSet)
			}
			p.cache[cacheKey] = sentinelBitSet
			p.mu.Unlock()
		}
		return nil, nil
	}

	bitSet, _ := util.NewFixedBitSet(maxDoc)
	for {
		doc, err := scorer.Iterator().NextDoc()
		if err != nil {
			return nil, err
		}
		if doc == search.NO_MORE_DOCS {
			break
		}
		bitSet.Set(doc)
	}

	if cacheKey != nil {
		p.mu.Lock()
		if p.cache == nil {
			p.cache = make(map[interface{}]util.BitSet)
		}
		p.cache[cacheKey] = bitSet
		p.mu.Unlock()
	}

	return bitSet, nil
}

// GetQuery returns the wrapped query.
func (p *QueryBitSetProducer) GetQuery() search.Query {
	return p.query
}

// String returns a string representation of this QueryBitSetProducer.
func (p *QueryBitSetProducer) String() string {
	return fmt.Sprintf("QueryBitSetProducer(%v)", p.query)
}

// Equals returns true if this QueryBitSetProducer is equal to another.
func (p *QueryBitSetProducer) Equals(other interface{}) bool {
	if other == nil {
		return false
	}
	o, ok := other.(*QueryBitSetProducer)
	if !ok {
		return false
	}
	return p.query.Equals(o.query)
}

// HashCode returns the hash code for this QueryBitSetProducer.
func (p *QueryBitSetProducer) HashCode() int {
	return 31*31 + p.query.HashCode()
}
