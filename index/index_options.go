// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "github.com/FlavioCFOliveira/Gocene/schema"

// This file is the index-side facade for the IndexOptions enum after the
// SPI unification (rmp #4669 / Sprint 117 phase 1.2). The canonical
// declaration site lives in schema/; index/ re-exports the type as a Go
// alias and re-declares the constants as values of the aliased type.

// IndexOptions is an alias of schema.IndexOptions.
type IndexOptions = schema.IndexOptions

const (
	IndexOptionsNone                               = schema.IndexOptionsNone
	IndexOptionsDocs                               = schema.IndexOptionsDocs
	IndexOptionsDocsAndFreqs                       = schema.IndexOptionsDocsAndFreqs
	IndexOptionsDocsAndFreqsAndPositions           = schema.IndexOptionsDocsAndFreqsAndPositions
	IndexOptionsDocsAndFreqsAndPositionsAndOffsets = schema.IndexOptionsDocsAndFreqsAndPositionsAndOffsets
)
