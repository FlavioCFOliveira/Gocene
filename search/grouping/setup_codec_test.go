// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package grouping

// Blank-import the codecs packages so the grouping test binary installs the
// production Lucene 10.4 codec as the default. Without this, a
// NewIndexWriterConfig leaves the codec nil and AddDocument/Commit cannot
// persist postings, stored fields, points, or vectors, which the grouping
// tests rely on (they build their indexes with RandomIndexWriter and
// read them back through NRT readers).
//
// The compressing package supplies the stored-fields / term-vectors formats
// that the default codec composes; both are required for a non-nil, fully
// wired default codec. This file declares no test functions; it exists solely
// for the side-effecting registration. Mirrors index/setup_codec_test.go.
import (
	_ "github.com/FlavioCFOliveira/Gocene/codecs"
	_ "github.com/FlavioCFOliveira/Gocene/codecs/lucene90/compressing"
)
