// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"reflect"

	"github.com/FlavioCFOliveira/Gocene/analysis/tokenattributes"
	"github.com/FlavioCFOliveira/Gocene/util"
)

func init() {
	// Register core attributes.
	// The registry uses the interface reflect.Type as the key.

	// CharTermAttribute
	util.RegisterAttributeImpl(CharTermAttributeType, func() util.AttributeImpl {
		return NewCharTermAttribute()
	})

	// OffsetAttribute
	util.RegisterAttributeImpl(OffsetAttributeType, func() util.AttributeImpl {
		return NewOffsetAttribute()
	})

	// TypeAttribute
	util.RegisterAttributeImpl(TypeAttributeType, func() util.AttributeImpl {
		return NewTypeAttribute()
	})

	// PayloadAttribute
	util.RegisterAttributeImpl(PayloadAttributeType, func() util.AttributeImpl {
		return NewPayloadAttribute()
	})

	// FlagsAttribute
	util.RegisterAttributeImpl(FlagsAttributeType, func() util.AttributeImpl {
		return NewFlagsAttribute()
	})

	// KeywordAttribute
	util.RegisterAttributeImpl(KeywordAttributeType, func() util.AttributeImpl {
		return NewKeywordAttribute()
	})

	// PositionLengthAttribute
	util.RegisterAttributeImpl(PositionLengthAttributeType, func() util.AttributeImpl {
		return NewPositionLengthAttribute()
	})

	// TermFrequencyAttribute
	util.RegisterAttributeImpl(TermFrequencyAttributeType, func() util.AttributeImpl {
		return NewTermFrequencyAttribute()
	})

	// PositionIncrementAttribute (from tokenattributes package)
	util.RegisterAttributeImpl(
		reflect.TypeOf((*tokenattributes.PositionIncrementAttribute)(nil)).Elem(),
		func() util.AttributeImpl {
			return tokenattributes.NewPositionIncrementAttribute()
		},
	)
}
