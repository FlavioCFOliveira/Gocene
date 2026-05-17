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
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/util/hnsw"
)

// FlatVectorsReader is the Go port of
// org.apache.lucene.codecs.hnsw.FlatVectorsReader (Lucene 10.4.0). The
// reader walks every vector in the index when searched — useful for
// small fields or when used alongside an additional indexing structure
// (HNSW) that drives the search and only consults the flat reader to
// retrieve raw vectors.
//
// The Java reference is an abstract class that also implements
// Accountable; the Go port encodes that abstract surface as an
// interface composed of [codecs.KnnVectorsReader]. Concrete subclasses
// embed [BaseFlatVectorsReader] to inherit the scorer accessor and
// the no-op Search* methods, then provide GetRandomVectorScorer*
// implementations.
//
// The Java search(String, float[]/byte[], KnnCollector, AcceptDocs)
// overrides intentionally do nothing — "don't scan stored field data;
// if we didn't index it, produce no search results". Gocene mirrors
// that contract: the no-op Search* methods are part of the embeddable
// base struct so subclasses do not have to re-implement them.
//
// AcceptDocs and KnnCollector are not yet ported into Gocene; the
// Search methods therefore accept any-typed parameters to keep the
// signatures byte-for-byte aligned with the Java reference while the
// search package matures. Concrete callers will pass typed values once
// those types are ported (search-base sprint).
type FlatVectorsReader interface {
	codecs.KnnVectorsReader

	// GetFlatVectorScorer returns the scorer used by this reader to
	// score against random vectors. Mirrors the Java accessor
	// getFlatVectorScorer().
	GetFlatVectorScorer() FlatVectorsScorer

	// SearchFloat scores all stored vectors against target and
	// publishes results through knnCollector, filtered by acceptDocs.
	// The default in the Java reference is a no-op; the embeddable
	// [BaseFlatVectorsReader] preserves that no-op behaviour.
	SearchFloat(field string, target []float32, knnCollector any, acceptDocs any) error

	// SearchByte scores all stored byte vectors against target and
	// publishes results through knnCollector. Same no-op contract as
	// SearchFloat.
	SearchByte(field string, target []byte, knnCollector any, acceptDocs any) error

	// GetRandomVectorScorerFloat returns a [hnsw.RandomVectorScorer]
	// for the named field and float target vector. Mirrors the
	// abstract method getRandomVectorScorer(String, float[]).
	GetRandomVectorScorerFloat(field string, target []float32) (hnsw.RandomVectorScorer, error)

	// GetRandomVectorScorerByte returns a [hnsw.RandomVectorScorer]
	// for the named field and byte target vector. Mirrors the abstract
	// method getRandomVectorScorer(String, byte[]).
	GetRandomVectorScorerByte(field string, target []byte) (hnsw.RandomVectorScorer, error)

	// GetMergeInstance returns an instance optimized for merging. The
	// default in the Java reference returns the receiver; the
	// embeddable base reuses that default.
	GetMergeInstance() (FlatVectorsReader, error)
}

// BaseFlatVectorsReader carries the [FlatVectorsScorer] handle and
// supplies the default behaviour for GetFlatVectorScorer, SearchFloat,
// SearchByte, and GetMergeInstance. Concrete subclasses embed
// *BaseFlatVectorsReader and provide:
//
//   - CheckIntegrity, Close (from [codecs.KnnVectorsReader]);
//   - GetRandomVectorScorerFloat / GetRandomVectorScorerByte (the only
//     abstract methods on the Java original);
//   - any reader-specific accessors their codec needs.
//
// BaseFlatVectorsReader does NOT implement [codecs.KnnVectorsReader]
// itself: CheckIntegrity/Close are subclass responsibilities so each
// reader controls its own resource lifecycle. The base supplies only
// the surface that the Java reference makes concrete.
//
// The Java class also marks itself Accountable. Accountable's only
// method is ramBytesUsed(); the Gocene equivalent is not yet ported,
// so this base does not yet expose RAMBytesUsed. Subclasses that need
// it should implement it directly until the Accountable port lands.
type BaseFlatVectorsReader struct {
	vectorScorer FlatVectorsScorer
}

// NewBaseFlatVectorsReader builds a base reader bound to the supplied
// scorer. Mirrors the protected constructor FlatVectorsReader(FlatVectorsScorer).
func NewBaseFlatVectorsReader(scorer FlatVectorsScorer) *BaseFlatVectorsReader {
	return &BaseFlatVectorsReader{vectorScorer: scorer}
}

// GetFlatVectorScorer returns the scorer this reader was constructed
// with.
func (r *BaseFlatVectorsReader) GetFlatVectorScorer() FlatVectorsScorer {
	return r.vectorScorer
}

// SearchFloat is a no-op, matching the Java reference comment "don't
// scan stored field data. If we didn't index it, produce no search
// results".
func (r *BaseFlatVectorsReader) SearchFloat(_ string, _ []float32, _ any, _ any) error {
	return nil
}

// SearchByte is a no-op for the same reason as [SearchFloat].
func (r *BaseFlatVectorsReader) SearchByte(_ string, _ []byte, _ any, _ any) error {
	return nil
}

// getMergeInstanceSelf is a helper for embedders: the Java default
// returns `this`, and Go embedders pass their typed self to
// GetMergeInstance so the returned interface header carries the
// concrete subclass. See the [FlatVectorsReader.GetMergeInstance]
// godoc for the recommended subclass implementation.
//
// The base struct intentionally does NOT implement GetMergeInstance
// itself because returning a *BaseFlatVectorsReader would lose the
// subclass identity required by callers.
func (r *BaseFlatVectorsReader) getMergeInstanceSelf(self FlatVectorsReader) (FlatVectorsReader, error) {
	return self, nil
}
