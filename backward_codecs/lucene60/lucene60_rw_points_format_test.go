// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene60

// Lucene60RWPointsFormat is a test-support type mirroring the Java class
// org.apache.lucene.backward_codecs.lucene60.Lucene60RWPointsFormat (in the
// Lucene test tree).
//
// In the Java test tree this type overrides Lucene60PointsFormat.fieldsWriter
// to return a Lucene60PointsWriter, enabling write-then-read integration tests
// against the Lucene 6.0 format.  In Gocene it is kept as a test-support stub
// because:
//   - Lucene60PointsWriter (and its dependency BKDWriter60) has not yet been
//     fully ported; the write path is unsupported on legacy formats.
//   - The Java class carries no @Test methods; it is purely a factory helper
//     for other test classes.
//
// Deviations from the Java reference (Lucene 10.4.0):
//   - Full write-path integration requires BKDWriter60, tracked in backlog
//     task #2693.
//   - The Java class lives in the test source tree; Gocene follows the same
//     convention (this file lives in the test package, _test.go suffix).
//
// Port of org.apache.lucene.backward_codecs.lucene60.Lucene60RWPointsFormat
// (Lucene 10.4.0, backward-codecs/src/test).
