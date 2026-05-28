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
import _ "github.com/FlavioCFOliveira/Gocene/codecs"
