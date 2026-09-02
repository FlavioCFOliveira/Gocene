//go:build ignore

// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// StoredFieldVisitorStatus represents the result of StoredFieldVisitor.NeedsField.
type StoredFieldVisitorStatus int

const (
	// StoredFieldVisitorStatusYes means the field should be visited.
	StoredFieldVisitorStatusYes StoredFieldVisitorStatus = iota
	// StoredFieldVisitorStatusNo means don't visit this field, but continue processing fields.
	StoredFieldVisitorStatusNo
	// StoredFieldVisitorStatusStop means don't visit this field and stop processing entirely.
	StoredFieldVisitorStatusStop
)

// StoredFieldVisitor provides a low-level means of accessing the stored field values in an index.
// Mirrors org.apache.lucene.index.StoredFieldVisitor from Apache Lucene 10.5.0.
type StoredFieldVisitor interface {
	// NeedsField is called before processing a field.
	NeedsField(fieldInfo *FieldInfo) (StoredFieldVisitorStatus, error)

	// BinaryField processes a binary field.
	BinaryField(fieldInfo *FieldInfo, value []byte) error

	// StringField processes a string field.
	StringField(fieldInfo *FieldInfo, value string) error

	// IntField processes an int numeric field.
	IntField(fieldInfo *FieldInfo, value int32) error

	// LongField processes a long numeric field.
	LongField(fieldInfo *FieldInfo, value int64) error

	// FloatField processes a float numeric field.
	FloatField(fieldInfo *FieldInfo, value float32) error

	// DoubleField processes a double numeric field.
	DoubleField(fieldInfo *FieldInfo, value float64) error

	// BinaryFieldDirect processes a binary field directly from the StoredFieldDataInput.
	// Default implementation reads all bytes into a buffer and calls BinaryField.
	BinaryFieldDirect(fieldInfo *FieldInfo, value *store.StoredFieldDataInput) error
}

// BaseStoredFieldVisitor provides a base implementation of StoredFieldVisitor.
type BaseStoredFieldVisitor struct{}

func (v *BaseStoredFieldVisitor) BinaryField(fieldInfo *FieldInfo, value []byte) error { return nil }
func (v *BaseStoredFieldVisitor) StringField(fieldInfo *FieldInfo, value string) error { return nil }
func (v *BaseStoredFieldVisitor) IntField(fieldInfo *FieldInfo, value int32) error { return nil }
func (v *BaseStoredFieldVisitor) LongField(fieldInfo *FieldInfo, value int64) error { return nil }
func (v *BaseStoredFieldVisitor) FloatField(fieldInfo *FieldInfo, value float32) error { return nil }
func (v *BaseStoredFieldVisitor) DoubleField(fieldInfo *FieldInfo, value float64) error { return nil }

func (v *BaseStoredFieldVisitor) BinaryFieldDirect(fieldInfo *FieldInfo, value *store.StoredFieldDataInput) error {
	length := value.Length()
	data := make([]byte, length)
	if _, err := io.ReadFull(value.GetDataInput(), data); err != nil {
		return err
	}
	return v.BinaryField(fieldInfo, data)
}
