// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Portions adapted from Apache Lucene 10.5.0:
//
//	Licensed to the Apache Software Foundation (ASF) under one or more
//	contributor license agreements. See the NOTICE file distributed with
//	this work for additional information regarding copyright ownership.
//	The ASF licenses this file to You under the Apache License, Version 2.0
//	(the "License"); you may not use this file except in compliance with
//	the License. You may obtain a copy of the License at
//
//	    http://www.apache.org/licenses/LICENSE-2.0

package lucene102

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/quantization"
)

// BinarizedByteVectorValues is the Go port of the package-private abstract
// class org.apache.lucene.backward_codecs.lucene102.BinarizedByteVectorValues
// (Apache Lucene 10.5.0): binarized byte vector values.
//
// The abstract class extends ByteVectorValues. Its concrete members
// discretizedDimensions() and getCentroidDP() are declared here and their
// bodies are carried by [DefaultDiscretizedDimensions] and
// [DefaultGetCentroidDP]. The scorer(float[]) overload is ScorerFloat, and
// the covariant copy() is CopyBinarizedByteVectorValues.
type BinarizedByteVectorValues interface {
	index.ByteVectorValues

	// GetCorrectiveTerms retrieves the corrective terms for the given vector
	// ordinal. For the dot-product family of distances they are, in order,
	// the lower optimized interval, the upper optimized interval, the
	// dot-product of the non-centered vector with the centroid and the sum of
	// quantized components. For euclidean they are the lower optimized
	// interval, the upper optimized interval, the l2norm of the centered
	// vector and the sum of quantized components.
	GetCorrectiveTerms(vectorOrd int) (quantization.QuantizationResult, error)

	// GetQuantizer returns the quantizer used to quantize the vectors.
	GetQuantizer() *quantization.OptimizedScalarQuantizer

	// GetCentroid returns the centroid the vectors were quantized against.
	GetCentroid() ([]float32, error)

	// DiscretizedDimensions returns the dimension discretized to a multiple
	// of 64.
	DiscretizedDimensions() int

	// ScorerFloat returns a VectorScorer for the given query vector, or nil.
	ScorerFloat(query []float32) (util.VectorScorer, error)

	// CopyBinarizedByteVectorValues is the covariant copy().
	CopyBinarizedByteVectorValues() (BinarizedByteVectorValues, error)

	// GetCentroidDP returns the dot product of the centroid with itself.
	GetCentroidDP() (float32, error)
}

// DefaultDiscretizedDimensions carries the body of
// BinarizedByteVectorValues.discretizedDimensions():
// discretize(dimension(), 64).
func DefaultDiscretizedDimensions(values BinarizedByteVectorValues) int {
	return quantization.Discretize(values.Dimension(), 64)
}

// DefaultGetCentroidDP carries the body of
// BinarizedByteVectorValues.getCentroidDP(): the dot product of the centroid
// with itself.
func DefaultGetCentroidDP(values BinarizedByteVectorValues) (float32, error) {
	// this only gets executed on-merge
	centroid, err := values.GetCentroid()
	if err != nil {
		return 0, err
	}
	return util.DotProduct(centroid, centroid), nil
}
