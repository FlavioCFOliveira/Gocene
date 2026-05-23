// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package bkd

// BKDWriter60 is a test-support type mirroring the Java class
// org.apache.lucene.backward_codecs.lucene60.bkd.BKDWriter60 (in the
// Lucene test tree).
//
// In the Java test tree this class provides the full Lucene 6.0 BKD write
// path (2304 lines) used by integration tests that exercise the
// read-then-write round trip for the old format.  In Gocene it is deferred
// as a stub because:
//   - The implementation is a large, standalone port (~2300 lines) of the
//     legacy BKD write algorithm, not yet required by any live code path.
//   - The Java class lives in the test source tree but carries zero @Test
//     methods; Gocene follows the same pattern (this file is the only
//     artifact for this task).
//
// Deviations from the Java reference (Lucene 10.4.0):
//   - Full write logic (finish, build, split, radix sort, etc.) is not yet
//     implemented; the port is tracked in backlog task #2693.
//   - The Java class is in the test source tree; Gocene places this stub
//     under the same sub-package (backward_codecs/lucene60/bkd).
//
// Port of org.apache.lucene.backward_codecs.lucene60.bkd.BKDWriter60
// (Lucene 10.4.0, backward-codecs/src/test).
