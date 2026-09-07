// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/schema"
)

// Status represents the return value of NeedsField, indicating whether the
// field should be visited or if processing should stop.
type Status int

const (
	// StatusYes indicates the field should be visited.
	StatusYes Status = iota
	// StatusNo indicates the field should not be visited, but continue processing other fields.
	StatusNo
	// StatusStop indicates that processing of fields for the current document should stop.
	StatusStop
)

// StoredFieldVisitor provides a low-level means of accessing the stored field values in an index.
//
// NOTE: a StoredFieldVisitor implementation should not try to load or visit other
// stored documents in the same reader because the implementation of stored fields for most codecs
// is not reentrant and you will see strange exceptions as a result.
//
// This is a port of Lucene's org.apache.lucene.index.StoredFieldVisitor.
type StoredFieldVisitor interface {
	// NeedsField is called before processing a field. It allows the visitor to decide
	// whether it needs the field or if it should stop processing entirely.
	NeedsField(fieldInfo *schema.FieldInfo) (Status, error)

	// BinaryField processes a binary field.
	BinaryField(fieldInfo *schema.FieldInfo, value []byte) error

	// StringField processes a string field.
	StringField(fieldInfo *schema.FieldInfo, value string) error

	// IntField processes an int numeric field.
	IntField(fieldInfo *schema.FieldInfo, value int32) error

	// LongField processes a long numeric field.
	LongField(fieldInfo *schema.FieldInfo, value int64) error

	// FloatField processes a float numeric field.
	FloatField(fieldInfo *schema.FieldInfo, value float32) error

	// DoubleField processes a double numeric field.
	DoubleField(fieldInfo *schema.FieldInfo, value float64) error
}

// BaseStoredFieldVisitor provides default no-op implementations of the StoredFieldVisitor
// methods. It can be embedded in other structs to avoid implementing every method.
type BaseStoredFieldVisitor struct{}

func (v *BaseStoredFieldVisitor) BinaryField(fieldInfo *schema.FieldInfo, value []byte) error {
	return nil
}

func (v *BaseStoredFieldVisitor) StringField(fieldInfo *schema.FieldInfo, value string) error {
	return nil
}

func (v *BaseStoredFieldVisitor) IntField(fieldInfo *schema.FieldInfo, value int32) error {
	return nil
}

func (v *BaseStoredFieldVisitor) LongField(fieldInfo *schema.FieldInfo, value int64) error {
	return nil
}

func (v *BaseStoredFieldVisitor) FloatField(fieldInfo *schema.FieldInfo, value float32) error {
	return nil
}

func (v *BaseStoredFieldVisitor) DoubleField(fieldInfo *schema.FieldInfo, value float64) error {
	return nil
}

// StoredFieldDataInput is a wrapper around the raw stored field data.
type StoredFieldDataInput interface {
	// Length returns the length of the binary field.
	Length() int
	// Read reads the binary data into the provided byte slice.
	Read(b []byte) (int, error)
}

// BinaryFieldInput is a helper that reads a binary field from StoredFieldDataInput
// and passes it to the visitor's BinaryField method.
// This replaces the concrete method in the Java abstract class.
func BinaryFieldInput(v StoredFieldVisitor, fieldInfo *schema.FieldInfo, value StoredFieldDataInput) error {
	length := value.Length()
	data := make([]byte, length)
	if _, err := value.Read(data); err != nil {
		return err
	}
	return v.BinaryField(fieldInfo, data)
}
