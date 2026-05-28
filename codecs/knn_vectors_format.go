// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// KnnVectorsFormat is an alias of [spi.KnnVectorsFormat]. The canonical
// wide interface was lifted into the SPI by rmp #4707; this alias keeps
// the codecs-package identifier source-compatible with every codec port
// that historically reached for codecs.KnnVectorsFormat.
type KnnVectorsFormat = spi.KnnVectorsFormat

// KnnVectorsWriter is an alias of [spi.KnnVectorsWriter]. Lifted by rmp
// #4707 alongside KnnVectorsFormat.
type KnnVectorsWriter = spi.KnnVectorsWriter

// KnnVectorsReader is an alias of [spi.KnnVectorsReader]. The narrow
// Accountable-style read-side surface (CheckIntegrity / Close) is the
// only contract the wide [spi.KnnVectorsWriter.WriteField] entrypoint
// requires today; the per-encoding read methods (getFloatVectorValues,
// getByteVectorValues, search, …) live in this codecs package as
// concrete-typed helpers on per-format reader implementations.
type KnnVectorsReader = spi.KnnVectorsReader

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

// Compile-time check that BaseKnnVectorsFormat satisfies KnnVectorsFormat.
var _ KnnVectorsFormat = (*BaseKnnVectorsFormat)(nil)
