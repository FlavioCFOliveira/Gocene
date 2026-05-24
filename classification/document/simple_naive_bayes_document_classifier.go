// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/classification"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SimpleNaiveBayesDocumentClassifier classifies documents using a Naive Bayes
// model backed by a Lucene index. It extends SimpleNaiveBayesClassifier to
// operate on Document objects.
//
// Port of org.apache.lucene.classification.document.SimpleNaiveBayesDocumentClassifier.
//
// Deviation: Full implementation deferred to backlog #2693 (requires
// IndexReader, IndexSearcher, and MultiTerms).
type SimpleNaiveBayesDocumentClassifier struct {
	field2analyzer map[string]analysis.Analyzer
	classFieldName string
	textFieldNames []string
}

// NewSimpleNaiveBayesDocumentClassifier creates the document classifier.
func NewSimpleNaiveBayesDocumentClassifier(
	_ interface{},
	_ interface{},
	classFieldName string,
	field2analyzer map[string]analysis.Analyzer,
	textFieldNames ...string,
) *SimpleNaiveBayesDocumentClassifier {
	return &SimpleNaiveBayesDocumentClassifier{
		field2analyzer: field2analyzer,
		classFieldName: classFieldName,
		textFieldNames: textFieldNames,
	}
}

// AssignClass returns nil — deferred to #2693.
func (c *SimpleNaiveBayesDocumentClassifier) AssignClass(_ interface{}) (*classification.ClassificationResult[*util.BytesRef], error) {
	return nil, nil
}

// GetClasses returns nil — deferred to #2693.
func (c *SimpleNaiveBayesDocumentClassifier) GetClasses(_ interface{}) ([]*classification.ClassificationResult[*util.BytesRef], error) {
	return nil, nil
}

// GetClassesMax returns nil — deferred to #2693.
func (c *SimpleNaiveBayesDocumentClassifier) GetClassesMax(_ interface{}, _ int) ([]*classification.ClassificationResult[*util.BytesRef], error) {
	return nil, nil
}
