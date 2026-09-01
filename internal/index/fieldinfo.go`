package index

import (
	"errors"
	"fmt"
)

// FieldInfo describes document fields and whether or not they are indexed.
type FieldInfo struct {
	Name                    string
	Number                  int
	docValuesType           DocValuesType
	docValuesSkipIndex      DocValuesSkipIndexType
	storeTermVector         bool
	omitNorms               bool
	IndexOptions            IndexOptions
	storePayloads           bool
	Attributes              map[string]string
	dvGen                   int64
	pointDimensionCount     int
	pointIndexDimensionCount int
	pointNumBytes           int
	vectorDimension         int
	vectorEncoding          VectorEncoding
	vectorSimilarityFunction VectorSimilarityFunction
	softDeletesField        bool
	isParentField           bool
}

func NewFieldInfo(
	name string,
	number int,
	storeTermVector bool,
	omitNorms bool,
	storePayloads bool,
	indexOptions IndexOptions,
	docValues DocValuesType,
	docValuesSkipIndex DocValuesSkipIndexType,
	dvGen int64,
	attributes map[string]string,
	pointDimensionCount int,
	pointIndexDimensionCount int,
	pointNumBytes int,
	vectorDimension int,
	vectorEncoding VectorEncoding,
	vectorSimilarityFunction VectorSimilarityFunction,
	softDeletesField bool,
	isParentField bool,
) (*FieldInfo, error) {
	fi := &FieldInfo{
		Name:                    name,
		Number:                  number,
		docValuesType:           docValues,
		docValuesSkipIndex:      docValuesSkipIndex,
		IndexOptions:            indexOptions,
		Attributes:              attributes,
		dvGen:                   dvGen,
		pointDimensionCount:     pointDimensionCount,
		pointIndexDimensionCount: pointIndexDimensionCount,
		pointNumBytes:           pointNumBytes,
		vectorDimension:         vectorDimension,
		vectorEncoding:          vectorEncoding,
		vectorSimilarityFunction: vectorSimilarityFunction,
		softDeletesField:        softDeletesField,
		isParentField:           isParentField,
	}

	if indexOptions != IndexOptionsNone {
		fi.storeTermVector = storeTermVector
		fi.storePayloads = storePayloads
		fi.omitNorms = omitNorms
	}

	if err := fi.CheckConsistency(); err != nil {
		return nil, err
	}

	return fi, nil
}

func (fi *FieldInfo) CheckConsistency() error {
	if fi.IndexOptions == IndexOptionsNone {
		if fi.storeTermVector || fi.storePayloads || fi.omitNorms {
			return errors.New("non-indexed field cannot store term vectors, payloads or omit norms")
		}
	} else {
		if !fi.IndexOptions.Subsumes(IndexOptionsDocsAndFreqsAndPositions) && fi.storePayloads {
			return fmt.Errorf("indexed field '%s' cannot have payloads without positions", fi.Name)
		}
	}

	if !fi.docValuesSkipIndex.IsCompatibleWith(fi.docValuesType) {
		return fmt.Errorf("field '%s' cannot have docValuesSkipIndexType=%v with doc values type %v",
			fi.Name, fi.docValuesSkipIndex, fi.docValuesType)
	}

	if fi.dvGen != -1 && fi.docValuesType == DocValuesTypeNone {
		return fmt.Errorf("field '%s' cannot have a docvalues update generation without having docvalues", fi.Name)
	}

	if fi.pointDimensionCount < 0 || fi.pointIndexDimensionCount < 0 || fi.pointNumBytes < 0 {
		return fmt.Errorf("point dimensions/bytes must be >= 0 for field '%s'", fi.Name)
	}

	if fi.pointDimensionCount != 0 && fi.pointNumBytes == 0 {
		return fmt.Errorf("pointNumBytes must be > 0 when pointDimensionCount=%d (field: '%s')",
			fi.pointDimensionCount, fi.Name)
	}

	if fi.pointIndexDimensionCount != 0 && fi.pointDimensionCount == 0 {
		return fmt.Errorf("pointIndexDimensionCount must be 0 when pointDimensionCount=0 for field '%s'", fi.Name)
	}

	if fi.pointNumBytes != 0 && fi.pointDimensionCount == 0 {
		return fmt.Errorf("pointDimensionCount must be > 0 when pointNumBytes=%d (field: '%s')",
			fi.pointNumBytes, fi.Name)
	}

	if fi.vectorDimension < 0 {
		return fmt.Errorf("vectorDimension must be >= 0 for field '%s'", fi.Name)
	}

	if fi.softDeletesField && fi.isParentField {
		return fmt.Errorf("field '%s' cannot be used as soft-deletes field and parent document field", fi.Name)
	}

	return nil
}

func (fi *FieldInfo) SetPointDimensions(dimensionCount, indexDimensionCount, numBytes int) error {
	if dimensionCount <= 0 {
		return fmt.Errorf("point dimension count must be > 0 for field '%s'", fi.Name)
	}
	if indexDimensionCount > dimensionCount {
		return fmt.Errorf("point index dimension count must be <= point dimension count for field '%s'", fi.Name)
	}
	if numBytes <= 0 {
		return fmt.Errorf("point numBytes must be > 0 for field '%s'", fi.Name)
	}

	fi.pointDimensionCount = dimensionCount
	fi.pointIndexDimensionCount = indexDimensionCount
	fi.pointNumBytes = numBytes

	return fi.CheckConsistency()
}

func (fi *FieldInfo) Name() string {
	return fi.Name
}

func (fi *FieldInfo) Number() int {
	return fi.Number
}

func (fi *FieldInfo) DocValuesType() DocValuesType {
	return fi.docValuesType
}

func (fi *FieldInfo) OmitsNorms() bool {
	return fi.omitNorms
}

func (fi *FieldInfo) HasNorms() bool {
	return fi.IndexOptions != IndexOptionsNone && !fi.omitNorms
}

func (fi *FieldInfo) HasPayloads() bool {
	return fi.storePayloads
}

func (fi *FieldInfo) HasTermVectors() bool {
	return fi.storeTermVector
}

func (fi *FieldInfo) HasVectorValues() bool {
	return fi.vectorDimension > 0
}
