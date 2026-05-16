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

// UpdateableRandomVectorScorer is a RandomVectorScorer that allows
// its scoring ordinal to be changed in place. Useful during
// indexing to avoid creating a new scorer per node. Port of
// org.apache.lucene.util.hnsw.UpdateableRandomVectorScorer
// (Lucene 10.4.0). lucene.internal.
type UpdateableRandomVectorScorer interface {
	RandomVectorScorer

	// SetScoringOrdinal changes the scoring ordinal to the given
	// node. Returning an error here corresponds to Java's IOException.
	SetScoringOrdinal(node int) error
}

// AbstractUpdateableRandomVectorScorer is the Go counterpart to
// Lucene's inner class
// UpdateableRandomVectorScorer.AbstractUpdateableRandomVectorScorer.
// It embeds AbstractRandomVectorScorer (sharing MaxOrd / OrdToDoc /
// GetAcceptOrds / Values implementations) and concrete subclasses
// supply Score and SetScoringOrdinal.
type AbstractUpdateableRandomVectorScorer struct {
	*AbstractRandomVectorScorer
}

// NewAbstractUpdateableRandomVectorScorer builds a base scorer
// bound to the given vector values.
func NewAbstractUpdateableRandomVectorScorer(values KnnVectorValues) *AbstractUpdateableRandomVectorScorer {
	return &AbstractUpdateableRandomVectorScorer{
		AbstractRandomVectorScorer: NewAbstractRandomVectorScorer(values),
	}
}
