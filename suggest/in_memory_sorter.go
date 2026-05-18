package suggest

import (
	"bytes"
	"sort"
)

// InMemorySorter buffers ([]byte, []byte) input pairs and emits them
// sorted by key. Mirrors org.apache.lucene.search.suggest.InMemorySorter.
type InMemorySorter struct {
	pairs [][2][]byte
}

// NewInMemorySorter builds an empty sorter.
func NewInMemorySorter() *InMemorySorter { return &InMemorySorter{} }

// Add records a (key, value) pair.
func (s *InMemorySorter) Add(key, value []byte) {
	kCopy := append([]byte(nil), key...)
	vCopy := append([]byte(nil), value...)
	s.pairs = append(s.pairs, [2][]byte{kCopy, vCopy})
}

// Sorted returns the buffered pairs in key-sorted order.
func (s *InMemorySorter) Sorted() [][2][]byte {
	out := make([][2][]byte, len(s.pairs))
	copy(out, s.pairs)
	sort.SliceStable(out, func(i, j int) bool {
		return bytes.Compare(out[i][0], out[j][0]) < 0
	})
	return out
}

// Size returns the number of buffered pairs.
func (s *InMemorySorter) Size() int { return len(s.pairs) }
