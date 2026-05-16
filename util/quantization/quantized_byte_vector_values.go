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

// Package quantization is the Go port of
// org.apache.lucene.util.quantization (Lucene 10.4.0). It hosts the
// scalar-quantized vector value abstractions consumed by the codec
// and HNSW layers.
package quantization

import (
	"errors"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// ErrUnsupportedOperation is the sentinel error returned by the
// default implementations of [QuantizedByteVectorValues] members that
// throw java.lang.UnsupportedOperationException in the Lucene
// reference. Concrete subtypes override the corresponding methods
// when they can provide meaningful behavior.
var ErrUnsupportedOperation = errors.New("quantization: unsupported operation")

// QuantizedByteVectorValues is the Go port of
// org.apache.lucene.util.quantization.QuantizedByteVectorValues
// (Lucene 10.4.0).
//
// It exposes a [ByteVectorValues] view of scalar-quantized vector
// bytes augmented with a per-ordinal score-correction constant used
// to recover the dequantized inner product. Implementations also
// satisfy [HasIndexSlice] so consumers can address the underlying
// storage directly for optimized scoring.
//
// All methods that surface IOException in the Java reference return
// errors here. Methods whose Java defaults throw
// UnsupportedOperationException return [ErrUnsupportedOperation] from
// the [AbstractQuantizedByteVectorValues] base; concrete embedders
// override them when they can do better.
type QuantizedByteVectorValues interface {
	ByteVectorValues
	HasIndexSlice

	// GetScalarQuantizer returns the quantizer used to produce the
	// stored byte vectors, or an error wrapping [ErrUnsupportedOperation]
	// when the implementation has no quantizer to expose. Mirrors
	// Java's `getScalarQuantizer()` default.
	GetScalarQuantizer() (ScalarQuantizer, error)

	// GetScoreCorrectionConstant returns the score correction
	// constant for the vector at the given ordinal. Mirrors Java's
	// abstract `getScoreCorrectionConstant(int ord)`.
	GetScoreCorrectionConstant(ord int) (float32, error)

	// Scorer returns a [VectorScorer] for the supplied float32 query,
	// or an error wrapping [ErrUnsupportedOperation] when scoring is
	// not implemented. The returned scorer may be nil when scoring is
	// supported but not applicable to the current view (e.g. an empty
	// segment), matching Java's contract that the scorer may be null.
	Scorer(query []float32) (VectorScorer, error)

	// Copy returns a copy of this view with independent iterator
	// state. Mirrors Java's override of `ByteVectorValues.copy()`,
	// which in the Lucene reference returns `this`. In Go the
	// embedder is responsible for returning a typed
	// QuantizedByteVectorValues; see [AbstractQuantizedByteVectorValues]
	// for the canonical "return self" pattern.
	Copy() (QuantizedByteVectorValues, error)
}

// AbstractQuantizedByteVectorValues supplies the default-method
// behavior of Lucene's QuantizedByteVectorValues so concrete
// implementations only have to provide the abstract members.
//
// It is the Go counterpart to inheriting from the Java abstract
// class: concrete types embed *AbstractQuantizedByteVectorValues and
// gain the default GetScalarQuantizer, Scorer, and GetSlice
// behaviors for free. They MUST still supply:
//
//   - the inherited [ByteVectorValues] / [KnnVectorValues] surface
//     (Dimension, Size, OrdToDoc, GetAcceptOrds, Iterator, VectorValue,
//     CopyByteVectorValues);
//   - GetScoreCorrectionConstant (Java's only abstract method on
//     QuantizedByteVectorValues);
//   - Copy returning the concrete QuantizedByteVectorValues. Most
//     embedders will simply `return self, nil` (see
//     [DefaultCopySelf]) to mirror the Java `return this` default.
//
// AbstractQuantizedByteVectorValues holds no state; embedders may
// also embed it by value if zero-cost composition is desired.
type AbstractQuantizedByteVectorValues struct{}

// GetScalarQuantizer mirrors the Java default, which throws
// UnsupportedOperationException. Concrete embedders override this
// when they can expose their quantizer.
func (*AbstractQuantizedByteVectorValues) GetScalarQuantizer() (ScalarQuantizer, error) {
	return nil, ErrUnsupportedOperation
}

// Scorer mirrors the Java default, which throws
// UnsupportedOperationException for the float32-query overload.
// Concrete embedders override this when they can produce a scorer.
func (*AbstractQuantizedByteVectorValues) Scorer(_ []float32) (VectorScorer, error) {
	return nil, ErrUnsupportedOperation
}

// GetSlice mirrors the Java default of HasIndexSlice on
// QuantizedByteVectorValues, which returns null. Concrete embedders
// override this when their values are backed by an [store.IndexInput].
func (*AbstractQuantizedByteVectorValues) GetSlice() store.IndexInput {
	return nil
}

// DefaultCopySelf is the canonical "return self" Copy implementation
// matching Lucene's default `public QuantizedByteVectorValues copy()
// { return this; }`. Concrete types whose Copy semantics are the same
// as the Java default can implement Copy as a single-line wrapper
// around DefaultCopySelf:
//
//	func (v *MyValues) Copy() (QuantizedByteVectorValues, error) {
//	    return DefaultCopySelf(v)
//	}
//
// This helper exists because Go interfaces cannot supply a
// covariant "return self" default — the base struct does not have a
// reference to the concrete enclosing value.
func DefaultCopySelf(self QuantizedByteVectorValues) (QuantizedByteVectorValues, error) {
	return self, nil
}
