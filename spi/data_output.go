// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

// DataOutput is an interface for performing write operations of Lucene's low-level data types.
// It may only be used from one thread, because it is not thread safe (it keeps
// internal state like file position).
//
// This is a port of org.apache.lucene.store.DataOutput.
type DataOutput interface {
	WriteByte(b byte) error
	WriteBytes(b []byte, offset, length int) error
	WriteBytesN(b []byte, n int) error
	WriteInt(i int32) error
	WriteShort(i int16) error
	WriteVInt(i int32) error
	WriteZInt(i int32) error
	WriteLong(i int64) error
	WriteVLong(i int64) error
	WriteZLong(i int64) error
	WriteString(s string) error
	CopyBytes(input DataInput, numBytes int64) error
	WriteMapOfStrings(m map[string]string) error
	WriteSetOfStrings(s []string) error
	WriteGroupVInts(values []int32, limit int) error
}
