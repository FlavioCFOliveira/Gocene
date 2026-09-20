// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package api

import "github.com/FlavioCFOliveira/Gocene/util"

// CharTermAttribute is the interface for the character term attribute.
type CharTermAttribute interface {
	util.Attribute
	Get() string
	Set(val string)
}

// OffsetAttribute is the interface for the offset attribute.
type OffsetAttribute interface {
	util.Attribute
	Get() int
	Set(val int)
}

// PositionIncrementAttribute is the interface for the position increment attribute.
type PositionIncrementAttribute interface {
	util.Attribute
	Get() int
	Set(val int)
}

// TypeAttribute is the interface for the type attribute.
type TypeAttribute interface {
	util.Attribute
	Get() string
	Set(val string)
}

// PayloadAttribute is the interface for the payload attribute.
type PayloadAttribute interface {
	util.Attribute
	Get() any
	Set(val any)
}

// TermToBytesRefAttribute is the interface for the term-to-bytes-ref attribute.
type TermToBytesRefAttribute interface {
	util.Attribute
	Get() *util.BytesRef
	Set(val *util.BytesRef)
}

// TermFrequencyAttribute is the interface for the term frequency attribute.
type TermFrequencyAttribute interface {
	util.Attribute
	Get() int
	Set(val int)
}

// AttributeSource is the interface for providing access to attributes.
type AttributeSource interface {
	GetAttributeTermToBytesRef() TermToBytesRefAttribute
	AddAttributeTermFrequency() TermFrequencyAttribute
	AddAttributePositionIncrement() PositionIncrementAttribute
	AddAttributeOffset() OffsetAttribute
	GetAttributePayload() PayloadAttribute
}
