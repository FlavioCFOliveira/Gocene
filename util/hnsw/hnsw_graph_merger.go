// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

package hnsw

import "github.com/FlavioCFOliveira/Gocene/util"

// KnnVectorsReader is a temporary local stub of
// org.apache.lucene.codecs.KnnVectorsReader. The full type lives in
// the codecs/ package; importing it here would create a cycle until
// the codec-side KNN port settles. The merger only needs the
// HnswGraph() and GetFloatVectorValues() accessors for now.
//
// TODO(rmp): unify with codecs.KnnVectorsReader once the
// dependency direction is finalised (likely after the L25-knn-codec
// sprint).
type KnnVectorsReader interface {
	// HnswGraph returns the HNSW graph for the given field, or nil.
	HnswGraph(field string) (HnswGraph, error)

	// GetFloatVectorValues returns the float vector values for the
	// given field.
	GetFloatVectorValues(field string) (KnnVectorValues, error)
}

// DocMap is a temporary local stub of
// org.apache.lucene.index.MergeState.DocMap. It maps a per-segment
// doc id to its merged doc id. Returns -1 for deleted docs.
//
// TODO(rmp): unify with index.MergeState.DocMap when the index port
// lands.
type DocMap interface {
	// Get returns the merged doc id for the given per-segment doc.
	Get(docID int) int
}

// HnswGraphMerger abstracts the merging of multiple HNSW graphs
// into a single on-heap graph. Port of
// org.apache.lucene.util.hnsw.HnswGraphMerger (Lucene 10.4.0).
//
// Methods returning error correspond to Java's IOException
// throwers.
type HnswGraphMerger interface {
	// AddReader records a reader to merge from. liveDocs may be nil.
	// Returns the receiver for chaining.
	AddReader(reader KnnVectorsReader, docMap DocMap, liveDocs util.Bits) (HnswGraphMerger, error)

	// Merge produces the merged on-heap graph from the recorded
	// readers and the merged vector values view.
	Merge(mergedVectorValues KnnVectorValues, infoStream util.InfoStream, maxOrd int) (*OnHeapHnswGraph, error)
}
