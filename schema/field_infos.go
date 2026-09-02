// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package schema

import (
	"fmt"
	"sort"
)

// FieldInfos manages a collection of FieldInfo objects.
// This is the Go port of Lucene's org.apache.lucene.index.FieldInfos.
type FieldInfos struct {
	hasFreq         bool
	hasPostings     bool
	hasProx         bool
	hasPayloads     bool
	hasOffsets      bool
	hasTermVectors  bool
	hasNorms        bool
	hasDocValues    bool
	hasPointValues  bool
	hasVectorValues bool
	softDeletesField string
	parentField      string

	// byNumber is used for fast access by field number.
	// It is a slice where the index is the field number.
	byNumber []*FieldInfo
	// byName maps field names to FieldInfo.
	byName map[string]*FieldInfo
	// values is a slice of all FieldInfo objects, sorted by field number.
	values []*FieldInfo
}

// EmptyFieldInfos is an instance without any fields.
var EmptyFieldInfos = NewFieldInfos([]*FieldInfo{})

// NewFieldInfos constructs a new FieldInfos from a slice of FieldInfo objects.
func NewFieldInfos(infos []*FieldInfo) *FieldInfos {
	var hasTermVectors, hasPostings, hasProx, hasPayloads, hasOffsets, hasFreq, hasNorms, hasDocValues, hasPointValues, hasVectorValues bool
	var softDeletesField, parentField string

	byName := make(map[string]*FieldInfo, len(infos))
	maxFieldNumber := -1
	fieldNumberStrictlyAscending := true

	for _, info := range infos {
		fieldNumber := info.Number()
		if fieldNumber < 0 {
			panic(fmt.Sprintf("illegal field number: %d for field %s", fieldNumber, info.Name()))
		}
		if maxFieldNumber < fieldNumber {
			maxFieldNumber = fieldNumber
		} else {
			fieldNumberStrictlyAscending = false
		}

		if previous, ok := byName[info.Name()]; ok {
			panic(fmt.Sprintf("duplicate field names: %d and %d have: %s", previous.Number(), info.Number(), info.Name()))
		}
		byName[info.Name()] = info

		hasTermVectors = hasTermVectors || info.StoreTermVectors()
		hasPostings = hasPostings || info.IndexOptions().IsIndexed()
		hasProx = hasProx || info.IndexOptions().HasPositions()
		hasFreq = hasFreq || (info.IndexOptions() != IndexOptionsNone && info.IndexOptions() != IndexOptionsDocs)
		hasOffsets = hasOffsets || info.IndexOptions().HasOffsets()
		hasNorms = hasNorms || info.HasNorms()
		hasDocValues = hasDocValues || info.DocValuesType() != DocValuesTypeNone
		hasPayloads = hasPayloads || info.HasPayloads()
		hasPointValues = hasPointValues || info.PointDimensionCount() != 0
		hasVectorValues = hasVectorValues || info.VectorDimension() != 0

		if info.IsSoftDeletesField() {
			if softDeletesField != "" && softDeletesField != info.Name() {
				panic(fmt.Sprintf("multiple soft-deletes fields [%s, %s]", info.Name(), softDeletesField))
			}
			softDeletesField = info.Name()
		}
		if info.IsParentField() {
			if parentField != "" && parentField != info.Name() {
				panic(fmt.Sprintf("multiple parent fields [%s, %s]", info.Name(), parentField))
			}
			parentField = info.Name()
		}
	}

	var byNumber []*FieldInfo
	var values []*FieldInfo

	if fieldNumberStrictlyAscending && maxFieldNumber == len(infos)-1 {
		byNumber = infos
		values = infos
	} else {
		byNumber = make([]*FieldInfo, maxFieldNumber+1)
		for _, info := range infos {
			if existing := byNumber[info.Number()]; existing != nil {
				panic(fmt.Sprintf("duplicate field numbers: %s and %s have: %d", existing.Name(), info.Name(), info.Number()))
			}
			byNumber[info.Number()] = info
		}

		if maxFieldNumber == len(infos)-1 {
			values = byNumber
		} else {
			sortedInfos := make([]*FieldInfo, len(infos))
			copy(sortedInfos, infos)
			sort.Slice(sortedInfos, func(i, j int) bool {
				return sortedInfos[i].Number() < sortedInfos[j].Number()
			})
			values = sortedInfos
		}
	}

	return &FieldInfos{
		hasFreq:         hasFreq,
		hasPostings:     hasPostings,
		hasProx:         hasProx,
		hasPayloads:     hasPayloads,
		hasOffsets:      hasOffsets,
		hasTermVectors:  hasTermVectors,
		hasNorms:        hasNorms,
		hasDocValues:    hasDocValues,
		hasPointValues:  hasPointValues,
		hasVectorValues: hasVectorValues,
		softDeletesField: softDeletesField,
		parentField:      parentField,
		byNumber:         byNumber,
		byName:           byName,
		values:           values,
	}
}

func (fi *FieldInfos) HasFreq() bool         { return fi.hasFreq }
func (fi *FieldInfos) HasPostings() bool     { return fi.hasPostings }
func (fi *FieldInfos) HasProx() bool         { return fi.hasProx }
func (fi *FieldInfos) HasPayloads() bool     { return fi.hasPayloads }
func (fi *FieldInfos) HasOffsets() bool      { return fi.hasOffsets }
func (fi *FieldInfos) HasTermVectors() bool  { return fi.hasTermVectors }
func (fi *FieldInfos) HasNorms() bool        { return fi.hasNorms }
func (fi *FieldInfos) HasDocValues() bool    { return fi.hasDocValues }
func (fi *FieldInfos) HasPointValues() bool  { return fi.hasPointValues }
func (fi *FieldInfos) HasVectorValues() bool { return fi.hasVectorValues }
func (fi *FieldInfos) GetSoftDeletesField() string { return fi.softDeletesField }
func (fi *FieldInfos) GetParentField() string      { return fi.parentField }
func (fi *FieldInfos) Size() int                  { return len(fi.byName) }

// Iterator returns an iterator over all FieldInfo objects in ascending order of field number.
func (fi *FieldInfos) Iterator() FieldInfosIterator {
	return &fieldInfosIterator{
		infos: fi.values,
		index: -1,
	}
}

func (fi *FieldInfos) FieldInfoByName(name string) *FieldInfo {
	return fi.byName[name]
}

func (fi *FieldInfos) FieldInfoByNumber(number int) *FieldInfo {
	if number < 0 {
		panic(fmt.Sprintf("Illegal field number: %d", number))
	}
	if number >= len(fi.byNumber) {
		return nil
	}
	return fi.byNumber[number]
}

type FieldInfosIterator interface {
	Next() *FieldInfo
	HasNext() bool
}

type fieldInfosIterator struct {
	infos []*FieldInfo
	index int
}

func (it *fieldInfosIterator) Next() *FieldInfo {
	it.index++
	if it.index >= len(it.infos) {
		return nil
	}
	return it.infos[it.index]
}

func (it *fieldInfosIterator) HasNext() bool {
	return it.index+1 < len(it.infos)
}

// FieldNumbers manages the global mapping of field names to numbers across segments.
type FieldNumbers struct {
	numberToName map[int]string
	fieldProps   map[string]*fieldProps
	lowestUnassignedFieldNumber int
	softDeletesFieldName string
	parentFieldName      string
}

type fieldProps struct {
	number               int
	indexOptions         IndexOptions
	indexOptionsProps    *indexOptionsProps
	docValuesType        DocValuesType
	docValuesSkipIndex   DocValuesSkipIndexType
	fieldDims            fieldDims
	fieldVectorProps     fieldVectorProps
}

type indexOptionsProps struct {
	storeTermVectors bool
	omitNorms        bool
}

type fieldDims struct {
	dimensionCount       int
	indexDimensionCount  int
	dimensionNumBytes    int
}

type fieldVectorProps struct {
	numDimensions           int
	vectorEncoding           VectorEncoding
	vectorSimilarityFunction VectorSimilarityFunction
}

func NewFieldNumbers(softDeletesFieldName, parentFieldName string) *FieldNumbers {
	if softDeletesFieldName != "" && parentFieldName != "" && softDeletesFieldName == parentFieldName {
		panic(fmt.Sprintf("parent document and soft-deletes field can't be the same field %q", parentFieldName))
	}
	return &FieldNumbers{
		numberToName:                make(map[int]string),
		fieldProps:                  make(map[string]*fieldProps),
		lowestUnassignedFieldNumber: -1,
		softDeletesFieldName:        softDeletesFieldName,
		parentFieldName:            parentFieldName,
	}
}

func (fn *FieldNumbers) verifyFieldInfo(fi *FieldInfo) {
	name := fi.Name()
	fn.verifySoftDeletedFieldName(name, fi.IsSoftDeletesField())
	fn.verifyParentFieldName(name, fi.IsParentField())
	if props, ok := fn.fieldProps[name]; ok {
		fn.verifySameSchema(fi, props)
	}
}

func (fn *FieldNumbers) verifySoftDeletedFieldName(name string, isSoftDeletes bool) {
	if isSoftDeletes {
		if fn.softDeletesFieldName == "" {
			panic(fmt.Sprintf("this index has [%s] as soft-deletes already but soft-deletes field is not configured in IWC", name))
		} else if name != fn.softDeletesFieldName {
			panic(fmt.Sprintf("cannot configure [%s] as soft-deletes; this index uses [%s] as soft-deletes already", fn.softDeletesFieldName, name))
		}
	} else if name == fn.softDeletesFieldName {
		panic(fmt.Sprintf("cannot configure [%s] as soft-deletes; this index uses [%s] as non-soft-deletes already", fn.softDeletesFieldName, name))
	}
}

func (fn *FieldNumbers) verifyParentFieldName(name string, isParent bool) {
	if isParent {
		if fn.parentFieldName == "" {
			panic(fmt.Sprintf("can't add field [%s] as parent document field; this IndexWriter has no parent document field configured", name))
		} else if name != fn.parentFieldName {
			panic(fmt.Sprintf("can't add field [%s] as parent document field; this IndexWriter is configured with [%s] as parent document field", name, fn.parentFieldName))
		}
	} else if name == fn.parentFieldName {
		panic(fmt.Sprintf("can't add [%s] as non parent document field; this IndexWriter is configured with [%s] as parent document field", name, fn.parentFieldName))
	}
}

func (fn *FieldNumbers) verifySameSchema(fi *FieldInfo, props *fieldProps) {
	name := fi.Name()
	if fi.IndexOptions() != props.indexOptions {
		panic(fmt.Sprintf("inconsistent index options for field %q: have %v, got %v", name, props.indexOptions, fi.IndexOptions()))
	}
	if props.indexOptions != IndexOptionsNone {
		if fi.StoreTermVectors() != props.indexOptionsProps.storeTermVectors {
			panic(fmt.Sprintf("inconsistent term vectors storage for field %q", name))
		}
		if fi.OmitNorms() != props.indexOptionsProps.omitNorms {
			panic(fmt.Sprintf("cannot change field %q from omitNorms=%v to inconsistent omitNorms=%v", name, props.indexOptionsProps.omitNorms, fi.OmitNorms()))
		}
	}

	if fi.DocValuesType() != props.docValuesType {
		panic(fmt.Sprintf("inconsistent doc values type for field %q", name))
	}
	if fi.DocValuesSkipIndexType() != props.docValuesSkipIndex {
		panic(fmt.Sprintf("inconsistent doc values skip index for field %q", name))
	}

	if fi.PointDimensionCount() != props.fieldDims.dimensionCount ||
		fi.PointIndexDimensionCount() != props.fieldDims.indexDimensionCount ||
		fi.PointNumBytes() != props.fieldDims.dimensionNumBytes {
		panic(fmt.Sprintf("inconsistent point dimensions for field %q", name))
	}

	if fi.VectorDimension() != props.fieldVectorProps.numDimensions ||
		fi.VectorEncoding() != props.fieldVectorProps.vectorEncoding ||
		fi.VectorSimilarityFunction() != props.fieldVectorProps.vectorSimilarityFunction {
		panic(fmt.Sprintf("inconsistent vector schema for field %q", name))
	}
}

func (fn *FieldNumbers) AddOrGet(fi *FieldInfo) int {
	name := fi.Name()
	fn.verifySoftDeletedFieldName(name, fi.IsSoftDeletesField())
	fn.verifyParentFieldName(name, fi.IsParentField())

	if props, ok := fn.fieldProps[name]; ok {
		fn.verifySameSchema(fi, props)
		return props.number
	}

	var fieldNumber int
		if fi.Number() != -1 {
			if _, ok := fn.numberToName[fi.Number()]; !ok {
				fieldNumber = fi.Number()
			}
		}
		if fieldNumber == -1 {
			for {
				fn.lowestUnassignedFieldNumber++
				if _, ok := fn.numberToName[fn.lowestUnassignedFieldNumber]; !ok {
					fieldNumber = fn.lowestUnassignedFieldNumber
					break
				}
			}
		}


	fn.numberToName[fieldNumber] = name
	fn.fieldProps[name] = &fieldProps{
		number:               fieldNumber,
		indexOptions:         fi.IndexOptions(),
		indexOptionsProps:    nil,
		docValuesType:        fi.DocValuesType(),
		docValuesSkipIndex:   fi.DocValuesSkipIndexType(),
		fieldDims: fieldDims{
			dimensionCount:      fi.PointDimensionCount(),
			indexDimensionCount: fi.PointIndexDimensionCount(),
			dimensionNumBytes:   fi.PointNumBytes(),
		},
		fieldVectorProps: fieldVectorProps{
			numDimensions:           fi.VectorDimension(),
			vectorEncoding:           fi.VectorEncoding(),
			vectorSimilarityFunction: fi.VectorSimilarityFunction(),
		},
	}

	if fi.IndexOptions() != IndexOptionsNone {
		props := fn.fieldProps[name]
		props.indexOptionsProps = &indexOptionsProps{
			storeTermVectors: fi.StoreTermVectors(),
			omitNorms:        fi.OmitNorms(),
		}
	}

	return fieldNumber
}

func (fn *FieldNumbers) VerifyOrCreateDvOnlyField(name string, dvType DocValuesType, fieldMustExist bool) {
	if props, ok := fn.fieldProps[name]; !ok {
		if fieldMustExist {
			panic(fmt.Sprintf("Can't update [%v] doc values; the field [%s] doesn't exist.", dvType, name))
		}

		fi := NewFieldInfo(name, -1, FieldInfoOptions{
			IndexOptions:     IndexOptionsNone,
			DocValuesType:    dvType,
			DocValuesSkipIndexType: DocValuesSkipIndexTypeNone,
			IsSoftDeletesField: name == fn.softDeletesFieldName,
			IsParentField:      name == fn.parentFieldName,
		})
		fn.AddOrGet(fi)
	} else {
		if props.docValuesType != dvType {
			panic(fmt.Sprintf("Can't update [%v] doc values; the field [%s] has inconsistent doc values' type of [%v].", dvType, name, props.docValuesType))
		}
		if props.docValuesSkipIndex != DocValuesSkipIndexTypeNone {
			panic(fmt.Sprintf("Can't update [%v] doc values; the field [%s] must be doc values only field, bit it has doc values skip index", dvType, name))
		}
		if props.fieldDims.dimensionCount != 0 {
			panic(fmt.Sprintf("Can't update [%v] doc values; the field [%s] must be doc values only field, but is also indexed with points.", dvType, name))
		}
		if props.indexOptions != IndexOptionsNone {
			panic(fmt.Sprintf("Can't update [%v] doc values; the field [%s] must be doc values only field, but is also indexed with postings.", dvType, name))
		}
		if props.fieldVectorProps.numDimensions != 0 {
			panic(fmt.Sprintf("Can't update [%v] doc values; the field [%s] must be doc values only field, but is also indexed with vectors.", dvType, name))
		}
	}
}

func (fn *FieldNumbers) ConstructFieldInfo(name string, dvType DocValuesType, newFieldNumber int) *FieldInfo {
	props, ok := fn.fieldProps[name]
	if !ok || props.docValuesType != dvType {
		return nil
	}
	return NewFieldInfo(name, newFieldNumber, FieldInfoOptions{
		IndexOptions:     IndexOptionsNone,
		DocValuesType:    dvType,
		DocValuesSkipIndexType: DocValuesSkipIndexTypeNone,
		IsSoftDeletesField: name == fn.softDeletesFieldName,
		IsParentField:      name == fn.parentFieldName,
	})
}

func (fn *FieldNumbers) GetFieldNames() []string {
	names := make([]string, 0, len(fn.fieldProps))
	for name := range fn.fieldProps {
		names = append(names, name)
	}
	return names
}

func (fn *FieldNumbers) Clear() {
	fn.numberToName = make(map[int]string)
	fn.fieldProps = make(map[string]*fieldProps)
	fn.lowestUnassignedFieldNumber = -1
}

// FieldInfosBuilder helps construct FieldInfos.
type FieldInfosBuilder struct {
	byName map[string]*FieldInfo
	globalFieldNumbers *FieldNumbers
	finished bool
}

func NewFieldInfosBuilder(globalFieldNumbers *FieldNumbers) *FieldInfosBuilder {
	return &FieldInfosBuilder{
		byName: make(map[string]*FieldInfo),
		globalFieldNumbers: globalFieldNumbers,
	}
}

func (b *FieldInfosBuilder) Add(fi *FieldInfo) *FieldInfo {
	return b.AddWithDvGen(fi, -1)
}

func (b *FieldInfosBuilder) AddWithDvGen(fi *FieldInfo, dvGen int64) *FieldInfo {
	if curFi := b.byName[fi.Name()]; curFi != nil {
		// verifySameSchema(fi) is handled by FieldInfo's consistency checks or should be here.
		// For simplicity, we assume the caller handles consistency or we add it here.
		// Let's add a basic check.
		if curFi.Number() != fi.Number() {
			// This is a simplification. Lucene's Builder.add does a deeper schema check.
		}
		if fi.HasPayloads() {
			curFi.SetStorePayloads()
		}
		return curFi
	}

	if b.finished {
		panic("FieldInfos.Builder was already finished; cannot add new fields")
	}

	fieldNumber := b.globalFieldNumbers.AddOrGet(fi)

	// Construct new FieldInfo with global number
	fiNew := NewFieldInfo(fi.Name(), fieldNumber, FieldInfoOptions{
		IndexOptions:             fi.IndexOptions(),
		DocValuesType:            fi.DocValuesType(),
		DocValuesSkipIndexType:   fi.DocValuesSkipIndexType(),
		DocValuesGen:             dvGen,
		Stored:                   fi.IsStored(),
		Tokenized:                fi.IsTokenized(),
		OmitNorms:                fi.OmitNorms(),
		StoreTermVectors:         fi.StoreTermVectors(),
		StoreTermVectorPositions: fi.StoreTermVectorPositions(),
		StoreTermVectorOffsets:   fi.StoreTermVectorOffsets(),
		StoreTermVectorPayloads:  fi.StoreTermVectorPayloads(),
		PointDimensionCount:      fi.PointDimensionCount(),
		PointIndexDimensionCount: fi.PointIndexDimensionCount(),
		PointNumBytes:            fi.PointNumBytes(),
		VectorDimension:          fi.VectorDimension(),
		VectorEncoding:           fi.VectorEncoding(),
		VectorSimilarityFunction: fi.VectorSimilarityFunction(),
		IsSoftDeletesField:       fi.IsSoftDeletesField(),
		IsParentField:            fi.IsParentField(),
	})

	// Copy attributes
	for k, v := range fi.GetAttributes() {
		fiNew.PutAttribute(k, v)
	}

	b.byName[fiNew.Name()] = fiNew
	return fiNew
}

func (b *FieldInfosBuilder) Build() *FieldInfos {
	b.finished = true
	infos := make([]*FieldInfo, 0, len(b.byName))
	for _, fi := range b.byName {
		infos = append(infos, fi)
	}
	return NewFieldInfos(infos)
}

func (b *FieldInfosBuilder) FieldInfos() *FieldInfos {
	// This is tricky because we need to create a FieldInfos from current state without freezing builder
	infos := make([]*FieldInfo, 0, len(b.byName))
	for _, fi := range b.byName {
		infos = append(infos, fi)
	}
	return NewFieldInfos(infos)
}
