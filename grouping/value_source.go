// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// ValueSource provides values for documents.
// This is the base interface for value sources used in grouping.
//
// This is the Go port of Lucene's org.apache.lucene.queries.function.ValueSource.
type ValueSource interface {
	// GetValues returns the values for the given context.
	GetValues(context *index.LeafReaderContext) (ValueSourceValues, error)

	// Description returns a description of this value source.
	Description() string
}

// ValueSourceValues provides access to values for documents.
type ValueSourceValues interface {
	// DoubleVal returns the double value for the given document.
	DoubleVal(doc int) (float64, error)

	// FloatVal returns the float value for the given document.
	FloatVal(doc int) (float32, error)

	// IntVal returns the int value for the given document.
	IntVal(doc int) (int, error)

	// LongVal returns the long value for the given document.
	LongVal(doc int) (int64, error)

	// StrVal returns the string value for the given document.
	StrVal(doc int) (string, error)

	// Exists returns true if a value exists for the given document.
	Exists(doc int) bool
}

// DoubleValueSource is a value source that provides double values.
type DoubleValueSource struct {
	field string
}

// NewDoubleValueSource creates a new DoubleValueSource.
func NewDoubleValueSource(field string) *DoubleValueSource {
	return &DoubleValueSource{field: field}
}

// GetValues returns the values for the given context.
func (dvs *DoubleValueSource) GetValues(context *index.LeafReaderContext) (ValueSourceValues, error) {
	return &doubleValueSourceValues{
		field:  dvs.field,
		reader: context.LeafReader(),
		values: make(map[int]float64),
	}, nil
}

// Description returns a description of this value source.
func (dvs *DoubleValueSource) Description() string {
	return fmt.Sprintf("double(%s)", dvs.field)
}

type doubleValueSourceValues struct {
	field  string
	reader index.LeafReaderInterface
	values map[int]float64
}

func (dvv *doubleValueSourceValues) DoubleVal(doc int) (float64, error) {
	if val, ok := dvv.values[doc]; ok {
		return val, nil
	}
	return 0, nil
}

func (dvv *doubleValueSourceValues) FloatVal(doc int) (float32, error) {
	val, err := dvv.DoubleVal(doc)
	return float32(val), err
}

func (dvv *doubleValueSourceValues) IntVal(doc int) (int, error) {
	val, err := dvv.DoubleVal(doc)
	return int(val), err
}

func (dvv *doubleValueSourceValues) LongVal(doc int) (int64, error) {
	val, err := dvv.DoubleVal(doc)
	return int64(val), err
}

func (dvv *doubleValueSourceValues) StrVal(doc int) (string, error) {
	val, err := dvv.DoubleVal(doc)
	return fmt.Sprintf("%f", val), err
}

func (dvv *doubleValueSourceValues) Exists(doc int) bool {
	_, ok := dvv.values[doc]
	return ok
}

// LongValueSource is a value source that provides long values.
type LongValueSource struct {
	field string
}

// NewLongValueSource creates a new LongValueSource.
func NewLongValueSource(field string) *LongValueSource {
	return &LongValueSource{field: field}
}

// GetValues returns the values for the given context.
func (lvs *LongValueSource) GetValues(context *index.LeafReaderContext) (ValueSourceValues, error) {
	return &longValueSourceValues{
		field:  lvs.field,
		reader: context.LeafReader(),
		values: make(map[int]int64),
	}, nil
}

// Description returns a description of this value source.
func (lvs *LongValueSource) Description() string {
	return fmt.Sprintf("long(%s)", lvs.field)
}

type longValueSourceValues struct {
	field  string
	reader index.LeafReaderInterface
	values map[int]int64
}

func (lvv *longValueSourceValues) DoubleVal(doc int) (float64, error) {
	val, err := lvv.LongVal(doc)
	return float64(val), err
}

func (lvv *longValueSourceValues) FloatVal(doc int) (float32, error) {
	val, err := lvv.LongVal(doc)
	return float32(val), err
}

func (lvv *longValueSourceValues) IntVal(doc int) (int, error) {
	val, err := lvv.LongVal(doc)
	return int(val), err
}

func (lvv *longValueSourceValues) LongVal(doc int) (int64, error) {
	if val, ok := lvv.values[doc]; ok {
		return val, nil
	}
	return 0, nil
}

func (lvv *longValueSourceValues) StrVal(doc int) (string, error) {
	val, err := lvv.LongVal(doc)
	return fmt.Sprintf("%d", val), err
}

func (lvv *longValueSourceValues) Exists(doc int) bool {
	_, ok := lvv.values[doc]
	return ok
}

// ValueSourceGroupSelector selects groups based on a ValueSource.
//
// This is the Go port of Lucene's org.apache.lucene.search.grouping.ValueSourceGroupSelector.
type ValueSourceGroupSelector struct {
	// valueSource provides the values for grouping
	valueSource ValueSource

	// context is the leaf reader context
	context *index.LeafReaderContext

	// values caches the values for each document
	values map[int]interface{}
}

// NewValueSourceGroupSelector creates a new ValueSourceGroupSelector.
func NewValueSourceGroupSelector(valueSource ValueSource, context *index.LeafReaderContext) *ValueSourceGroupSelector {
	return &ValueSourceGroupSelector{
		valueSource: valueSource,
		context:     context,
		values:      make(map[int]interface{}),
	}
}

// Select returns the group value for the given document.
func (vsgs *ValueSourceGroupSelector) Select(doc int) interface{} {
	// Check cache first
	if value, ok := vsgs.values[doc]; ok {
		return value
	}

	// Get values from value source
	values, err := vsgs.valueSource.GetValues(vsgs.context)
	if err != nil {
		return nil
	}

	// Get the value for this document
	var value interface{}

	// Try different value types
	if strVal, err := values.StrVal(doc); err == nil && values.Exists(doc) {
		value = strVal
	} else if longVal, err := values.LongVal(doc); err == nil && values.Exists(doc) {
		value = longVal
	} else if doubleVal, err := values.DoubleVal(doc); err == nil && values.Exists(doc) {
		value = doubleVal
	}

	// Cache the value
	vsgs.values[doc] = value

	return value
}

// SetValue sets the value for a document (for caching/testing).
func (vsgs *ValueSourceGroupSelector) SetValue(doc int, value interface{}) {
	vsgs.values[doc] = value
}

// GetValueSource returns the value source.
func (vsgs *ValueSourceGroupSelector) GetValueSource() ValueSource {
	return vsgs.valueSource
}

// GetContext returns the leaf reader context.
func (vsgs *ValueSourceGroupSelector) GetContext() *index.LeafReaderContext {
	return vsgs.context
}

// Reset clears the value cache.
func (vsgs *ValueSourceGroupSelector) Reset() {
	vsgs.values = make(map[int]interface{})
}

// Ensure ValueSourceGroupSelector implements GroupSelector
var _ GroupSelector = (*ValueSourceGroupSelector)(nil)
