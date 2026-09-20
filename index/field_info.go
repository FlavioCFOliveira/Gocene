// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "github.com/FlavioCFOliveira/Gocene/spi"

// This file is the index-side facade for FieldInfo + FieldInfoOptions +
// FieldInfoBuilder + DefaultFieldInfoOptions after the SPI unification
// (rmp #4669 / Sprint 117 phase 1.2). The canonical declaration site
// lives in schema/; index/ re-exports the types via Go aliases and
// re-exports the constructor / builder factories as thin wrappers so
// existing callers continue to compile unchanged.

// FieldInfo is an alias of spi.FieldInfo.
type FieldInfo = spi.FieldInfo

// FieldInfoOptions is an alias of spi.FieldInfoOptions.
type FieldInfoOptions = spi.FieldInfoOptions

// FieldInfoBuilder is an alias of spi.FieldInfoBuilder.
type FieldInfoBuilder = spi.FieldInfoBuilder

// DefaultFieldInfoOptions re-exports spi.DefaultFieldInfoOptions.
func DefaultFieldInfoOptions() FieldInfoOptions {
	return spi.DefaultFieldInfoOptions()
}

// NewFieldInfo re-exports spi.NewFieldInfo.
func NewFieldInfo(name string, number int, opts FieldInfoOptions) *FieldInfo {
	return spi.NewFieldInfo(name, number, opts)
}

// NewFieldInfoBuilder re-exports spi.NewFieldInfoBuilder.
func NewFieldInfoBuilder(name string, number int) *FieldInfoBuilder {
	return spi.NewFieldInfoBuilder(name, number)
}
