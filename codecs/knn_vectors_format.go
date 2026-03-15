// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// KnnVectorsFormat handles encoding/decoding of KNN (K-Nearest Neighbors) vector values.
// This is the Go port of Lucene's org.apache.lucene.codecs.KnnVectorsFormat.
//
// KNN vectors are used for vector search and similarity queries. They are stored
// in a format optimized for approximate nearest neighbor search using HNSW
// (Hierarchical Navigable Small World) graphs.
type KnnVectorsFormat interface {
	// Name returns the name of this format.
	Name() string

	// FieldsWriter returns a writer for writing KNN vectors.
	// The caller should close the returned writer when done.
	FieldsWriter(state *SegmentWriteState) (KnnVectorsWriter, error)

	// FieldsReader returns a reader for reading KNN vectors.
	// The caller should close the returned reader when done.
	FieldsReader(state *SegmentReadState) (KnnVectorsReader, error)
}

// BaseKnnVectorsFormat provides common functionality for KnnVectorsFormat implementations.
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

// KnnVectorsWriter is a writer for KNN vector values.
// This is the Go port of Lucene's org.apache.lucene.codecs.KnnVectorsWriter.
type KnnVectorsWriter interface {
	// WriteField writes a KNN vector field.
	// The values are provided through the reader.
	WriteField(fieldInfo *index.FieldInfo, reader KnnVectorsReader) error

	// Finish finalizes the writing process.
	Finish() error

	// Close releases resources.
	Close() error
}

// KnnVectorsReader is a reader for KNN vector values.
// This is the Go port of Lucene's org.apache.lucene.codecs.KnnVectorsReader.
type KnnVectorsReader interface {
	// CheckIntegrity checks the integrity of the vectors.
	CheckIntegrity() error

	// Close releases resources.
	Close() error
}

// FloatVectorValues provides access to float vector values.
// This is the Go port of Lucene's org.apache.lucene.index.FloatVectorValues.
type FloatVectorValues interface {
	// Dimension returns the dimension of the vectors.
	Dimension() int

	// Size returns the number of vectors.
	Size() int

	// GetVector returns the vector value for the given document.
	GetVector(docID int) ([]float32, error)

	// DocID returns the current document ID.
	DocID() int

	// NextDoc advances to the next document that has a vector.
	// Returns NO_MORE_DOCS if there are no more documents.
	NextDoc() (int, error)

	// Advance advances to the first document >= target that has a vector.
	// Returns NO_MORE_DOCS if there are no more documents.
	Advance(target int) (int, error)
}

// ByteVectorValues provides access to byte vector values.
// This is the Go port of Lucene's org.apache.lucene.index.ByteVectorValues.
type ByteVectorValues interface {
	// Dimension returns the dimension of the vectors.
	Dimension() int

	// Size returns the number of vectors.
	Size() int

	// GetVector returns the vector value for the given document.
	GetVector(docID int) ([]byte, error)

	// DocID returns the current document ID.
	DocID() int

	// NextDoc advances to the next document that has a vector.
	// Returns NO_MORE_DOCS if there are no more documents.
	NextDoc() (int, error)

	// Advance advances to the first document >= target that has a vector.
	// Returns NO_MORE_DOCS if there are no more documents.
	Advance(target int) (int, error)
}

// RandomVectorScorer scores vectors randomly.
// This is the Go port of Lucene's org.apache.lucene.util.RandomVectorScorer.
type RandomVectorScorer interface {
	// Score returns the score for the given document.
	Score(docID int) (float32, error)

	// GetMaxScore returns the maximum possible score.
	GetMaxScore() float32
}

// RandomVectorScorerSupplier supplies RandomVectorScorer instances.
// This is the Go port of Lucene's org.apache.lucene.util.RandomVectorScorerSupplier.
type RandomVectorScorerSupplier interface {
	// GetScorer returns a RandomVectorScorer for the given query vector.
	GetScorer(queryVector []float32) (RandomVectorScorer, error)
}

// KnnVectorsWriterHelper is a helper for writing KNN vectors.
type KnnVectorsWriterHelper struct {
	out    store.IndexOutput
	closed bool
}

// NewKnnVectorsWriterHelper creates a new KnnVectorsWriterHelper.
func NewKnnVectorsWriterHelper(out store.IndexOutput) *KnnVectorsWriterHelper {
	return &KnnVectorsWriterHelper{out: out}
}

// WriteHeader writes the KNN vectors file header.
func (w *KnnVectorsWriterHelper) WriteHeader() error {
	// Write magic number (KNN = K-Nearest Neighbors)
	if err := store.WriteUint32(w.out, 0x4B4E4E00); err != nil {
		return fmt.Errorf("failed to write magic number: %w", err)
	}
	// Write version
	if err := store.WriteUint32(w.out, 1); err != nil {
		return fmt.Errorf("failed to write version: %w", err)
	}
	return nil
}

// Close closes the writer.
func (w *KnnVectorsWriterHelper) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	return w.out.Close()
}

// KnnVectorsReaderHelper is a helper for reading KNN vectors.
type KnnVectorsReaderHelper struct {
	in     store.IndexInput
	closed bool
}

// NewKnnVectorsReaderHelper creates a new KnnVectorsReaderHelper.
func NewKnnVectorsReaderHelper(in store.IndexInput) *KnnVectorsReaderHelper {
	return &KnnVectorsReaderHelper{in: in}
}

// ReadHeader reads and validates the KNN vectors file header.
func (r *KnnVectorsReaderHelper) ReadHeader() error {
	// Read magic number
	magic, err := store.ReadUint32(r.in)
	if err != nil {
		return fmt.Errorf("failed to read magic number: %w", err)
	}
	if magic != 0x4B4E4E00 {
		return fmt.Errorf("invalid magic number: expected 0x4B4E4E00, got 0x%08x", magic)
	}

	// Read version
	version, err := store.ReadUint32(r.in)
	if err != nil {
		return fmt.Errorf("failed to read version: %w", err)
	}
	if version != 1 {
		return fmt.Errorf("unsupported version: %d", version)
	}

	return nil
}

// Close closes the reader.
func (r *KnnVectorsReaderHelper) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	return r.in.Close()
}
