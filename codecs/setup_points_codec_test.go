// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs_test

// Blank-import codecs/lucene90 so the Lucene90PointsFormat BKD writer/reader
// implementation is installed (via codecs.SetLucene90PointsImpl) for the
// external codecs_test binary. The codecs package itself cannot import its
// codecs/lucene90 sub-package (util/bkd imports codecs, so the BKD-backed
// points implementation lives one level up to avoid an import cycle), but the
// external test package can. Without this, any codecs_test that indexes a
// point field through IndexWriter fails its commit with "BKD writer impl not
// linked" now that points are flushed end-to-end (rmp #4769).
//
// This file declares no test functions; it exists solely for the
// side-effecting registration.
import _ "github.com/FlavioCFOliveira/Gocene/codecs/lucene90"
