// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// PointsReader is the wide interface for point value readers.
// This is the Go port of org.apache.lucene.codecs.PointsReader.
type PointsReader interface {
	spi.PointsReader

	// GetValues returns PointValues for the given field.
	// The behavior is undefined if the given field doesn't have points enabled on its FieldInfo.
	GetValues(field string) (index.PointValues, error)

	// GetMergeInstance returns an instance optimized for merging.
	// This instance may only be used in the thread that acquires it.
	GetMergeInstance() PointsReader
}
