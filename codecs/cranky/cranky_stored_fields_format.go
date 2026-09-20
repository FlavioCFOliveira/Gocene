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

// CrankyStoredFieldsFormat is a StoredFieldsFormat that randomly throws IOExceptions
// to test the robustness of the indexing chain.
type CrankyStoredFieldsFormat struct {
	delegate spi.StoredFieldsFormat
	random   *rand.Rand
}

// NewCrankyStoredFieldsFormat creates a new CrankyStoredFieldsFormat.
func NewCrankyStoredFieldsFormat(delegate spi.StoredFieldsFormat, random *rand.Rand) *CrankyStoredFieldsFormat {
	return &CrankyStoredFieldsFormat{
		delegate: delegate,
		random:   random,
	}
}

// Name returns the format name.
func (f *CrankyStoredFieldsFormat) Name() string {
	return f.delegate.Name()
}

// FieldsReader returns a fields reader.
func (f *CrankyStoredFieldsFormat) FieldsReader(dir store.Directory, segmentInfo *index.SegmentInfo, fieldInfos *index.FieldInfos, context store.IOContext) (spi.StoredFieldsReader, error) {
	return f.delegate.FieldsReader(dir, segmentInfo, fieldInfos, context)
}

// FieldsWriter returns a fields writer.
func (f *CrankyStoredFieldsFormat) FieldsWriter(dir store.Directory, segmentInfo *index.SegmentInfo, context store.IOContext) (spi.StoredFieldsWriter, error) {
	if f.random.Intn(100) == 0 {
		return nil, fmt.Errorf("Fake IOException from StoredFieldsFormat.fieldsWriter()")
	}
	writer, err := f.delegate.FieldsWriter(dir, segmentInfo, context)
	if err != nil {
		return nil, err
	}
	return &crankyStoredFieldsWriter{
		delegate: writer,
		random:   f.random,
	}, nil
}

type crankyStoredFieldsWriter struct {
	delegate spi.StoredFieldsWriter
	random   *rand.Rand
}

func (w *crankyStoredFieldsWriter) StartDocument() error {
	if w.random.Intn(10000) == 0 {
		return fmt.Errorf("Fake IOException from StoredFieldsWriter.startDocument()")
	}
	return w.delegate.StartDocument()
}

func (w *crankyStoredFieldsWriter) FinishDocument() error {
	if w.random.Intn(10000) == 0 {
		return fmt.Errorf("Fake IOException from StoredFieldsWriter.finishDocument()")
	}
	return w.delegate.FinishDocument()
}

func (w *crankyStoredFieldsWriter) WriteField(info *spi.FieldInfo, field spi.IndexableField) error {
	if w.random.Intn(10000) == 0 {
		return fmt.Errorf("Fake IOException from StoredFieldsWriter.writeField()")
	}
	return w.delegate.WriteField(info, field)
}

func (w *crankyStoredFieldsWriter) Finish(numDocs int) error {
	if w.random.Intn(100) == 0 {
		return fmt.Errorf("Fake IOException from StoredFieldsWriter.finish()")
	}
	return w.delegate.Finish(numDocs)
}

func (w *crankyStoredFieldsWriter) Close() error {
	err := w.delegate.Close()
	if w.random.Intn(1000) == 0 {
		return fmt.Errorf("Fake IOException from StoredFieldsWriter.close()")
	}
	return err
}
