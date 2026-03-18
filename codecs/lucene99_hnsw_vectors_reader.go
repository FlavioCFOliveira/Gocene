// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// Lucene99HnswVectorsReader reads HNSW vector data for Lucene 9.9+ format.
type Lucene99HnswVectorsReader struct {
	state           *SegmentReadState
	closed          bool
	vectorDataIn    store.IndexInput
	vectorIndexIn   store.IndexInput
	vectorDataSize  int64
	vectorIndexSize int64
}

// NewLucene99HnswVectorsReader creates a new HNSW vectors reader.
func NewLucene99HnswVectorsReader(state *SegmentReadState) (*Lucene99HnswVectorsReader, error) {
	// Open input files for vector data and index
	vectorDataFileName := fmt.Sprintf("%s_Lucene99HnswVectorsFormat_0.vec", state.SegmentInfo.Name())
	vectorIndexFileName := fmt.Sprintf("%s_Lucene99HnswVectorsFormat_0.vex", state.SegmentInfo.Name())

	vectorDataIn, err := state.Directory.OpenInput(vectorDataFileName, store.IOContextRead)
	if err != nil {
		return nil, fmt.Errorf("failed to open vector data input: %w", err)
	}

	vectorIndexIn, err := state.Directory.OpenInput(vectorIndexFileName, store.IOContextRead)
	if err != nil {
		vectorDataIn.Close()
		return nil, fmt.Errorf("failed to open vector index input: %w", err)
	}

	reader := &Lucene99HnswVectorsReader{
		state:           state,
		vectorDataIn:    vectorDataIn,
		vectorIndexIn:   vectorIndexIn,
		vectorDataSize:  vectorDataIn.Length(),
		vectorIndexSize: vectorIndexIn.Length(),
	}

	// Read and validate headers
	if err := reader.readHeaders(); err != nil {
		reader.Close()
		return nil, err
	}

	return reader, nil
}

// readHeaders reads and validates the file headers for both .vec and .vex files.
func (r *Lucene99HnswVectorsReader) readHeaders() error {
	// Read header for vector data file (.vec)
	vecHelper := NewKnnVectorsReaderHelper(r.vectorDataIn)
	if err := vecHelper.ReadHeader(); err != nil {
		return fmt.Errorf("failed to read vector data header: %w", err)
	}

	// Read header for vector index file (.vex)
	vexHelper := NewKnnVectorsReaderHelper(r.vectorIndexIn)
	if err := vexHelper.ReadHeader(); err != nil {
		return fmt.Errorf("failed to read vector index header: %w", err)
	}

	return nil
}

// CheckIntegrity checks the integrity of the vectors.
func (r *Lucene99HnswVectorsReader) CheckIntegrity() error {
	if r.closed {
		return fmt.Errorf("reader is closed")
	}

	// TODO: Implement actual integrity checking
	// This should verify checksums and data consistency

	return nil
}

// Close releases resources.
func (r *Lucene99HnswVectorsReader) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true

	var firstErr error
	if r.vectorDataIn != nil {
		if err := r.vectorDataIn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if r.vectorIndexIn != nil {
		if err := r.vectorIndexIn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// GetVectorDataSize returns the size of the vector data file.
func (r *Lucene99HnswVectorsReader) GetVectorDataSize() int64 {
	return r.vectorDataSize
}

// GetVectorIndexSize returns the size of the vector index file.
func (r *Lucene99HnswVectorsReader) GetVectorIndexSize() int64 {
	return r.vectorIndexSize
}
