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
	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/hnsw"
)

// FlatVectorsReader is the Go port of
// org.apache.lucene.codecs.hnsw.FlatVectorsReader (Apache Lucene 10.5.0).
// It reads vectors from an index. When searching this reader, it iterates
// every vector in the index and scores them. It is useful when the number of
// vectors is small, or when used alongside some additional indexing
// structure that can be used to better search the vectors (like HNSW).
//
// The Java reference is an abstract class extending KnnVectorsReader and
// implementing Accountable. Its two search overrides do nothing ("don't scan
// stored field data. If we didn't index it, produce no search results");
// concrete readers embed [BaseFlatVectorsReader] to inherit them. Go has no
// overloading, so the float[] and byte[] overloads are suffixed Float and
// Byte.
//
// getMergeInstance() narrows its return type to FlatVectorsReader in Java.
// A Go interface cannot redeclare the inherited
// [spi.KnnVectorsReader.GetMergeInstance] with a covariant result, so
// implementers return themselves as an spi.KnnVectorsReader and callers
// that need the flat surface assert it.
type FlatVectorsReader interface {
	spi.KnnVectorsReader
	util.Accountable

	// GetFlatVectorScorer returns a [FlatVectorsScorer] for the given field.
	GetFlatVectorScorer(field string) (FlatVectorsScorer, error)

	// SearchFloat is search(String, float[], KnnCollector, AcceptDocs).
	SearchFloat(field string, target []float32, knnCollector spi.KnnCollector, acceptDocs search.AcceptDocs) error

	// SearchByte is search(String, byte[], KnnCollector, AcceptDocs).
	SearchByte(field string, target []byte, knnCollector spi.KnnCollector, acceptDocs search.AcceptDocs) error

	// GetRandomVectorScorerFloat returns a [hnsw.RandomVectorScorer] for the
	// given field and target vector. Mirrors the abstract
	// getRandomVectorScorer(String, float[]).
	GetRandomVectorScorerFloat(field string, target []float32) (hnsw.RandomVectorScorer, error)

	// GetRandomVectorScorerByte returns a [hnsw.RandomVectorScorer] for the
	// given field and target vector. Mirrors the abstract
	// getRandomVectorScorer(String, byte[]).
	GetRandomVectorScorerByte(field string, target []byte) (hnsw.RandomVectorScorer, error)
}

// BaseFlatVectorsReader carries the concrete search overrides of the
// abstract Java class. Concrete readers embed it and provide every other
// member of [FlatVectorsReader].
type BaseFlatVectorsReader struct{}

// SearchFloat does nothing: "don't scan stored field data. If we didn't
// index it, produce no search results".
func (BaseFlatVectorsReader) SearchFloat(_ string, _ []float32, _ spi.KnnCollector, _ search.AcceptDocs) error {
	return nil
}

// SearchByte does nothing, for the same reason as SearchFloat.
func (BaseFlatVectorsReader) SearchByte(_ string, _ []byte, _ spi.KnnCollector, _ search.AcceptDocs) error {
	return nil
}
