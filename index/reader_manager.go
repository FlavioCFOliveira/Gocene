// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"github.com/FlavioCFOliveira/Gocene/store"
)

// ReaderManager is the Go port of org.apache.lucene.index.ReaderManager
// (Apache Lucene 10.5.0): utility class to safely share [DirectoryReader]
// instances across multiple threads, while periodically reopening. This class
// ensures each reader is closed only once all threads have finished using it.
//
// See the search package's SearcherManager.
type ReaderManager struct {
	*ReferenceManager[*DirectoryReader]
}

// newReaderManagerBase wires the ReferenceManager base to rm.
func newReaderManagerBase() *ReaderManager {
	rm := &ReaderManager{}
	rm.ReferenceManager = NewReferenceManager[*DirectoryReader](rm)
	return rm
}

// NewReaderManager creates and returns a new ReaderManager from the given
// [IndexWriter]; it renders ReaderManager(IndexWriter), which applies all
// deletes and does not write them.
func NewReaderManager(writer *IndexWriter) (*ReaderManager, error) {
	return NewReaderManagerWithDeletes(writer, true, false)
}

// NewReaderManagerWithDeletes creates and returns a new ReaderManager from
// the given [IndexWriter]; it renders ReaderManager(IndexWriter, boolean
// applyAllDeletes, boolean writeAllDeletes).
//
// If applyAllDeletes is true, all buffered deletes will be applied (made
// visible) in the returned reader. If false, the deletes are not applied but
// remain buffered (in IndexWriter) so that they will be applied in the
// future. Applying deletes can be costly, so if your app can tolerate deleted
// documents being returned you might gain some performance by passing false.
// If writeAllDeletes is true, new deletes will be forcefully written to index
// files.
func NewReaderManagerWithDeletes(writer *IndexWriter, applyAllDeletes, writeAllDeletes bool) (*ReaderManager, error) {
	current, err := OpenDirectoryReaderFromWriterWithOptions(writer, applyAllDeletes, writeAllDeletes)
	if err != nil {
		return nil, err
	}
	rm := newReaderManagerBase()
	rm.SetCurrent(current)
	return rm, nil
}

// NewReaderManagerFromDir creates and returns a new ReaderManager from the
// given [store.Directory]; it renders ReaderManager(Directory).
func NewReaderManagerFromDir(dir store.Directory) (*ReaderManager, error) {
	current, err := OpenDirectoryReader(dir)
	if err != nil {
		return nil, err
	}
	rm := newReaderManagerBase()
	rm.SetCurrent(current)
	return rm, nil
}

// NewReaderManagerFromReader creates and returns a new ReaderManager from
// the given already-opened [DirectoryReader], stealing the incoming reference;
// it renders ReaderManager(DirectoryReader).
func NewReaderManagerFromReader(reader *DirectoryReader) (*ReaderManager, error) {
	rm := newReaderManagerBase()
	rm.SetCurrent(reader)
	return rm, nil
}

// DecRef renders protected void decRef(DirectoryReader reference).
func (rm *ReaderManager) DecRef(reference *DirectoryReader) error {
	return reference.DecRef()
}

// RefreshIfNeeded renders protected DirectoryReader
// refreshIfNeeded(DirectoryReader referenceToRefresh).
func (rm *ReaderManager) RefreshIfNeeded(referenceToRefresh *DirectoryReader) (*DirectoryReader, error) {
	return OpenIfChanged(referenceToRefresh)
}

// TryIncRef renders protected boolean tryIncRef(DirectoryReader reference).
func (rm *ReaderManager) TryIncRef(reference *DirectoryReader) (bool, error) {
	return reference.TryIncRef(), nil
}

// GetRefCount renders protected int getRefCount(DirectoryReader reference).
func (rm *ReaderManager) GetRefCount(reference *DirectoryReader) int {
	return int(reference.GetRefCount())
}

var _ ReferenceManagerOverrides[*DirectoryReader] = (*ReaderManager)(nil)
