// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package classification

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// BooleanPerceptronClassifier classifies text using a binary perceptron
// model backed by a Lucene index.
//
// Port of org.apache.lucene.classification.BooleanPerceptronClassifier.
//
// Deviation: Full implementation deferred to backlog #2693.
type BooleanPerceptronClassifier struct {
	analyzer       analysis.Analyzer
	classFieldName string
	textFieldName  string
	threshold      float64
}

// NewBooleanPerceptronClassifier creates the classifier.
func NewBooleanPerceptronClassifier(
	_ interface{},
	analyzer analysis.Analyzer,
	_ interface{},
	threshold *float64,
	classFieldName string,
	textFieldName string,
) *BooleanPerceptronClassifier {
	t := 0.0
	if threshold != nil {
		t = *threshold
	}
	return &BooleanPerceptronClassifier{
		analyzer:       analyzer,
		classFieldName: classFieldName,
		textFieldName:  textFieldName,
		threshold:      t,
	}
}

// AssignClass returns nil — deferred to #2693.
func (c *BooleanPerceptronClassifier) AssignClass(_ string) (*ClassificationResult[*util.BytesRef], error) {
	return nil, nil
}

// GetClasses returns nil — deferred to #2693.
func (c *BooleanPerceptronClassifier) GetClasses(_ string) ([]*ClassificationResult[*util.BytesRef], error) {
	return nil, nil
}

// GetClassesMax returns nil — deferred to #2693.
func (c *BooleanPerceptronClassifier) GetClassesMax(_ string, _ int) ([]*ClassificationResult[*util.BytesRef], error) {
	return nil, nil
}
