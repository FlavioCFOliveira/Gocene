// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"math"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Codec envelope constants for Lucene104StoredFieldsWriter.
// NOTE (DEVIATION): Lucene 10.4.0 uses "Lucene90StoredFieldsFastData" with
// the LZ4-compressed block format. Gocene uses a simpler in-memory format
// under a distinct codec name so that these files can be packed into CFS via
// copyFileBody (which requires the standard 16-byte CodecUtil footer).
const (
	lucene104SFDataCodec   = "Gocene104StoredFieldsData"
	lucene104SFDataVersion = int32(0)
)

// StoredFieldsFormat is an alias of spi.StoredFieldsFormat.
type StoredFieldsFormat = spi.StoredFieldsFormat

// BaseStoredFieldsFormat provides common functionality.
type BaseStoredFieldsFormat struct {
	name string
}

// NewBaseStoredFieldsFormat creates a new BaseStoredFieldsFormat.
func NewBaseStoredFieldsFormat(name string) *BaseStoredFieldsFormat {
	return &BaseStoredFieldsFormat{name: name}
}

// Name returns the format name.
func (f *BaseStoredFieldsFormat) Name() string {
	return f.name
}

// FieldsReader returns a fields reader (must be implemented by subclasses).
func (f *BaseStoredFieldsFormat) FieldsReader(dir store.Directory, segmentInfo *index.SegmentInfo, fieldInfos *index.FieldInfos, context store.IOContext) (StoredFieldsReader, error) {
	return nil, fmt.Errorf("FieldsReader not implemented")
}

// FieldsWriter returns a fields writer (must be implemented by subclasses).
func (f *BaseStoredFieldsFormat) FieldsWriter(dir store.Directory, segmentInfo *index.SegmentInfo, context store.IOContext) (StoredFieldsWriter, error) {
	return nil, fmt.Errorf("FieldsWriter not implemented")
}

// lucene90StoredFieldsFormatFactory is populated by package codecs/lucene90
// via init().  When non-nil, Lucene104StoredFieldsFormat.FieldsReader tries it
// first so that indexes written with Lucene90StoredFieldsFormat (the wire
// format used by Apache Lucene 10.4.0) can be read back.
var lucene90StoredFieldsFormatFactory func() StoredFieldsFormat

// RegisterLucene90StoredFieldsFormat sets the factory used by
// Lucene104StoredFieldsFormat to attempt Lucene90-format stored fields
// before falling back to the legacy Gocene104 simple format.
func RegisterLucene90StoredFieldsFormat(factory func() StoredFieldsFormat) {
	lucene90StoredFieldsFormatFactory = factory
}

// Lucene104StoredFieldsFormat is the Lucene 10.4 stored fields format.
type Lucene104StoredFieldsFormat struct {
	*BaseStoredFieldsFormat
}

// NewLucene104StoredFieldsFormat creates a new Lucene104StoredFieldsFormat.
func NewLucene104StoredFieldsFormat() *Lucene104StoredFieldsFormat {
	return &Lucene104StoredFieldsFormat{
		BaseStoredFieldsFormat: NewBaseStoredFieldsFormat("Lucene104StoredFieldsFormat"),
	}
}

// FieldsReader returns a stored fields reader.
//
// Apache Lucene's Lucene104Codec.storedFieldsFormat() returns a
// Lucene90StoredFieldsFormat (Lucene104Codec.java:120) — there is no
// Lucene104StoredFieldsFormat and no Lucene104 stored-fields reader in Lucene
// 10.5.0 — so the only faithful reader is the Lucene90 one, opened through the
// factory registered by importing codecs/lucene90. The invented
// "Gocene104" fallback reader that used to sit here read a file format Apache
// Lucene never writes, and it was reached by swallowing the real reader's
// error, so a genuinely corrupt or unreadable segment was reported as an
// unrelated parse failure instead.
func (f *Lucene104StoredFieldsFormat) FieldsReader(dir store.Directory, segmentInfo *index.SegmentInfo, fieldInfos *index.FieldInfos, context store.IOContext) (StoredFieldsReader, error) {
	if lucene90StoredFieldsFormatFactory == nil {
		return nil, fmt.Errorf("codecs: no Lucene90 stored-fields format registered; import codecs/lucene90")
	}
	lucene90Format := lucene90StoredFieldsFormatFactory()
	if lucene90Format == nil {
		return nil, fmt.Errorf("codecs: the registered Lucene90 stored-fields factory produced no format")
	}
	return lucene90Format.FieldsReader(dir, segmentInfo, fieldInfos, context)
}

// FieldsWriter returns a stored fields writer.
func (f *Lucene104StoredFieldsFormat) FieldsWriter(dir store.Directory, segmentInfo *index.SegmentInfo, context store.IOContext) (StoredFieldsWriter, error) {
	return NewLucene104StoredFieldsWriter(dir, segmentInfo)
}

// StoredFieldsReader is an alias of spi.StoredFieldsReader.
type StoredFieldsReader = spi.StoredFieldsReader

// StoredFieldsWriter is an alias of spi.StoredFieldsWriter. The
// WriteField method consumes the narrow spi.IndexableField surface;
// every concrete field type implemented by package document satisfies
// it implicitly.
type StoredFieldsWriter = spi.StoredFieldsWriter

// StoredFieldVisitor is an alias of spi.StoredFieldVisitor.
type StoredFieldVisitor = spi.StoredFieldVisitor

// IndexableField is an alias of spi.IndexableField — the narrow,
// codec-facing surface that StoredFieldsWriter.WriteField receives.
type IndexableField = spi.IndexableField

// field type constants for serialization
const (
	fieldTypeString = 1
	fieldTypeBinary = 2
	fieldTypeInt    = 3
	fieldTypeLong   = 4
	fieldTypeFloat  = 5
	fieldTypeDouble = 6
)

// storedDoc represents a document with its stored fields
type storedDoc struct {
	fields []storedField
}

// storedField represents a single stored field
type storedField struct {
	name      string
	fieldType byte
	value     interface{}
}

// Lucene104StoredFieldsWriter is a StoredFieldsWriter implementation for Lucene 10.4.
type Lucene104StoredFieldsWriter struct {
	directory   store.Directory
	segmentInfo *index.SegmentInfo
	out         store.IndexOutput
	docs        []storedDoc
	currentDoc  *storedDoc
	mu          sync.Mutex
	closed      bool
}

// NewLucene104StoredFieldsWriter creates a new Lucene104StoredFieldsWriter.
func NewLucene104StoredFieldsWriter(dir store.Directory, segmentInfo *index.SegmentInfo) (*Lucene104StoredFieldsWriter, error) {
	return &Lucene104StoredFieldsWriter{
		directory:   dir,
		segmentInfo: segmentInfo,
		docs:        make([]storedDoc, 0),
	}, nil
}

// StartDocument starts writing a document.
func (w *Lucene104StoredFieldsWriter) StartDocument() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.currentDoc = &storedDoc{fields: make([]storedField, 0)}
	return nil
}

// FinishDocument finishes writing the current document.
func (w *Lucene104StoredFieldsWriter) FinishDocument() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.currentDoc == nil {
		return fmt.Errorf("no document started")
	}

	w.docs = append(w.docs, *w.currentDoc)
	w.currentDoc = nil
	return nil
}

// WriteField writes a field.
func (w *Lucene104StoredFieldsWriter) WriteField(field spi.IndexableField) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.currentDoc == nil {
		return fmt.Errorf("no document started")
	}

	sf := storedField{name: field.Name()}

	// Determine field type and value
	if field.StringValue() != "" {
		sf.fieldType = fieldTypeString
		sf.value = field.StringValue()
	} else if field.BinaryValue() != nil && len(field.BinaryValue()) > 0 {
		sf.fieldType = fieldTypeBinary
		sf.value = field.BinaryValue()
	} else if field.NumericValue() != nil {
		switch v := field.NumericValue().(type) {
		case int:
			sf.fieldType = fieldTypeInt
			sf.value = v
		case int32:
			sf.fieldType = fieldTypeInt
			sf.value = int(v)
		case int64:
			sf.fieldType = fieldTypeLong
			sf.value = v
		case float32:
			sf.fieldType = fieldTypeFloat
			sf.value = v
		case float64:
			sf.fieldType = fieldTypeDouble
			sf.value = v
		default:
			// Default to storing as string
			sf.fieldType = fieldTypeString
			sf.value = fmt.Sprintf("%v", v)
		}
	} else {
		// Empty field - skip
		return nil
	}

	w.currentDoc.fields = append(w.currentDoc.fields, sf)
	return nil
}

// Finish finalises the segment after numDocs documents have been
// written. Lucene104StoredFieldsWriter performs its on-disk flush
// inside Close (called immediately afterwards by the indexing chain),
// so Finish is a no-op here.
func (w *Lucene104StoredFieldsWriter) Finish(numDocs int) error {
	return nil
}

// Close releases resources and flushes all documents to disk.
func (w *Lucene104StoredFieldsWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}
	w.closed = true

	// Create output file
	fileName := w.segmentInfo.Name() + ".fdt"
	rawOut, err := w.directory.CreateOutput(fileName, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		return fmt.Errorf("failed to create stored fields file: %w", err)
	}
	out := store.NewChecksumIndexOutput(rawOut)
	defer out.Close()

	// Write standard CodecUtil index header so this file can be packed into a
	// CFS compound file via copyFileBody (which requires a 16-byte footer).
	segID := w.segmentInfo.GetID()
	if err := WriteIndexHeader(out, lucene104SFDataCodec, lucene104SFDataVersion, segID, ""); err != nil {
		return fmt.Errorf("failed to write stored fields header: %w", err)
	}

	// Write number of documents
	if err := out.WriteVInt(int32(len(w.docs))); err != nil {
		return fmt.Errorf("failed to write doc count: %w", err)
	}

	// Write each document
	for _, doc := range w.docs {
		if err := w.writeDocument(out, doc); err != nil {
			return fmt.Errorf("failed to write document: %w", err)
		}
	}

	// Write standard 16-byte CodecUtil footer.
	if err := WriteFooter(out); err != nil {
		return fmt.Errorf("failed to write stored fields footer: %w", err)
	}
	return nil
}

// writeDocument writes a single document to the output.
func (w *Lucene104StoredFieldsWriter) writeDocument(out store.IndexOutput, doc storedDoc) error {
	// Write number of fields
	if err := out.WriteVInt(int32(len(doc.fields))); err != nil {
		return fmt.Errorf("failed to write field count: %w", err)
	}

	// Write each field
	for _, field := range doc.fields {
		if err := w.writeField(out, field); err != nil {
			return fmt.Errorf("failed to write field: %w", err)
		}
	}

	return nil
}

// writeField writes a single field to the output.
func (w *Lucene104StoredFieldsWriter) writeField(out store.IndexOutput, field storedField) error {
	// Write field name
	if err := store.WriteString(out, field.name); err != nil {
		return fmt.Errorf("failed to write field name: %w", err)
	}

	// Write field type
	if err := out.WriteByte(field.fieldType); err != nil {
		return fmt.Errorf("failed to write field type: %w", err)
	}

	// Write value based on type
	switch field.fieldType {
	case fieldTypeString:
		if err := store.WriteString(out, field.value.(string)); err != nil {
			return fmt.Errorf("failed to write string value: %w", err)
		}

	case fieldTypeBinary:
		data := field.value.([]byte)
		if err := out.WriteVInt(int32(len(data))); err != nil {
			return fmt.Errorf("failed to write binary length: %w", err)
		}
		if err := out.WriteBytes(data, 0, len(data)); err != nil {
			return fmt.Errorf("failed to write binary value: %w", err)
		}

	case fieldTypeInt:
		if err := out.WriteVInt(int32(field.value.(int))); err != nil {
			return fmt.Errorf("failed to write int value: %w", err)
		}

	case fieldTypeLong:
		if err := out.WriteVLong(field.value.(int64)); err != nil {
			return fmt.Errorf("failed to write long value: %w", err)
		}

	case fieldTypeFloat:
		if err := binaryWriteFloat(out, field.value.(float32)); err != nil {
			return fmt.Errorf("failed to write float value: %w", err)
		}

	case fieldTypeDouble:
		if err := binaryWriteDouble(out, field.value.(float64)); err != nil {
			return fmt.Errorf("failed to write double value: %w", err)
		}
	}

	return nil
}

// binary read/write helpers for float/double
func binaryReadFloat(in store.IndexInput, v *float32) error {
	b, err := in.ReadBytesN(4)
	if err != nil {
		return err
	}
	// IEEE 754 big-endian
	val := uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
	*v = float32frombits(val)
	return nil
}

func binaryWriteFloat(out store.IndexOutput, v float32) error {
	val := float32bits(v)
	b := []byte{
		byte(val >> 24),
		byte(val >> 16),
		byte(val >> 8),
		byte(val),
	}
	return out.WriteBytes(b, 0, len(b))
}

func binaryReadDouble(in store.IndexInput, v *float64) error {
	b, err := in.ReadBytesN(8)
	if err != nil {
		return err
	}
	// IEEE 754 big-endian
	val := uint64(b[0])<<56 | uint64(b[1])<<48 | uint64(b[2])<<40 | uint64(b[3])<<32 |
		uint64(b[4])<<24 | uint64(b[5])<<16 | uint64(b[6])<<8 | uint64(b[7])
	*v = float64frombits(val)
	return nil
}

func binaryWriteDouble(out store.IndexOutput, v float64) error {
	val := float64bits(v)
	b := []byte{
		byte(val >> 56),
		byte(val >> 48),
		byte(val >> 40),
		byte(val >> 32),
		byte(val >> 24),
		byte(val >> 16),
		byte(val >> 8),
		byte(val),
	}
	return out.WriteBytes(b, 0, len(b))
}

func float32bits(f float32) uint32 {
	return math.Float32bits(f)
}

func float32frombits(b uint32) float32 {
	return math.Float32frombits(b)
}

func float64bits(f float64) uint64 {
	return math.Float64bits(f)
}

func float64frombits(b uint64) float64 {
	return math.Float64frombits(b)
}
