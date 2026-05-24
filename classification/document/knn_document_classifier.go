// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/classification"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// KNearestNeighborDocumentClassifier classifies documents using a
// k-nearest-neighbor approach backed by a Lucene index. It extends
// KNearestNeighborClassifier to operate on Document objects.
//
// Port of org.apache.lucene.classification.document.KNearestNeighborDocumentClassifier.
//
// Deviation: Full implementation deferred to backlog #2693 (requires
// IndexReader, IndexSearcher, and MoreLikeThis).
type KNearestNeighborDocumentClassifier struct {
	field2analyzer map[string]analysis.Analyzer
	classFieldName string
	textFieldNames []string
	k              int
	minDocsFreq    int
	minTermFreq    int
}

// NewKNearestNeighborDocumentClassifier creates the document classifier.
func NewKNearestNeighborDocumentClassifier(
	_ interface{},
	_ interface{},
	_ interface{},
	k, minDocsFreq, minTermFreq int,
	classFieldName string,
	field2analyzer map[string]analysis.Analyzer,
	textFieldNames ...string,
) *KNearestNeighborDocumentClassifier {
	return &KNearestNeighborDocumentClassifier{
		field2analyzer: field2analyzer,
		classFieldName: classFieldName,
		textFieldNames: textFieldNames,
		k:              k,
		minDocsFreq:    minDocsFreq,
		minTermFreq:    minTermFreq,
	}
}

// AssignClass returns nil — deferred to #2693.
func (c *KNearestNeighborDocumentClassifier) AssignClass(_ interface{}) (*classification.ClassificationResult[*util.BytesRef], error) {
	return nil, nil
}

// GetClasses returns nil — deferred to #2693.
func (c *KNearestNeighborDocumentClassifier) GetClasses(_ interface{}) ([]*classification.ClassificationResult[*util.BytesRef], error) {
	return nil, nil
}

// GetClassesMax returns nil — deferred to #2693.
func (c *KNearestNeighborDocumentClassifier) GetClassesMax(_ interface{}, _ int) ([]*classification.ClassificationResult[*util.BytesRef], error) {
	return nil, nil
}
