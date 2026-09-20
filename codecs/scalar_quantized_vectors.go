// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// This file holds shared helpers for the scalar quantization writers. The
// ScalarEncoding enum it used to declare is the nested enum
// org.apache.lucene.util.quantization.QuantizedByteVectorValues.ScalarEncoding
// of Apache Lucene 10.5.0 and lives in util/quantization
// ([quantization.ScalarEncoding]).

package codecs

import (
	"fmt"
)

// PackNibbles packs the per-dimension 4-bit values in unpacked into packed,
// striped so packed[i] = (unpacked[i] << 4) | unpacked[len(packed)+i]. Mirrors
// org.apache.lucene.codecs.lucene104.OffHeapScalarQuantizedVectorValues.packNibbles
// (Lucene 10.4.0) and is the exact inverse of the read-side unpackNibblesPacked.
func PackNibbles(unpacked, packed []byte) error {
	if len(unpacked) != len(packed)*2 {
		return fmt.Errorf("lucene104 sq: packNibbles: unpacked len %d != 2*packed len %d", len(unpacked), len(packed))
	}
	n := len(packed)
	for i := 0; i < n; i++ {
		packed[i] = byte(int(unpacked[i])<<4 | int(unpacked[n+i]))
	}
	return nil
}

// FlatDelegateFieldWriter wraps a KnnFieldVectorsWriter so that non-FLOAT32
// (BYTE) fields registered through a scalar writer still flow their values
// to the raw flat writer.
type FlatDelegateFieldWriter struct {
	Delegate KnnFieldVectorsWriter
}

// AddValue forwards the value to the flat field writer.
func (f *FlatDelegateFieldWriter) AddValue(docID int, vectorValue any) error {
	return f.Delegate.AddValue(docID, vectorValue)
}

// RamBytesUsed reports the delegate's footprint.
func (f *FlatDelegateFieldWriter) RamBytesUsed() int64 {
	return f.Delegate.RamBytesUsed()
}

// Finish marks the delegate field complete.
func (f *FlatDelegateFieldWriter) Finish() error {
	return f.Delegate.Finish()
}
