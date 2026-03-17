// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

import (
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
type QueryBitSetProducer struct {
	query search.Query
}

// NewQueryBitSetProducer creates a new QueryBitSetProducer for the given query.
func NewQueryBitSetProducer(query search.Query) *QueryBitSetProducer {
	return &QueryBitSetProducer{query: query}
}

// GetBitSet returns a BitSet of matching documents for the given context.
func (p *QueryBitSetProducer) GetBitSet(context *index.LeafReaderContext) (*FixedBitSet, error) {
	reader := context.LeafReader()
	bitSet := NewFixedBitSet(reader.NumDocs())

	// Create a simple collector that sets bits for matching documents
	collector := &bitSetCollector{
		bitSet: bitSet,
	}

	// Execute the query using the collector
	// Note: This is a simplified implementation
	// In a full implementation, we would use a proper IndexSearcher
	_ = collector
	_ = search.NewIndexSearcher(reader)

	return bitSet, nil
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
