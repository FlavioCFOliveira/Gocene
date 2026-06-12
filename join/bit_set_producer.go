// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
	"sync"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// BitSetProducer is an interface for producing BitSets.
// This is used by join queries to identify parent/child documents.
//
// This is the Go port of Lucene's org.apache.lucene.search.join.BitSetProducer.
type BitSetProducer interface {
	// GetBitSet returns a BitSet for the given context.
	// The BitSet identifies matching documents.
	GetBitSet(context *index.LeafReaderContext) (*FixedBitSet, error)
}

// FixedBitSet is a fixed-size bit set implementation.
// This is a simple implementation for use with join queries.
type FixedBitSet struct {
	bits []uint64
	size int
}

// NewFixedBitSet creates a new FixedBitSet with the given size.
func NewFixedBitSet(size int) *FixedBitSet {
	numWords := (size + 63) / 64
	return &FixedBitSet{
		bits: make([]uint64, numWords),
		size: size,
	}
}

// Set sets the bit at the given index.
func (bs *FixedBitSet) Set(index int) {
	if index < 0 || index >= bs.size {
		return
	}
	bs.bits[index/64] |= 1 << (index % 64)
}

// Get returns true if the bit at the given index is set.
func (bs *FixedBitSet) Get(index int) bool {
	if index < 0 || index >= bs.size {
		return false
	}
	return bs.bits[index/64]&(1<<(index%64)) != 0
}

// Clear clears the bit at the given index.
func (bs *FixedBitSet) Clear(index int) {
	if index < 0 || index >= bs.size {
		return
	}
	bs.bits[index/64] &^= 1 << (index % 64)
}

// Cardinality returns the number of set bits.
func (bs *FixedBitSet) Cardinality() int {
	count := 0
	for _, word := range bs.bits {
		count += popcount(word)
	}
	return count
}

// popcount returns the number of set bits in a uint64.
func popcount(x uint64) int {
	// Hamming weight algorithm
	x = x - ((x >> 1) & 0x5555555555555555)
	x = (x & 0x3333333333333333) + ((x >> 2) & 0x3333333333333333)
	x = (x + (x >> 4)) & 0x0f0f0f0f0f0f0f0f
	return int((x * 0x0101010101010101) >> 56)
}

// Size returns the size of this bit set.
func (bs *FixedBitSet) Size() int {
	return bs.size
}

// Length returns the number of bits in this bitset (its capacity).
//
// This mirrors org.apache.lucene.util.FixedBitSet.length(), which returns the
// fixed numBits rather than the index of the highest set bit. The block-join
// scorers rely on length()-1 being the last document in the segment (the
// trailing parent, per the block-join indexing invariant).
func (bs *FixedBitSet) Length() int {
	return bs.size
}

// PrevSetBit returns the index of the last set bit at or before the given
// index. Returns -1 if there is no set bit in [0, index].
//
// This mirrors org.apache.lucene.util.FixedBitSet.prevSetBit(int) and is the
// inverse of NextSetBit. Indices outside [0, size) are clamped: a negative
// index yields -1; an index >= size scans from the highest valid bit.
func (bs *FixedBitSet) PrevSetBit(index int) int {
	if index < 0 {
		return -1
	}
	if index >= bs.size {
		index = bs.size - 1
	}

	wordIndex := index / 64
	// Mask off the bits strictly above index within the current word.
	subIndex := index % 64
	word := bs.bits[wordIndex] << (63 - subIndex)
	if word != 0 {
		return wordIndex*64 + subIndex - leadingZeros(word)
	}

	for i := wordIndex - 1; i >= 0; i-- {
		if bs.bits[i] != 0 {
			return i*64 + 63 - leadingZeros(bs.bits[i])
		}
	}
	return -1
}

// leadingZeros returns the number of leading zero bits in a uint64
// (equivalent to Long.numberOfLeadingZeros).
func leadingZeros(x uint64) int {
	if x == 0 {
		return 64
	}
	n := 0
	for (x & (1 << 63)) == 0 {
		x <<= 1
		n++
	}
	return n
}

// NextSetBit returns the index of the next set bit at or after the given index.
// Returns -1 if no more set bits.
func (bs *FixedBitSet) NextSetBit(fromIndex int) int {
	if fromIndex >= bs.size {
		return -1
	}
	if fromIndex < 0 {
		fromIndex = 0
	}

	wordIndex := fromIndex / 64
	bitIndex := fromIndex % 64

	// Check remaining bits in current word
	word := bs.bits[wordIndex] >> bitIndex
	if word != 0 {
		return fromIndex + trailingZeros(word)
	}

	// Check subsequent words
	for i := wordIndex + 1; i < len(bs.bits); i++ {
		if bs.bits[i] != 0 {
			return i*64 + trailingZeros(bs.bits[i])
		}
	}

	return -1
}

// trailingZeros returns the number of trailing zeros in a uint64.
func trailingZeros(x uint64) int {
	if x == 0 {
		return 64
	}
	n := 0
	for (x & 1) == 0 {
		x >>= 1
		n++
	}
	return n
}

// QueryBitSetProducer produces a FixedBitSet from a query.
//
// This is the Go port of Lucene's org.apache.lucene.search.join.QueryBitSetProducer.
// It caches per-segment bitsets using the leaf reader's core cache key.
type QueryBitSetProducer struct {
	query search.Query
	mu    sync.Mutex
	cache map[*index.CacheKey]*FixedBitSet
}

// sentinelBitSet is used to cache a negative (no matches) result without
// returning nil from the internal cache lookup. Mirrors Java SENTINEL.
var sentinelBitSet = NewFixedBitSet(0)

// NewQueryBitSetProducer creates a new QueryBitSetProducer for the given query.
func NewQueryBitSetProducer(query search.Query) *QueryBitSetProducer {
	return &QueryBitSetProducer{query: query}
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
func (p *QueryBitSetProducer) GetBitSet(context *index.LeafReaderContext) (*FixedBitSet, error) {
	reader := context.LeafReader()
	if reader == nil {
		return NewFixedBitSet(0), nil
	}
	maxDoc := reader.MaxDoc()
	if p.query == nil || maxDoc == 0 {
		return NewFixedBitSet(maxDoc), nil
	}

	// Attempt cache lookup using the leaf reader's core cache key.
	var cacheKey *index.CacheKey
	if ck, ok := reader.(interface{ GetCoreCacheKey() interface{} }); ok {
		if key, ok := ck.GetCoreCacheKey().(*index.CacheKey); ok {
			cacheKey = key
		}
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

	// Build a searcher over the leaf reader. The leaf is itself an
	// IndexReaderInterface, so we can pass it directly.
	leafReader, ok := reader.(index.IndexReaderInterface)
	if !ok {
		return NewFixedBitSet(maxDoc), nil
	}
	searcher := search.NewIndexSearcher(leafReader)

	// Rewrite + create a non-scoring Weight. BitSetProducer only needs the
	// doc-id stream, so we pass needsScores=false (boost=1.0).
	rewritten, err := p.query.Rewrite(leafReader)
	if err != nil {
		return nil, err
	}
	weight, err := rewritten.CreateWeight(searcher, false, 1.0)
	if err != nil {
		return nil, err
	}
	if weight == nil {
		bitSet := NewFixedBitSet(maxDoc)
		if cacheKey != nil {
			p.mu.Lock()
			if p.cache == nil {
				p.cache = make(map[*index.CacheKey]*FixedBitSet)
			}
			p.cache[cacheKey] = bitSet
			p.mu.Unlock()
		}
		return bitSet, nil
	}

	scorer, err := weight.Scorer(context)
	if err != nil {
		return nil, err
	}
	if scorer == nil {
		if cacheKey != nil {
			p.mu.Lock()
			if p.cache == nil {
				p.cache = make(map[*index.CacheKey]*FixedBitSet)
			}
			p.cache[cacheKey] = sentinelBitSet
			p.mu.Unlock()
		}
		return nil, nil
	}

	bitSet := NewFixedBitSet(maxDoc)
	for {
		doc, err := scorer.NextDoc()
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
			p.cache = make(map[*index.CacheKey]*FixedBitSet)
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
	return "QueryBitSetProducer(...)"
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

// bitSetCollector is a simple collector that sets bits for matching documents.
type bitSetCollector struct {
	bitSet *FixedBitSet
}

// Collect sets the bit for the given document.
func (c *bitSetCollector) Collect(doc int) error {
	c.bitSet.Set(doc)
	return nil
}

// GetTotalHits returns the total number of hits collected.
func (c *bitSetCollector) GetTotalHits() int {
	return c.bitSet.Cardinality()
}

// FixedBitSetCachingWrapper caches FixedBitSets for multiple contexts.
//
// This is the Go port of Lucene's caching wrapper for BitSetProducer.
type FixedBitSetCachingWrapper struct {
	producer BitSetProducer
	cache    map[*index.LeafReaderContext]*FixedBitSet
}

// NewFixedBitSetCachingWrapper creates a new caching wrapper for the given producer.
func NewFixedBitSetCachingWrapper(producer BitSetProducer) *FixedBitSetCachingWrapper {
	return &FixedBitSetCachingWrapper{
		producer: producer,
		cache:    make(map[*index.LeafReaderContext]*FixedBitSet),
	}
}

// GetBitSet returns a BitSet for the given context, using cache if available.
func (w *FixedBitSetCachingWrapper) GetBitSet(context *index.LeafReaderContext) (*FixedBitSet, error) {
	if bitSet, ok := w.cache[context]; ok {
		return bitSet, nil
	}

	bitSet, err := w.producer.GetBitSet(context)
	if err != nil {
		return nil, err
	}

	w.cache[context] = bitSet
	return bitSet, nil
}

// Clear clears the cache.
func (w *FixedBitSetCachingWrapper) Clear() {
	w.cache = make(map[*index.LeafReaderContext]*FixedBitSet)
}
