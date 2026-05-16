// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

// PreloadHint is the Go port of org.apache.lucene.store.PreloadHint.
//
// In Lucene this is a single-instance enum (INSTANCE) used as a hint that the
// file should be preloaded into memory. The Go equivalent is a zero-sized
// singleton type with a single value, PreloadInstance.
type PreloadHint struct{}

// PreloadInstance is the singleton PreloadHint value, matching Lucene's
// PreloadHint.INSTANCE.
var PreloadInstance = PreloadHint{}

// fileOpenHint satisfies the FileOpenHint marker interface.
func (PreloadHint) fileOpenHint() {}

// String returns the Lucene-equivalent constant name.
func (PreloadHint) String() string { return "INSTANCE" }
