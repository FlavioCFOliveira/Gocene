// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package sharedterms

import (
	"github.com/FlavioCFOliveira/Gocene/codecs/uniformsplit"
	"github.com/FlavioCFOliveira/Gocene/index"
)

// FieldMetadataTermState is a pair of uniformsplit.FieldMetadata and
// BlockTermState for a specific field.
//
// Mirrors the record
// org.apache.lucene.codecs.uniformsplit.sharedterms.FieldMetadataTermState
// from Apache Lucene 10.5.0. A Java record declares one accessor per
// component; the components are the exported fields below and the accessors
// are the methods of the same name, so both spellings of the Java source are
// available.
type FieldMetadataTermState struct {
	fieldMetadata *uniformsplit.FieldMetadata
	state         index.TermState
}

// NewFieldMetadataTermState mirrors the canonical record constructor
// FieldMetadataTermState(FieldMetadata, BlockTermState)
// (FieldMetadataTermState.java:28).
func NewFieldMetadataTermState(fieldMetadata *uniformsplit.FieldMetadata, state index.TermState) *FieldMetadataTermState {
	return &FieldMetadataTermState{fieldMetadata: fieldMetadata, state: state}
}

// FieldMetadata mirrors the record accessor
// FieldMetadataTermState.fieldMetadata() (FieldMetadataTermState.java:28).
func (s *FieldMetadataTermState) FieldMetadata() *uniformsplit.FieldMetadata {
	return s.fieldMetadata
}

// State mirrors the record accessor FieldMetadataTermState.state()
// (FieldMetadataTermState.java:28).
func (s *FieldMetadataTermState) State() index.TermState {
	return s.state
}
