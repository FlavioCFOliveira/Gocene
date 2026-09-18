// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package cranky

import (
	"fmt"
	"math/rand"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// CrankyTermVectorsFormat is a TermVectorsFormat that randomly throws IOExceptions
// to test the robustness of the indexing chain.
type CrankyTermVectorsFormat struct {
	delegate spi.TermVectorsFormat
	random   *rand.Rand
}

// NewCrankyTermVectorsFormat creates a new CrankyTermVectorsFormat.
func NewCrankyTermVectorsFormat(delegate spi.TermVectorsFormat, random *rand.Rand) *CrankyTermVectorsFormat {
	return &CrankyTermVectorsFormat{
		delegate: delegate,
		random:   random,
	}
}

// Name returns the format name.
func (f *CrankyTermVectorsFormat) Name() string {
	return f.delegate.Name()
}

// VectorsReader returns a vectors reader.
func (f *CrankyTermVectorsFormat) VectorsReader(dir store.Directory, segmentInfo *index.SegmentInfo, fieldInfos *index.FieldInfos, context store.IOContext) (spi.TermVectorsReader, error) {
	return f.delegate.VectorsReader(dir, segmentInfo, fieldInfos, context)
}

// VectorsWriter returns a vectors writer.
func (f *CrankyTermVectorsFormat) VectorsWriter(dir store.Directory, segmentInfo *index.SegmentInfo, context store.IOContext) (spi.TermVectorsWriter, error) {
	if f.random.Intn(100) == 0 {
		return nil, fmt.Errorf("Fake IOException from TermVectorsFormat.vectorsWriter()")
	}
	writer, err := f.delegate.VectorsWriter(dir, segmentInfo, context)
	if err != nil {
		return nil, err
	}
	return &crankyTermVectorsWriter{
		delegate: writer,
		random:   f.random,
	}, nil
}

type crankyTermVectorsWriter struct {
	delegate spi.TermVectorsWriter
	random   *rand.Rand
}

func (w *crankyTermVectorsWriter) StartDocument(numVectorFields int) error {
	if w.random.Intn(10000) == 0 {
		return fmt.Errorf("Fake IOException from TermVectorsWriter.startDocument()")
	}
	return w.delegate.StartDocument(numVectorFields)
}

func (w *crankyTermVectorsWriter) FinishDocument() error {
	if w.random.Intn(10000) == 0 {
		return fmt.Errorf("Fake IOException from TermVectorsWriter.finishDocument()")
	}
	return w.delegate.FinishDocument()
}

func (w *crankyTermVectorsWriter) StartField(info *index.FieldInfo, numTerms int, positions, offsets, payloads bool) error {
	if w.random.Intn(10000) == 0 {
		return fmt.Errorf("Fake IOException from TermVectorsWriter.startField()")
	}
	return w.delegate.StartField(info, numTerms, positions, offsets, payloads)
}

func (w *crankyTermVectorsWriter) FinishField() error {
	if w.random.Intn(10000) == 0 {
		return fmt.Errorf("Fake IOException from TermVectorsWriter.finishField()")
	}
	return w.delegate.FinishField()
}

func (w *crankyTermVectorsWriter) StartTerm(term []byte, freq int) error {
	if w.random.Intn(10000) == 0 {
		return fmt.Errorf("Fake IOException from TermVectorsWriter.startTerm()")
	}
	return w.delegate.StartTerm(term, freq)
}

func (w *crankyTermVectorsWriter) FinishTerm() error {
	if w.random.Intn(10000) == 0 {
		return fmt.Errorf("Fake IOException from TermVectorsWriter.finishTerm()")
	}
	return w.delegate.FinishTerm()
}

func (w *crankyTermVectorsWriter) AddPosition(position, startOffset, endOffset int, payload []byte) error {
	if w.random.Intn(10000) == 0 {
		return fmt.Errorf("Fake IOException from TermVectorsWriter.addPosition()")
	}
	return w.delegate.AddPosition(position, startOffset, endOffset, payload)
}

func (w *crankyTermVectorsWriter) Finish(numDocs int) error {
	if w.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException from TermVectorsWriter.finish()")
	}
	return w.delegate.Finish(numDocs)
}

func (w *crankyTermVectorsWriter) Close() error {
	err := w.delegate.Close()
	if w.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException from TermVectorsWriter.close()")
	}
	return err
}
