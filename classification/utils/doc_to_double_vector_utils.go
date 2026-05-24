// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package utils provides utility types for the classification package:
// dataset splitting, confusion-matrix generation, fuzzy nearest-neighbour
// queries, and document-to-vector conversion.
//
// Port of org.apache.lucene.classification.utils.
package utils

// DocToDoubleVectorUtils provides utility functions for converting Lucene
// document term-vector data to float64 slices.
//
// Port of org.apache.lucene.classification.utils.DocToDoubleVectorUtils.
//
// Deviation: Full implementation deferred to backlog #2693 (requires Terms
// and TermsEnum from the index package).
type DocToDoubleVectorUtils struct{}

// ToSparseLocalFreqDoubleArray creates a sparse float64 vector from doc and
// field term vectors using local term frequency.
//
// Both docTerms and fieldTerms are interface{} placeholders until
// index.Terms is available.
//
// Returns nil — deferred to #2693.
func ToSparseLocalFreqDoubleArray(_, _ interface{}) ([]float64, error) {
	return nil, nil
}

// ToDenseLocalFreqDoubleArray creates a dense float64 vector from doc term
// vectors using local term frequency.
//
// docTerms is an interface{} placeholder until index.Terms is available.
//
// Returns nil — deferred to #2693.
func ToDenseLocalFreqDoubleArray(_ interface{}) ([]float64, error) {
	return nil, nil
}
