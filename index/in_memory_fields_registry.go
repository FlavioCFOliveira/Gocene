// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"sync"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// inMemoryFieldsRegistry is a package-level registry that maps
// (directory, segmentName) pairs to FieldsProducer instances.
//
// This registry bridges the gap between IndexWriter.Commit() — which assigns
// in-memory postings to SegmentCommitInfo objects — and OpenDirectoryReader()
// — which deserializes fresh SegmentCommitInfo objects from disk that do not
// carry the in-memory FieldsProducer.
//
// The registry is keyed by the directory interface value (pointer equality for
// concrete types such as *ByteBuffersDirectory) combined with the segment name.
// Both keys are stable for the lifetime of a test: the same *ByteBuffersDirectory
// pointer is used by the writer and the reader, and segment names are unique
// monotonically increasing strings assigned by IndexWriter.
//
// Entries are never evicted by this registry; a production implementation would
// need a reference-counting scheme.  For unit-test usage the overhead is
// negligible.
var inMemoryFieldsRegistry inMemFieldsRegistry

type inMemFieldsRegistry struct {
	mu    sync.RWMutex
	store map[inMemFieldsKey]FieldsProducer
}

type inMemFieldsKey struct {
	dir         store.Directory
	segmentName string
}

// register stores a FieldsProducer under the given (dir, segmentName) key.
// Silently overwrites any previous entry.
func (r *inMemFieldsRegistry) register(dir store.Directory, segmentName string, fp FieldsProducer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.store == nil {
		r.store = make(map[inMemFieldsKey]FieldsProducer)
	}
	r.store[inMemFieldsKey{dir: dir, segmentName: segmentName}] = fp
}

// lookup returns the FieldsProducer for the given (dir, segmentName) pair,
// or nil if none was registered.
func (r *inMemFieldsRegistry) lookup(dir store.Directory, segmentName string) FieldsProducer {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.store[inMemFieldsKey{dir: dir, segmentName: segmentName}]
}

// RegisterInMemoryFields stores fp under (dir, segmentName) in the package-level
// registry so that SegmentReader.Terms() can retrieve it even after the
// SegmentCommitInfo object carrying the producer has been discarded by
// ReadSegmentInfos.
//
// Called by IndexWriter.Commit() for every segment that was built without a
// codec (codec-less path).
func RegisterInMemoryFields(dir store.Directory, segmentName string, fp FieldsProducer) {
	inMemoryFieldsRegistry.register(dir, segmentName, fp)
}

// LookupInMemoryFields returns the FieldsProducer previously registered under
// (dir, segmentName), or nil.
//
// Called by SegmentReader.Terms() as a last-resort fallback when neither
// coreReaders nor the SegmentCommitInfo's inMemoryFields field is set.
func LookupInMemoryFields(dir store.Directory, segmentName string) FieldsProducer {
	return inMemoryFieldsRegistry.lookup(dir, segmentName)
}
