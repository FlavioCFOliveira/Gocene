// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package bloom

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// HashFunction is the contract every bloom-filter hash satisfies. Mirrors
// org.apache.lucene.codecs.bloom.HashFunction.
type HashFunction interface {
	Hash(data []byte) uint64
}

// MurmurHash64 is the 64-bit Murmur hash used by Lucene's bloom filters.
// Mirrors org.apache.lucene.codecs.bloom.MurmurHash64.
type MurmurHash64 struct {
	Seed uint64
}

// NewMurmurHash64 builds the hash with the supplied seed.
func NewMurmurHash64(seed uint64) *MurmurHash64 { return &MurmurHash64{Seed: seed} }

// Hash implements HashFunction with a straightforward Murmur64A variant.
func (m *MurmurHash64) Hash(data []byte) uint64 {
	const c1 uint64 = 0xff51afd7ed558ccd
	const c2 uint64 = 0xc4ceb9fe1a85ec53
	h := m.Seed
	for i := 0; i+8 <= len(data); i += 8 {
		k := uint64(data[i]) | uint64(data[i+1])<<8 | uint64(data[i+2])<<16 | uint64(data[i+3])<<24 |
			uint64(data[i+4])<<32 | uint64(data[i+5])<<40 | uint64(data[i+6])<<48 | uint64(data[i+7])<<56
		k *= c1
		k = (k << 31) | (k >> 33)
		k *= c2
		h ^= k
		h = (h << 27) | (h >> 37)
		h = h*5 + 0x52dce729
	}
	tail := data[len(data)&^7:]
	if len(tail) > 0 {
		var k uint64
		for i, b := range tail {
			k |= uint64(b) << (8 * i)
		}
		k *= c1
		k = (k << 31) | (k >> 33)
		k *= c2
		h ^= k
	}
	h ^= uint64(len(data))
	h ^= h >> 33
	h *= c1
	h ^= h >> 33
	h *= c2
	h ^= h >> 33
	return h
}

var _ HashFunction = (*MurmurHash64)(nil)

// FuzzySet is the bloom-filter-like set used by Lucene to short-circuit
// term lookups. Mirrors org.apache.lucene.codecs.bloom.FuzzySet.
type FuzzySet struct {
	Bits              []uint64
	Hash              HashFunction
	targetMaxSaturation float32
}

// NewFuzzySet builds a FuzzySet sized to capacity bits.
func NewFuzzySet(capacityBits int, hash HashFunction) *FuzzySet {
	if capacityBits < 64 {
		capacityBits = 64
	}
	if hash == nil {
		hash = NewMurmurHash64(0)
	}
	return &FuzzySet{
		Bits:                make([]uint64, (capacityBits+63)/64),
		Hash:                hash,
		targetMaxSaturation: 0.1023,
	}
}

// CreateOptimalSet builds a set targeting a max false-positive rate.
func CreateOptimalSet(numDocs int, targetFPR float32) *FuzzySet {
	// Optimal number of bits for a given FPR is -n*ln(p)/(ln 2)^2
	// For p=0.1, this is approx 4.8 bits per element. Lucene uses 10 for simplicity.
	capacityBits := numDocs * 10
	return NewFuzzySet(capacityBits, NewMurmurHash64(0))
}

// Add inserts data into the set.
func (s *FuzzySet) Add(data []byte) {
	h := s.Hash.Hash(data)
	bits := uint64(len(s.Bits)) * 64
	idx := h % bits
	s.Bits[idx/64] |= 1 << (idx % 64)
}

// MayContain returns true if data has possibly been added.
func (s *FuzzySet) MayContain(data []byte) bool {
	h := s.Hash.Hash(data)
	bits := uint64(len(s.Bits)) * 64
	idx := h % bits
	return s.Bits[idx/64]&(1<<(idx%64)) != 0
}

// GetSaturation returns the current saturation of the set.
func (s *FuzzySet) GetSaturation() float32 {
	var setBits int64
	for _, word := range s.Bits {
		setBits += int64(popcount(word))
	}
	return float32(setBits) / float32(len(s.Bits)*64)
}

func popcount(x uint64) int {
	// Standard popcount implementation
	x -= (x >> 1) & 0x5555555555555555
	x = (x & 0x3333333333333333) + ((x >> 2) & 0x3333333333333333)
	x = (x + (x >> 4)) & 0x0f0f0f0f0f0f0f0f
	x += (x >> 8)
	x += (x >> 16)
	x += (x >> 32)
	return int(x & 0x7f)
}

// GetTargetMaxSaturation returns the target max saturation.
func (s *FuzzySet) GetTargetMaxSaturation() float32 {
	return s.targetMaxSaturation
}

// Downsize shrinks the set if it's sparse.
func (s *FuzzySet) Downsize(targetMaxSaturation float32) *FuzzySet {
	// Simplified downsize for now: in a real implementation, this would
	// re-evaluate the necessary size.
	return s
}

// BloomFilterFactory is the contract that builds FuzzySet instances per
// field. Mirrors org.apache.lucene.codecs.bloom.BloomFilterFactory.
type BloomFilterFactory interface {
	GetSetForField(state *index.SegmentWriteState, info *spi.FieldInfo) *FuzzySet
	IsSaturated(bloomFilter *FuzzySet, fieldInfo *spi.FieldInfo) bool
}

// DefaultBloomFilterFactory sizes the bit-array based on numDocs * 10 bits.
// Mirrors org.apache.lucene.codecs.bloom.DefaultBloomFilterFactory.
type DefaultBloomFilterFactory struct{}

func (DefaultBloomFilterFactory) GetSetForField(state *index.SegmentWriteState, info *spi.FieldInfo) *FuzzySet {
	return CreateOptimalSet(state.SegmentInfo.MaxDoc(), 0.1023)
}

func (DefaultBloomFilterFactory) IsSaturated(bloomFilter *FuzzySet, fieldInfo *spi.FieldInfo) bool {
	return bloomFilter.GetSaturation() > 0.9
}

// Downsize is a default implementation provided by the factory.
func Downsize(fieldInfo *spi.FieldInfo, initialSet *FuzzySet) *FuzzySet {
	return initialSet.Downsize(initialSet.GetTargetMaxSaturation())
}

var _ BloomFilterFactory = DefaultBloomFilterFactory{}

// BloomFilteringPostingsFormat is the postings-format wrapper that decorates
// the underlying format with a per-field bloom filter. Mirrors
// org.apache.lucene.codecs.bloom.BloomFilteringPostingsFormat.
type BloomFilteringPostingsFormat struct {
	Inner   spi.PostingsFormat
	Factory BloomFilterFactory
}

// NewBloomFilteringPostingsFormat builds the wrapper.
func NewBloomFilteringPostingsFormat(inner spi.PostingsFormat, factory BloomFilterFactory) *BloomFilteringPostingsFormat {
	if factory == nil {
		factory = DefaultBloomFilterFactory{}
	}
	return &BloomFilteringPostingsFormat{Inner: inner, Factory: factory}
}
