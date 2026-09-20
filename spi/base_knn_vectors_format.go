// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

import (
	"fmt"
)

// BaseKnnVectorsFormat provides common functionality for KnnVectorsFormat
// implementations: the format name recorded by the protected constructor
// KnnVectorsFormat(String name) of Apache Lucene 10.5.0 and returned by
// getName().
//
// The type lives in spi rather than next to codecs.KnnVectorsFormat because
// org.apache.lucene.codecs.hnsw.FlatVectorsFormat extends KnnVectorsFormat:
// the codecs/hnsw package embeds this base, and the codecs package imports
// codecs/hnsw for the Lucene99 flat vectors format, so codecs/hnsw cannot
// import codecs. codecs.BaseKnnVectorsFormat is an alias of this type.
type BaseKnnVectorsFormat struct {
	name string
}

// NewBaseKnnVectorsFormat creates a new BaseKnnVectorsFormat.
func NewBaseKnnVectorsFormat(name string) *BaseKnnVectorsFormat {
	return &BaseKnnVectorsFormat{name: name}
}

// Name returns the format name.
func (f *BaseKnnVectorsFormat) Name() string {
	return f.name
}

// FieldsWriter returns a fields writer (must be implemented by subclasses).
func (f *BaseKnnVectorsFormat) FieldsWriter(state *SegmentWriteState) (KnnVectorsWriter, error) {
	return nil, fmt.Errorf("FieldsWriter not implemented")
}

// FieldsReader returns a fields reader (must be implemented by subclasses).
func (f *BaseKnnVectorsFormat) FieldsReader(state *SegmentReadState) (KnnVectorsReader, error) {
	return nil, fmt.Errorf("FieldsReader not implemented")
}

// Compile-time check that BaseKnnVectorsFormat satisfies KnnVectorsFormat.
var _ KnnVectorsFormat = (*BaseKnnVectorsFormat)(nil)
