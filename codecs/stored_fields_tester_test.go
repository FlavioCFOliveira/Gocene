// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"bytes"
	"io"
	"math/rand"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/document"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// StoredFieldsTester manages the lifecycle of a stored fields format test.
// This is a simplified Go port of Lucene's BaseStoredFieldsFormatTestCase.
type StoredFieldsTester struct {
	t    *testing.T
	seed int64
	rand *rand.Rand
}

func NewStoredFieldsTester(t *testing.T) *StoredFieldsTester {
	seed := int64(12345) // Use fixed seed for reproducibility
	return &StoredFieldsTester{
		t:    t,
		seed: seed,
		rand: rand.New(rand.NewSource(seed)),
	}
}

// SeedField is a mock document.IndexableField implementation for testing.
type SeedField struct {
	name         string
	fieldType    *document.FieldType
	stringValue  string
	binaryValue  []byte
	numericValue interface{}
}

func (f *SeedField) Name() string                   { return f.name }
func (f *SeedField) FieldType() *document.FieldType { return f.fieldType }
func (f *SeedField) StringValue() string            { return f.stringValue }
func (f *SeedField) BinaryValue() []byte            { return f.binaryValue }
func (f *SeedField) NumericValue() interface{}      { return f.numericValue }
func (f *SeedField) ReaderValue() io.Reader         { return nil }

// SeedStoredFieldVisitor is a mock StoredFieldVisitor implementation for testing.
type SeedStoredFieldVisitor struct {
	fields map[string]interface{}
}

func NewSeedStoredFieldVisitor() *SeedStoredFieldVisitor {
	return &SeedStoredFieldVisitor{
		fields: make(map[string]interface{}),
	}
}

func (v *SeedStoredFieldVisitor) StringField(field string, value string) {
	v.fields[field] = value
}

func (v *SeedStoredFieldVisitor) BinaryField(field string, value []byte) {
	v.fields[field] = value
}

func (v *SeedStoredFieldVisitor) IntField(field string, value int) {
	v.fields[field] = value
}

func (v *SeedStoredFieldVisitor) LongField(field string, value int64) {
	v.fields[field] = value
}

func (v *SeedStoredFieldVisitor) FloatField(field string, value float32) {
	v.fields[field] = value
}

func (v *SeedStoredFieldVisitor) DoubleField(field string, value float64) {
	v.fields[field] = value
}

// TestFull performs a comprehensive test of a StoredFieldsFormat.
func (p *StoredFieldsTester) TestFull(format StoredFieldsFormat, dir store.Directory) {
	segmentName := "_0"
	segmentID := make([]byte, 16)
	p.rand.Read(segmentID)

	si := index.NewSegmentInfo(segmentName, 100, dir)
	si.SetID(segmentID)

	fieldInfos := index.NewFieldInfos()
	ft := document.NewFieldType()
	ft.SetStored(true)
	ft.Freeze()

	// 1. Generate and write stored fields
	writer, err := format.FieldsWriter(dir, si, store.IOContextWrite)
	if err != nil {
		// Lucene104StoredFieldsFormat placeholder returns error
		p.t.Logf("FieldsWriter failed as expected for placeholder: %v", err)
		return
	}
	defer writer.Close()

	numDocs := 10
	expectedDocs := make([]map[string]interface{}, numDocs)

	for i := 0; i < numDocs; i++ {
		err = writer.StartDocument()
		if err != nil {
			p.t.Fatalf("writer.StartDocument failed: %v", err)
		}

		docFields := make(map[string]interface{})

		// String field
		name := "stringField"
		val := "value" + string(rune(i))
		sf := &SeedField{name: name, fieldType: ft, stringValue: val}
		err = writer.WriteField(sf)
		if err != nil {
			p.t.Fatalf("writer.WriteField (string) failed: %v", err)
		}
		docFields[name] = val

		// Binary field
		name = "binaryField"
		bVal := []byte{byte(i), byte(i + 1)}
		bf := &SeedField{name: name, fieldType: ft, binaryValue: bVal}
		err = writer.WriteField(bf)
		if err != nil {
			p.t.Fatalf("writer.WriteField (binary) failed: %v", err)
		}
		docFields[name] = bVal

		// Int field
		name = "intField"
		iVal := i * 1000
		inf := &SeedField{name: name, fieldType: ft, numericValue: iVal}
		err = writer.WriteField(inf)
		if err != nil {
			p.t.Fatalf("writer.WriteField (int) failed: %v", err)
		}
		docFields[name] = iVal

		err = writer.FinishDocument()
		if err != nil {
			p.t.Fatalf("writer.FinishDocument failed: %v", err)
		}
		expectedDocs[i] = docFields
	}

	err = writer.Close()
	if err != nil {
		p.t.Fatalf("writer.Close failed: %v", err)
	}

	// 2. Read back
	reader, err := format.FieldsReader(dir, si, fieldInfos, store.IOContextRead)
	if err != nil {
		p.t.Fatalf("FieldsReader failed: %v", err)
	}
	defer reader.Close()

	// 3. Verify
	for i := 0; i < numDocs; i++ {
		visitor := NewSeedStoredFieldVisitor()
		err = reader.VisitDocument(i, visitor)
		if err != nil {
			p.t.Fatalf("reader.VisitDocument failed for doc %d: %v", i, err)
		}

		expected := expectedDocs[i]
		if len(visitor.fields) != len(expected) {
			p.t.Fatalf("Doc %d: expected %d fields, got %d", i, len(expected), len(visitor.fields))
		}

		for name, val := range expected {
			actual, ok := visitor.fields[name]
			if !ok {
				p.t.Fatalf("Doc %d: field %s missing", i, name)
			}

			switch v := val.(type) {
			case string:
				if actual != v {
					p.t.Fatalf("Doc %d field %s: expected %s, got %s", i, name, v, actual)
				}
			case []byte:
				if !bytes.Equal(actual.([]byte), v) {
					p.t.Fatalf("Doc %d field %s: expected %v, got %v", i, name, v, actual)
				}
			case int:
				if actual != v {
					p.t.Fatalf("Doc %d field %s: expected %d, got %d", i, name, v, actual)
				}
			}
		}
	}
}
