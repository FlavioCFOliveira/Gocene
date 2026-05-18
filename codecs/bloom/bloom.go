// Package bloom implements org.apache.lucene.codecs.bloom: bloom-filter
// support for the term dictionary.
package bloom

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
	Bits []uint64
	Hash HashFunction
}

// NewFuzzySet builds a FuzzySet sized to capacity bits.
func NewFuzzySet(capacityBits int, hash HashFunction) *FuzzySet {
	if capacityBits < 64 {
		capacityBits = 64
	}
	if hash == nil {
		hash = NewMurmurHash64(0)
	}
	return &FuzzySet{Bits: make([]uint64, (capacityBits+63)/64), Hash: hash}
}

// Add inserts data into the set.
func (s *FuzzySet) Add(data []byte) {
	h := s.Hash.Hash(data)
	bits := uint64(len(s.Bits)) * 64
	idx := h % bits
	s.Bits[idx/64] |= 1 << (idx % 64)
}

// MayContain returns true if data has possibly been added. False positives
// are possible (per the bloom-filter contract); false negatives are not.
func (s *FuzzySet) MayContain(data []byte) bool {
	h := s.Hash.Hash(data)
	bits := uint64(len(s.Bits)) * 64
	idx := h % bits
	return s.Bits[idx/64]&(1<<(idx%64)) != 0
}

// BloomFilterFactory is the contract that builds FuzzySet instances per
// field. Mirrors org.apache.lucene.codecs.bloom.BloomFilterFactory.
type BloomFilterFactory interface {
	NewFilter(numDocs int) *FuzzySet
}

// DefaultBloomFilterFactory sizes the bit-array based on numDocs * 10 bits
// (a 1% false-positive rate). Mirrors
// org.apache.lucene.codecs.bloom.DefaultBloomFilterFactory.
type DefaultBloomFilterFactory struct{}

// NewFilter sizes the bloom filter from the document count.
func (DefaultBloomFilterFactory) NewFilter(numDocs int) *FuzzySet {
	if numDocs < 1 {
		numDocs = 1
	}
	return NewFuzzySet(numDocs*10, NewMurmurHash64(0))
}

var _ BloomFilterFactory = DefaultBloomFilterFactory{}

// BloomFilteringPostingsFormat is the postings-format wrapper that decorates
// the underlying format with a per-field bloom filter. Mirrors
// org.apache.lucene.codecs.bloom.BloomFilteringPostingsFormat.
type BloomFilteringPostingsFormat struct {
	Inner   any
	Factory BloomFilterFactory
}

// NewBloomFilteringPostingsFormat builds the wrapper.
func NewBloomFilteringPostingsFormat(inner any, factory BloomFilterFactory) *BloomFilteringPostingsFormat {
	if factory == nil {
		factory = DefaultBloomFilterFactory{}
	}
	return &BloomFilteringPostingsFormat{Inner: inner, Factory: factory}
}
