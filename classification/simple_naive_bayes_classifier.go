// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package classification

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SimpleNaiveBayesClassifier classifies text using a Naive Bayes model
// backed by a Lucene index.
//
// Port of org.apache.lucene.classification.SimpleNaiveBayesClassifier.
//
// Deviation: Full implementation requires IndexReader + term statistics.
// This is a stub that satisfies the Classifier[*util.BytesRef] interface;
// algorithmic body deferred to backlog #2693.
type SimpleNaiveBayesClassifier struct {
	analyzer       analysis.Analyzer
	classFieldName string
	textFieldNames []string
}

// NewSimpleNaiveBayesClassifier creates the classifier.
//
//   - reader: an IndexReader over the training index (interface{} until Gocene
//     index.IndexReader is stable)
//   - analyzer: the Analyzer used to tokenize text
//   - query: an optional filter query (interface{} placeholder)
//   - classFieldName: the field that holds the class label
//   - textFieldNames: the text fields to use for classification
func NewSimpleNaiveBayesClassifier(
	_ interface{},
	analyzer analysis.Analyzer,
	_ interface{},
	classFieldName string,
	textFieldNames ...string,
) *SimpleNaiveBayesClassifier {
	return &SimpleNaiveBayesClassifier{
		analyzer:       analyzer,
		classFieldName: classFieldName,
		textFieldNames: textFieldNames,
	}
}

// AssignClass returns nil — deferred to #2693.
func (c *SimpleNaiveBayesClassifier) AssignClass(_ string) (*ClassificationResult[*util.BytesRef], error) {
	return nil, nil
}

// GetClasses returns nil — deferred to #2693.
func (c *SimpleNaiveBayesClassifier) GetClasses(_ string) ([]*ClassificationResult[*util.BytesRef], error) {
	return nil, nil
}

// GetClassesMax returns nil — deferred to #2693.
func (c *SimpleNaiveBayesClassifier) GetClassesMax(_ string, _ int) ([]*ClassificationResult[*util.BytesRef], error) {
	return nil, nil
}
