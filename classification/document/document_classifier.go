// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package document provides document-level classifiers that operate on
// [org.apache.lucene.document.Document] objects rather than plain text.
//
// Port of org.apache.lucene.classification.document.
package document

import (
	"github.com/FlavioCFOliveira/Gocene/classification"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// DocumentClassifier assigns class labels of type T to documents.
//
// Port of org.apache.lucene.classification.document.DocumentClassifier.
type DocumentClassifier[T any] interface {
	// AssignClass returns the best class (with score) for the given document.
	AssignClass(document interface{}) (*classification.ClassificationResult[T], error)

	// GetClasses returns all classes sorted by score descending for the document.
	// Returns nil if the classifier cannot produce a ranked list.
	GetClasses(document interface{}) ([]*classification.ClassificationResult[T], error)

	// GetClassesMax returns the top max classes sorted by score descending.
	// Returns nil if the classifier cannot produce a ranked list.
	GetClassesMax(document interface{}, max int) ([]*classification.ClassificationResult[T], error)
}

// Ensure the concrete stubs satisfy the interface at compile time.
var _ DocumentClassifier[*util.BytesRef] = (*KNearestNeighborDocumentClassifier)(nil)
var _ DocumentClassifier[*util.BytesRef] = (*SimpleNaiveBayesDocumentClassifier)(nil)
