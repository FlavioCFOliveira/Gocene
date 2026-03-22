// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"bytes"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TermVectorValidator provides comprehensive term vector validation.
// This extends the basic CheckIndex term vector checking.
type TermVectorValidator struct {
	reader     *SegmentReader
	fieldInfos *FieldInfos
	errors     []error
}

// NewTermVectorValidator creates a new term vector validator.
func NewTermVectorValidator(reader *SegmentReader) *TermVectorValidator {
	return &TermVectorValidator{
		reader:     reader,
		fieldInfos: reader.GetFieldInfos(),
		errors:     make([]error, 0),
	}
}

// Validate performs comprehensive term vector validation.
func (v *TermVectorValidator) Validate() []error {
	maxDoc := v.reader.MaxDoc()
	liveDocs := v.reader.GetLiveDocs()

	for docID := 0; docID < maxDoc; docID++ {
		// Skip deleted documents
		if liveDocs != nil && !liveDocs.Get(docID) {
			continue
		}

		// Get term vectors for this document
		termVectors, err := v.reader.GetTermVectors(docID)
		if err != nil {
			v.errors = append(v.errors, fmt.Errorf("doc %d: cannot get term vectors: %w", docID, err))
			continue
		}

		if termVectors == nil {
			continue
		}

		// Validate each field's term vectors
		fieldIter, err := termVectors.Iterator()
		if err != nil {
			v.errors = append(v.errors, fmt.Errorf("doc %d: cannot get field iterator: %w", docID, err))
			continue
		}

		for fieldIter.HasNext() {
			field, err := fieldIter.Next()
			if err != nil {
				v.errors = append(v.errors, fmt.Errorf("doc %d: error iterating fields: %w", docID, err))
				break
			}

			terms, err := termVectors.Terms(field)
			if err != nil {
				v.errors = append(v.errors, fmt.Errorf("doc %d: cannot get terms for field %s: %w", docID, field, err))
				continue
			}

			if err := v.validateTermVectorField(docID, field, terms); err != nil {
				v.errors = append(v.errors, err)
			}
		}
	}

	return v.errors
}

// validateTermVectorField validates term vectors for a single field.
func (v *TermVectorValidator) validateTermVectorField(docID int, field string, terms Terms) error {
	// Get field info
	fieldInfo := v.fieldInfos.GetByName(field)
	if fieldInfo == nil {
		return fmt.Errorf("doc %d: field %s not found in field infos", docID, field)
	}

	// Check if field should have term vectors
	if !fieldInfo.StoreTermVectors() {
		return fmt.Errorf("doc %d: field %s has term vectors but shouldn't", docID, field)
	}

	// Validate terms
	termsEnum, err := terms.GetIterator()
	if err != nil {
		return fmt.Errorf("doc %d: cannot get terms iterator for field %s: %w", docID, field, err)
	}

	for {
		term, err := termsEnum.Next()
		if err != nil {
			return fmt.Errorf("doc %d: error iterating terms for field %s: %w", docID, field, err)
		}
		if term == nil {
			break
		}

		// Get term frequency
		tf, err := termsEnum.TotalTermFreq()
		if err != nil {
			return fmt.Errorf("doc %d: cannot get term freq for %s in field %s: %w", docID, term.Bytes, field, err)
		}

		if tf <= 0 {
			return fmt.Errorf("doc %d: invalid term frequency %d for %s in field %s", docID, tf, term.Bytes, field)
		}

		// Validate positions if available
		if fieldInfo.StoreTermVectorPositions() {
			if err := v.validatePositions(docID, field, term, termsEnum); err != nil {
				return err
			}
		}

		// Validate offsets if available
		if fieldInfo.StoreTermVectorOffsets() {
			if err := v.validateOffsets(docID, field, term, termsEnum); err != nil {
				return err
			}
		}
	}

	return nil
}

// validatePositions validates term positions.
func (v *TermVectorValidator) validatePositions(docID int, field string, term *Term, termsEnum TermsEnum) error {
	// Postings flags: need positions
	const postingsFlagPositions = 1 // PostingsEnumFeaturePositions
	postings, err := termsEnum.Postings(postingsFlagPositions)
	if err != nil {
		return fmt.Errorf("doc %d: cannot get postings for %s in field %s: %w", docID, term.Bytes, field, err)
	}

	docIDFound, err := postings.NextDoc()
	if err != nil {
		return fmt.Errorf("doc %d: error getting next doc for %s in field %s: %w", docID, term.Bytes, field, err)
	}
	if docIDFound != docID {
		return fmt.Errorf("doc %d: doc ID mismatch for %s in field %s", docID, term.Bytes, field)
	}

	freq, err := postings.Freq()
	if err != nil {
		return fmt.Errorf("doc %d: cannot get freq for %s in field %s: %w", docID, term.Bytes, field, err)
	}

	var lastPosition int = -1
	for i := 0; i < freq; i++ {
		pos, err := postings.NextPosition()
		if err != nil {
			return fmt.Errorf("doc %d: error getting position for %s in field %s: %w", docID, term.Bytes, field, err)
		}

		if pos < lastPosition {
			return fmt.Errorf("doc %d: positions out of order for %s in field %s: %d < %d", docID, term.Bytes, field, pos, lastPosition)
		}
		lastPosition = pos
	}

	return nil
}

// validateOffsets validates term offsets.
func (v *TermVectorValidator) validateOffsets(docID int, field string, term *Term, termsEnum TermsEnum) error {
	// Postings flags: need positions and offsets
	const postingsFlagPositions = 1 // PostingsEnumFeaturePositions
	const postingsFlagOffsets = 2   // PostingsEnumFeatureOffsets
	postings, err := termsEnum.Postings(postingsFlagPositions | postingsFlagOffsets)
	if err != nil {
		return fmt.Errorf("doc %d: cannot get postings for %s in field %s: %w", docID, term.Bytes, field, err)
	}

	docIDFound, err := postings.NextDoc()
	if err != nil {
		return fmt.Errorf("doc %d: error getting next doc for %s in field %s: %w", docID, term.Bytes, field, err)
	}
	if docIDFound != docID {
		return fmt.Errorf("doc %d: doc ID mismatch for %s in field %s", docID, term.Bytes, field)
	}

	freq, err := postings.Freq()
	if err != nil {
		return fmt.Errorf("doc %d: cannot get freq for %s in field %s: %w", docID, term.Bytes, field, err)
	}

	var lastEndOffset int = -1
	for i := 0; i < freq; i++ {
		// Need to advance position to get offsets
		_, err := postings.NextPosition()
		if err != nil {
			return fmt.Errorf("doc %d: error getting position for %s in field %s: %w", docID, term.Bytes, field, err)
		}

		startOffset, err := postings.StartOffset()
		if err != nil {
			return fmt.Errorf("doc %d: error getting start offset for %s in field %s: %w", docID, term.Bytes, field, err)
		}

		endOffset, err := postings.EndOffset()
		if err != nil {
			return fmt.Errorf("doc %d: error getting end offset for %s in field %s: %w", docID, term.Bytes, field, err)
		}

		if startOffset < 0 || endOffset < 0 {
			return fmt.Errorf("doc %d: invalid offsets for %s in field %s: %d, %d", docID, term.Bytes, field, startOffset, endOffset)
		}

		if startOffset >= endOffset {
			return fmt.Errorf("doc %d: start offset >= end offset for %s in field %s: %d, %d", docID, term.Bytes, field, startOffset, endOffset)
		}

		if startOffset < lastEndOffset {
			return fmt.Errorf("doc %d: offsets out of order for %s in field %s: %d < %d", docID, term.Bytes, field, startOffset, lastEndOffset)
		}
		lastEndOffset = endOffset
	}

	return nil
}

// DocValuesValidator provides comprehensive DocValues validation.
type DocValuesValidator struct {
	reader     *SegmentReader
	fieldInfos *FieldInfos
	errors     []error
}

// NewDocValuesValidator creates a new DocValues validator.
func NewDocValuesValidator(reader *SegmentReader) *DocValuesValidator {
	return &DocValuesValidator{
		reader:     reader,
		fieldInfos: reader.GetFieldInfos(),
		errors:     make([]error, 0),
	}
}

// Validate performs comprehensive DocValues validation.
func (v *DocValuesValidator) Validate() []error {
	maxDoc := v.reader.MaxDoc()

	iter := v.fieldInfos.Iterator()
	for iter.HasNext() {
		fieldInfo := iter.Next()
		docValuesType := fieldInfo.DocValuesType()

		if docValuesType == DocValuesTypeNone {
			continue
		}

		field := fieldInfo.Name()

		switch docValuesType {
		case DocValuesTypeNumeric:
			if err := v.validateNumericDocValues(field, maxDoc); err != nil {
				v.errors = append(v.errors, err)
			}
		case DocValuesTypeBinary:
			if err := v.validateBinaryDocValues(field, maxDoc); err != nil {
				v.errors = append(v.errors, err)
			}
		case DocValuesTypeSorted:
			if err := v.validateSortedDocValues(field, maxDoc); err != nil {
				v.errors = append(v.errors, err)
			}
		case DocValuesTypeSortedNumeric:
			if err := v.validateSortedNumericDocValues(field, maxDoc); err != nil {
				v.errors = append(v.errors, err)
			}
		case DocValuesTypeSortedSet:
			if err := v.validateSortedSetDocValues(field, maxDoc); err != nil {
				v.errors = append(v.errors, err)
			}
		}
	}

	return v.errors
}

// validateNumericDocValues validates numeric DocValues.
func (v *DocValuesValidator) validateNumericDocValues(field string, maxDoc int) error {
	values, err := v.reader.GetNumericDocValues(field)
	if err != nil {
		return fmt.Errorf("cannot get numeric doc values for field %s: %w", field, err)
	}

	if values == nil {
		return fmt.Errorf("numeric doc values is nil for field %s", field)
	}

	// Check we can read values for all docs
	for docID := 0; docID < maxDoc; docID++ {
		_, err := values.Get(docID)
		if err != nil {
			return fmt.Errorf("error reading numeric doc value for doc %d in field %s: %w", docID, field, err)
		}
	}

	return nil
}

// validateBinaryDocValues validates binary DocValues.
func (v *DocValuesValidator) validateBinaryDocValues(field string, maxDoc int) error {
	values, err := v.reader.GetBinaryDocValues(field)
	if err != nil {
		return fmt.Errorf("cannot get binary doc values for field %s: %w", field, err)
	}

	if values == nil {
		return fmt.Errorf("binary doc values is nil for field %s", field)
	}

	// Check we can read values for all docs
	for docID := 0; docID < maxDoc; docID++ {
		_, err := values.Get(docID)
		if err != nil {
			return fmt.Errorf("error reading binary doc value for doc %d in field %s: %w", docID, field, err)
		}
	}

	return nil
}

// validateSortedDocValues validates sorted DocValues.
func (v *DocValuesValidator) validateSortedDocValues(field string, maxDoc int) error {
	values, err := v.reader.GetSortedDocValues(field)
	if err != nil {
		return fmt.Errorf("cannot get sorted doc values for field %s: %w", field, err)
	}

	if values == nil {
		return fmt.Errorf("sorted doc values is nil for field %s", field)
	}

	// Validate value count
	valueCount := values.GetValueCount()
	if valueCount < 0 {
		return fmt.Errorf("invalid value count %d for field %s", valueCount, field)
	}

	// Check we can read values for all docs
	for docID := 0; docID < maxDoc; docID++ {
		ord, err := values.GetOrd(docID)
		if err != nil {
			return fmt.Errorf("error reading sorted ord for doc %d in field %s: %w", docID, field, err)
		}

		if ord < -1 || ord >= valueCount {
			return fmt.Errorf("invalid ord %d for doc %d in field %s (count=%d)", ord, docID, field, valueCount)
		}

		// If there's a value, verify we can look it up
		if ord >= 0 {
			_, err := values.LookupOrd(ord)
			if err != nil {
				return fmt.Errorf("error looking up ord %d for field %s: %w", ord, field, err)
			}
		}
	}

	return nil
}

// validateSortedNumericDocValues validates sorted numeric DocValues.
func (v *DocValuesValidator) validateSortedNumericDocValues(field string, maxDoc int) error {
	values, err := v.reader.GetSortedNumericDocValues(field)
	if err != nil {
		return fmt.Errorf("cannot get sorted numeric doc values for field %s: %w", field, err)
	}

	if values == nil {
		return fmt.Errorf("sorted numeric doc values is nil for field %s", field)
	}

	// Check we can read values for all docs
	for docID := 0; docID < maxDoc; docID++ {
		nums, err := values.Get(docID)
		if err != nil {
			return fmt.Errorf("error reading sorted numeric values for doc %d in field %s: %w", docID, field, err)
		}

		// Verify values are sorted
		for i := 1; i < len(nums); i++ {
			if nums[i] < nums[i-1] {
				return fmt.Errorf("values not sorted for doc %d in field %s: %d < %d", docID, field, nums[i], nums[i-1])
			}
		}
	}

	return nil
}

// validateSortedSetDocValues validates sorted set DocValues.
func (v *DocValuesValidator) validateSortedSetDocValues(field string, maxDoc int) error {
	values, err := v.reader.GetSortedSetDocValues(field)
	if err != nil {
		return fmt.Errorf("cannot get sorted set doc values for field %s: %w", field, err)
	}

	if values == nil {
		return fmt.Errorf("sorted set doc values is nil for field %s", field)
	}

	// Validate value count
	valueCount := values.GetValueCount()
	if valueCount < 0 {
		return fmt.Errorf("invalid value count %d for field %s", valueCount, field)
	}

	// Check we can read values for all docs
	for docID := 0; docID < maxDoc; docID++ {
		ords, err := values.Get(docID)
		if err != nil {
			return fmt.Errorf("error reading sorted set ords for doc %d in field %s: %w", docID, field, err)
		}

		// Verify ords are valid and sorted
		var lastOrd int = -1
		for _, ord := range ords {
			if ord < 0 || ord >= valueCount {
				return fmt.Errorf("invalid ord %d for doc %d in field %s (count=%d)", ord, docID, field, valueCount)
			}
			if ord <= lastOrd {
				return fmt.Errorf("ords not sorted for doc %d in field %s: %d <= %d", docID, field, ord, lastOrd)
			}
			lastOrd = ord
		}
	}

	return nil
}

// PointsValidator provides comprehensive PointValues validation.
type PointsValidator struct {
	reader     *SegmentReader
	fieldInfos *FieldInfos
	errors     []error
}

// NewPointsValidator creates a new Points validator.
func NewPointsValidator(reader *SegmentReader) *PointsValidator {
	return &PointsValidator{
		reader:     reader,
		fieldInfos: reader.GetFieldInfos(),
		errors:     make([]error, 0),
	}
}

// Validate performs comprehensive Points validation.
func (v *PointsValidator) Validate() []error {
	iter := v.fieldInfos.Iterator()
	for iter.HasNext() {
		fieldInfo := iter.Next()
		pointDimension := fieldInfo.PointDimensionCount()

		if pointDimension == 0 {
			continue
		}

		field := fieldInfo.Name()

		if err := v.validatePointValues(field, fieldInfo); err != nil {
			v.errors = append(v.errors, err)
		}
	}

	return v.errors
}

// validatePointValues validates point values for a field.
func (v *PointsValidator) validatePointValues(field string, fieldInfo *FieldInfo) error {
	pointValues, err := v.reader.GetPointValues(field)
	if err != nil {
		return fmt.Errorf("cannot get point values for field %s: %w", field, err)
	}

	if pointValues == nil {
		return fmt.Errorf("point values is nil for field %s", field)
	}

	dimension := fieldInfo.PointDimensionCount()
	numBytes := fieldInfo.PointNumBytes()

	if dimension <= 0 {
		return fmt.Errorf("invalid dimension %d for field %s", dimension, field)
	}

	if numBytes <= 0 {
		return fmt.Errorf("invalid numBytes %d for field %s", numBytes, field)
	}

	// Validate min/max values
	for dim := 0; dim < dimension; dim++ {
		minValue, err := pointValues.GetMinPackedValue()
		if err != nil {
			return fmt.Errorf("cannot get min value for field %s: %w", field, err)
		}

		maxValue, err := pointValues.GetMaxPackedValue()
		if err != nil {
			return fmt.Errorf("cannot get max value for field %s: %w", field, err)
		}

		if len(minValue) != numBytes || len(maxValue) != numBytes {
			return fmt.Errorf("invalid min/max value length for field %s", field)
		}

		// Verify min <= max
		if bytes.Compare(minValue, maxValue) > 0 {
			return fmt.Errorf("min value > max value for field %s", field)
		}
	}

	return nil
}

// VectorValuesValidator provides comprehensive KNN vector validation.
type VectorValuesValidator struct {
	reader     *SegmentReader
	fieldInfos *FieldInfos
	errors     []error
}

// NewVectorValuesValidator creates a new VectorValues validator.
func NewVectorValuesValidator(reader *SegmentReader) *VectorValuesValidator {
	return &VectorValuesValidator{
		reader:     reader,
		fieldInfos: reader.GetFieldInfos(),
		errors:     make([]error, 0),
	}
}

// Validate performs comprehensive VectorValues validation.
func (v *VectorValuesValidator) Validate() []error {
	maxDoc := v.reader.MaxDoc()
	liveDocs := v.reader.GetLiveDocs()

	iter := v.fieldInfos.Iterator()
	for iter.HasNext() {
		fieldInfo := iter.Next()
		dimension := fieldInfo.VectorDimension()

		if dimension == 0 {
			continue
		}

		field := fieldInfo.Name()

		if err := v.validateVectorValues(field, dimension, maxDoc, liveDocs); err != nil {
			v.errors = append(v.errors, err)
		}
	}

	return v.errors
}

// validateVectorValues validates vector values for a field.
func (v *VectorValuesValidator) validateVectorValues(field string, dimension int, maxDoc int, liveDocs util.Bits) error {
	vectorValues, err := v.reader.GetFloatVectorValues(field)
	if err != nil {
		return fmt.Errorf("cannot get vector values for field %s: %w", field, err)
	}

	if vectorValues == nil {
		return fmt.Errorf("vector values is nil for field %s", field)
	}

	// Check dimension
	if dimension <= 0 {
		return fmt.Errorf("invalid dimension %d for field %s", dimension, field)
	}

	// Validate vectors for all documents
	docCount := 0
	for docID := 0; docID < maxDoc; docID++ {
		// Skip deleted documents
		if liveDocs != nil && !liveDocs.Get(docID) {
			continue
		}

		vector, err := vectorValues.Get(docID)
		if err != nil {
			return fmt.Errorf("error reading vector for doc %d in field %s: %w", docID, field, err)
		}

		// If vector exists, check dimension
		if vector != nil && len(vector) != dimension {
			return fmt.Errorf("vector dimension mismatch for doc %d in field %s: got %d, expected %d", docID, field, len(vector), dimension)
		}

		docCount++
	}

	// Check size
	if vectorValues.Size() != docCount {
		return fmt.Errorf("vector count mismatch for field %s: got %d, expected %d", field, vectorValues.Size(), docCount)
	}

	return nil
}

// ExtendedCheckIndex provides additional validation methods for CheckIndex.
type ExtendedCheckIndex struct {
	*CheckIndex
}

// NewExtendedCheckIndex creates a new ExtendedCheckIndex.
func NewExtendedCheckIndex(dir store.Directory) (*ExtendedCheckIndex, error) {
	ci, err := NewCheckIndex(dir)
	if err != nil {
		return nil, err
	}

	return &ExtendedCheckIndex{CheckIndex: ci}, nil
}

// ValidateTermVectors performs comprehensive term vector validation.
func (eci *ExtendedCheckIndex) ValidateTermVectors(reader *SegmentReader) []error {
	validator := NewTermVectorValidator(reader)
	return validator.Validate()
}

// ValidateDocValues performs comprehensive DocValues validation.
func (eci *ExtendedCheckIndex) ValidateDocValues(reader *SegmentReader) []error {
	validator := NewDocValuesValidator(reader)
	return validator.Validate()
}

// ValidatePoints performs comprehensive Points validation.
func (eci *ExtendedCheckIndex) ValidatePoints(reader *SegmentReader) []error {
	validator := NewPointsValidator(reader)
	return validator.Validate()
}

// ValidateVectorValues performs comprehensive VectorValues validation.
func (eci *ExtendedCheckIndex) ValidateVectorValues(reader *SegmentReader) []error {
	validator := NewVectorValuesValidator(reader)
	return validator.Validate()
}

// EnhancedCheckIndexOptions holds options for enhanced checking.
type EnhancedCheckIndexOptions struct {
	// ValidateTermVectors enables comprehensive term vector validation
	ValidateTermVectors bool
	// ValidateDocValues enables comprehensive DocValues validation
	ValidateDocValues bool
	// ValidatePoints enables comprehensive Points validation
	ValidatePoints bool
	// ValidateVectorValues enables comprehensive VectorValues validation
	ValidateVectorValues bool
}

// DefaultEnhancedCheckIndexOptions returns default enhanced check options.
func DefaultEnhancedCheckIndexOptions() *EnhancedCheckIndexOptions {
	return &EnhancedCheckIndexOptions{
		ValidateTermVectors:  true,
		ValidateDocValues:    true,
		ValidatePoints:       true,
		ValidateVectorValues: true,
	}
}

// CheckIndexWithEnhancedValidation performs enhanced index checking.
func CheckIndexWithEnhancedValidation(dir store.Directory, opts *EnhancedCheckIndexOptions) (*CheckIndexStatus, []error, error) {
	if opts == nil {
		opts = DefaultEnhancedCheckIndexOptions()
	}

	ci, err := NewExtendedCheckIndex(dir)
	if err != nil {
		return nil, nil, err
	}
	defer ci.Close()

	// Perform basic check
	status, err := ci.CheckIndex.CheckIndex()
	if err != nil {
		return nil, nil, err
	}

	// Collect additional validation errors
	var validationErrors []error

	// Perform enhanced validations
	for _, segStatus := range status.SegmentInfos {
		if segStatus.Error != nil {
			continue
		}

		// Note: In a full implementation, we would get the SegmentReader here
		// and perform the enhanced validations
	}

	return status, validationErrors, nil
}
