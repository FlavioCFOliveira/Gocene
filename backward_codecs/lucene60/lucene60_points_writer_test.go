// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene60

// Lucene60PointsWriter is a test-support type mirroring the Java class
// org.apache.lucene.backward_codecs.lucene60.Lucene60PointsWriter (in the
// Lucene test tree).
//
// In the Java test tree this writer is used to create Lucene 6.0 formatted
// point data for integration tests that exercise the read path. In Gocene it
// is kept as a test-support stub because:
//   - The BKDWriter60 (required by the write path) has not yet been ported.
//   - The write path is deliberately unsupported on legacy formats via
//     Lucene60PointsFormat.FieldsWriter (returns an error).
//
// Deviations from the Java reference (Lucene 10.4.0):
//   - WriteField, Merge, Finish, and Close are stubs; full BKD write logic
//     depends on BKDWriter60, which is tracked in backlog task #2693.
//   - The Java class is in the *test* source tree (no @Test methods); Gocene
//     follows the same pattern: this file lives in the test package.
//
// Port of org.apache.lucene.backward_codecs.lucene60.Lucene60PointsWriter
// (Lucene 10.4.0, backward-codecs/src/test).
