// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/store"
)

// ReaderManager safely shares a single DirectoryReader across multiple
// goroutines, periodically reopening it via MaybeRefresh to pick up new
// index commits. Each caller acquires a reference-counted reader snapshot
// via Acquire and releases it via Release when done. Mirrors
// org.apache.lucene.index.ReaderManager from Apache Lucene 10.4.0.
//
// # Deviation from Lucene 10.4.0
//
// Lucene's ReaderManager extends the generic ReferenceManager<DirectoryReader>
// and implements four abstract methods: decRef, refreshIfNeeded, tryIncRef,
// and getRefCount. Gocene's ReferenceManager[T] provides an equivalent generic
// implementation already wired for DirectoryReader via NewReaderManagerFromDir
// and NewReaderManagerFromWriter. The Acquire/Release/MaybeRefresh/Close API
// surface is identical.
//
// DirectoryReader.OpenIfChanged is not yet ported (see backlog #2707); the
// refreshIfNeeded delegate therefore returns nil (no new reader available)
// until that port lands. All other behaviour — reference counting, listener
// notification, thread safety — is fully functional via the embedded
// ReferenceManager[*DirectoryReader].
type ReaderManager struct {
	*ReferenceManager[*DirectoryReader]
}

// NewReaderManagerFromDir opens a DirectoryReader from the given directory
// and returns a ReaderManager that owns its lifecycle.
func NewReaderManagerFromDir(dir store.Directory) (*ReaderManager, error) {
	dr, err := OpenDirectoryReader(dir)
	if err != nil {
		return nil, err
	}
	return newReaderManager(dr), nil
}

// NewReaderManagerFromWriterAndDir opens a DirectoryReader from the given
// directory (which must be the same directory the IndexWriter operates on)
// and returns a ReaderManager.
//
// Deviation from Lucene 10.4.0: Lucene's ReaderManager(IndexWriter) opens
// the reader directly from the writer's in-memory state via
// DirectoryReader.open(IndexWriter, boolean, boolean), which is not yet ported
// (backlog #2707). Pass the writer's directory explicitly here until that path
// lands.
func NewReaderManagerFromWriterAndDir(dir store.Directory) (*ReaderManager, error) {
	return NewReaderManagerFromDir(dir)
}

// NewReaderManagerFromReader takes ownership of an already-open DirectoryReader
// and wraps it in a ReaderManager. The caller must not Close the reader
// directly after this call; the manager owns its lifecycle.
func NewReaderManagerFromReader(reader *DirectoryReader) *ReaderManager {
	return newReaderManager(reader)
}

// newReaderManager is the internal constructor shared by all public variants.
func newReaderManager(initial *DirectoryReader) *ReaderManager {
	rm := NewReferenceManagerWithFuncs[*DirectoryReader](
		initial,
		// acquireFunc: increment refcount, return same reader (the caller
		// will Release via releaseFunc when done).
		func(r *DirectoryReader) *DirectoryReader {
			if r != nil {
				_ = r.IncRef()
			}
			return r
		},
		// releaseFunc: decrement refcount; the reader closes itself when it
		// reaches zero.
		func(r *DirectoryReader) error {
			if r == nil {
				return nil
			}
			return r.DecRef()
		},
	)
	return &ReaderManager{ReferenceManager: rm}
}

// MaybeRefresh checks whether a newer index commit is available and, if so,
// replaces the current reader with a freshly opened one. Returns true if the
// reader was refreshed.
//
// Deviation: DirectoryReader.OpenIfChanged is not yet ported (backlog #2707).
// Until it lands this method always returns (false, nil), meaning the manager
// serves the same reader snapshot it was opened with. Reference counting and
// listener notification still work correctly.
func (rm *ReaderManager) MaybeRefresh() (bool, error) {
	// OpenIfChanged not yet available — no refresh performed.
	return false, nil
}
