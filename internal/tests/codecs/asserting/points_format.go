// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package asserting

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/codecs/asserting"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// AssertingPointsFormat is a wrapper around a PointsFormat that adds additional assertions.
// Mirrors org.apache.lucene.tests.codecs.asserting.AssertingPointsFormat in Apache Lucene 10.5.0.
type AssertingPointsFormat struct {
	in spi.PointsFormat
}

// NewAssertingPointsFormat creates a new AssertingPointsFormat wrapping the given format.
func NewAssertingPointsFormat(in spi.PointsFormat) *AssertingPointsFormat {
	return &AssertingPointsFormat{in: in}
}

// NewAssertingPointsFormatDefault creates a new AssertingPointsFormat wrapping the default codec's points format.
func NewAssertingPointsFormatDefault() *AssertingPointsFormat {
	return NewAssertingPointsFormat(index.GetDefaultCodec().PointsFormat())
}

func (f *AssertingPointsFormat) Name() string {
	return "AssertingPointsFormat(" + f.in.Name() + ")"
}

func (f *AssertingPointsFormat) FieldsWriter(state *spi.SegmentWriteState) (spi.PointsWriter, error) {
	writer, err := f.in.FieldsWriter(state)
	if err != nil {
		return nil, err
	}
	return &assertingPointsWriter{
		in: writer,
	}, nil
}

func (f *AssertingPointsFormat) FieldsReader(state *spi.SegmentReadState) (spi.PointsReader, error) {
	reader, err := f.in.FieldsReader(state)
	if err != nil {
		return nil, err
	}
	return &assertingPointsReader{
		in:         reader,
		maxDoc:     state.SegmentInfo.MaxDoc,
		fieldInfos: state.FieldInfos,
		merging:    false,
	}, nil
}

type assertingPointsReader struct {
	in         spi.PointsReader
	maxDoc     int
	fieldInfos *spi.FieldInfos
	merging    bool
	creationThread interface{}
}

func (r *assertingPointsReader) Close() error {
	err := r.in.Close()
	_ = r.in.Close() // Close again to test idempotency
	return err
}

func (r *assertingPointsReader) CheckIntegrity() error {
	return r.in.CheckIntegrity()
}

// GetValues returns the point values for the given field, with additional assertions.
// This method is part of the wider codecs-side reader interface.
func (r *assertingPointsReader) GetValues(field string) (index.PointValues, error) {
	fi := r.fieldInfos.GetByName(field)
	if fi == nil || fi.PointDimensionCount() == 0 {
		panic(fmt.Sprintf("AssertingPointsReader: field %q must exist and have point dimensions", field))
	}
	if r.merging {
		assertingcodec.AssertThread("PointsReader", r.creationThread)
	}

	// Use type assertion to access the wide read surface (GetValues)
	wideReader, ok := r.in.(interface {
		GetValues(field string) (index.PointValues, error)
	})
	if !ok {
		panic("AssertingPointsReader: underlying PointsReader does not expose GetValues")
	}

	values, err := wideReader.GetValues(field)
	if err != nil {
		return nil, err
	}
	if values == nil {
		return nil, nil
	}

	return &index.AssertingPointValues{
		in:     values,
		maxDoc: r.maxDoc,
	}, nil
}

func (r *assertingPointsReader) GetMergeInstance() spi.PointsReader {
	// Use type assertion to access GetMergeInstance on the wide interface
	wideReader, ok := r.in.(interface {
		GetMergeInstance() spi.PointsReader
	})
	if !ok {
		panic("AssertingPointsReader: underlying PointsReader does not expose GetMergeInstance")
	}

	mergeIn := wideReader.GetMergeInstance()
	return &assertingPointsReader{
		in:         mergeIn,
		maxDoc:     r.maxDoc,
		fieldInfos: r.fieldInfos,
		merging:    true,
	}
}

type assertingPointsWriter struct {
	in spi.PointsWriter
}

func (w *assertingPointsWriter) WriteField(fieldInfo *spi.FieldInfo, reader spi.PointsReader) error {
	if fieldInfo.PointDimensionCount() == 0 {
		panic(fmt.Sprintf("AssertingPointsWriter: writing field %q but pointDimensionCount is 0", fieldInfo.Name()))
	}
	return w.in.WriteField(fieldInfo, reader)
}

func (w *assertingPointsWriter) Finish() error {
	return w.in.Finish()
}

func (w *assertingPointsWriter) Close() error {
	err := w.in.Close()
	_ = w.in.Close() // Close again to test idempotency
	return err
}
