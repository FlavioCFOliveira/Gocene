// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package classification

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// KNearestNeighborClassifier classifies text using a k-nearest-neighbor
// approach backed by a Lucene index.
//
// Port of org.apache.lucene.classification.KNearestNeighborClassifier.
//
// Deviation: Full implementation deferred to backlog #2693.
type KNearestNeighborClassifier struct {
	analyzer       analysis.Analyzer
	classFieldName string
	textFieldNames []string
	k              int
	minDocsFreq    int
	minTermFreq    int
}

// NewKNearestNeighborClassifier creates the classifier.
func NewKNearestNeighborClassifier(
	_ interface{},
	analyzer analysis.Analyzer,
	_ interface{},
	k, minDocsFreq, minTermFreq int,
	classFieldName string,
	textFieldNames ...string,
) *KNearestNeighborClassifier {
	return &KNearestNeighborClassifier{
		analyzer:       analyzer,
		classFieldName: classFieldName,
		textFieldNames: textFieldNames,
		k:              k,
		minDocsFreq:    minDocsFreq,
		minTermFreq:    minTermFreq,
	}
}

// AssignClass returns nil — deferred to #2693.
func (c *KNearestNeighborClassifier) AssignClass(_ string) (*ClassificationResult[*util.BytesRef], error) {
	return nil, nil
}

// GetClasses returns nil — deferred to #2693.
func (c *KNearestNeighborClassifier) GetClasses(_ string) ([]*ClassificationResult[*util.BytesRef], error) {
	return nil, nil
}

// GetClassesMax returns nil — deferred to #2693.
func (c *KNearestNeighborClassifier) GetClassesMax(_ string, _ int) ([]*ClassificationResult[*util.BytesRef], error) {
	return nil, nil
}

// KNearestFuzzyClassifier extends KNN with fuzzy matching.
//
// Port of org.apache.lucene.classification.KNearestFuzzyClassifier.
//
// Deviation: Full implementation deferred to backlog #2693.
type KNearestFuzzyClassifier struct {
	KNearestNeighborClassifier
}

// NewKNearestFuzzyClassifier creates the fuzzy classifier.
func NewKNearestFuzzyClassifier(
	reader interface{},
	analyzer analysis.Analyzer,
	query interface{},
	k int,
	classFieldName string,
	textFieldNames ...string,
) *KNearestFuzzyClassifier {
	return &KNearestFuzzyClassifier{
		KNearestNeighborClassifier: *NewKNearestNeighborClassifier(
			reader, analyzer, query, k, 0, 0, classFieldName, textFieldNames...),
	}
}
