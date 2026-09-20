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

package quantization

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// The org.apache.lucene.util.quantization classes of Apache Lucene 10.5.0 name
// org.apache.lucene.index.KnnVectorValues, ByteVectorValues and
// FloatVectorValues in their signatures. Gocene declares those classes once,
// in spi (see [spi.KnnVectorValues]); the aliases below let the quantization
// spelling resolve to that single declaration and member set.

// KnnVectorValues is org.apache.lucene.index.KnnVectorValues.
type KnnVectorValues = spi.KnnVectorValues

// DocIndexIterator is org.apache.lucene.index.KnnVectorValues.DocIndexIterator.
type DocIndexIterator = spi.DocIndexIterator

// ByteVectorValues is org.apache.lucene.index.ByteVectorValues.
type ByteVectorValues = spi.ByteVectorValues

// FloatVectorValues is org.apache.lucene.index.FloatVectorValues.
type FloatVectorValues = spi.FloatVectorValues

// VectorScorer mirrors org.apache.lucene.search.VectorScorer
// (Lucene 10.5.0).
type VectorScorer = util.VectorScorer

// DocIdSetIterator is an opaque handle for a util.DocIdSetIterator
// carried through the quantization layer.
type DocIdSetIterator = util.DocIdSetIterator

// HasIndexSlice mirrors org.apache.lucene.codecs.lucene95.HasIndexSlice
// (Lucene 10.5.0). Implementors expose the [store.IndexInput] backing
// their values for use by vector quantizers. The interface mirrors the
// Java original exactly: a single method returning an IndexInput or nil.
//
// This local definition avoids a circular import between util/quantization
// and codecs/lucene95.
type HasIndexSlice interface {
	// GetSlice returns the [store.IndexInput] from which this
	// instance's values are read, or nil if not available.
	GetSlice() store.IndexInput
}
