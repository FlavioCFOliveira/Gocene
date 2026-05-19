// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs_test

import "testing"

// Go port of org.apache.lucene.codecs.TestMergedVectorValues from Apache
// Lucene 10.4.0 (core/src/test/org/apache/lucene/codecs/TestMergedVectorValues.java).
//
// Sprint 55 gap (option c): the supporting Java types
// KnnVectorsWriter.MergedVectorValues, MergedByteVectorValues,
// MergedFloat32VectorValues, ByteVectorValuesSub and FloatVectorValuesSub are
// not yet ported to Gocene. The peer tests are scaffolded here so the
// porting backlog has explicit hooks; both cases skip until those types and
// the matching ByteVectorValues.FromBytes / FloatVectorValues.FromFloats
// constructors land.

// TestSkipsInMergedByteVectorValues mirrors testSkipsInMergedByteVectorValues
// in the Java peer: skipping doc 0 via iterator.NextDoc and then loading
// doc 1 via vectorValue must return the second source vector.
func TestSkipsInMergedByteVectorValues(t *testing.T) {
	t.Skip("requires KnnVectorsWriter.MergedByteVectorValues + ByteVectorValuesSub + ByteVectorValues.FromBytes (Sprint 55 gap)")
}

// TestSkipsInMergedFloat32VectorValues mirrors
// testSkipsInMergedFloat32VectorValues in the Java peer: skipping doc 0 via
// iterator.NextDoc and then loading doc 1 via vectorValue must return the
// second source vector.
func TestSkipsInMergedFloat32VectorValues(t *testing.T) {
	t.Skip("requires KnnVectorsWriter.MergedFloat32VectorValues + FloatVectorValuesSub + FloatVectorValues.FromFloats (Sprint 55 gap)")
}
