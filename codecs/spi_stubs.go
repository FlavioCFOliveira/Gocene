// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import "fmt"

// readOnlyPostingsFormatStub is a named PostingsFormat placeholder registered
// for backward-compatibility codec names whose full port is deferred. It
// rejects writes (ErrWriteNotSupported) and returns a descriptive not-yet-
// implemented error on reads.
//
// It is used by backward_codecs sub-packages to ensure PostingsFormatByName
// resolves the historic format name even before the deep-port sprint lands.
type readOnlyPostingsFormatStub struct {
	*BasePostingsFormat
}

// NewReadOnlyPostingsFormat returns a read-only PostingsFormat stub registered
// under name. Write attempts return ErrReadOnlyFormat; read attempts return a
// not-yet-implemented error.
func NewReadOnlyPostingsFormat(name string) PostingsFormat {
	return &readOnlyPostingsFormatStub{
		BasePostingsFormat: NewBasePostingsFormat(name),
	}
}

// FieldsConsumer returns ErrReadOnlyFormat; this stub is write-prohibited.
func (f *readOnlyPostingsFormatStub) FieldsConsumer(_ *SegmentWriteState) (FieldsConsumer, error) {
	return nil, fmt.Errorf("%s: %w", f.Name(), ErrReadOnlyFormat)
}

// FieldsProducer returns a not-yet-implemented error; the deep-port sprint has
// not landed for this backward format.
func (f *readOnlyPostingsFormatStub) FieldsProducer(_ *SegmentReadState) (FieldsProducer, error) {
	return nil, fmt.Errorf("%s: FieldsProducer not yet implemented (backward format deferred sprint)", f.Name())
}

// ErrReadOnlyFormat is returned when a write operation is attempted on a
// backward-compatibility format that is strictly read-only.
var ErrReadOnlyFormat = fmt.Errorf("old codecs may only be used for reading")

// readOnlyDocValuesFormatStub is the DocValuesFormat equivalent of
// readOnlyPostingsFormatStub.
type readOnlyDocValuesFormatStub struct {
	*BaseDocValuesFormat
}

// NewReadOnlyDocValuesFormat returns a read-only DocValuesFormat stub registered
// under name.
func NewReadOnlyDocValuesFormat(name string) DocValuesFormat {
	return &readOnlyDocValuesFormatStub{
		BaseDocValuesFormat: NewBaseDocValuesFormat(name),
	}
}

// FieldsConsumer returns ErrReadOnlyFormat.
func (f *readOnlyDocValuesFormatStub) FieldsConsumer(_ *SegmentWriteState) (DocValuesConsumer, error) {
	return nil, fmt.Errorf("%s: %w", f.Name(), ErrReadOnlyFormat)
}

// FieldsProducer returns a not-yet-implemented error.
func (f *readOnlyDocValuesFormatStub) FieldsProducer(_ *SegmentReadState) (DocValuesProducer, error) {
	return nil, fmt.Errorf("%s: FieldsProducer not yet implemented (backward format deferred sprint)", f.Name())
}

// readOnlyKnnVectorsFormatStub is the KnnVectorsFormat equivalent of
// readOnlyPostingsFormatStub.
type readOnlyKnnVectorsFormatStub struct {
	*BaseKnnVectorsFormat
}

// NewReadOnlyKnnVectorsFormat returns a read-only KnnVectorsFormat stub
// registered under name.
func NewReadOnlyKnnVectorsFormat(name string) KnnVectorsFormat {
	return &readOnlyKnnVectorsFormatStub{
		BaseKnnVectorsFormat: NewBaseKnnVectorsFormat(name),
	}
}

// FieldsWriter returns ErrReadOnlyFormat.
func (f *readOnlyKnnVectorsFormatStub) FieldsWriter(_ *SegmentWriteState) (KnnVectorsWriter, error) {
	return nil, fmt.Errorf("%s: %w", f.Name(), ErrReadOnlyFormat)
}

// FieldsReader returns a not-yet-implemented error.
func (f *readOnlyKnnVectorsFormatStub) FieldsReader(_ *SegmentReadState) (KnnVectorsReader, error) {
	return nil, fmt.Errorf("%s: FieldsReader not yet implemented (backward format deferred sprint)", f.Name())
}
