// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import "fmt"

// FeatureDoubleValuesSource is a data carrier that identifies a
// feature stored in a FeatureField by field name and feature name.
// It mirrors the class
// org.apache.lucene.document.FeatureDoubleValuesSource (Lucene 10.4.0).
//
// The actual DoubleValuesSource logic lives in the search package
// (search.FeatureDoubleValuesSource); this type holds the parameters
// that identify which feature to read.
type FeatureDoubleValuesSource struct {
	field       string
	featureName string
}

// NewFeatureDoubleValuesSource constructs a FeatureDoubleValuesSource
// data carrier. Both field and featureName must be non-empty.
func NewFeatureDoubleValuesSource(field, featureName string) (*FeatureDoubleValuesSource, error) {
	if field == "" {
		return nil, fmt.Errorf("field must not be empty")
	}
	if featureName == "" {
		return nil, fmt.Errorf("featureName must not be empty")
	}
	return &FeatureDoubleValuesSource{field: field, featureName: featureName}, nil
}

// Field returns the indexed field name.
func (s *FeatureDoubleValuesSource) Field() string { return s.field }

// FeatureName returns the feature name.
func (s *FeatureDoubleValuesSource) FeatureName() string { return s.featureName }

// String returns a human-readable representation.
func (s *FeatureDoubleValuesSource) String() string {
	return fmt.Sprintf("FeatureDoubleValuesSource(field=%s, featureName=%s)", s.field, s.featureName)
}

// Equals reports whether two FeatureDoubleValuesSource carriers are
// equal.
func (s *FeatureDoubleValuesSource) Equals(other *FeatureDoubleValuesSource) bool {
	if s == other {
		return true
	}
	if s == nil || other == nil {
		return false
	}
	return s.field == other.field && s.featureName == other.featureName
}
