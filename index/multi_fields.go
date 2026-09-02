//go:build ignore

// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// MultiFields provides a single Fields term index view over an IndexReader.
// This is useful when you're interacting with an IndexReader implementation
// that consists of sequential sub-readers (e.g., DirectoryReader or MultiReader)
// and you must treat it as a LeafReader.
//
// NOTE: for composite readers, you'll get better performance by gathering the
// sub readers using IndexReader.GetContext() to get the atomic leaves and then
// operate per-LeafReader, instead of using this class.
//
// Mirrors org.apache.lucene.index.MultiFields (Apache Lucene 10.5.0).
type MultiFields struct {
	subs       []Fields
	subSlices  []ReaderSlice
	termsCache map[string]Terms
	mu         sync.RWMutex
}

// NewMultiFields creates a new MultiFields view over the provided sub-Fields
// and their corresponding reader slices.
func NewMultiFields(subs []Fields, subSlices []ReaderSlice) *MultiFields {
	return &MultiFields{
		subs:       subs,
		subSlices:  subSlices,
		termsCache: make(map[string]Terms),
	}
}

// Iterator returns an iterator over all field names in all sub-Fields.
// The resulting iterator is sorted and deduplicated.
func (m *MultiFields) Iterator() (schema.FieldIterator, error) {
	subIterators := make([]util.IteratorG[string], len(m.subs))
	for i, sub := range m.subs {
		it, err := sub.Iterator()
		if err != nil {
			return nil, fmt.Errorf("MultiFields.Iterator: sub %d: %w", i, err)
		}
		subIterators[i] = &fieldIteratorWrapper{it: it}
	}

	merged, err := util.NewMergedIteratorG[string](subIterators...)
	if err != nil {
		return nil, fmt.Errorf("MultiFields.Iterator: merged: %w", err)
	}

	return &multiFieldIterator{it: merged}, nil
}

// Terms returns the Terms for the specified field. If the field exists in
// multiple sub-Fields, they are aggregated into a MultiTerms instance.
func (m *MultiFields) Terms(field string) (Terms, error) {
	m.mu.RLock()
	result, exists := m.termsCache[field]
	m.mu.RUnlock()
	if exists {
		return result, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-checked locking
	if result, exists = m.termsCache[field]; exists {
		return result, nil
	}

	var subTerms []Terms
	var slices []ReaderSlice

	for i, sub := range m.subs {
		t, err := sub.Terms(field)
		if err != nil {
			return nil, fmt.Errorf("MultiFields.Terms(%s): sub %d: %w", field, i, err)
		}
		if t != nil {
			subTerms = append(subTerms, t)
			slices = append(slices, m.subSlices[i])
		}
	}

	if len(subTerms) > 0 {
		result, err := NewMultiTermsForField(field, subTerms, slices)
		if err != nil {
			return nil, fmt.Errorf("MultiFields.Terms(%s): create MultiTerms: %w", field, err)
		}
		m.termsCache[field] = result
	}

	return result, nil
}

// Size returns -1 as the size is unknown for aggregated fields.
func (m *MultiFields) Size() int {
	return -1
}

// FieldsList returns the wrapped sub-Fields slice.
func (m *MultiFields) FieldsList() []Fields {
	return m.subs
}

// fieldIteratorWrapper adapts a schema.FieldIterator to util.IteratorG[string].
type fieldIteratorWrapper struct {
	it schema.FieldIterator
}

func (w *fieldIteratorWrapper) HasNext() bool {
	return w.it.HasNext()
}

func (w *fieldIteratorWrapper) Next() string {
	s, err := w.it.Next()
	if err != nil {
		panic(err)
	}
	return s
}

// multiFieldIterator adapts a util.MergedIteratorG[string] to schema.FieldIterator.
type multiFieldIterator struct {
	it *util.MergedIteratorG[string]
}

func (m *multiFieldIterator) HasNext() bool {
	return m.it.HasNext()
}

func (m *multiFieldIterator) Next() (string, error) {
	// MergedIteratorG.Next() panics if called when HasNext() is false.
	// Callers of FieldIterator.Next() are expected to check HasNext() first.
	return m.it.Next(), nil
}
