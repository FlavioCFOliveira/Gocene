// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "github.com/FlavioCFOliveira/Gocene/spi"

// This file is the index-side facade for FieldInfos + FieldInfosIterator
// + FieldInfosBuilder + EmptyFieldInfos after the SPI unification (rmp
// #4669 / Sprint 117 phase 1.2). The canonical declaration site lives
// in schema/; index/ re-exports the types via Go aliases and re-exports
// the factories as thin wrappers so existing callers continue to
// compile unchanged.

// FieldInfos is an alias of spi.FieldInfos.
type FieldInfos = spi.FieldInfos

// FieldInfosIterator is an alias of spi.FieldInfosIterator.
type FieldInfosIterator = spi.FieldInfosIterator

// FieldInfosBuilder is an alias of spi.FieldInfosBuilder.
type FieldInfosBuilder = spi.FieldInfosBuilder

// EmptyFieldInfos re-exports spi.EmptyFieldInfos.
var EmptyFieldInfos = spi.EmptyFieldInfos

// FieldNumbers is an alias of spi.FieldNumbers.
type FieldNumbers = spi.FieldNumbers

// NewFieldNumbers re-exports spi.NewFieldNumbers.
func NewFieldNumbers(softDeletesFieldName, parentFieldName string) *FieldNumbers {
	return spi.NewFieldNumbers(softDeletesFieldName, parentFieldName)
}

// NewFieldInfos re-exports spi.NewFieldInfos.
func NewFieldInfos() *FieldInfos {
	return spi.NewFieldInfos()
}

// NewFieldInfosBuilder re-exports spi.NewFieldInfosBuilder.
//
// PORT NOTE: Java's FieldInfos.Builder(FieldNumbers) asserts a non-null
// registry, because Builder#add funnels every field through
// globalFieldNumbers.addOrGet. Gocene's zero-argument facade therefore hands
// the builder a private, empty registry rather than nil; callers that need a
// registry shared across segments must use NewFieldInfosBuilderFor.
func NewFieldInfosBuilder() *FieldInfosBuilder {
	return spi.NewFieldInfosBuilder(spi.NewFieldNumbers("", ""))
}

// NewFieldInfosBuilderFor re-exports spi.NewFieldInfosBuilder with an
// explicit global field-number registry, mirroring Java's
// FieldInfos.Builder(FieldNumbers globalFieldNumbers).
func NewFieldInfosBuilderFor(globalFieldNumbers *spi.FieldNumbers) *FieldInfosBuilder {
	return spi.NewFieldInfosBuilder(globalFieldNumbers)
}

// FieldInfosGetMergedFieldInfos returns a single FieldInfos describing every field of
// every leaf of reader, with field numbers made consistent across the leaves.
//
// Mirrors org.apache.lucene.index.FieldInfos#getMergedFieldInfos(IndexReader)
// of Apache Lucene 10.5.0.
func FieldInfosGetMergedFieldInfos(reader IndexReader) (*FieldInfos, error) {
	return spi.GetMergedFieldInfos(reader)
}
