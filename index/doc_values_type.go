// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "github.com/FlavioCFOliveira/Gocene/schema"

// This file is the index-side facade for the DocValuesType /
// DocValuesSkipIndexType enums after the SPI unification (rmp #4669 /
// Sprint 117 phase 1.2). The canonical declaration site lives in
// schema/; index/ re-exports the types as Go aliases and re-declares
// the constants as values of the aliased types.

// DocValuesType is an alias of schema.DocValuesType.
type DocValuesType = schema.DocValuesType

// DocValuesSkipIndexType is an alias of schema.DocValuesSkipIndexType.
type DocValuesSkipIndexType = schema.DocValuesSkipIndexType

const (
	DocValuesTypeNone          = schema.DocValuesTypeNone
	DocValuesTypeNumeric       = schema.DocValuesTypeNumeric
	DocValuesTypeBinary        = schema.DocValuesTypeBinary
	DocValuesTypeSorted        = schema.DocValuesTypeSorted
	DocValuesTypeSortedNumeric = schema.DocValuesTypeSortedNumeric
	DocValuesTypeSortedSet     = schema.DocValuesTypeSortedSet
)

const (
	DocValuesSkipIndexTypeNone  = schema.DocValuesSkipIndexTypeNone
	DocValuesSkipIndexTypeRange = schema.DocValuesSkipIndexTypeRange
)
