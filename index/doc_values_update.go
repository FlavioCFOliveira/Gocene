// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// DocValuesUpdate represents an in-place update to a DocValues field.
type DocValuesUpdate interface {
	Type() DocValuesType
	Term() *Term
	Field() string
	DocIDUpTo() int
	HasValue() bool
	ValueSizeInBytes() int64
	ValueToString() string
	WriteTo(w store.DataOutput) error
}

type docValuesUpdateBase struct {
	dvType    DocValuesType
	term      *Term
	field     string
	docIDUpTo int
	hasValue  bool
}

func (b *docValuesUpdateBase) Type() DocValuesType { return b.dvType }
func (b *docValuesUpdateBase) Term() *Term         { return b.term }
func (b *docValuesUpdateBase) Field() string       { return b.field }
func (b *docValuesUpdateBase) DocIDUpTo() int      { return b.docIDUpTo }
func (b *docValuesUpdateBase) HasValue() bool      { return b.hasValue }

// BinaryDocValuesUpdate is an in-place update to a binary DocValues field.
type BinaryDocValuesUpdate struct {
	docValuesUpdateBase
	value []byte
}

func NewBinaryDocValuesUpdate(term *Term, field string, value []byte) *BinaryDocValuesUpdate {
	// Default docIDUpTo is MAX_INT (2147483647)
	return &BinaryDocValuesUpdate{
		docValuesUpdateBase: docValuesUpdateBase{
			dvType:    DocValuesTypeBinary,
			term:      term,
			field:     field,
			docIDUpTo: 2147483647,
			hasValue:  value != nil,
		},
		value: value,
	}
}

func (u *BinaryDocValuesUpdate) PrepareForApply(docIDUpTo int) *BinaryDocValuesUpdate {
	if docIDUpTo == u.docIDUpTo {
		return u
	}
	return &BinaryDocValuesUpdate{
		docValuesUpdateBase: docValuesUpdateBase{
			dvType:    u.dvType,
			term:      u.term,
			field:     u.field,
			docIDUpTo: docIDUpTo,
			hasValue:  u.hasValue,
		},
		value: u.value,
	}
}

func (u *BinaryDocValuesUpdate) ValueSizeInBytes() int64 {
	var size int64 = 16 // Rough estimate for RAW_VALUE_SIZE_IN_BYTES
	if u.value != nil {
		size += int64(len(u.value))
	}
	return size
}

func (u *BinaryDocValuesUpdate) ValueToString() string {
	if u.value == nil {
		return "null"
	}
	return string(u.value)
}

func (u *BinaryDocValuesUpdate) WriteTo(w store.DataOutput) error {
	if !u.hasValue {
		return fmt.Errorf("cannot write DocValuesUpdate without value")
	}
	if err := w.WriteVInt(int32(len(u.value))); err != nil {
		return err
	}
	return w.WriteBytes(u.value, 0, len(u.value))
}

func ReadBinaryDocValuesUpdate(r store.DataInput, scratch []byte) ([]byte, error) {
	length, err := r.ReadVInt()
	if err != nil {
		return nil, err
	}

	// In Java: scratch.bytes = ArrayUtil.grow(scratch.bytes, scratch.length);
	// Since we return a new slice or use a buffer, we'll just read.
	buf := make([]byte, int(length))
	if err := r.ReadBytes(buf, 0, int(length)); err != nil {
		return nil, err
	}
	return buf, nil
}

// NumericDocValuesUpdate is an in-place update to a numeric DocValues field.
type NumericDocValuesUpdate struct {
	docValuesUpdateBase
	value int64
}

func NewNumericDocValuesUpdate(term *Term, field string, value *int64) *NumericDocValuesUpdate {
	var val int64 = -1
	hasValue := false
	if value != nil {
		val = *value
		hasValue = true
	}
	return &NumericDocValuesUpdate{
		docValuesUpdateBase: docValuesUpdateBase{
			dvType:    DocValuesTypeNumeric,
			term:      term,
			field:     field,
			docIDUpTo: 2147483647,
			hasValue:  hasValue,
		},
		value: val,
	}
}

func (u *NumericDocValuesUpdate) PrepareForApply(docIDUpTo int) *NumericDocValuesUpdate {
	if docIDUpTo == u.docIDUpTo {
		return u
	}
	return &NumericDocValuesUpdate{
		docValuesUpdateBase: docValuesUpdateBase{
			dvType:    u.dvType,
			term:      u.term,
			field:     u.field,
			docIDUpTo: docIDUpTo,
			hasValue:  u.hasValue,
		},
		value: u.value,
	}
}

func (u *NumericDocValuesUpdate) ValueSizeInBytes() int64 {
	return 8
}

func (u *NumericDocValuesUpdate) ValueToString() string {
	if !u.hasValue {
		return "null"
	}
	return strconv.FormatInt(u.value, 10)
}

func (u *NumericDocValuesUpdate) WriteTo(w store.DataOutput) error {
	if !u.hasValue {
		return fmt.Errorf("cannot write DocValuesUpdate without value")
	}
	return w.WriteZLong(u.value)
}

func ReadNumericDocValuesUpdate(r store.DataInput) (int64, error) {
	return r.ReadZLong()
}
