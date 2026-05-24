// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package classification

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// BM25NBClassifier classifies text using BM25 scoring as the likelihood
// estimator inside a Naive Bayes model.
//
// Port of org.apache.lucene.classification.BM25NBClassifier.
//
// Deviation: Full implementation deferred to backlog #2693.
type BM25NBClassifier struct {
	analyzer       analysis.Analyzer
	classFieldName string
	textFieldNames []string
}

// NewBM25NBClassifier creates the classifier.
func NewBM25NBClassifier(
	_ interface{},
	analyzer analysis.Analyzer,
	_ interface{},
	classFieldName string,
	textFieldNames ...string,
) *BM25NBClassifier {
	return &BM25NBClassifier{
		analyzer:       analyzer,
		classFieldName: classFieldName,
		textFieldNames: textFieldNames,
	}
}

// AssignClass returns nil — deferred to #2693.
func (c *BM25NBClassifier) AssignClass(_ string) (*ClassificationResult[*util.BytesRef], error) {
	return nil, nil
}

// GetClasses returns nil — deferred to #2693.
func (c *BM25NBClassifier) GetClasses(_ string) ([]*ClassificationResult[*util.BytesRef], error) {
	return nil, nil
}

// GetClassesMax returns nil — deferred to #2693.
func (c *BM25NBClassifier) GetClassesMax(_ string, _ int) ([]*ClassificationResult[*util.BytesRef], error) {
	return nil, nil
}
