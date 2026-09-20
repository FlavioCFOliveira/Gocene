// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package join

// Blank-import the codecs packages so the join test binary installs the
// production Lucene 10.4 codec as the default. Without this, a
// NewIndexWriterConfig leaves the codec nil and AddDocument/Commit cannot
// persist postings, stored fields, points, or vectors, which the block-join
// round-trip tests rely on (they build a parent/child block index with
// IndexWriter.AddDocuments and read it back through OpenDirectoryReader).
//
// The compressing package supplies the stored-fields / term-vectors formats
// that the default codec composes; both are required for a non-nil, fully
// wired default codec. This file declares no test functions; it exists solely
// for the side-effecting registration. Mirrors index/setup_codec_test.go.
import (
	_ "github.com/FlavioCFOliveira/Gocene/codecs"
	_ "github.com/FlavioCFOliveira/Gocene/codecs/lucene90/compressing"
)
