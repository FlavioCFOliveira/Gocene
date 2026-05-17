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
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/hnsw"
	"github.com/FlavioCFOliveira/Gocene/util/quantization"
)

// quantizedKnnView bridges a [quantization.QuantizedByteVectorValues]
// to the [hnsw.KnnVectorValues] surface expected by the random-vector-
// scorer base classes in util/hnsw. The bridge is mechanically
// equivalent to a one-line `extends KnnVectorValues` in the Java
// reference — the divergence only exists because Gocene currently has
// two parallel KnnVectorValues stubs (one in util/hnsw, one in
// util/quantization) pending the canonical index port.
//
// The bridge is held by value and forwards every method by delegation;
// no state is carried in the wrapper itself. The iterator returned by
// Iterator() is wrapped through [quantizedDocIndexIterator] so the
// hnsw.DocIndexIterator surface is satisfied without copying the
// underlying state.
//
// TODO(rmp): drop the bridge when index.KnnVectorValues lands and both
// util packages converge on the canonical type.
type quantizedKnnView struct {
	values quantization.QuantizedByteVectorValues
}

// AsHnswKnnVectorValues wraps q so it satisfies the
// [hnsw.KnnVectorValues] interface consumed by util/hnsw scorer
// bases. Concrete callers (codec writers/readers, test peers) use
// this helper whenever they need to feed a quantized view into the
// scorer pipeline — the wrapper is the Gocene equivalent of the
// implicit upcast that the Java type system performs for free.
//
// The wrapper is cheap: it stores a single reference and forwards
// every method.
func AsHnswKnnVectorValues(q quantization.QuantizedByteVectorValues) hnsw.KnnVectorValues {
	return &quantizedKnnView{values: q}
}

// AsQuantizedByteVectorValues recovers a wrapped
// [quantization.QuantizedByteVectorValues] from a
// [hnsw.KnnVectorValues] if and only if the supplied view was produced
// by [AsHnswKnnVectorValues]. Returns the inner quantized view and
// true on a successful unwrap; otherwise returns (nil, false).
//
// The Java reference uses an `instanceof QuantizedByteVectorValues`
// downcast that succeeds whenever the canonical KnnVectorValues
// hierarchy carries a quantized leaf. Gocene's parallel stubs make
// that downcast structurally impossible: this helper exists to bridge
// the gap until the index.KnnVectorValues port unifies the two stubs.
func AsQuantizedByteVectorValues(v hnsw.KnnVectorValues) (quantization.QuantizedByteVectorValues, bool) {
	wrapper, ok := v.(*quantizedKnnView)
	if !ok {
		return nil, false
	}
	return wrapper.values, true
}

// Dimension forwards to the wrapped values.
func (v *quantizedKnnView) Dimension() int { return v.values.Dimension() }

// Size forwards to the wrapped values.
func (v *quantizedKnnView) Size() int { return v.values.Size() }

// OrdToDoc forwards to the wrapped values.
func (v *quantizedKnnView) OrdToDoc(ord int) int { return v.values.OrdToDoc(ord) }

// GetAcceptOrds forwards to the wrapped values.
func (v *quantizedKnnView) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	return v.values.GetAcceptOrds(acceptDocs)
}

// Iterator returns a thin adapter that satisfies
// [hnsw.DocIndexIterator] by delegating to the wrapped
// [quantization.DocIndexIterator].
func (v *quantizedKnnView) Iterator() hnsw.DocIndexIterator {
	return &quantizedDocIndexIterator{inner: v.values.Iterator()}
}

// quantizedDocIndexIterator wraps a quantization.DocIndexIterator so
// it satisfies the otherwise-identical hnsw.DocIndexIterator
// interface. The two interfaces are byte-for-byte the same; only the
// import path differs.
type quantizedDocIndexIterator struct {
	inner quantization.DocIndexIterator
}

// NextDoc forwards to the wrapped iterator.
func (it *quantizedDocIndexIterator) NextDoc() (int, error) {
	return it.inner.NextDoc()
}

// Index forwards to the wrapped iterator.
func (it *quantizedDocIndexIterator) Index() int {
	return it.inner.Index()
}
