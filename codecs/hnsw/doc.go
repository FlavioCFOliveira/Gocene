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
//
// Package hnsw is the Go port of org.apache.lucene.codecs.hnsw
// (Lucene 10.4.0). It hosts the codec-side helpers used by HNSW vector
// formats: the flat (sequential, graph-less) vectors format/reader/writer
// surfaces, the FlatVectorsScorer abstraction layered on top of
// util/hnsw scorers, the per-field flat vectors writer, and the
// HnswGraphProvider/ScalarQuantizedVectorScorer integration glue
// consumed by concrete codec implementations such as
// Lucene99HnswVectorsFormat.
//
// The Java reference splits codec-side and util-side concerns: the
// abstractions defined here in codecs/hnsw delegate the heavy lifting
// (graph search, raw scorer arithmetic, scalar-quantized similarity)
// to util/hnsw and util/quantization. This package therefore depends
// on the parent codecs package (for KnnVectorsFormat/Reader/Writer,
// SegmentWriteState, SegmentReadState, KnnFieldVectorsWriter) and on
// util/hnsw + util/quantization for the random-vector-scorer plumbing.
//
// Concrete codec implementations live elsewhere (Lucene99HnswVectorsFormat,
// Lucene99FlatVectorsFormat, Lucene104ScalarQuantizedVectorsFormat); this
// package only ships the abstract bases and helpers those concrete
// formats compose against.
package hnsw
