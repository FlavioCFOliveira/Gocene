// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// AssertingTermVectorsFormat is just like the default vectors format but with additional asserts.
//
// This is the Go port of Lucene's org.apache.lucene.tests.codecs.asserting.AssertingTermVectorsFormat.
type AssertingTermVectorsFormat struct {
	in spi.TermVectorsFormat
}

// NewAssertingTermVectorsFormat creates a new AssertingTermVectorsFormat wrapping the given format.
func NewAssertingTermVectorsFormat(in spi.TermVectorsFormat) *AssertingTermVectorsFormat {
	return &AssertingTermVectorsFormat{in: in}
}

// VectorsReader opens a reader that produces the per-segment term-vector files.
func (f *AssertingTermVectorsFormat) VectorsReader(dir store.Directory, segmentInfo *schema.SegmentInfo, fieldInfos *schema.FieldInfos, context store.IOContext) (spi.TermVectorsReader, error) {
	reader, err := f.in.VectorsReader(dir, segmentInfo, fieldInfos, context)
	if err != nil {
		return nil, err
	}
	return &AssertingTermVectorsReader{in: reader}, nil
}

// VectorsWriter opens a writer that produces the per-segment term-vector files.
func (f *AssertingTermVectorsFormat) VectorsWriter(state *spi.SegmentWriteState) (spi.TermVectorsWriter, error) {
	writer, err := f.in.VectorsWriter(state)
	if err != nil {
		return nil, err
	}
	return &AssertingTermVectorsWriter{in: writer}, nil
}

// Name returns the codec name.
func (f *AssertingTermVectorsFormat) Name() string {
	return f.in.Name()
}

// AssertingTermVectorsReader wraps a TermVectorsReader with additional checks.
type AssertingTermVectorsReader struct {
	in spi.TermVectorsReader
}

func (r *AssertingTermVectorsReader) Get(docID int) (schema.Fields, error) {
	fields, err := r.in.Get(docID)
	if err != nil {
		return nil, err
	}
	if fields == nil {
		return nil, nil
	}
	return &index.AssertingFields{In: fields}, nil
}

func (r *AssertingTermVectorsReader) GetField(docID int, field string) (schema.Terms, error) {
	// Lucene's AssertingTermVectorsReader doesn't override getField, it uses the base.
	return r.in.GetField(docID, field)
}

func (r *AssertingTermVectorsReader) Close() error {
	err := r.in.Close()
	_ = r.in.Close() // close again to test double-close
	return err
}

type writerStatus int

const (
	statusUndefined writerStatus = iota
	statusStarted
	statusFinished
)

// AssertingTermVectorsWriter wraps a TermVectorsWriter with state tracking and assertions.
type AssertingTermVectorsWriter struct {
	in spi.TermVectorsWriter

	docStatus    writerStatus
	fieldStatus  writerStatus
	termStatus   writerStatus
	fieldCount   int
	docCount     int
	termCount    int
	hasPositions bool
}

func (w *AssertingTermVectorsWriter) StartDocument(numFields int) error {
	if w.fieldCount != 0 {
		panic("AssertingTermVectorsWriter: fieldCount != 0 at startDocument")
	}
	if w.docStatus == statusStarted {
		panic("AssertingTermVectorsWriter: docStatus is already STARTED at startDocument")
	}

	if err := w.in.StartDocument(numFields); err != nil {
		return err
	}

	w.docStatus = statusStarted
	w.fieldCount = numFields
	w.docCount++
	return nil
}

func (w *AssertingTermVectorsWriter) StartField(fieldInfo *schema.FieldInfo, numTerms int, hasPositions, hasOffsets, hasPayloads bool) error {
	if w.termCount != 0 {
		panic("AssertingTermVectorsWriter: termCount != 0 at startField")
	}
	if w.docStatus != statusStarted {
		panic(fmt.Sprintf("AssertingTermVectorsWriter: docStatus is %v, expected STARTED at startField", w.docStatus))
	}
	if w.fieldStatus == statusStarted {
		panic("AssertingTermVectorsWriter: fieldStatus is already STARTED at startField")
	}

	if err := w.in.StartField(fieldInfo, numTerms, hasPositions, hasOffsets, hasPayloads); err != nil {
		return err
	}

	w.fieldStatus = statusStarted
	w.termCount = numTerms
	w.hasPositions = hasPositions || hasOffsets || hasPayloads
	return nil
}

func (w *AssertingTermVectorsWriter) StartTerm(term []byte) error {
	if w.docStatus != statusStarted {
		panic(fmt.Sprintf("AssertingTermVectorsWriter: docStatus is %v, expected STARTED at startTerm", w.docStatus))
	}
	if w.fieldStatus != statusStarted {
		panic(fmt.Sprintf("AssertingTermVectorsWriter: fieldStatus is %v, expected STARTED at startTerm", w.fieldStatus))
	}
	if w.termStatus == statusStarted {
		panic("AssertingTermVectorsWriter: termStatus is already STARTED at startTerm")
	}

	if err := w.in.StartTerm(term); err != nil {
		return err
	}

	w.termStatus = statusStarted
	return nil
}

func (w *AssertingTermVectorsWriter) AddPosition(position int, startOffset, endOffset int, payload []byte) error {
	if w.docStatus != statusStarted {
		panic(fmt.Sprintf("AssertingTermVectorsWriter: docStatus is %v, expected STARTED at addPosition", w.docStatus))
	}
	if w.fieldStatus != statusStarted {
		panic(fmt.Sprintf("AssertingTermVectorsWriter: fieldStatus is %v, expected STARTED at addPosition", w.fieldStatus))
	}
	if w.termStatus != statusStarted {
		panic(fmt.Sprintf("AssertingTermVectorsWriter: termStatus is %v, expected STARTED at addPosition", w.termStatus))
	}

	return w.in.AddPosition(position, startOffset, endOffset, payload)
}

func (w *AssertingTermVectorsWriter) FinishTerm() error {
	if w.docStatus != statusStarted {
		panic(fmt.Sprintf("AssertingTermVectorsWriter: docStatus is %v, expected STARTED at finishTerm", w.docStatus))
	}
	if w.fieldStatus != statusStarted {
		panic(fmt.Sprintf("AssertingTermVectorsWriter: fieldStatus is %v, expected STARTED at finishTerm", w.fieldStatus))
	}
	if w.termStatus != statusStarted {
		panic("AssertingTermVectorsWriter: termStatus is not STARTED at finishTerm")
	}

	if err := w.in.FinishTerm(); err != nil {
		return err
	}

	w.termStatus = statusFinished
	w.termCount--
	return nil
}

func (w *AssertingTermVectorsWriter) FinishField() error {
	if w.termCount != 0 {
		panic(fmt.Sprintf("AssertingTermVectorsWriter: termCount (%d) != 0 at finishField", w.termCount))
	}
	if w.fieldStatus != statusStarted {
		panic(fmt.Sprintf("AssertingTermVectorsWriter: fieldStatus is %v, expected STARTED at finishField", w.fieldStatus))
	}

	if err := w.in.FinishField(); err != nil {
		return err
	}

	w.fieldStatus = statusFinished
	w.fieldCount--
	return nil
}

func (w *AssertingTermVectorsWriter) FinishDocument() error {
	if w.fieldCount != 0 {
		panic(fmt.Sprintf("AssertingTermVectorsWriter: fieldCount (%d) != 0 at finishDocument", w.fieldCount))
	}
	if w.docStatus != statusStarted {
		panic(fmt.Sprintf("AssertingTermVectorsWriter: docStatus is %v, expected STARTED at finishDocument", w.docStatus))
	}

	if err := w.in.FinishDocument(); err != nil {
		return err
	}

	w.docStatus = statusFinished
	return nil
}

func (w *AssertingTermVectorsWriter) Close() error {
	err := w.in.Close()
	_ = w.in.Close() // close again to test double-close
	return err
}
