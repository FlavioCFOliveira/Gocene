// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

import (
	"fmt"
)

// BinarySortField is a SortField for BinaryDocValues, usable as an index sort.
// It is the Go port of Lucene's org.apache.lucene.search.BinarySortField.
type BinarySortField struct {
	*SortField
	providerName string
}

// NewBinarySortField creates a sort, possibly in reverse, by the unsigned byte order of the field's value.
func NewBinarySortField(field string, reverse bool) *BinarySortField {
	return NewBinarySortFieldWithMissing(field, reverse, nil)
}

// NewBinarySortFieldWithMissing creates a sort, possibly in reverse, by the unsigned byte order of the field's value,
// with the given handling of missing values.
func NewBinarySortFieldWithMissing(field string, reverse bool, missingValue interface{}) *BinarySortField {
	return NewBinarySortFieldCustom(field, reverse, missingValue, "BinarySortField")
}

// NewBinarySortFieldCustom creates a sort for a subclass that overrides GetSortKeyDocValues.
func NewBinarySortFieldCustom(field string, reverse bool, missingValue interface{}, providerName string) *BinarySortField {
	if missingValue != nil && missingValue != STRING_FIRST && missingValue != STRING_LAST {
		panic("missing value for BinarySortField must be nil, STRING_FIRST or STRING_LAST")
	}
	if providerName == "" {
		panic("providerName must not be empty")
	}

	sf := NewSortField(field, SortFieldTypeCustom)
	sf.Reverse = reverse
	sf.SetMissingValue(missingValue)

	return &BinarySortField{
		SortField:    sf,
		providerName: providerName,
	}
}

// ProviderName returns the registered SortFieldProvider name that must be used to serialise this SortField.
func (bsf *BinarySortField) ProviderName() string {
	return bsf.providerName
}

// GetSortKeyDocValues returns the per-document sort key, as a BinaryDocValues.
// The provider is passed in to avoid a circular dependency.
func (bsf *BinarySortField) GetSortKeyDocValues(provider interface{ GetBinaryDocValues(field string) (BinaryDocValues, error) }) (BinaryDocValues, error) {
	return provider.GetBinaryDocValues(bsf.Field)
}

func (bsf *BinarySortField) String() string {
	s := fmt.Sprintf("<binary: \"%s\">", bsf.Field)
	if bsf.Reverse {
		s += "!"
	}
	if bsf.MissingValue != nil {
		s += fmt.Sprintf(" missingValue=%v", bsf.MissingValue)
	}
	return s
}
