// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import (
	"fmt"
	"reflect"
)

// NewDefaultIOContext is the Go port of Lucene 10.4.0's package-private
// record DefaultIOContext (core/src/java/org/apache/lucene/store/DefaultIOContext.java).
//
// In Lucene, DefaultIOContext is the concrete IOContext returned by
// IOContext.DEFAULT and IOContext.READONCE; its Context.context() is
// Context.DEFAULT, mergeInfo()/flushInfo() are null, and the constructor
// canonicalises the supplied hints into an immutable Set, rejecting duplicate
// hint classes with IllegalArgumentException.
//
// Gocene models IOContext as a value struct (see io_context.go) rather than
// an interface with named record implementations, so the Lucene record is
// surfaced here as a constructor that returns the same shape: a read-context
// IOContext seeded with the supplied hints. The "one hint per type" invariant
// is enforced by panicking on duplicate hint types, which matches the
// Java contract (IllegalArgumentException is unchecked).
//
// The returned IOContext is safe to share across goroutines provided callers
// do not mutate its Hints slice.
func NewDefaultIOContext(hints ...FileOpenHint) IOContext {
	checked := dedupedHints(hints)
	return IOContext{
		Context: ContextRead,
		Hints:   checked,
	}
}

// dedupedHints validates that no two hints share the same concrete type and
// returns a defensive copy of the slice. It mirrors the Java record's
// canonical constructor, which builds an immutable Set<FileOpenHint> and
// throws IllegalArgumentException when duplicates exist.
//
// A nil or empty input yields nil to keep zero-hint IOContext values free of
// allocations.
func dedupedHints(hints []FileOpenHint) []FileOpenHint {
	if len(hints) == 0 {
		return nil
	}
	seen := make(map[reflect.Type]struct{}, len(hints))
	for _, h := range hints {
		if h == nil {
			panic("store: nil FileOpenHint supplied to DefaultIOContext")
		}
		t := reflect.TypeOf(h)
		if _, dup := seen[t]; dup {
			panic(fmt.Sprintf("store: multiple hints of type %s specified", t))
		}
		seen[t] = struct{}{}
	}
	out := make([]FileOpenHint, len(hints))
	copy(out, hints)
	return out
}
