// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "github.com/FlavioCFOliveira/Gocene/schema"

// This file is the index-side facade for FieldInfos + FieldInfosIterator
// + FieldInfosBuilder + EmptyFieldInfos after the SPI unification (rmp
// #4669 / Sprint 117 phase 1.2). The canonical declaration site lives
// in schema/; index/ re-exports the types via Go aliases and re-exports
// the factories as thin wrappers so existing callers continue to
// compile unchanged.

// FieldInfos is an alias of schema.FieldInfos.
type FieldInfos = schema.FieldInfos

// FieldInfosIterator is an alias of schema.FieldInfosIterator.
type FieldInfosIterator = schema.FieldInfosIterator

// FieldInfosBuilder is an alias of schema.FieldInfosBuilder.
type FieldInfosBuilder = schema.FieldInfosBuilder

// EmptyFieldInfos re-exports schema.EmptyFieldInfos.
var EmptyFieldInfos = schema.EmptyFieldInfos

// NewFieldInfos re-exports schema.NewFieldInfos.
func NewFieldInfos() *FieldInfos {
	return schema.NewFieldInfos()
}

// NewFieldInfosBuilder re-exports schema.NewFieldInfosBuilder.
func NewFieldInfosBuilder() *FieldInfosBuilder {
	return schema.NewFieldInfosBuilder()
}
