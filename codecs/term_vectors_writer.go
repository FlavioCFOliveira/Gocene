// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/index"
)

// BaseTermVectorsWriter provides a base implementation of TermVectorsWriter.
// This can be embedded in custom TermVectorsWriter implementations to get
// default implementations for common methods.
type BaseTermVectorsWriter struct {
	mu          sync.Mutex
	closed      bool
	state       *SegmentWriteState
	currentDoc  int
	currentField string
	currentTerm []byte
}

// NewBaseTermVectorsWriter creates a new BaseTermVectorsWriter.
func NewBaseTermVectorsWriter(state *SegmentWriteState) *BaseTermVectorsWriter {
	return &BaseTermVectorsWriter{
		state:      state,
		currentDoc: -1,
	}
}

// GetState returns the segment write state.
func (w *BaseTermVectorsWriter) GetState() *SegmentWriteState {
	return w.state
}

// GetCurrentDoc returns the current document number.
func (w *BaseTermVectorsWriter) GetCurrentDoc() int {
	return w.currentDoc
}

// GetCurrentField returns the current field name.
func (w *BaseTermVectorsWriter) GetCurrentField() string {
	return w.currentField
}

// GetCurrentTerm returns the current term bytes.
func (w *BaseTermVectorsWriter) GetCurrentTerm() []byte {
	return w.currentTerm
}

// IsClosed returns true if this writer has been closed.
func (w *BaseTermVectorsWriter) IsClosed() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.closed
}

// StartDocument starts writing term vectors for a document.
// This implements the TermVectorsWriter interface.
func (w *BaseTermVectorsWriter) StartDocument(numFields int) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("TermVectorsWriter is closed")
	}

	w.currentDoc++
	return nil
}

// StartField starts writing a term vector for a field.
// This must be implemented by subclasses.
func (w *BaseTermVectorsWriter) StartField(fieldInfo *index.FieldInfo, numTerms int, hasPositions, hasOffsets, hasPayloads bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("TermVectorsWriter is closed")
	}

	w.currentField = fieldInfo.Name()
	return nil
}

// StartTerm starts a new term in the current field.
// This must be implemented by subclasses.
func (w *BaseTermVectorsWriter) StartTerm(term []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("TermVectorsWriter is closed")
	}

	w.currentTerm = term
	return nil
}

// AddPosition adds a position for the current term.
// This must be implemented by subclasses.
func (w *BaseTermVectorsWriter) AddPosition(position int, startOffset, endOffset int, payload []byte) error {
	return nil
}

// FinishTerm finishes the current term.
// This must be implemented by subclasses.
func (w *BaseTermVectorsWriter) FinishTerm() error {
	return nil
}

// FinishField finishes the current field.
// This must be implemented by subclasses.
func (w *BaseTermVectorsWriter) FinishField() error {
	return nil
}

// FinishDocument finishes the current document.
// This must be implemented by subclasses.
func (w *BaseTermVectorsWriter) FinishDocument() error {
	return nil
}

// Close releases resources.
// This implements the TermVectorsWriter interface.
func (w *BaseTermVectorsWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}

	w.closed = true
	return nil
}

// TermVectorsWriterImpl is a concrete implementation of TermVectorsWriter
// that writes term vectors to memory.
type TermVectorsWriterImpl struct {
	*BaseTermVectorsWriter
	docs        []TermVectorDocumentWriter
	currentDocW *TermVectorDocumentWriter
	currentFieldW *TermVectorFieldWriter
	currentTermW  *TermVectorTermWriter
}

// TermVectorDocumentWriter represents a document being written.
type TermVectorDocumentWriter struct {
	NumFields int
	Fields    []TermVectorFieldWriter
}

// TermVectorFieldWriter represents a field being written.
type TermVectorFieldWriter struct {
	Name         string
	NumTerms     int
	HasPositions bool
	HasOffsets   bool
	HasPayloads  bool
	Terms        []TermVectorTermWriter
}

// TermVectorTermWriter represents a term being written.
type TermVectorTermWriter struct {
	Term      []byte
	Positions []TermVectorPositionWriter
}

// TermVectorPositionWriter represents a position being written.
type TermVectorPositionWriter struct {
	Position    int
	StartOffset int
	EndOffset   int
	Payload     []byte
}

// NewTermVectorsWriterImpl creates a new TermVectorsWriterImpl.
func NewTermVectorsWriterImpl(state *SegmentWriteState) *TermVectorsWriterImpl {
	return &TermVectorsWriterImpl{
		BaseTermVectorsWriter: NewBaseTermVectorsWriter(state),
		docs:                  make([]TermVectorDocumentWriter, 0),
	}
}

// StartDocument starts writing term vectors for a document.
func (w *TermVectorsWriterImpl) StartDocument(numFields int) error {
	if err := w.BaseTermVectorsWriter.StartDocument(numFields); err != nil {
		return err
	}

	w.currentDocW = &TermVectorDocumentWriter{
		NumFields: numFields,
		Fields:    make([]TermVectorFieldWriter, 0),
	}
	return nil
}

// StartField starts writing a term vector for a field.
func (w *TermVectorsWriterImpl) StartField(fieldInfo *index.FieldInfo, numTerms int, hasPositions, hasOffsets, hasPayloads bool) error {
	if err := w.BaseTermVectorsWriter.StartField(fieldInfo, numTerms, hasPositions, hasOffsets, hasPayloads); err != nil {
		return err
	}

	w.currentFieldW = &TermVectorFieldWriter{
		Name:         fieldInfo.Name(),
		NumTerms:     numTerms,
		HasPositions: hasPositions,
		HasOffsets:   hasOffsets,
		HasPayloads:  hasPayloads,
		Terms:        make([]TermVectorTermWriter, 0),
	}
	return nil
}

// StartTerm starts a new term in the current field.
func (w *TermVectorsWriterImpl) StartTerm(term []byte) error {
	if err := w.BaseTermVectorsWriter.StartTerm(term); err != nil {
		return err
	}

	w.currentTermW = &TermVectorTermWriter{
		Term:      term,
		Positions: make([]TermVectorPositionWriter, 0),
	}
	return nil
}

// AddPosition adds a position for the current term.
func (w *TermVectorsWriterImpl) AddPosition(position int, startOffset, endOffset int, payload []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("TermVectorsWriter is closed")
	}

	if w.currentTermW == nil {
		return fmt.Errorf("no term started")
	}

	pos := TermVectorPositionWriter{
		Position:    position,
		StartOffset: startOffset,
		EndOffset:   endOffset,
		Payload:     payload,
	}
	w.currentTermW.Positions = append(w.currentTermW.Positions, pos)
	return nil
}

// FinishTerm finishes the current term.
func (w *TermVectorsWriterImpl) FinishTerm() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("TermVectorsWriter is closed")
	}

	if w.currentTermW == nil {
		return fmt.Errorf("no term started")
	}

	if w.currentFieldW == nil {
		return fmt.Errorf("no field started")
	}

	w.currentFieldW.Terms = append(w.currentFieldW.Terms, *w.currentTermW)
	w.currentTermW = nil
	return nil
}

// FinishField finishes the current field.
func (w *TermVectorsWriterImpl) FinishField() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("TermVectorsWriter is closed")
	}

	if w.currentFieldW == nil {
		return fmt.Errorf("no field started")
	}

	if w.currentDocW == nil {
		return fmt.Errorf("no document started")
	}

	w.currentDocW.Fields = append(w.currentDocW.Fields, *w.currentFieldW)
	w.currentFieldW = nil
	return nil
}

// FinishDocument finishes the current document.
func (w *TermVectorsWriterImpl) FinishDocument() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("TermVectorsWriter is closed")
	}

	if w.currentDocW == nil {
		return fmt.Errorf("no document started")
	}

	w.docs = append(w.docs, *w.currentDocW)
	w.currentDocW = nil
	return nil
}

// GetDocuments returns the documents that have been written.
func (w *TermVectorsWriterImpl) GetDocuments() []TermVectorDocumentWriter {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.docs
}

// NoOpTermVectorsWriter is a TermVectorsWriter that does nothing.
// This is useful for testing or when term vectors are not needed.
type NoOpTermVectorsWriter struct {
	*BaseTermVectorsWriter
}

// NewNoOpTermVectorsWriter creates a new NoOpTermVectorsWriter.
func NewNoOpTermVectorsWriter(state *SegmentWriteState) *NoOpTermVectorsWriter {
	return &NoOpTermVectorsWriter{
		BaseTermVectorsWriter: NewBaseTermVectorsWriter(state),
	}
}

// StartDocument does nothing.
func (w *NoOpTermVectorsWriter) StartDocument(numFields int) error {
	return nil
}

// StartField does nothing.
func (w *NoOpTermVectorsWriter) StartField(fieldInfo *index.FieldInfo, numTerms int, hasPositions, hasOffsets, hasPayloads bool) error {
	return nil
}

// StartTerm does nothing.
func (w *NoOpTermVectorsWriter) StartTerm(term []byte) error {
	return nil
}

// AddPosition does nothing.
func (w *NoOpTermVectorsWriter) AddPosition(position int, startOffset, endOffset int, payload []byte) error {
	return nil
}

// FinishTerm does nothing.
func (w *NoOpTermVectorsWriter) FinishTerm() error {
	return nil
}

// FinishField does nothing.
func (w *NoOpTermVectorsWriter) FinishField() error {
	return nil
}

// FinishDocument does nothing.
func (w *NoOpTermVectorsWriter) FinishDocument() error {
	return nil
}

// Close does nothing.
func (w *NoOpTermVectorsWriter) Close() error {
	return nil
}

// Ensure implementations satisfy the interface
var _ TermVectorsWriter = (*BaseTermVectorsWriter)(nil)
var _ TermVectorsWriter = (*TermVectorsWriterImpl)(nil)
var _ TermVectorsWriter = (*NoOpTermVectorsWriter)(nil)
