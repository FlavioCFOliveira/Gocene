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
	"math"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// RandomVectorScorer scores random nodes in batches against an
// abstract query. Port of org.apache.lucene.util.hnsw.RandomVectorScorer
// (Lucene 10.4.0). Not thread-safe; intended for single-thread use.
//
// Java's default methods bulkScore, ordToDoc, getAcceptOrds are
// provided as the free function BulkScore plus default
// implementations on AbstractRandomVectorScorer.
type RandomVectorScorer interface {
	// Score returns the score between the query and the provided
	// node.
	Score(node int) (float32, error)

	// BulkScore scores a list of numNodes and stores the results in
	// the scores array. Returns the maximum score, or
	// float32(math.Inf(-1)) when numNodes == 0.
	BulkScore(nodes []int, scores []float32, numNodes int) (float32, error)

	// MaxOrd returns the maximum possible ordinal for this scorer.
	MaxOrd() int

	// OrdToDoc translates a vector ordinal to a document ID. Default
	// is identity.
	OrdToDoc(ord int) int

	// GetAcceptOrds returns the Bits representing live documents.
	// Default is identity.
	GetAcceptOrds(acceptDocs util.Bits) util.Bits
}

// BulkScoreDefault is the default implementation of the Lucene
// bulkScore default method, exposed so concrete scorers can embed it
// trivially. Returns -Inf when numNodes == 0.
func BulkScoreDefault(scorer RandomVectorScorer, nodes []int, scores []float32, numNodes int) (float32, error) {
	max := float32(math.Inf(-1))
	for i := 0; i < numNodes; i++ {
		s, err := scorer.Score(nodes[i])
		if err != nil {
			return 0, err
		}
		scores[i] = s
		if s > max {
			max = s
		}
	}
	return max, nil
}

// AbstractRandomVectorScorer is the Go counterpart to Lucene's
// inner class RandomVectorScorer.AbstractRandomVectorScorer. It
// implements MaxOrd, OrdToDoc, GetAcceptOrds, Values, and delegates
// Score (plus BulkScore) to the embedding concrete type. Concrete
// scorers should embed *AbstractRandomVectorScorer (constructed via
// NewAbstractRandomVectorScorer) and supply their own Score method.
type AbstractRandomVectorScorer struct {
	values KnnVectorValues
}

// NewAbstractRandomVectorScorer builds a base scorer bound to the
// given vector values. Mirrors the Java constructor.
func NewAbstractRandomVectorScorer(values KnnVectorValues) *AbstractRandomVectorScorer {
	return &AbstractRandomVectorScorer{values: values}
}

// MaxOrd returns values.Size().
func (b *AbstractRandomVectorScorer) MaxOrd() int { return b.values.Size() }

// OrdToDoc delegates to values.OrdToDoc.
func (b *AbstractRandomVectorScorer) OrdToDoc(ord int) int { return b.values.OrdToDoc(ord) }

// GetAcceptOrds delegates to values.GetAcceptOrds.
func (b *AbstractRandomVectorScorer) GetAcceptOrds(acceptDocs util.Bits) util.Bits {
	return b.values.GetAcceptOrds(acceptDocs)
}

// Values returns the backing vector values, implementing
// HasKnnVectorValues.
func (b *AbstractRandomVectorScorer) Values() KnnVectorValues { return b.values }
