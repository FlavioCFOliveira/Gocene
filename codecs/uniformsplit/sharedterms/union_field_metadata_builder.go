// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package sharedterms

import (
	"errors"
	"math"

	"github.com/FlavioCFOliveira/Gocene/codecs/uniformsplit"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// errUnionFieldMetadataBuilderNoMetadata renders the IllegalStateException
// thrown by UnionFieldMetadataBuilder.build
// (UnionFieldMetadataBuilder.java:50).
var errUnionFieldMetadataBuilderNoMetadata = errors.New("no field metadata was provided")

// UnionFieldMetadataBuilder builds a uniformsplit.FieldMetadata that is the
// union of multiple uniformsplit.FieldMetadata.
//
// Mirrors
// org.apache.lucene.codecs.uniformsplit.sharedterms.UnionFieldMetadataBuilder
// from Apache Lucene 10.5.0.
type UnionFieldMetadataBuilder struct {
	dictionaryStartFP int64
	minStartBlockFP   int64
	maxEndBlockFP     int64
	maxLastTerm       *util.BytesRef
}

// NewUnionFieldMetadataBuilder mirrors the public UnionFieldMetadataBuilder()
// constructor (UnionFieldMetadataBuilder.java:32).
func NewUnionFieldMetadataBuilder() *UnionFieldMetadataBuilder {
	return &UnionFieldMetadataBuilder{
		dictionaryStartFP: -1,
		minStartBlockFP:   math.MaxInt64,
		maxEndBlockFP:     math.MinInt64,
	}
}

// AddFieldMetadata mirrors
// UnionFieldMetadataBuilder.addFieldMetadata(FieldMetadata)
// (UnionFieldMetadataBuilder.java:38).
func (b *UnionFieldMetadataBuilder) AddFieldMetadata(fieldMetadata *uniformsplit.FieldMetadata) *UnionFieldMetadataBuilder {
	// assert dictionaryStartFP == -1 || dictionaryStartFP == fieldMetadata.getDictionaryStartFP();
	b.dictionaryStartFP = fieldMetadata.GetDictionaryStartFP()
	b.minStartBlockFP = min(b.minStartBlockFP, fieldMetadata.GetFirstBlockStartFP())
	b.maxEndBlockFP = max(b.maxEndBlockFP, fieldMetadata.GetLastBlockStartFP())
	if b.maxLastTerm == nil || util.BytesRefCompare(b.maxLastTerm, fieldMetadata.GetLastTerm()) < 0 {
		b.maxLastTerm = fieldMetadata.GetLastTerm()
	}
	return b
}

// Build mirrors UnionFieldMetadataBuilder.build()
// (UnionFieldMetadataBuilder.java:48). Java throws IllegalStateException when
// no field metadata was provided; Gocene reports it as an error.
func (b *UnionFieldMetadataBuilder) Build() (*uniformsplit.FieldMetadata, error) {
	if b.maxLastTerm == nil {
		return nil, errUnionFieldMetadataBuilderNoMetadata
	}
	return uniformsplit.NewFieldMetadataForReading(b.dictionaryStartFP, b.minStartBlockFP, b.maxEndBlockFP, b.maxLastTerm)
}
