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

// KnnVectorValues is a temporary local stub of
// org.apache.lucene.index.KnnVectorValues. The full type lives in
// Lucene's `index` package and has not been ported yet to Gocene.
// HasKnnVectorValues only needs to reference an opaque type for
// future binding, so this local interface intentionally exposes the
// minimal surface needed across the hnsw package: dimension and size.
//
// TODO(rmp): unify with the canonical index.KnnVectorValues once
// that port lands (index sprint, currently L22 in the roadmap).
type KnnVectorValues interface {
	// Dimension returns the dimensionality of the vectors.
	Dimension() int

	// Size returns the number of vectors for the field.
	Size() int
}

// HasKnnVectorValues is implemented by types that can return the
// KnnVectorValues backing their scorers. Port of
// org.apache.lucene.util.hnsw.HasKnnVectorValues (Lucene 10.4.0).
type HasKnnVectorValues interface {
	// Values returns the backing vector values, or nil.
	Values() KnnVectorValues
}
