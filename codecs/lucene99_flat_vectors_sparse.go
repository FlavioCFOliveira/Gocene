// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Portions adapted from Apache Lucene 10.4.0:
//
//	Licensed to the Apache Software Foundation (ASF) under one or more
//	contributor license agreements. See the NOTICE file distributed with
//	this work for additional information regarding copyright ownership.
//	The ASF licenses this file to You under the Apache License, Version 2.0
//	(the "License"); you may not use this file except in compliance with
//	the License. You may obtain a copy of the License at
//
//	    http://www.apache.org/licenses/LICENSE-2.0

package codecs

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	utilhnsw "github.com/FlavioCFOliveira/Gocene/util/hnsw"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// This file implements the sparse off-heap vector views for the Lucene99 flat
// vectors format (rmp #4755). They are the Go port of the
// SparseOffHeapVectorValues inner classes of
// org.apache.lucene.codecs.lucene99.OffHeap{Float,Byte}VectorValues
// (Lucene 10.4.0).
//
// In the sparse layout the .vec file still stores the per-document vectors
// packed by ordinal (ord 0..size-1, byte-for-byte the dense layout), so
// VectorValue(ord) seeks ord*byteSize exactly as in the dense case. What is
// sparse is the ordinal->document mapping: only a subset of the segment's
// documents carry a value. That mapping is recovered from two structures
// appended to the .vec file:
//
//   - an IndexedDISI doc-id set enumerating the documents that have a value,
//     in ascending order (consumed as a DocIndexIterator whose Index() yields
//     the matching ordinal);
//   - a DirectMonotonicReader giving ord -> docID for random ordinal access.
//
// Cycle note: the concrete IndexedDISI reader is the package-local,
// little-endian dvIndexedDISI (lucene90_doc_values_disi.go), because the
// codecs package cannot import codecs/lucene90 (that package imports codecs).

// ---------------------------------------------------------------------------
// flatSparseFloatVectorValues — sparse off-heap float32 vectors.
// ---------------------------------------------------------------------------

type flatSparseFloatVectorValues struct {
	dimension int
	size      int
	byteSize  int
	slice     store.IndexInput
	sim       index.VectorSimilarityFunction

	ordToDoc    *packed.DirectMonotonicReader
	disiFactory func() (*dvIndexedDISI, error)

	lastOrd int
	value   []float32
}

func (v *flatSparseFloatVectorValues) Dimension() int { return v.dimension }
func (v *flatSparseFloatVectorValues) Size() int      { return v.size }
func (v *flatSparseFloatVectorValues) similarity() index.VectorSimilarityFunction {
	return v.sim
}

// OrdToDoc maps a vector ordinal to its document id via the
// DirectMonotonicReader. Mirrors SparseOffHeapVectorValues.ordToDoc.
func (v *flatSparseFloatVectorValues) OrdToDoc(ord int) int {
	doc, err := v.ordToDoc.Get(int64(ord))
	if err != nil {
		// OrdToDoc satisfies the hnsw.KnnVectorValues interface which
		// cannot return an error. I/O errors here are treated as
		// unrecoverable, matching Lucene's RuntimeException contract.
		return 0
	}
	return int(doc)
}

// GetAcceptOrds wraps acceptDocs (doc-keyed) in a Bits over ordinals, so the
// HNSW search filters on the original document space. Mirrors
// SparseOffHeapVectorValues.getAcceptOrds.
func (v *flatSparseFloatVectorValues) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	if acceptDocs == nil {
		return nil
	}
	return &flatSparseAcceptOrds{acceptDocs: acceptDocs, ordToDoc: v.ordToDoc, size: v.size}
}

// VectorValue returns the float32 vector at ordinal ord. The returned slice
// is the receiver's reusable buffer; callers must copy to retain it.
func (v *flatSparseFloatVectorValues) VectorValue(ord int) ([]float32, error) {
	if ord < 0 || ord >= v.size {
		return nil, fmt.Errorf("lucene99 flat: sparse float ordinal %d out of range [0,%d)", ord, v.size)
	}
	if v.lastOrd == ord {
		return v.value, nil
	}
	if err := v.slice.SetPosition(int64(ord) * int64(v.byteSize)); err != nil {
		return nil, err
	}
	if err := readFloatsLE(v.slice, v.value); err != nil {
		return nil, err
	}
	v.lastOrd = ord
	return v.value, nil
}

// Iterator returns a DISI-backed iterator over (docID, ordinal) pairs. Mirrors
// IndexedDISI.asDocIndexIterator(disi).
func (v *flatSparseFloatVectorValues) Iterator() utilhnsw.DocIndexIterator {
	disi, err := v.disiFactory()
	return &flatSparseIterator{disi: disi, err: err}
}

// ---------------------------------------------------------------------------
// flatSparseByteVectorValues — sparse off-heap byte vectors.
// ---------------------------------------------------------------------------

type flatSparseByteVectorValues struct {
	dimension int
	size      int
	byteSize  int
	slice     store.IndexInput
	sim       index.VectorSimilarityFunction

	ordToDoc    *packed.DirectMonotonicReader
	disiFactory func() (*dvIndexedDISI, error)

	lastOrd int
	value   []byte
}

func (v *flatSparseByteVectorValues) Dimension() int { return v.dimension }
func (v *flatSparseByteVectorValues) Size() int      { return v.size }
func (v *flatSparseByteVectorValues) similarity() index.VectorSimilarityFunction {
	return v.sim
}

func (v *flatSparseByteVectorValues) OrdToDoc(ord int) int {
	doc, err := v.ordToDoc.Get(int64(ord))
	if err != nil {
		return 0
	}
	return int(doc)
}

func (v *flatSparseByteVectorValues) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	if acceptDocs == nil {
		return nil
	}
	return &flatSparseAcceptOrds{acceptDocs: acceptDocs, ordToDoc: v.ordToDoc, size: v.size}
}

func (v *flatSparseByteVectorValues) VectorValue(ord int) ([]byte, error) {
	if ord < 0 || ord >= v.size {
		return nil, fmt.Errorf("lucene99 flat: sparse byte ordinal %d out of range [0,%d)", ord, v.size)
	}
	if v.lastOrd == ord {
		return v.value, nil
	}
	if err := v.slice.SetPosition(int64(ord) * int64(v.byteSize)); err != nil {
		return nil, err
	}
	if err := v.slice.ReadBytes(v.value); err != nil {
		return nil, err
	}
	v.lastOrd = ord
	return v.value, nil
}

func (v *flatSparseByteVectorValues) Iterator() utilhnsw.DocIndexIterator {
	disi, err := v.disiFactory()
	return &flatSparseIterator{disi: disi, err: err}
}

// ---------------------------------------------------------------------------
// flatSparseIterator — DocIndexIterator backed by an IndexedDISI.
//
// NextDoc advances the DISI to the next docID; Index returns the DISI's
// internal ordinal (the count of preceding set bits). Mirrors the adapter
// returned by IndexedDISI.asDocIndexIterator.
// ---------------------------------------------------------------------------

type flatSparseIterator struct {
	disi *dvIndexedDISI
	err  error
}

func (it *flatSparseIterator) NextDoc() (int, error) {
	if it.err != nil {
		return 0, it.err
	}
	return it.disi.NextDoc()
}

func (it *flatSparseIterator) Index() int {
	if it.disi == nil {
		return -1
	}
	return it.disi.Index()
}

// ---------------------------------------------------------------------------
// flatSparseAcceptOrds — Bits over ordinals derived from doc-keyed
// acceptDocs and the ord->doc mapping. Mirrors the anonymous Bits returned by
// SparseOffHeapVectorValues.getAcceptOrds.
// ---------------------------------------------------------------------------

type flatSparseAcceptOrds struct {
	acceptDocs util.Bits
	ordToDoc   *packed.DirectMonotonicReader
	size       int
}

func (s *flatSparseAcceptOrds) Get(index int) bool {
	doc, err := s.ordToDoc.Get(int64(index))
	if err != nil {
		return false
	}
	return s.acceptDocs.Get(int(doc))
}

func (s *flatSparseAcceptOrds) Length() int { return s.size }

// Compile-time guards: the sparse views satisfy the shared interfaces.
var (
	_ flatFloatVectorValues     = (*flatSparseFloatVectorValues)(nil)
	_ flatByteVectorValues      = (*flatSparseByteVectorValues)(nil)
	_ utilhnsw.DocIndexIterator = (*flatSparseIterator)(nil)
	_ util.Bits                 = (*flatSparseAcceptOrds)(nil)
)
