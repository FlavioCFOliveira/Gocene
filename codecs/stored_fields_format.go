// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// StoredFieldsFormat handles encoding/decoding of stored fields.
// This is the Go port of Lucene's org.apache.lucene.codecs.StoredFieldsFormat.
//
// Stored fields are kept in files like _X.fdt (data) and _X.fdx (index)
// and contain the original field values that can be retrieved at search time.
type StoredFieldsFormat interface {
	// Name returns the name of this format.
	Name() string

	// FieldsReader returns a reader for stored fields.
	// The caller should close the returned reader when done.
	FieldsReader(dir store.Directory, segmentInfo *index.SegmentInfo, fieldInfos *index.FieldInfos, context store.IOContext) (StoredFieldsReader, error)

	// FieldsWriter returns a writer for stored fields.
	// The caller should close the returned writer when done.
	FieldsWriter(dir store.Directory, segmentInfo *index.SegmentInfo, context store.IOContext) (StoredFieldsWriter, error)
}

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
func (f *Lucene104StoredFieldsFormat) FieldsReader(dir store.Directory, segmentInfo *index.SegmentInfo, fieldInfos *index.FieldInfos, context store.IOContext) (StoredFieldsReader, error) {
	return NewLucene104StoredFieldsReader(dir, segmentInfo, fieldInfos)
}

// FieldsWriter returns a stored fields writer.
func (f *Lucene104StoredFieldsFormat) FieldsWriter(dir store.Directory, segmentInfo *index.SegmentInfo, context store.IOContext) (StoredFieldsWriter, error) {
	return NewLucene104StoredFieldsWriter(dir, segmentInfo)
}

// StoredFieldsReader is a reader for stored fields.
// This is the Go port of Lucene's org.apache.lucene.codecs.StoredFieldsReader.
type StoredFieldsReader interface {
	// VisitDocument visits the stored fields for a document.
	// The visitor is called for each stored field in the document.
	VisitDocument(docID int, visitor StoredFieldVisitor) error

	// Close releases resources.
	Close() error
}

// StoredFieldsWriter is a writer for stored fields.
// This is the Go port of Lucene's org.apache.lucene.codecs.StoredFieldsWriter.
type StoredFieldsWriter interface {
	// StartDocument starts writing a document.
	StartDocument() error

	// FinishDocument finishes writing the current document.
	FinishDocument() error

	// WriteField writes a field.
	WriteField(field document.IndexableField) error

	// Close releases resources.
	Close() error
}

// StoredFieldVisitor is called for each stored field when visiting a document.
type StoredFieldVisitor interface {
	// StringField is called for a stored string field.
	StringField(field string, value string)

	// BinaryField is called for a stored binary field.
	BinaryField(field string, value []byte)

	// IntField is called for a stored int field.
	IntField(field string, value int)

	// LongField is called for a stored long field.
	LongField(field string, value int64)

	// FloatField is called for a stored float field.
	FloatField(field string, value float32)

	// DoubleField is called for a stored double field.
	DoubleField(field string, value float64)
}

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

// Lucene104StoredFieldsReader is a StoredFieldsReader implementation for Lucene 10.4.
type Lucene104StoredFieldsReader struct {
	directory   store.Directory
	segmentInfo *index.SegmentInfo
	fieldInfos  *index.FieldInfos
	docs        []storedDoc
	mu          sync.RWMutex
}

// NewLucene104StoredFieldsReader creates a new Lucene104StoredFieldsReader.
func NewLucene104StoredFieldsReader(dir store.Directory, segmentInfo *index.SegmentInfo, fieldInfos *index.FieldInfos) (*Lucene104StoredFieldsReader, error) {
	reader := &Lucene104StoredFieldsReader{
		directory:   dir,
		segmentInfo: segmentInfo,
		fieldInfos:  fieldInfos,
		docs:        make([]storedDoc, 0),
	}
	if err := reader.load(); err != nil {
		return nil, err
	}
	return reader, nil
}

// load reads stored fields from disk.
func (r *Lucene104StoredFieldsReader) load() error {
	fileName := r.segmentInfo.Name() + ".fdt"

	if !r.directory.FileExists(fileName) {
		// No stored fields file - return empty reader
		return nil
	}

	in, err := r.directory.OpenInput(fileName, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return fmt.Errorf("failed to open stored fields file: %w", err)
	}
	defer in.Close()

	// Read magic number
	magic, err := store.ReadUint32(in)
	if err != nil {
		return fmt.Errorf("failed to read magic number: %w", err)
	}
	if magic != 0x46445400 { // "FDT\0"
		return fmt.Errorf("invalid magic number: expected 0x46445400, got 0x%08x", magic)
	}

	// Read version
	version, err := store.ReadVInt(in)
	if err != nil {
		return fmt.Errorf("failed to read version: %w", err)
	}
	if version != 1 {
		return fmt.Errorf("unsupported version: %d", version)
	}

	// Read number of documents
	numDocs, err := store.ReadVInt(in)
	if err != nil {
		return fmt.Errorf("failed to read doc count: %w", err)
	}

	// Read each document
	for i := int32(0); i < numDocs; i++ {
		doc, err := r.readDocument(in)
		if err != nil {
			return fmt.Errorf("failed to read document %d: %w", i, err)
		}
		r.docs = append(r.docs, doc)
	}

	return nil
}

// readDocument reads a single document from the input.
func (r *Lucene104StoredFieldsReader) readDocument(in store.IndexInput) (storedDoc, error) {
	doc := storedDoc{}

	// Read number of fields
	numFields, err := store.ReadVInt(in)
	if err != nil {
		return doc, fmt.Errorf("failed to read field count: %w", err)
	}

	doc.fields = make([]storedField, numFields)

	// Read each field
	for i := int32(0); i < numFields; i++ {
		field, err := r.readField(in)
		if err != nil {
			return doc, fmt.Errorf("failed to read field: %w", err)
		}
		doc.fields[i] = field
	}

	return doc, nil
}

// readField reads a single field from the input.
func (r *Lucene104StoredFieldsReader) readField(in store.IndexInput) (storedField, error) {
	field := storedField{}

	// Read field name
	name, err := store.ReadString(in)
	if err != nil {
		return field, fmt.Errorf("failed to read field name: %w", err)
	}
	field.name = name

	// Read field type
	ft, err := in.ReadByte()
	if err != nil {
		return field, fmt.Errorf("failed to read field type: %w", err)
	}
	field.fieldType = ft

	// Read value based on type
	switch ft {
	case fieldTypeString:
		val, err := store.ReadString(in)
		if err != nil {
			return field, fmt.Errorf("failed to read string value: %w", err)
		}
		field.value = val

	case fieldTypeBinary:
		length, err := store.ReadVInt(in)
		if err != nil {
			return field, fmt.Errorf("failed to read binary length: %w", err)
		}
		data := make([]byte, length)
		if err := in.ReadBytes(data); err != nil {
			return field, fmt.Errorf("failed to read binary value: %w", err)
		}
		field.value = data

	case fieldTypeInt:
		val, err := store.ReadVInt(in)
		if err != nil {
			return field, fmt.Errorf("failed to read int value: %w", err)
		}
		field.value = int(val)

	case fieldTypeLong:
		val, err := store.ReadVLong(in)
		if err != nil {
			return field, fmt.Errorf("failed to read long value: %w", err)
		}
		field.value = val

	case fieldTypeFloat:
		var val float32
		if err := binaryReadFloat(in, &val); err != nil {
			return field, fmt.Errorf("failed to read float value: %w", err)
		}
		field.value = val

	case fieldTypeDouble:
		var val float64
		if err := binaryReadDouble(in, &val); err != nil {
			return field, fmt.Errorf("failed to read double value: %w", err)
		}
		field.value = val

	default:
		return field, fmt.Errorf("unknown field type: %d", ft)
	}

	return field, nil
}

// VisitDocument visits the stored fields for a document.
func (r *Lucene104StoredFieldsReader) VisitDocument(docID int, visitor StoredFieldVisitor) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if docID < 0 || docID >= len(r.docs) {
		return fmt.Errorf("document ID %d out of range [0, %d)", docID, len(r.docs))
	}

	doc := r.docs[docID]
	for _, field := range doc.fields {
		switch field.fieldType {
		case fieldTypeString:
			visitor.StringField(field.name, field.value.(string))
		case fieldTypeBinary:
			visitor.BinaryField(field.name, field.value.([]byte))
		case fieldTypeInt:
			visitor.IntField(field.name, field.value.(int))
		case fieldTypeLong:
			visitor.LongField(field.name, field.value.(int64))
		case fieldTypeFloat:
			visitor.FloatField(field.name, field.value.(float32))
		case fieldTypeDouble:
			visitor.DoubleField(field.name, field.value.(float64))
		}
	}

	return nil
}

// Close releases resources.
func (r *Lucene104StoredFieldsReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.docs = nil
	return nil
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
func (w *Lucene104StoredFieldsWriter) WriteField(field document.IndexableField) error {
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

// Close releases resources.
func (w *Lucene104StoredFieldsWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}
	w.closed = true

	// Create output file
	fileName := w.segmentInfo.Name() + ".fdt"
	out, err := w.directory.CreateOutput(fileName, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		return fmt.Errorf("failed to create stored fields file: %w", err)
	}
	defer out.Close()

	// Write magic number
	if err := store.WriteUint32(out, 0x46445400); err != nil {
		return fmt.Errorf("failed to write magic number: %w", err)
	}

	// Write version
	if err := store.WriteVInt(out, 1); err != nil {
		return fmt.Errorf("failed to write version: %w", err)
	}

	// Write number of documents
	if err := store.WriteVInt(out, int32(len(w.docs))); err != nil {
		return fmt.Errorf("failed to write doc count: %w", err)
	}

	// Write each document
	for _, doc := range w.docs {
		if err := w.writeDocument(out, doc); err != nil {
			return fmt.Errorf("failed to write document: %w", err)
		}
	}

	return nil
}

// writeDocument writes a single document to the output.
func (w *Lucene104StoredFieldsWriter) writeDocument(out store.IndexOutput, doc storedDoc) error {
	// Write number of fields
	if err := store.WriteVInt(out, int32(len(doc.fields))); err != nil {
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
		if err := store.WriteVInt(out, int32(len(data))); err != nil {
			return fmt.Errorf("failed to write binary length: %w", err)
		}
		if err := out.WriteBytes(data); err != nil {
			return fmt.Errorf("failed to write binary value: %w", err)
		}

	case fieldTypeInt:
		if err := store.WriteVInt(out, int32(field.value.(int))); err != nil {
			return fmt.Errorf("failed to write int value: %w", err)
		}

	case fieldTypeLong:
		if err := store.WriteVLong(out, field.value.(int64)); err != nil {
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
	return out.WriteBytes(b)
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
	return out.WriteBytes(b)
}

// math.Float32bits and Float64bits equivalents
func float32bits(f float32) uint32 {
	return uint32(float32(f))
}

func float32frombits(b uint32) float32 {
	return float32(b)
}

func float64bits(f float64) uint64 {
	return uint64(f)
}

func float64frombits(b uint64) float64 {
	return float64(b)
}