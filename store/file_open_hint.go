// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

// FileOpenHint is the Go equivalent of Lucene's IOContext.FileOpenHint marker
// interface. Concrete hint types (e.g. DataAccessHint, FileDataHint,
// FileTypeHint, PreloadHint, ReadOnceHint) implement this interface so they
// can be carried as a hint set on an IOContext.
//
// Implementations must be value types (or pointers to immutable types) so
// that the IOContext that carries them remains safe to share across
// goroutines.
type FileOpenHint interface {
	// fileOpenHint is an unexported marker method that prevents arbitrary types
	// from satisfying FileOpenHint by accident.
	fileOpenHint()
}
