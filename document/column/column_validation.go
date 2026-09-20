// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package column

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

const (
	// FeatureInversion is the bit for inverted-index feature (indexOptions != NONE).
	FeatureInversion = 1 << 0
	// FeatureStored is the bit for stored-fields feature (stored()).
	FeatureStored = 1 << 1
	// FeatureDocValues is the bit for doc-values feature (docValuesType != NONE).
	FeatureDocValues = 1 << 2
	// FeaturePoints is the bit for points feature (pointDimensionCount != 0).
	FeaturePoints = 1 << 3
	// FeatureVector is the bit for KNN-vector feature (vectorDimension != 0).
	FeatureVector = 1 << 4
)

// FeatureMask returns a bitmask of the indexing features (Feature*) declared by fieldType.
// Used by the column-batch path to enforce that, when several columns share a field name,
// each feature is carried by at most one column.
func FeatureMask(fieldType spi.IndexableFieldType) int {
	mask := 0
	if fieldType.IndexOptions() != spi.IndexOptionsNone {
		mask |= FeatureInversion
	}
	if fieldType.Stored() {
		mask |= FeatureStored
	}
	if fieldType.DocValuesType() != spi.DocValuesTypeNone {
		mask |= FeatureDocValues
	}
	if fieldType.PointDimensionCount() != 0 {
		mask |= FeaturePoints
	}
	if fieldType.VectorDimension() != 0 {
		mask |= FeatureVector
	}
	return mask
}

// FeatureNames returns a human-readable comma-separated list of the feature names set in mask.
func FeatureNames(mask int) string {
	var sb strings.Builder
	if (mask & FeatureInversion) != 0 {
		sb.WriteString("inversion")
	}
	if (mask & FeatureStored) != 0 {
		if sb.Len() > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("stored")
	}
	if (mask & FeatureDocValues) != 0 {
		if sb.Len() > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("doc values")
	}
	if (mask & FeaturePoints) != 0 {
		if sb.Len() > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("points")
	}
	if (mask & FeatureVector) != 0 {
		if sb.Len() > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("vectors")
	}
	return "[" + sb.String() + "]"
}

// ValidateColumnHasIndexingFeature panics if fieldType declares no indexing feature
// (no doc values, no points, not stored, no index options, no vectors).
func ValidateColumnHasIndexingFeature(fieldName string, fieldType spi.IndexableFieldType) {
	if fieldType.DocValuesType() == spi.DocValuesTypeNone &&
		fieldType.PointDimensionCount() == 0 &&
		fieldType.Stored() == false &&
		fieldType.IndexOptions() == spi.IndexOptionsNone &&
		fieldType.VectorDimension() == 0 {
		panic(fmt.Sprintf("Column %q must have a non-NONE docValuesType, point dimensions, be stored, have index options, or have vector dimensions", fieldName))
	}
}

// ValidateLongColumn validates a LongColumn against the field type it will feed.
func ValidateLongColumn(column LongColumn, fieldType spi.IndexableFieldType) {
	pointDims := fieldType.PointDimensionCount()
	if pointDims != 0 {
		if pointDims != 1 {
			panic(fmt.Sprintf("LongColumn %q only supports 1-dimensional point fields, got pointDimensionCount=%d", column.Name(), pointDims))
		}
		expectedPointBytes := 8
		if column.NumericKind() == LongNumericKindInt || column.NumericKind() == LongNumericKindFloat {
			expectedPointBytes = 4
		}
		if fieldType.PointNumBytes() != expectedPointBytes {
			panic(fmt.Sprintf("LongColumn %q numericKind=%d requires pointNumBytes=%d, got %d", column.Name(), column.NumericKind(), expectedPointBytes, fieldType.PointNumBytes()))
		}
	}
	if fieldType.Stored() {
		storedType := column.StoredType()
		switch storedType {
		case document.StoredValueTypeInteger, document.StoredValueTypeLong, document.StoredValueTypeFloat, document.StoredValueTypeDouble:
			// OK.
		case document.StoredValueTypeString, document.StoredValueTypeBinary:
			panic(fmt.Sprintf("LongColumn %q storedType=%s is not supported; use a BinaryColumn for non-numeric stored data", column.Name(), storedType))
		case document.StoredValueTypeDataInput:
			panic(fmt.Sprintf("LongColumn %q storedType DATA_INPUT is not supported for columns", column.Name()))
		}
	}
}

// ValidateBinaryColumn validates a BinaryColumn against the field type it will feed.
func ValidateBinaryColumn(column BinaryColumn, fieldType spi.IndexableFieldType) {
	dvType := fieldType.DocValuesType()
	if dvType == spi.DocValuesTypeNumeric || dvType == spi.DocValuesTypeSortedNumeric {
		panic(fmt.Sprintf("BinaryColumn %q cannot feed docValuesType=%s; use a LongColumn", column.Name(), dvType))
	}
	if fieldType.Stored() {
		storedType := column.StoredType()
		switch storedType {
		case document.StoredValueTypeBinary, document.StoredValueTypeString:
			// OK.
		case document.StoredValueTypeInteger, document.StoredValueTypeLong, document.StoredValueTypeFloat, document.StoredValueTypeDouble:
			panic(fmt.Sprintf("BinaryColumn %q storedType=%s is not supported; use a LongColumn for numeric stored data", column.Name(), storedType))
		case document.StoredValueTypeDataInput:
			panic(fmt.Sprintf("BinaryColumn %q storedType DATA_INPUT is not supported for columns", column.Name()))
		}
	}
}

// ValidateDictionaryColumn validates a DictionaryColumn against the field type it will feed.
func ValidateDictionaryColumn(column DictionaryColumn, fieldType spi.IndexableFieldType) {
	dv := fieldType.DocValuesType()
	if dv == spi.DocValuesTypeNumeric || dv == spi.DocValuesTypeSortedNumeric {
		panic(fmt.Sprintf("DictionaryColumn %q cannot feed docValuesType=%s; use a LongColumn", column.Name(), dv))
	}
	if dv == spi.DocValuesTypeBinary {
		panic(fmt.Sprintf("DictionaryColumn %q cannot feed docValuesType=BINARY (the writer does not dedup terms, so the dictionary provides no benefit); use a BinaryColumn", column.Name()))
	}
	if fieldType.PointDimensionCount() != 0 {
		panic(fmt.Sprintf("DictionaryColumn %q does not support points (pointDimensionCount must be 0)", column.Name()))
	}
	if fieldType.Stored() {
		storedType := column.StoredType()
		switch storedType {
		case document.StoredValueTypeBinary, document.StoredValueTypeString:
			// OK.
		case document.StoredValueTypeInteger, document.StoredValueTypeLong, document.StoredValueTypeFloat, document.StoredValueTypeDouble:
			panic(fmt.Sprintf("DictionaryColumn %q storedType=%s is not supported; use a LongColumn for numeric stored data", column.Name(), storedType))
		case document.StoredValueTypeDataInput:
			panic(fmt.Sprintf("DictionaryColumn %q storedType DATA_INPUT is not supported for columns", column.Name()))
		}
	}
}

// ValidateTokenStreamColumn validates a TokenStreamColumn against the field type it will feed.
func ValidateTokenStreamColumn(column TokenStreamColumn, fieldType spi.IndexableFieldType) {
	if fieldType.IndexOptions() == spi.IndexOptionsNone || fieldType.Tokenized() == false {
		panic(fmt.Sprintf("TokenStreamColumn %q requires indexOptions != NONE and tokenized == true; got indexOptions=%s, tokenized=%t", column.Name(), fieldType.IndexOptions(), fieldType.Tokenized()))
	}
	if fieldType.Stored() ||
		fieldType.DocValuesType() != spi.DocValuesTypeNone ||
		fieldType.PointDimensionCount() != 0 ||
		fieldType.VectorDimension() != 0 {
		panic(fmt.Sprintf("TokenStreamColumn %q must be inverted-only: stored=false, docValuesType=NONE, pointDimensionCount=0, vectorDimension=0", column.Name()))
	}
}

// ValidateVectorColumn validates a VectorColumn against the field type it will feed.
func ValidateVectorColumn(column VectorColumn, fieldType spi.IndexableFieldType) {
	if fieldType.VectorDimension() <= 0 {
		panic(fmt.Sprintf("VectorColumn %q requires fieldType.vectorDimension() > 0; got %d", column.Name(), fieldType.VectorDimension()))
	}
	if fieldType.DocValuesType() != spi.DocValuesTypeNone ||
		fieldType.PointDimensionCount() != 0 ||
		fieldType.Stored() ||
		fieldType.IndexOptions() != spi.IndexOptionsNone {
		panic(fmt.Sprintf("VectorColumn %q must be vector-only: docValuesType=NONE, pointDimensionCount=0, stored=false, indexOptions=NONE", column.Name()))
	}
}

// CheckDocID panics if batchDocID is outside [0, numDocs).
func CheckDocID(column Column, batchDocID, numDocs int) {
	if batchDocID < 0 || batchDocID >= numDocs {
		panic(fmt.Sprintf("Column %q returned batch doc-id %d which is out of range [0, %d)", column.Name(), batchDocID, numDocs))
	}
}

// CheckDenseCount panics if a dense column did not produce exactly numDocs values.
func CheckDenseCount(column Column, consumed, numDocs int) {
	if consumed != numDocs {
		panic(fmt.Sprintf("Dense column %q provided %d values but batch has %d documents", column.Name(), consumed, numDocs))
	}
}

// CheckVectorDocIDStrictlyIncreasing panics if a vector cursor doc-id is not strictly greater than the previous one.
func CheckVectorDocIDStrictlyIncreasing(column VectorColumn, batchDocID, prevBatchDocID int) {
	if batchDocID <= prevBatchDocID {
		panic(fmt.Sprintf("VectorColumn %q must yield strictly increasing batch doc-ids; got %d after %d", column.Name(), batchDocID, prevBatchDocID))
	}
}

// CheckVectorDimension panics if a vector value's length does not match the field's declared dimension.
func CheckVectorDimension(column VectorColumn, actual, expected, batchDocID int) {
	if actual != expected {
		panic(fmt.Sprintf("VectorColumn %q expected dimension %d but got vector of length %d at batch doc %d", column.Name(), expected, actual, batchDocID))
	}
}
