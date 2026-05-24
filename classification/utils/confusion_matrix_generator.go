// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package utils

import (
	"fmt"
	"strings"
)

// ConfusionMatrix holds the result of evaluating a classifier across a
// labelled dataset.
//
// Port of org.apache.lucene.classification.utils.ConfusionMatrixGenerator.ConfusionMatrix.
type ConfusionMatrix struct {
	// counts[expected][actual] → number of documents
	counts map[string]map[string]int64
	// linearizedMatrix is the flattened [expected][actual] counts table
	linearizedMatrix  [][]int64
	classes           []string
	avgClassCorrect   float64
	avgClassPrecision float64
}

// GetLinearizedMatrix returns the confusion matrix as a 2-D slice indexed
// [expected][actual].
func (m *ConfusionMatrix) GetLinearizedMatrix() [][]int64 { return m.linearizedMatrix }

// GetClasses returns the ordered list of class labels in the matrix.
func (m *ConfusionMatrix) GetClasses() []string { return m.classes }

// GetAvgClassCorrectness returns the average per-class correctness (recall).
func (m *ConfusionMatrix) GetAvgClassCorrectness() float64 { return m.avgClassCorrect }

// GetAvgClassPrecision returns the average per-class precision.
func (m *ConfusionMatrix) GetAvgClassPrecision() float64 { return m.avgClassPrecision }

// String returns a human-readable tabular representation.
func (m *ConfusionMatrix) String() string {
	if len(m.classes) == 0 {
		return "ConfusionMatrix{}"
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("ConfusionMatrix{classes=%v, avgCorrectness=%.4f, avgPrecision=%.4f}",
		m.classes, m.avgClassCorrect, m.avgClassPrecision))
	return sb.String()
}

// ConfusionMatrixGenerator generates the confusion matrix of a Classifier.
//
// Port of org.apache.lucene.classification.utils.ConfusionMatrixGenerator.
//
// Deviation: Full implementation deferred to backlog #2693 (requires
// IndexReader, IndexSearcher, and Classifier).
type ConfusionMatrixGenerator struct{}

// GetConfusionMatrix evaluates classifier on the given index and returns the
// confusion matrix.
//
// All parameters are interface{} placeholders until the corresponding Gocene
// types are stable. Returns nil — deferred to #2693.
func GetConfusionMatrix(
	_ interface{}, // reader IndexReader
	_ interface{}, // classifier Classifier[T]
	_ string, // classFieldName
	_ string, // textFieldName
	_ int64, // timeoutMilliseconds
) (*ConfusionMatrix, error) {
	return nil, nil
}
