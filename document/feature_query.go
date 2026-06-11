// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import "fmt"

// FeatureQuery is a data carrier for a feature query that scores
// documents by their FeatureField values. It mirrors the package-private
// class org.apache.lucene.document.FeatureQuery (Lucene 10.4.0).
//
// The actual Query / Weight / Scorer logic lives in the search package;
// this type holds the field name, the feature name and the scoring
// function that the search-layer implementation consumes.
type FeatureQuery struct {
	fieldName   string
	featureName string
	function    string // place-holder: Lucene's FeatureFunction (LinearFunction,
	                   // LogFunction, SaturationFunction, SigmoidFunction) are
	                   // complex scoring closures that live in the search package
	                   // alongside the full FeatureQuery implementation.
	                   // This field holds a human-readable description of the
	                   // function for String() and Equals(); the search-package
	                   // FeatureQuery carries the real function object.
}

// NewFeatureQuery constructs a FeatureQuery data carrier.
func NewFeatureQuery(fieldName, featureName, function string) *FeatureQuery {
	return &FeatureQuery{
		fieldName:   fieldName,
		featureName: featureName,
		function:    function,
	}
}

// FieldName returns the indexed field name.
func (q *FeatureQuery) FieldName() string { return q.fieldName }

// FeatureName returns the feature name (the term under which the
// feature value is indexed).
func (q *FeatureQuery) FeatureName() string { return q.featureName }

// Function returns the human-readable description of the scoring
// function.
func (q *FeatureQuery) Function() string { return q.function }

// String returns a human-readable representation.
func (q *FeatureQuery) String() string {
	return fmt.Sprintf("FeatureQuery(field=%s, feature=%s, function=%s)", q.fieldName, q.featureName, q.function)
}

// Equals reports whether two FeatureQuery carriers are equal.
func (q *FeatureQuery) Equals(other *FeatureQuery) bool {
	if q == other {
		return true
	}
	if q == nil || other == nil {
		return false
	}
	return q.fieldName == other.fieldName &&
		q.featureName == other.featureName &&
		q.function == other.function
}
