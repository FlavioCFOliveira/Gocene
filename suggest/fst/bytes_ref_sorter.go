// Package fst implements org.apache.lucene.search.suggest.fst: the FST-based
// suggester family.
package fst

import (
	"bytes"
	"sort"
)

// BytesRefSorter is the contract every byte-slice sorter implements. Mirrors
// org.apache.lucene.search.suggest.fst.BytesRefSorter.
type BytesRefSorter interface {
	Add(item []byte) error
	Iterate() ([][]byte, error)
}

// InMemoryBytesRefSorter buffers items in-memory and emits them sorted.
type InMemoryBytesRefSorter struct {
	items [][]byte
}

// NewInMemoryBytesRefSorter builds an empty sorter.
func NewInMemoryBytesRefSorter() *InMemoryBytesRefSorter { return &InMemoryBytesRefSorter{} }

// Add records item.
func (s *InMemoryBytesRefSorter) Add(item []byte) error {
	s.items = append(s.items, append([]byte(nil), item...))
	return nil
}

// Iterate returns the items sorted by bytes.Compare.
func (s *InMemoryBytesRefSorter) Iterate() ([][]byte, error) {
	out := make([][]byte, len(s.items))
	copy(out, s.items)
	sort.SliceStable(out, func(i, j int) bool { return bytes.Compare(out[i], out[j]) < 0 })
	return out, nil
}

var _ BytesRefSorter = (*InMemoryBytesRefSorter)(nil)
