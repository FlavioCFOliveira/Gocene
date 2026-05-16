// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

// ReadOnceHint is the Go port of org.apache.lucene.store.ReadOnceHint.
//
// In Lucene this is a single-instance enum (INSTANCE) used as a hint that the
// file will only be read once, sequentially. The Go equivalent is a
// zero-sized singleton type with a single value, ReadOnceInstance.
type ReadOnceHint struct{}

// ReadOnceInstance is the singleton ReadOnceHint value, matching Lucene's
// ReadOnceHint.INSTANCE.
var ReadOnceInstance = ReadOnceHint{}

// fileOpenHint satisfies the FileOpenHint marker interface.
func (ReadOnceHint) fileOpenHint() {}

// String returns the Lucene-equivalent constant name.
func (ReadOnceHint) String() string { return "INSTANCE" }
