// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "github.com/FlavioCFOliveira/Gocene/spi"

// DocValuesSkipIndexType is the Go port of
// org.apache.lucene.index.DocValuesSkipIndexType from Apache Lucene 10.5.0:
// the options for a skip index on doc values.
//
// Apache Lucene 10.5.0 declares this enum exactly once, in
// org.apache.lucene.index (DocValuesSkipIndexType.java). It had been declared
// twice here — once in this package and once in spi — so the same Lucene enum
// existed as two incompatible Go types and every value crossing the boundary
// was a compile error.
//
// The surviving declaration is spi's, for the same reason DocValuesType,
// FieldInfo, FieldInfos, IndexOptions and PostingsEnum survive there: packages
// below index in the dependency graph (codecs, search) must name the type
// without importing index, and index imports spi rather than the reverse.
// index re-exports it here under its Lucene name, exactly as
// doc_values_type.go does.
//
// The constant ordinals are the on-disk byte encoding written by
// FieldInfosFormat, so they MUST match the Java enum ordinals exactly:
// NONE=0, RANGE=1.
type DocValuesSkipIndexType = spi.DocValuesSkipIndexType

const (
	// DocValuesSkipIndexTypeNone means no skip index should be created.
	DocValuesSkipIndexTypeNone = spi.DocValuesSkipIndexTypeNone

	// DocValuesSkipIndexTypeRange records the range of values. Suitable for
	// NUMERIC, SORTED_NUMERIC, SORTED and SORTED_SET doc values; records the
	// min/max values per range of doc IDs.
	DocValuesSkipIndexTypeRange = spi.DocValuesSkipIndexTypeRange
)
