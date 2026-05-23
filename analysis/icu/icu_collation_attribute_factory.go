// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package icu

import (
	"github.com/FlavioCFOliveira/Gocene/analysis/icu/tokenattributes"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ICUCollationAttributeFactory is an AttributeFactory that substitutes
// ICUCollatedTermAttributeImpl for the default CharTermAttributeImpl,
// encoding each token as a binary Unicode collation key.
//
// Go port of
// org.apache.lucene.analysis.icu.ICUCollationAttributeFactory
// (Apache Lucene 10.4.0).
//
// Deviation: The Java original extends
// AttributeFactory.StaticImplementationAttributeFactory, delegating to
// TokenStream.DEFAULT_TOKEN_ATTRIBUTE_FACTORY. In Go,
// StaticImplementationAttributeFactory is used directly.
type ICUCollationAttributeFactory struct {
	*util.StaticImplementationAttributeFactory
}

// NewICUCollationAttributeFactory creates an ICUCollationAttributeFactory
// that delegates non-collation attributes to
// util.DefaultAttributeFactoryInstance.
func NewICUCollationAttributeFactory(collator tokenattributes.Collator) *ICUCollationAttributeFactory {
	return NewICUCollationAttributeFactoryWithDelegate(
		util.DefaultAttributeFactoryInstance,
		collator,
	)
}

// NewICUCollationAttributeFactoryWithDelegate creates an
// ICUCollationAttributeFactory using the supplied delegate for attribute
// types not satisfied by ICUCollatedTermAttributeImpl.
func NewICUCollationAttributeFactoryWithDelegate(
	delegate util.AttributeFactory,
	collator tokenattributes.Collator,
) *ICUCollationAttributeFactory {
	inner := util.NewStaticImplementationAttributeFactory(
		delegate,
		func() util.AttributeImpl {
			return tokenattributes.NewICUCollatedTermAttributeImpl(collator)
		},
	)
	return &ICUCollationAttributeFactory{StaticImplementationAttributeFactory: inner}
}

// Ensure compile-time interface satisfaction.
var _ util.AttributeFactory = (*ICUCollationAttributeFactory)(nil)
