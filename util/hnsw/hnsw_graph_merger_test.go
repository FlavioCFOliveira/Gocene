// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hnsw

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// stubKnnVectorsReader is a minimal type satisfying KnnVectorsReader.
type stubKnnVectorsReader struct {
	graph HnswGraph
}

func (s *stubKnnVectorsReader) HnswGraph(field string) (HnswGraph, error) {
	return s.graph, nil
}

func (s *stubKnnVectorsReader) GetFloatVectorValues(field string) (KnnVectorValues, error) {
	return nil, nil
}

// stubDocMap is a minimal DocMap.
type stubDocMap struct {
	mapping []int
}

func (s *stubDocMap) Get(docID int) int {
	if docID < 0 || docID >= len(s.mapping) {
		return -1
	}
	return s.mapping[docID]
}

// stubMerger satisfies HnswGraphMerger.
type stubMerger struct {
	readers []KnnVectorsReader
}

func (s *stubMerger) AddReader(reader KnnVectorsReader, docMap DocMap, liveDocs util.Bits) (HnswGraphMerger, error) {
	s.readers = append(s.readers, reader)
	return s, nil
}

func (s *stubMerger) Merge(mergedVectorValues KnnVectorValues, infoStream util.InfoStream, maxOrd int) (*OnHeapHnswGraph, error) {
	return NewOnHeapHnswGraph(4, maxOrd), nil
}

func TestHnswGraphMergerInterfaceShape(t *testing.T) {
	r := &stubKnnVectorsReader{graph: Empty()}
	dm := &stubDocMap{mapping: []int{0, 1, 2}}

	var m HnswGraphMerger = &stubMerger{}
	got, err := m.AddReader(r, dm, nil)
	if err != nil {
		t.Fatalf("AddReader: %v", err)
	}
	if got == nil {
		t.Fatalf("AddReader returned nil")
	}

	graph, err := m.Merge(&stubKnnVectorValues{dim: 4, n: 3}, util.DefaultInfoStream(), 3)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if graph == nil {
		t.Fatalf("Merge returned nil")
	}
}

func TestDocMapStubReturnsMinusOneOnOOB(t *testing.T) {
	dm := &stubDocMap{mapping: []int{5, 6, 7}}
	if dm.Get(0) != 5 {
		t.Errorf("Get(0) = %d want 5", dm.Get(0))
	}
	if dm.Get(-1) != -1 {
		t.Errorf("Get(-1) = %d want -1", dm.Get(-1))
	}
	if dm.Get(10) != -1 {
		t.Errorf("Get(10) = %d want -1", dm.Get(10))
	}
}
