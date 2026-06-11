// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

package document

// The four document audit rows from docs/compat-coverage.tsv (rows 98-101)
// are now covered by Sprint 8 T20 (rmp 142):
//
//   - Row 98: "Point binary encoding (BKD payloads)"
//     -> binary_point_compat_test.go (TestBinaryPoint_*)
//      + point_encoding_compat_test.go (structural encoding parity)
//
//   - Row 99: "StoredField visitor serialisation"
//     -> stored_field_visitor_compat_test.go (TestStoredFieldVisitor_*)
//
//   - Row 100: "LatLon / XY shape doc-values byte layout"
//     -> shape_doc_values_compat_test.go (TestShapeDocValues_*)
//
//   - Row 101: "Range field doc-values encoding"
//     -> range_doc_values_compat_test.go (TestRangeDocValues_*)
//
// This file exists solely to document that coverage is complete; no
// deferred tests remain for the document package. The `go build` tag
// ensures this file only compiles in the compat suite.
