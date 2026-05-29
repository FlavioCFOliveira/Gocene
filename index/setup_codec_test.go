// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index_test

// Blank-import the codecs package so the index_test test binary installs the
// production Lucene 10.4 codec as the default. Without this import,
// NewIndexWriterConfig leaves the codec nil and AddDocument/Commit surface
// index.ErrNoCodec on the first flush. This file exists solely to provide
// that side-effecting registration for every test in the external test
// package; it intentionally declares no test functions.
//
// The registration init() now lives in package codecs (see codecs/register.go)
// instead of the old internal/codecbridge bridge.
//
// codecs/lucene90 is blank-imported alongside it so the Lucene90PointsFormat
// BKD writer/reader implementation is installed (rmp #4769); without it, any
// test that indexes point fields fails its Commit with "BKD writer impl not
// linked" because the default codec advertises a PointsFormat whose
// FieldsWriter/FieldsReader hooks would be nil.
import (
	_ "github.com/FlavioCFOliveira/Gocene/codecs"
	_ "github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
)
