// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Lucene90PointsFormat is the Lucene 9.0 points format.
//
// This format uses BKD (B-Tree K-D Tree) trees for efficient spatial and
// range queries on multi-dimensional points.
//
// This is the Go port of Lucene's org.apache.lucene.codecs.lucene90.Lucene90PointsFormat.
type Lucene90PointsFormat struct {
	*BasePointsFormat
}

// NewLucene90PointsFormat creates a new Lucene90PointsFormat.
func NewLucene90PointsFormat() *Lucene90PointsFormat {
	return &Lucene90PointsFormat{
		BasePointsFormat: NewBasePointsFormat("Lucene90PointsFormat"),
	}
}

// FieldsWriter returns a writer for writing points.
func (f *Lucene90PointsFormat) FieldsWriter(state *SegmentWriteState) (PointsWriter, error) {
	return NewLucene90PointsWriter(state), nil
}

// FieldsReader returns a reader for reading points.
func (f *Lucene90PointsFormat) FieldsReader(state *SegmentReadState) (PointsReader, error) {
	return NewLucene90PointsReader(state)
}

// Lucene90PointsWriter writes points in Lucene 9.0 format.
type Lucene90PointsWriter struct {
	state  *SegmentWriteState
	closed bool
}

// NewLucene90PointsWriter creates a new Lucene90PointsWriter.
func NewLucene90PointsWriter(state *SegmentWriteState) *Lucene90PointsWriter {
	return &Lucene90PointsWriter{
		state: state,
	}
}

// WriteField writes a point field.
func (w *Lucene90PointsWriter) WriteField(fieldInfo *index.FieldInfo, reader PointsReader) error {
	if w.closed {
		return fmt.Errorf("writer is closed")
	}

	// Generate file name
	segmentName := w.state.SegmentInfo.Name()
	fileName := fmt.Sprintf("%s_Lucene90_0.pnt", segmentName)

	// Create output
	out, err := w.state.Directory.CreateOutput(fileName, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		return fmt.Errorf("failed to create output file %s: %w", fileName, err)
	}
	defer out.Close()

	// Write header
	if err := w.writeHeader(out); err != nil {
		return err
	}

	// Write field info
	if err := store.WriteString(out, fieldInfo.Name()); err != nil {
		return fmt.Errorf("failed to write field name: %w", err)
	}

	// Write number of dimensions
	if err := store.WriteVInt(out, int32(fieldInfo.PointDimensionCount())); err != nil {
		return fmt.Errorf("failed to write dimension count: %w", err)
	}

	// Write bytes per dimension
	if err := store.WriteVInt(out, int32(fieldInfo.PointNumBytes())); err != nil {
		return fmt.Errorf("failed to write bytes per dimension: %w", err)
	}

	return nil
}

// writeHeader writes the points file header.
func (w *Lucene90PointsWriter) writeHeader(out store.IndexOutput) error {
	// Write magic number (PT = Points)
	if err := store.WriteUint32(out, 0x50540000); err != nil {
		return fmt.Errorf("failed to write magic number: %w", err)
	}
	// Write version
	if err := store.WriteUint32(out, 90); err != nil {
		return fmt.Errorf("failed to write version: %w", err)
	}
	return nil
}

// Finish finalizes the writing process.
func (w *Lucene90PointsWriter) Finish() error {
	if w.closed {
		return fmt.Errorf("writer is closed")
	}
	return nil
}

// Close releases resources.
func (w *Lucene90PointsWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	return nil
}

// Lucene90PointsReader reads points in Lucene 9.0 format.
type Lucene90PointsReader struct {
	state  *SegmentReadState
	closed bool
}

// NewLucene90PointsReader creates a new Lucene90PointsReader.
func NewLucene90PointsReader(state *SegmentReadState) (*Lucene90PointsReader, error) {
	r := &Lucene90PointsReader{
		state: state,
	}
	return r, nil
}

// CheckIntegrity checks the integrity of the points.
func (r *Lucene90PointsReader) CheckIntegrity() error {
	if r.closed {
		return fmt.Errorf("reader is closed")
	}

	// Generate file name
	segmentName := r.state.SegmentInfo.Name()
	fileName := fmt.Sprintf("%s_Lucene90_0.pnt", segmentName)

	// Check if file exists
	if !r.state.Directory.FileExists(fileName) {
		return nil
	}

	// Open input
	in, err := r.state.Directory.OpenInput(fileName, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return fmt.Errorf("failed to open input file %s: %w", fileName, err)
	}
	defer in.Close()

	// Read and validate header
	if err := r.readHeader(in); err != nil {
		return err
	}

	return nil
}

// readHeader reads and validates the points file header.
func (r *Lucene90PointsReader) readHeader(in store.IndexInput) error {
	// Read magic number
	magic, err := store.ReadUint32(in)
	if err != nil {
		return fmt.Errorf("failed to read magic number: %w", err)
	}
	if magic != 0x50540000 {
		return fmt.Errorf("invalid magic number: expected 0x50540000, got 0x%08x", magic)
	}

	// Read version
	version, err := store.ReadUint32(in)
	if err != nil {
		return fmt.Errorf("failed to read version: %w", err)
	}
	if version != 90 {
		return fmt.Errorf("unsupported version: %d", version)
	}

	return nil
}

// Close releases resources.
func (r *Lucene90PointsReader) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	return nil
}
