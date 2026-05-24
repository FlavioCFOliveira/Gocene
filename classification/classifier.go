// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package classification is a port of org.apache.lucene.classification.
//
// The classification package provides machine-learning classifiers that can
// assign classes to text strings or documents using a Lucene index as
// training data.
//
// Deviation: All classifier implementations (NaiveBayes, KNN, BM25NB,
// BooleanPerceptron) require a live IndexReader / IndexSearcher.  Their
// algorithmic bodies are deferred to backlog #2693 when the Gocene search
// pipeline is fully available.  The present sprint delivers the public contract
// (interfaces, ClassificationResult, skeleton structs) so that callers can
// compile against the API.
package classification

// ClassificationResult holds an assigned class and its associated score.
//
// Port of org.apache.lucene.classification.ClassificationResult (Java record).
type ClassificationResult[T any] struct {
	// AssignedClass is the class assigned by the classifier.
	AssignedClass T
	// Score is the confidence score for the assignment (higher = more confident).
	Score float64
}

// Compare implements a descending score ordering (higher score first).
// Returns -1 if r has a higher score than other, 0 if equal, 1 if lower.
func (r ClassificationResult[T]) Compare(other ClassificationResult[T]) int {
	if other.Score > r.Score {
		return -1
	}
	if other.Score < r.Score {
		return 1
	}
	return 0
}

// Classifier assigns classes to text strings.
//
// Port of org.apache.lucene.classification.Classifier.
type Classifier[T any] interface {
	// AssignClass assigns a class (with score) to the given text.
	AssignClass(text string) (*ClassificationResult[T], error)

	// GetClasses returns all classes sorted by descending score.
	// Returns nil if the classifier does not support ranked lists.
	GetClasses(text string) ([]*ClassificationResult[T], error)

	// GetClassesMax returns the top max classes sorted by descending score.
	// Returns nil if the classifier does not support ranked lists.
	GetClassesMax(text string, max int) ([]*ClassificationResult[T], error)
}
