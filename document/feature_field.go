// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// FeatureField stores a single (featureName, featureValue) entry in a
// fieldName, leveraging the postings term-frequency to encode the
// feature value. Useful for static scoring factors (e.g. PageRank).
//
// Go port of Lucene 10.4.0's org.apache.lucene.document.FeatureField.
//
// The featureValue is encoded into the upper 16 bits of its IEEE 754 bit
// representation and stored as the term frequency; the term itself is the
// featureName. This sacrifices precision for compact storage.
//
// Static query factories (NewSaturationQuery / NewSigmoidQuery /
// NewLogQuery / NewLinearQuery) are deferred — backlog #2699.
type FeatureField struct {
	*Field
	featureName  string
	featureValue float32
}

var (
	// FeatureFieldType is the FieldType for a FeatureField. The field is
	// indexed with DOCS_AND_FREQS postings, norms omitted, tokenization
	// disabled. Mirrors Lucene's static TYPE.
	FeatureFieldType *FieldType

	// FeatureFieldTYPE is the Lucene-canonical alias.
	FeatureFieldTYPE *FieldType
)

func init() {
	FeatureFieldType = NewFieldType()
	FeatureFieldType.SetIndexed(true)
	FeatureFieldType.SetTokenized(false)
	FeatureFieldType.SetOmitNorms(true)
	FeatureFieldType.SetIndexOptions(index.IndexOptionsDocsAndFreqs)
	FeatureFieldType.Freeze()
	FeatureFieldTYPE = FeatureFieldType
}

// NewFeatureField creates a new FeatureField with the given fieldName,
// featureName and positive featureValue. Returns an error if value is
// non-positive, non-finite, or below the smallest positive normal float32.
func NewFeatureField(fieldName, featureName string, featureValue float32) (*FeatureField, error) {
	if err := checkFeatureValue(featureValue); err != nil {
		return nil, err
	}
	if featureName == "" {
		return nil, fmt.Errorf("featureName cannot be empty")
	}
	field, err := NewField(fieldName, featureName, FeatureFieldType)
	if err != nil {
		return nil, err
	}
	return &FeatureField{Field: field, featureName: featureName, featureValue: featureValue}, nil
}

// GetFeatureName returns the underlying feature name (the term that the
// field is indexed under).
func (f *FeatureField) GetFeatureName() string { return f.featureName }

// GetFeatureValue returns the configured feature value.
func (f *FeatureField) GetFeatureValue() float32 { return f.featureValue }

// SetFeatureValue updates the feature value. Returns an error if value
// is non-positive, non-finite, or sub-normal.
func (f *FeatureField) SetFeatureValue(value float32) error {
	if err := checkFeatureValue(value); err != nil {
		return err
	}
	f.featureValue = value
	return nil
}

// EncodeFeatureValueAsTermFreq returns the int32 term-frequency
// representation of the feature value, mirroring Lucene's compact
// upper-16-bits encoding.
func EncodeFeatureValueAsTermFreq(featureValue float32) int32 {
	bits := math.Float32bits(featureValue)
	return int32(bits >> 15)
}

// DecodeFeatureValueFromTermFreq reverses EncodeFeatureValueAsTermFreq.
func DecodeFeatureValueFromTermFreq(termFreq int32) float32 {
	return math.Float32frombits(uint32(termFreq) << 15)
}

func checkFeatureValue(v float32) error {
	if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
		return fmt.Errorf("featureValue must be finite; got %v", v)
	}
	if v <= 0 {
		return fmt.Errorf("featureValue must be positive; got %v", v)
	}
	if float64(v) < math.SmallestNonzeroFloat32 {
		return fmt.Errorf("featureValue %v is below smallest positive normal float32", v)
	}
	return nil
}
