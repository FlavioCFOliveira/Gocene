// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync"
)

// fieldDimensions mirrors Lucene's FieldDimensions record.
type fieldDimensions struct {
	dimensionCount      int
	indexDimensionCount int
	dimensionNumBytes   int
}

// fieldVectorProperties mirrors Lucene's FieldVectorProperties record.
type fieldVectorProperties struct {
	numDimensions      int
	vectorEncoding      VectorEncoding
	similarityFunction VectorSimilarityFunction
}

// indexOptionsProperties mirrors Lucene's IndexOptionsProperties record.
type indexOptionsProperties struct {
	storeTermVectors bool
	omitNorms        bool
}

// fieldProperties mirrors Lucene's FieldProperties record.
type fieldProperties struct {
	number                 int
	indexOptions           IndexOptions
	indexOptionsProperties *indexOptionsProperties
	docValuesType          DocValuesType
	docValuesSkipIndex     DocValuesSkipIndexType
	fieldDimensions        fieldDimensions
	fieldVectorProperties  fieldVectorProperties
}

// FieldNumbers maps field names to global field numbers and enforces schema consistency.
// It is a port of Lucene's org.apache.lucene.index.FieldInfos.FieldNumbers.
type FieldNumbers struct {
	mu sync.Mutex

	// numberToName maps field number to field name.
	numberToName map[int]string

	// fieldProperties maps field name to its schema properties.
	fieldProperties map[string]*fieldProperties

	// lowestUnassignedFieldNumber tracks the next available field number.
	lowestUnassignedFieldNumber int

	// softDeletesFieldName is the name of the soft-deletes field.
	softDeletesFieldName string

	// parentFieldName is the name of the parent document field.
	parentFieldName string
}

// NewFieldNumbers creates a new FieldNumbers instance.
func NewFieldNumbers(softDeletesFieldName, parentFieldName string) *FieldNumbers {
	if softDeletesFieldName != "" && parentFieldName != "" && parentFieldName == softDeletesFieldName {
		panic(fmt.Sprintf("parent document and soft-deletes field can't be the same field %q", parentFieldName))
	}

	return &FieldNumbers{
		numberToName:               make(map[int]string),
		fieldProperties:            make(map[string]*fieldProperties),
		lowestUnassignedFieldNumber: -1,
		softDeletesFieldName:      softDeletesFieldName,
		parentFieldName:           parentFieldName,
	}
}

// VerifyFieldInfo checks if the provided FieldInfo matches the global schema.
func (fn *FieldNumbers) VerifyFieldInfo(fi *FieldInfo) {
	fn.mu.Lock()
	defer fn.mu.Unlock()

	fieldName := fi.Name()
	fn.verifySoftDeletedFieldName(fieldName, fi.IsSoftDeletesField())
	fn.verifyParentFieldName(fieldName, fi.IsParentField())

	if props, ok := fn.fieldProperties[fieldName]; ok {
		fn.verifySameSchema(fi, props)
	}
}

// AddOrGet returns the global field number for the given FieldInfo.
// If the field does not exist, it is added with the provided number if available,
// otherwise the first unassigned number is used.
func (fn *FieldNumbers) AddOrGet(fi *FieldInfo) int {
	fn.mu.Lock()
	defer fn.mu.Unlock()

	fieldName := fi.Name()
	fn.verifySoftDeletedFieldName(fieldName, fi.IsSoftDeletesField())
	fn.verifyParentFieldName(fieldName, fi.IsParentField())

	props, ok := fn.fieldProperties[fieldName]
	if ok {
		fn.verifySameSchema(fi, props)
	} else {
		var fieldNumber int
		if fi.Number() != -1 {
			if _, taken := fn.numberToName[fi.Number()]; !taken {
				fieldNumber = fi.Number()
			} else {
				// find a new FieldNumber
				for {
					fn.lowestUnassignedFieldNumber++
					if _, taken := fn.numberToName[fn.lowestUnassignedFieldNumber]; !taken {
						fieldNumber = fn.lowestUnassignedFieldNumber
						break
					}
				}
			}
		} else {
			// find a new FieldNumber
			for {
				fn.lowestUnassignedFieldNumber++
				if _, taken := fn.numberToName[fn.lowestUnassignedFieldNumber]; !taken {
					fieldNumber = fn.lowestUnassignedFieldNumber
					break
				}
			}
		}

		fn.numberToName[fieldNumber] = fieldName
		props = &fieldProperties{
			number:       fieldNumber,
			indexOptions: fi.IndexOptions(),
			docValuesType: fi.DocValuesType(),
			docValuesSkipIndex: fi.DocValuesSkipIndexType(),
			fieldDimensions: fieldDimensions{
				dimensionCount:      fi.PointDimensionCount(),
				indexDimensionCount: fi.PointIndexDimensionCount(),
				dimensionNumBytes:   fi.PointNumBytes(),
			},
			fieldVectorProperties: fieldVectorProperties{
				numDimensions:      fi.VectorDimension(),
				vectorEncoding:      fi.VectorEncoding(),
				similarityFunction: fi.VectorSimilarityFunction(),
			},
		}

		if fi.IndexOptions() != IndexOptionsNone {
			props.indexOptionsProperties = &indexOptionsProperties{
				storeTermVectors: fi.HasTermVectors(),
				omitNorms:        fi.OmitNorms(),
			}
		}

		fn.fieldProperties[fieldName] = props
	}

	return props.number
}

func (fn *FieldNumbers) verifySoftDeletedFieldName(fieldName string, isSoftDeletesField bool) {
	if isSoftDeletesField {
		if fn.softDeletesFieldName == "" {
			panic(fmt.Sprintf("this index has [%s] as soft-deletes already but soft-deletes field is not configured in IWC", fieldName))
		} else if fieldName != fn.softDeletesFieldName {
			panic(fmt.Sprintf("cannot configure [%s] as soft-deletes; this index uses [%s] as soft-deletes already", fn.softDeletesFieldName, fieldName))
		}
	} else if fieldName == fn.softDeletesFieldName {
		panic(fmt.Sprintf("cannot configure [%s] as soft-deletes; this index uses [%s] as non-soft-deletes already", fn.softDeletesFieldName, fieldName))
	}
}

func (fn *FieldNumbers) verifyParentFieldName(fieldName string, isParentField bool) {
	if isParentField {
		if fn.parentFieldName == "" {
			panic(fmt.Sprintf("can't add field [%s] as parent document field; this IndexWriter has no parent document field configured", fieldName))
		} else if fieldName != fn.parentFieldName {
			panic(fmt.Sprintf("can't add field [%s] as parent document field; this IndexWriter is configured with [%s] as parent document field", fieldName, fn.parentFieldName))
		}
	} else if fieldName == fn.parentFieldName {
		panic(fmt.Sprintf("can't add [%s] as non parent document field; this IndexWriter is configured with [%s] as parent document field", fieldName, fn.parentFieldName))
	}
}

func (fn *FieldNumbers) verifySameSchema(fi *FieldInfo, props *fieldProperties) {
	fieldName := fi.Name()

	if fi.IndexOptions() != props.indexOptions {
		panic(fmt.Sprintf("cannot change field %q from index options=%v to inconsistent index options=%v", fieldName, props.indexOptions, fi.IndexOptions()))
	}

	if props.indexOptions != IndexOptionsNone {
		if props.indexOptionsProperties.storeTermVectors != fi.HasTermVectors() {
			panic(fmt.Sprintf("cannot change field %q from storeTermVector=%v to inconsistent storeTermVector=%v", fieldName, props.indexOptionsProperties.storeTermVectors, fi.HasTermVectors()))
		}
		if props.indexOptionsProperties.omitNorms != fi.OmitNorms() {
			panic(fmt.Sprintf("cannot change field %q from omitNorms=%v to inconsistent omitNorms=%v", fieldName, props.indexOptionsProperties.omitNorms, fi.OmitNorms()))
		}
	}

	if fi.DocValuesType() != props.docValuesType {
		panic(fmt.Sprintf("cannot change field %q from doc values type=%v to inconsistent doc values type=%v", fieldName, props.docValuesType, fi.DocValuesType()))
	}

	if fi.DocValuesSkipIndexType() != props.docValuesSkipIndex {
		panic(fmt.Sprintf("cannot change field %q from docValuesSkipIndexType=%v to inconsistent docValuesSkipIndexType=%v", fieldName, props.docValuesSkipIndex, fi.DocValuesSkipIndexType()))
	}

	if fi.PointDimensionCount() != props.fieldDimensions.dimensionCount ||
		fi.PointIndexDimensionCount() != props.fieldDimensions.indexDimensionCount ||
		fi.PointNumBytes() != props.fieldDimensions.dimensionNumBytes {
		panic(fmt.Sprintf("cannot change field %q from points dimensionCount=%d, indexDimensionCount=%d, numBytes=%d to inconsistent dimensionCount=%d, indexDimensionCount=%d, numBytes=%d",
			fieldName, props.fieldDimensions.dimensionCount, props.fieldDimensions.indexDimensionCount, props.fieldDimensions.dimensionNumBytes,
			fi.PointDimensionCount(), fi.PointIndexDimensionCount(), fi.PointNumBytes()))
	}

	if fi.VectorDimension() != props.fieldVectorProperties.numDimensions ||
		fi.VectorEncoding() != props.fieldVectorProperties.vectorEncoding ||
		fi.VectorSimilarityFunction() != props.fieldVectorProperties.similarityFunction {
		panic(fmt.Sprintf("cannot change field %q from vector dimension=%d, vector encoding=%v, vector similarity function=%v to inconsistent vector dimension=%d, vector encoding=%v, vector similarity function=%v",
			fieldName, props.fieldVectorProperties.numDimensions, props.fieldVectorProperties.vectorEncoding, props.fieldVectorProperties.similarityFunction,
			fi.VectorDimension(), fi.VectorEncoding(), fi.VectorSimilarityFunction()))
	}
}

// VerifyOrCreateDvOnlyField ensures that a field is DocValues-only, or creates it if it doesn't exist.
func (fn *FieldNumbers) VerifyOrCreateDvOnlyField(fieldName string, dvType DocValuesType, fieldMustExist bool) {
	fn.mu.Lock()
	defer fn.mu.Unlock()

	props, ok := fn.fieldProperties[fieldName]
	if !ok {
		if fieldMustExist {
			panic(fmt.Sprintf("Can't update [%v] doc values; the field [%s] doesn't exist.", dvType, fieldName))
		}

		// create dv only field
		fi := NewFieldInfo(fieldName, -1, FieldInfoOptions{
			IndexOptions:           IndexOptionsNone,
			DocValuesType:           dvType,
			DocValuesSkipIndexType: DocValuesSkipIndexTypeNone,
			VectorEncoding:         VectorEncodingFloat32,
			VectorSimilarityFunction: VectorSimilarityFunctionEuclidean,
			IsSoftDeletesField:     fieldName == fn.softDeletesFieldName,
			IsParentField:           fieldName == fn.parentFieldName,
		})
		fn.AddOrGetLocked(fi)
	} else {
		if dvType != props.docValuesType {
			panic(fmt.Sprintf("Can't update [%v] doc values; the field [%s] has inconsistent doc values' type of [%v].", dvType, fieldName, props.docValuesType))
		}
		if props.docValuesSkipIndex != DocValuesSkipIndexTypeNone {
			panic(fmt.Sprintf("Can't update [%v] doc values; the field [%s] must be doc values only field, bit it has doc values skip index", dvType, fieldName))
		}
		if props.fieldDimensions.dimensionCount != 0 {
			panic(fmt.Sprintf("Can't update [%v] doc values; the field [%s] must be doc values only field, but is also indexed with points.", dvType, fieldName))
		}
		if props.indexOptions != IndexOptionsNone {
			panic(fmt.Sprintf("Can't update [%v] doc values; the field [%s] must be doc values only field, but is also indexed with postings.", dvType, fieldName))
		}
		if props.fieldVectorProperties.numDimensions != 0 {
			panic(fmt.Sprintf("Can't update [%v] doc values; the field [%s] must be doc values only field, but is also indexed with vectors.", dvType, fieldName))
		}
	}
}

// ConstructFieldInfo creates a FieldInfo based on the global properties.
func (fn *FieldNumbers) ConstructFieldInfo(fieldName string, dvType DocValuesType, newFieldNumber int) *FieldInfo {
	fn.mu.Lock()
	props, ok := fn.fieldProperties[fieldName]
	fn.mu.Unlock()

	if !ok {
		return nil
	}
	if dvType != props.docValuesType {
		return nil
	}

	return NewFieldInfo(fieldName, newFieldNumber, FieldInfoOptions{
		IndexOptions:           IndexOptionsNone,
		DocValuesType:           dvType,
		DocValuesSkipIndexType: DocValuesSkipIndexTypeNone,
		VectorEncoding:         VectorEncodingFloat32,
		VectorSimilarityFunction: VectorSimilarityFunctionEuclidean,
		IsSoftDeletesField:     fieldName == fn.softDeletesFieldName,
		IsParentField:           fieldName == fn.parentFieldName,
	})
}

// GetFieldNames returns a slice of all field names.
func (fn *FieldNumbers) GetFieldNames() []string {
	fn.mu.Lock()
	defer fn.mu.Unlock()

	names := make([]string, 0, len(fn.fieldProperties))
	for name := range fn.fieldProperties {
		names = append(names, name)
	}
	return names
}

// Clear resets the global field numbers.
func (fn *FieldNumbers) Clear() {
	fn.mu.Lock()
	defer fn.mu.Unlock()

	fn.numberToName = make(map[int]string)
	fn.fieldProperties = make(map[string]*fieldProperties)
	fn.lowestUnassignedFieldNumber = -1
}

// AddOrGetLocked is a non-locking version of AddOrGet used internally.
func (fn *FieldNumbers) AddOrGetLocked(fi *FieldInfo) int {
	fieldName := fi.Name()
	var fieldNumber int
	if fi.Number() != -1 {
		if _, taken := fn.numberToName[fi.Number()]; !taken {
			fieldNumber = fi.Number()
		} else {
			for {
				fn.lowestUnassignedFieldNumber++
				if _, taken := fn.numberToName[fn.lowestUnassignedFieldNumber]; !taken {
					fieldNumber = fn.lowestUnassignedFieldNumber
					break
				}
			}
		}
	} else {
		for {
			fn.lowestUnassignedFieldNumber++
			if _, taken := fn.numberToName[fn.lowestUnassignedFieldNumber]; !taken {
				fieldNumber = fn.lowestUnassignedFieldNumber
				break
			}
		}
	}

	fn.numberToName[fieldNumber] = fieldName
	props := &fieldProperties{
		number:       fieldNumber,
		indexOptions: fi.IndexOptions(),
		docValuesType: fi.DocValuesType(),
		docValuesSkipIndex: fi.DocValuesSkipIndexType(),
		fieldDimensions: fieldDimensions{
			dimensionCount:      fi.PointDimensionCount(),
			indexDimensionCount: fi.PointIndexDimensionCount(),
			dimensionNumBytes:   fi.PointNumBytes(),
		},
		fieldVectorProperties: fieldVectorProperties{
			numDimensions:      fi.VectorDimension(),
			vectorEncoding:      fi.VectorEncoding(),
			similarityFunction: fi.VectorSimilarityFunction(),
		},
	}

	if fi.IndexOptions() != IndexOptionsNone {
		props.indexOptionsProperties = &indexOptionsProperties{
			storeTermVectors: fi.HasTermVectors(),
			omitNorms:        fi.OmitNorms(),
		}
	}

	fn.fieldProperties[fieldName] = props
	return props.number
}
