// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package asserting

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// Status represents the state of a document being written.
type Status int

const (
	StatusUndefined Status = iota
	StatusStarted
	StatusFinished
)

// AssertingStoredFieldsFormat is a wrapper around a StoredFieldsFormat
// that adds additional assertions to verify correct usage.
type AssertingStoredFieldsFormat struct {
	in spi.StoredFieldsFormat
}

// NewAssertingStoredFieldsFormat creates a new AssertingStoredFieldsFormat.
func NewAssertingStoredFieldsFormat(in spi.StoredFieldsFormat) *AssertingStoredFieldsFormat {
	return &AssertingStoredFieldsFormat{in: in}
}

// FieldsReader returns an AssertingStoredFieldsReader.
func (f *AssertingStoredFieldsFormat) FieldsReader(dir store.Directory, si *index.SegmentInfo, fn *index.FieldInfos, context store.IOContext) (spi.StoredFieldsReader, error) {
	reader, err := f.in.FieldsReader(dir, si, fn, context)
	if err != nil {
		return nil, err
	}
	return NewAssertingStoredFieldsReader(reader, si.MaxDoc(), false), nil
}

// FieldsWriter returns an AssertingStoredFieldsWriter.
func (f *AssertingStoredFieldsFormat) FieldsWriter(dir store.Directory, si *index.SegmentInfo, context store.IOContext) (spi.StoredFieldsWriter, error) {
	writer, err := f.in.FieldsWriter(dir, si, context)
	if err != nil {
		return nil, err
	}
	return NewAssertingStoredFieldsWriter(writer), nil
}

// Name returns the name of the format.
func (f *AssertingStoredFieldsFormat) Name() string {
	return "Asserting(" + f.in.Name() + ")"
}

type AssertingStoredFieldsReader struct {
	in             spi.StoredFieldsReader
	maxDoc         int
	merging        bool
	creationThread interface{}
}

func NewAssertingStoredFieldsReader(in spi.StoredFieldsReader, maxDoc int, merging bool) *AssertingStoredFieldsReader {
	return &AssertingStoredFieldsReader{
		in:             in,
		maxDoc:         maxDoc,
		merging:        merging,
		creationThread: nil, // Goroutine ID not available in Go
	}
}

func (r *AssertingStoredFieldsReader) VisitDocument(docID int, visitor spi.StoredFieldVisitor) error {
	AssertThread("StoredFieldsReader", r.creationThread)
	AssertState(docID >= 0 && docID < r.maxDoc, fmt.Sprintf("docID %d out of range [0, %d)", docID, r.maxDoc))
	return r.in.VisitDocument(docID, visitor)
}

func (r *AssertingStoredFieldsReader) Close() error {
	err := r.in.Close()
	// Lucene tests that close() can be called multiple times.
	_ = r.in.Close()
	return err
}

func (r *AssertingStoredFieldsReader) Clone() (spi.StoredFieldsReader, error) {
	AssertState(!r.merging, "Merge instances do not support cloning")
	clonedIn, err := r.in.Clone()
	if err != nil {
		return nil, err
	}
	return NewAssertingStoredFieldsReader(clonedIn, r.maxDoc, false), nil
}

func (r *AssertingStoredFieldsReader) GetMergeInstance() (spi.StoredFieldsReader, error) {
	mergeIn, err := r.in.GetMergeInstance()
	if err != nil {
		return nil, err
	}
	return NewAssertingStoredFieldsReader(mergeIn, r.maxDoc, true), nil
}

type AssertingStoredFieldsWriter struct {
	in         spi.StoredFieldsWriter
	numWritten int
	docStatus  Status
}

func NewAssertingStoredFieldsWriter(in spi.StoredFieldsWriter) *AssertingStoredFieldsWriter {
	return &AssertingStoredFieldsWriter{
		in:        in,
		docStatus: StatusUndefined,
	}
}

func (w *AssertingStoredFieldsWriter) StartDocument() error {
	AssertState(w.docStatus != StatusStarted, "StartDocument called while document already started")
	err := w.in.StartDocument()
	if err == nil {
		w.numWritten++
		w.docStatus = StatusStarted
	}
	return err
}

func (w *AssertingStoredFieldsWriter) FinishDocument() error {
	AssertState(w.docStatus == StatusStarted, "FinishDocument called without starting a document")
	err := w.in.FinishDocument()
	if err == nil {
		w.docStatus = StatusFinished
	}
	return err
}

func (w *AssertingStoredFieldsWriter) WriteField(field spi.IndexableField) error {
	AssertState(w.docStatus == StatusStarted, "WriteField called outside of a started document")
	return w.in.WriteField(field)
}

func (w *AssertingStoredFieldsWriter) Finish(numDocs int) error {
	expectedStatus := StatusUndefined
	if numDocs > 0 {
		expectedStatus = StatusFinished
	}
	AssertState(w.docStatus == expectedStatus, fmt.Sprintf("Finish called with status %v, expected %v for numDocs %d", w.docStatus, expectedStatus, numDocs))
	err := w.in.Finish(numDocs)
	if err == nil {
		AssertState(numDocs == w.numWritten, fmt.Sprintf("numDocs %d does not match number of documents started %d", numDocs, w.numWritten))
	}
	return err
}

func (w *AssertingStoredFieldsWriter) Close() error {
	err := w.in.Close()
	// Lucene tests that close() can be called multiple times.
	_ = w.in.Close()
	return err
}
