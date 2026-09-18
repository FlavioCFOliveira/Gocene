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

import "github.com/FlavioCFOliveira/Gocene/spi"

// KnnVectorValues is org.apache.lucene.index.KnnVectorValues, the type that
// every org.apache.lucene.util.hnsw class of Apache Lucene 10.5.0 names in
// its signatures (RandomVectorScorer.AbstractRandomVectorScorer,
// HasKnnVectorValues.values(), HnswGraphMerger.merge, ...). Lucene declares
// the class once; Gocene declares it once in spi (see
// [spi.KnnVectorValues]) and this alias lets the hnsw spelling resolve to
// that single declaration.
type KnnVectorValues = spi.KnnVectorValues

// DocIndexIterator is org.apache.lucene.index.KnnVectorValues.DocIndexIterator;
// see [spi.DocIndexIterator].
type DocIndexIterator = spi.DocIndexIterator

// HasKnnVectorValues is implemented by types that can return the
// KnnVectorValues backing their scorers. Port of
// org.apache.lucene.util.hnsw.HasKnnVectorValues (Lucene 10.5.0).
type HasKnnVectorValues interface {
	// Values returns the backing vector values, or nil.
	Values() KnnVectorValues
}
