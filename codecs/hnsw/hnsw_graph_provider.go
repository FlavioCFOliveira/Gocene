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

import (
	"github.com/FlavioCFOliveira/Gocene/util/hnsw"
)

// HnswGraphProvider is the Go port of
// org.apache.lucene.codecs.hnsw.HnswGraphProvider (Lucene 10.4.0). It
// is the single-method interface that a KnnVectorsReader implements to
// expose its on-disk (or off-heap) HNSW graph to the layer that
// gathers multiple graphs for segment merging. The merge orchestrator
// inspects each per-segment reader for HnswGraphProvider and, when
// present, reuses the segment's existing graph as a search seed
// instead of building from scratch.
//
// The Java reference exposes a single `getGraph(String field)` method
// returning `HnswGraph`. Gocene preserves that surface verbatim: the
// Go method returns the [hnsw.HnswGraph] type defined in util/hnsw
// (which is itself the port of org.apache.lucene.util.hnsw.HnswGraph).
//
// Implementations may return [hnsw.Empty] for a field that has no
// graph yet, matching the Java sentinel HnswGraph.EMPTY.
type HnswGraphProvider interface {
	// GetGraph returns the stored HnswGraph for the given field. The
	// graph may be backed by off-heap storage; readers that cannot
	// fulfil the request (unknown field, on-disk graph not present)
	// surface an error from the I/O layer.
	GetGraph(field string) (hnsw.HnswGraph, error)
}
