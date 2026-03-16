// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Lucene99HnswVectorsWriter writes HNSW vector data for Lucene 9.9+ format.
type Lucene99HnswVectorsWriter struct {
	state                 *SegmentWriteState
	maxConn               int
	beamWidth             int
	tinySegmentsThreshold int
	numMergeWorkers       int
	closed                bool
	helper                *KnnVectorsWriterHelper
	vectorDataOut         store.IndexOutput
	vectorIndexOut        store.IndexOutput
}

// NewLucene99HnswVectorsWriter creates a new HNSW vectors writer.
func NewLucene99HnswVectorsWriter(state *SegmentWriteState, maxConn, beamWidth, tinySegmentsThreshold, numMergeWorkers int) (*Lucene99HnswVectorsWriter, error) {
	// Create output files for vector data and index
	vectorDataFileName := fmt.Sprintf("%s_Lucene99HnswVectorsFormat_0.vec", state.SegmentInfo.Name())
	vectorIndexFileName := fmt.Sprintf("%s_Lucene99HnswVectorsFormat_0.vex", state.SegmentInfo.Name())

	vectorDataOut, err := state.Directory.CreateOutput(vectorDataFileName, store.IOContextWrite)
	if err != nil {
		return nil, fmt.Errorf("failed to create vector data output: %w", err)
	}

	vectorIndexOut, err := state.Directory.CreateOutput(vectorIndexFileName, store.IOContextWrite)
	if err != nil {
		vectorDataOut.Close()
		return nil, fmt.Errorf("failed to create vector index output: %w", err)
	}

	writer := &Lucene99HnswVectorsWriter{
		state:                 state,
		maxConn:               maxConn,
		beamWidth:             beamWidth,
		tinySegmentsThreshold: tinySegmentsThreshold,
		numMergeWorkers:       numMergeWorkers,
		vectorDataOut:         vectorDataOut,
		vectorIndexOut:        vectorIndexOut,
	}

	// Write headers
	if err := writer.writeHeaders(); err != nil {
		writer.Close()
		return nil, err
	}

	return writer, nil
}

// writeHeaders writes the file headers for both .vec and .vex files.
func (w *Lucene99HnswVectorsWriter) writeHeaders() error {
	// Write header for vector data file (.vec)
	vecHelper := NewKnnVectorsWriterHelper(w.vectorDataOut)
	if err := vecHelper.WriteHeader(); err != nil {
		return fmt.Errorf("failed to write vector data header: %w", err)
	}

	// Write header for vector index file (.vex)
	vexHelper := NewKnnVectorsWriterHelper(w.vectorIndexOut)
	if err := vexHelper.WriteHeader(); err != nil {
		return fmt.Errorf("failed to write vector index header: %w", err)
	}

	return nil
}

// WriteField writes a KNN vector field.
func (w *Lucene99HnswVectorsWriter) WriteField(fieldInfo *index.FieldInfo, reader KnnVectorsReader) error {
	if w.closed {
		return fmt.Errorf("writer is closed")
	}

	// TODO: Implement actual HNSW graph construction and writing
	// For now, this is a placeholder that acknowledges the field was written

	return nil
}

// Finish finalizes the writing process.
func (w *Lucene99HnswVectorsWriter) Finish() error {
	if w.closed {
		return fmt.Errorf("writer is closed")
	}

	// TODO: Finalize HNSW graph construction and write footer

	return nil
}

// Close releases resources.
func (w *Lucene99HnswVectorsWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true

	var firstErr error
	if w.vectorDataOut != nil {
		if err := w.vectorDataOut.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if w.vectorIndexOut != nil {
		if err := w.vectorIndexOut.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}
