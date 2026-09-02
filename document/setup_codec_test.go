// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package document_test

// Blank-import the codecs package (registers the production Lucene 10.4 codec
// as the default resolved by index.NewIndexWriterConfig) and codecs/lucene90
// (installs the Lucene90PointsFormat BKD writer/reader implementation via
// codecs.SetLucene90PointsImpl). Without the lucene90 import any document_test
// that indexes a point field fails its Commit with "BKD writer impl not
// linked", because the default codec advertises a PointsFormat whose
// FieldsWriter/FieldsReader hooks would otherwise be nil (rmp #4769).
//
// This file declares no test functions; it exists solely for the
// side-effecting registrations across the external document_test binary.
import (
	_ "github.com/FlavioCFOliveira/Gocene/codecs"
	_ "github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
)
