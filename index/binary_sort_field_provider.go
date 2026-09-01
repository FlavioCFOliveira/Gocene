// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// BinarySortFieldProvider is the provider for BinarySortField.
type BinarySortFieldProvider struct{}

func (p *BinarySortFieldProvider) Name() string {
	return "BinarySortField"
}

func (p *BinarySortFieldProvider) ReadSortField(in store.DataInput) (SortFieldValue, error) {
	field, err := in.ReadString()
	if err != nil {
		return nil, err
	}
	reverseInt, err := in.ReadInt()
	if err != nil {
		return nil, err
	}
	reverse := reverseInt == 1
	missingInt, err := in.ReadInt()
	if err != nil {
		return nil, err
	}

	var missingValue interface{}
	switch missingInt {
	case 1:
		missingValue = search.STRING_FIRST
	case 2:
		missingValue = search.STRING_LAST
	default:
		missingValue = nil
	}

	return search.NewBinarySortFieldWithMissing(field, reverse, missingValue), nil
}

func (p *BinarySortFieldProvider) WriteSortField(sf SortFieldValue, out store.DataOutput) error {
	bsf, ok := sf.(*search.BinarySortField)
	if !ok {
		return fmt.Errorf("sort field is not a *search.BinarySortField")
	}

	if err := out.WriteString(bsf.GetField()); err != nil {
		return err
	}

	var reverseInt int
	if bsf.GetReverse() {
		reverseInt = 1
	}
	if err := out.WriteInt(reverseInt); err != nil {
		return err
	}

	var missingInt int
	switch bsf.MissingValue {
	case search.STRING_FIRST:
		missingInt = 1
	case search.STRING_LAST:
		missingInt = 2
	default:
		missingInt = 0
	}
	if err := out.WriteInt(missingInt); err != nil {
		return err
	}

	return nil
}

func init() {
	RegisterSortFieldProvider(&BinarySortFieldProvider{})
}
