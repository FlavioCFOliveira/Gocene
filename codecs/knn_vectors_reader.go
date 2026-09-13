// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// KnnVectorsReader reads vectors from an index.
// It is the Go port of org.apache.lucene.codecs.KnnVectorsReader from
// Apache Lucene 10.5.0.
//
// The declaration itself lives in the spi package: index names this contract
// and index is imported by codecs, so hosting it here directly would close an
// index <-> codecs import cycle. spi.KnnVectorsFormat.FieldsReader likewise
// traffics in spi.KnnVectorsReader, which is why this is an alias rather than a
// wider codecs-side interface.
//
// getMergeInstance, finishMerge and getOffHeapByteSize are part of the SPI
// contract.
//
// KNOWN GAP: getFloatVectorValues, getByteVectorValues and the two search
// overloads of Apache Lucene 10.5.0 still have no counterpart on
// spi.KnnVectorsReader, and cannot be lifted onto it as the module stands:
//
//   - getFloatVectorValues/getByteVectorValues would have to name
//     FloatVectorValues/ByteVectorValues, and spi's copies are a different
//     contract from index's (spi's are docID-iterator shaped; index's are the
//     faithful Lucene 10 ordinal-addressed ones). Reconciling the two is the
//     vector-values lift spi/doc_values_interfaces.go already records as
//     deferred.
//   - Both search overloads take AcceptDocs, which lives in the search
//     package; search imports spi, so spi cannot name it without closing the
//     very cycle spi exists to break. Go also has no overloading, so the two
//     would need distinct names.
//
// The concrete readers expose all four as concrete-typed methods meanwhile.
type KnnVectorsReader = spi.KnnVectorsReader

// MergeOffHeapByteSizeMaps merges the maps returned by GetOffHeapByteSize.
// This method is a convenience for aggregating the desired off-heap memory requirements for
// several fields. The keys in the returned map are a union of the keys in the given maps.
// Entries with the same key are summed.
//
// Mirrors the static KnnVectorsReader.mergeOffHeapByteSizeMaps of Apache Lucene 10.5.0.
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
