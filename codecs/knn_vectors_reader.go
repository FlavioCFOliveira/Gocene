// Copyright 2026 Gocene. All rights reserved.
// Use this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
t"github.com/FlavioCFOliveira/Gocene/spi"
	"io"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// KnnVectorsReader reads vectors from an index.
// It is the Go port of org.apache.lucene.codecs.KnnVectorsReader from
// Apache Lucene 10.5.0.
type KnnVectorsReader interface {
	io.Closer

	// CheckIntegrity checks consistency of this reader.
	//
	// Note that this may be costly in terms of I/O, e.g. may involve computing a checksum value
	// against large data files.
	CheckIntegrity() error

	// GetFloatVectorValues returns the FloatVectorValues for the given field.
	// The behavior is undefined if the given field doesn't have KNN vectors enabled on its FieldInfo.
	// The return value is never nil.
	GetFloatVectorValues(field string) (index.FloatVectorValues, error)

	// GetByteVectorValues returns the ByteVectorValues for the given field.
	// The behavior is undefined if the given field doesn't have KNN vectors enabled on its FieldInfo.
	// The return value is never nil.
	GetByteVectorValues(field string) (index.ByteVectorValues, error)

	// SearchFloat returns the k nearest neighbor documents as determined by comparison of their vector values for
	// this field, to the given vector, by the field's similarity function. The score of each document
	// is derived from the vector similarity in a way that ensures scores are positive and that a
	// larger score corresponds to a higher ranking.
	//
	// The search is allowed to be approximate, meaning the results are not guaranteed to be the
	// true k closest neighbors. For large values of k (for example when k is close to the total
	// number of documents), the search may also retrieve fewer than k documents.
	//
	// The behavior is undefined if the given field doesn't have KNN vectors enabled on its FieldInfo.
	SearchFloat(field string, target []float32, knnCollector spi.KnnCollector, acceptDocs search.AcceptDocs) error

	// SearchByte returns the k nearest neighbor documents as determined by comparison of their vector values for
	// this field, to the given vector, by the field's similarity function. The score of each document
	// is derived from the vector similarity in a way that ensures scores are positive and that a
	// larger score corresponds to a higher ranking.
	//
	// The search is allowed to be approximate, meaning the results are not guaranteed to be the
	// true k closest neighbors. For large values of k (for example when k is close to the total
	// number of documents), the search may also retrieve fewer than k documents.
	//
	// The behavior is undefined if the given field doesn't have KNN vectors enabled on its FieldInfo.
	SearchByte(field string, target []byte, knnCollector spi.KnnCollector, acceptDocs search.AcceptDocs) error

	// GetMergeInstance returns an instance optimized for merging.
	// This instance may only be consumed in the thread that called GetMergeInstance.
	GetMergeInstance() (KnnVectorsReader, error)

	// FinishMerge optionally resets or closes merge resources used in the reader.
	FinishMerge() error

	// GetOffHeapByteSize returns the desired size of off-heap memory for the given field.
	// This size can be used to help determine the memory requirements for optimal search performance,
	// which can be greatly affected by page faults when not enough memory is available.
	//
	// For reporting purposes, the size of the off-heap index structures is broken down by their
	// file extension, which provides a logical categorization of their purpose.
	GetOffHeapByteSize(fieldInfo index.FieldInfo) map[string]int64
}

// MergeOffHeapByteSizeMaps merges the maps returned by GetOffHeapByteSize.
// This method is a convenience for aggregating the desired off-heap memory requirements for
// several fields. The keys in the returned map are a union of the keys in the given maps.
// Entries with the same key are summed.
func MergeOffHeapByteSizeMaps(map1, map2 map[string]int64) map[string]int64 {
	res := make(map[string]int64, len(map1)+len(map2))
	for k, v := range map1 {
		res[k] = v
	}
	for k, v := range map2 {
		res[k] += v
	}
	return res
}
