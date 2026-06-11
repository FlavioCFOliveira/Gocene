// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
)

// FieldCacheSource is a base struct for ValueSource implementations that
// retrieve values for a single field from DocValues. Concrete types embed
// this and provide their own GetValues implementation.
//
// Go port of org.apache.lucene.queries.function.valuesource.FieldCacheSource.
type FieldCacheSource struct {
	function.BaseValueSource
	field string
}

// NewFieldCacheSource returns a FieldCacheSource for the given field name.
func NewFieldCacheSource(field string) FieldCacheSource {
	return FieldCacheSource{field: field}
}

// GetField returns the field name.
func (s *FieldCacheSource) GetField() string { return s.field }

// Description returns the field name.
func (s *FieldCacheSource) Description() string { return s.field }

// getNumericDocValues retrieves NumericDocValues for the given field from
// the leaf reader context.
func getNumericDocValues(field string, context *index.LeafReaderContext) (index.NumericDocValues, error) {
	leaf := context.LeafReader()
	if leaf == nil {
		return nil, nil
	}
	ndv, ok := leaf.(numericDocValuesReader)
	if !ok {
		return nil, nil
	}
	return ndv.GetNumericDocValues(field)
}

// numericDocValuesReader is the local contract for leaf readers that
// expose NumericDocValues.
type numericDocValuesReader interface {
	GetNumericDocValues(field string) (index.NumericDocValues, error)
}

// binaryDocValuesReader allows reading BinaryDocValues from a leaf.
type binaryDocValuesReader interface {
	GetBinaryDocValues(field string) (index.BinaryDocValues, error)
}

// sortedNumericDocValuesReader allows reading SortedNumericDocValues.
type sortedNumericDocValuesReader interface {
	GetSortedNumericDocValues(field string) (index.SortedNumericDocValues, error)
}

// sortedSetDocValuesReader allows reading SortedSetDocValues.
type sortedSetDocValuesReader interface {
	GetSortedSetDocValues(field string) (index.SortedSetDocValues, error)
}

// fieldInfosReader allows reading FieldInfos from a leaf.
type fieldInfosReader interface {
	GetFieldInfos() *index.FieldInfos
}

// missingValuesBase is an embeddable struct that implements all the
// FunctionValues multi-methods as unsupported, plus common accessors.
// Concrete missing-values types embed this and override FloatVal etc.
type missingValuesBase struct {
	function.BaseFunctionValues
}

func (b *missingValuesBase) ByteVal(_ int) (int8, error)                           { return 0, nil }
func (b *missingValuesBase) ShortVal(_ int) (int16, error)                         { return 0, nil }
func (b *missingValuesBase) IntVal(_ int) (int32, error)                           { return 0, nil }
func (b *missingValuesBase) LongVal(_ int) (int64, error)                          { return 0, nil }
func (b *missingValuesBase) FloatVal(_ int) (float32, error)                       { return 0, nil }
func (b *missingValuesBase) DoubleVal(_ int) (float64, error)                      { return 0, nil }
func (b *missingValuesBase) StrVal(_ int) (string, error)                          { return "", nil }
func (b *missingValuesBase) BoolVal(_ int) (bool, error)                           { return false, nil }
func (b *missingValuesBase) ObjectVal(_ int) (any, error)                          { return nil, nil }
func (b *missingValuesBase) Exists(_ int) (bool, error)                            { return false, nil }
func (b *missingValuesBase) Cost() float32                                         { return 1 }
func (b *missingValuesBase) GetValueFiller() function.ValueFiller                   { return &missingValueFiller{} }

type missingValueFiller struct {
	mval function.MutableValueFloat
}

func (f *missingValueFiller) GetValue() *function.MutableValueFloat { return &f.mval }
func (f *missingValueFiller) FillValue(doc int) error {
	f.mval.Value = 0
	f.mval.Exists = false
	return nil
}

// newAllValueSourceScorer returns an always-matching ValueSourceScorer.
func newAllValueSourceScorer(readerContext *index.LeafReaderContext, values function.FunctionValues) function.ValueSourceScorer {
	return &allValuesScorer{
		readerContext: readerContext,
		values:        values,
	}
}

type allValuesScorer struct {
	readerContext *index.LeafReaderContext
	values        function.FunctionValues
}

func (s *allValuesScorer) Values() function.FunctionValues                { return s.values }
func (s *allValuesScorer) LeafContext() *index.LeafReaderContext           { return s.readerContext }
func (s *allValuesScorer) MaxDoc() int                                    { return 0 }
func (s *allValuesScorer) MatchCost() float32                             { return 0 }
func (s *allValuesScorer) Score(doc int) (float32, error)                 { return s.values.FloatVal(doc) }
func (s *allValuesScorer) MaxScore(_ int) float32                         { return float32(1 << 30) }
func (s *allValuesScorer) Matches(_ int) (bool, error)                    { return true, nil }

// docFieldExists checks whether the given NumericDocValues iterator has a
// value for doc, following the same out-of-order guard as Java field sources.
func docFieldExists(arr index.NumericDocValues, doc int) bool {
	curDocID := arr.DocID()
	if doc > curDocID {
		var err error
		curDocID, err = arr.Advance(doc)
		if err != nil {
			return false
		}
	}
	return doc == curDocID
}

// getSortedNumericDocValues retrieves SortedNumericDocValues.
func getSortedNumericDocValues(field string, context *index.LeafReaderContext) (index.SortedNumericDocValues, error) {
	leaf := context.LeafReader()
	if leaf == nil {
		return nil, nil
	}
	sndv, ok := leaf.(sortedNumericDocValuesReader)
	if !ok {
		return nil, nil
	}
	return sndv.GetSortedNumericDocValues(field)
}

// getSortedSetDocValues retrieves SortedSetDocValues.
func getSortedSetDocValues(field string, context *index.LeafReaderContext) (index.SortedSetDocValues, error) {
	leaf := context.LeafReader()
	if leaf == nil {
		return nil, nil
	}
	ssdv, ok := leaf.(sortedSetDocValuesReader)
	if !ok {
		return nil, nil
	}
	return ssdv.GetSortedSetDocValues(field)
}
