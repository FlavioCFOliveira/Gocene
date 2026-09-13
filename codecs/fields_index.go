// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"io"

	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// FieldsIndex provides random access to the start pointers of compressed chunks.
type FieldsIndex interface {
	GetStartPointer(docID int) int64
	MaxPointer() int64

	// CheckIntegrity validates the checksum of the fields-index file.
	//
	// Mirrors the abstract
	// org.apache.lucene.codecs.lucene90.compressing.FieldsIndex#checkIntegrity
	// (Lucene 10.5.0).
	CheckIntegrity() error

	Close() error
}

// fieldsIndexReader implements FieldsIndex for the .fdx file.
type fieldsIndexReader struct {
	in          store.IndexInput
	docBase     int
	chunkDocs   int
	offsets     []int64
	maxPointer  int64
}

func NewFieldsIndexReader(dir store.Directory, segmentName, suffix, extension, codecName string, segmentID int, metaIn store.IndexInput) (FieldsIndex, error) {
	fileName := segmentName + suffix + "." + extension
	in, err := dir.OpenInput(fileName, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return nil, err
	}

	// Check header
	version, err := store.CheckIndexHeader(in, codecName, 1, 4, segmentID, suffix)
	if err != nil {
		return nil, err
	}

	// In a real implementation, this would read the monotonic arrays.
	// For now, we'll implement a simplified version.
	// Since we are only porting for reading, we need to match the binary format.
	// The .fdx file for off-heap indexes typically contains the monotonic arrays.

	// This is a stub. I will refine it once I have more info on the .fdx layout for Lucene 8.7.
	return &fieldsIndexReader{
		in: in,
	}, nil
}

func (r *fieldsIndexReader) GetStartPointer(docID int) int64 {
	// Simplified: just return 0 or handle via binary search on offsets
	return 0
}

func (r *fieldsIndexReader) MaxPointer() int64 {
	return 0
}

func (r *fieldsIndexReader) Close() error {
	return r.in.Close()
}

// CheckIntegrity validates the checksum of the entire fields-index file.
//
// Port of
// org.apache.lucene.codecs.lucene90.compressing.FieldsIndexReader#checkIntegrity
// (Lucene 10.5.0): CodecUtil.checksumEntireFile(indexInput).
func (r *fieldsIndexReader) CheckIntegrity() error {
	_, err := ChecksumEntireFile(r.in)
	return err
}
