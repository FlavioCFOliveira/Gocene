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

package util

import (
	"fmt"
	"strings"
)

// TermAndVector is a Word2Vec unit composed of a term with its associated vector.
//
// This is a port of org.apache.lucene.util.TermAndVector. The original is a Java
// record with two components; here it is exposed as a plain struct holding a
// BytesRef term and a float32 vector. The struct preserves the contract of the
// Java record (size, normalization, toString) byte-for-byte.
//
// Lucene marks this class as experimental; consumers should treat the API
// stability the same way.
type TermAndVector struct {
	// Term is the term associated with the vector. The pointer aliases the
	// caller's BytesRef directly, matching Lucene's record semantics.
	Term *BytesRef

	// Vector holds the embedding values. The slice aliases the caller's array,
	// matching Lucene's record semantics.
	Vector []float32
}

// NewTermAndVector constructs a TermAndVector aliasing the provided term and
// vector slice. Inputs are not copied; this mirrors Lucene's record behaviour.
func NewTermAndVector(term *BytesRef, vector []float32) *TermAndVector {
	return &TermAndVector{Term: term, Vector: vector}
}

// Size returns the number of components in the vector.
func (tv *TermAndVector) Size() int {
	return len(tv.Vector)
}

// NormalizeVector returns a new TermAndVector whose vector is normalized
// according to the L2 norm. The original vector is not mutated; the term is
// aliased into the result. The normalization rule matches Lucene's
// VectorUtil.l2normalize: a zero-length vector triggers a panic to mirror the
// IllegalArgumentException thrown by Java; otherwise each component is divided
// by the L2 norm. Vectors already within EPSILON (1e-4) of unit length are
// returned unchanged, exactly as in Lucene.
func (tv *TermAndVector) NormalizeVector() *TermAndVector {
	cloned := make([]float32, len(tv.Vector))
	copy(cloned, tv.Vector)
	L2Normalize(cloned)
	return &TermAndVector{Term: tv.Term, Vector: cloned}
}

// String returns a textual representation matching Lucene's TermAndVector.toString.
// The term is rendered as UTF-8 text followed by the vector components formatted
// with three decimals using a "."-decimal separator (Locale.ROOT in Java).
func (tv *TermAndVector) String() string {
	var b strings.Builder
	if tv.Term != nil {
		b.WriteString(tv.Term.Utf8ToString())
	}
	b.WriteString(" [")
	n := len(tv.Vector)
	if n > 0 {
		for i := 0; i < n-1; i++ {
			fmt.Fprintf(&b, "%.3f,", tv.Vector[i])
		}
		fmt.Fprintf(&b, "%.3f]", tv.Vector[n-1])
	}
	return b.String()
}
