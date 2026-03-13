// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// SegmentCoreReaders holds core readers that are shared (unchanged) when
// SegmentReader is cloned or reopened.
//
// This is the Go port of Lucene's org.apache.lucene.index.SegmentCoreReaders.
//
// SegmentCoreReaders manages the lifecycle of codec readers (FieldsProducer,
// TermVectorsReader, StoredFieldsReader) for a segment. These readers are
// reference-counted and shared across multiple SegmentReader instances.
type SegmentCoreReaders struct {
	// refCount is the reference count for this core
	refCount atomic.Int32

	// fields is the postings (FieldsProducer) reader
	fields FieldsProducer

	// termVectorsReader is the term vectors reader
	termVectorsReader TermVectorsReader

	// storedFieldsReader is the stored fields reader
	storedFieldsReader StoredFieldsReader

	// fieldInfos is the field infos for this segment
	fieldInfos *FieldInfos

	// segmentName is the name of the segment
	segmentName string

	// directory is the directory containing the segment
	directory store.Directory

	// closedListeners are listeners to notify when the core is closed
	closedListeners []func()

	// mu protects closedListeners
	mu sync.Mutex

	// closed indicates if the core has been closed
	closed bool
}

// NewSegmentCoreReaders creates a new SegmentCoreReaders for the given segment.
//
// This function initializes all codec readers (FieldsProducer, TermVectorsReader,
// StoredFieldsReader) for the segment using the provided codec.
func NewSegmentCoreReaders(
	directory store.Directory,
	segmentInfo *SegmentInfo,
	fieldInfos *FieldInfos,
	codec Codec,
	context store.IOContext,
) (*SegmentCoreReaders, error) {
	core := &SegmentCoreReaders{
		refCount:     atomic.Int32{},
		fieldInfos:   fieldInfos,
		segmentName:  segmentInfo.Name(),
		directory:    directory,
	}
	core.refCount.Store(1)

	// Create the segment read state
	readState := &SegmentReadState{
		Directory:     directory,
		SegmentInfo:   segmentInfo,
		FieldInfos:    fieldInfos,
		SegmentSuffix: "",
	}

	// Initialize FieldsProducer (postings) if there are indexed fields
	if fieldInfos.HasPostings() {
		postingsFormat := codec.PostingsFormat()
		if postingsFormat != nil {
			fieldsProducer, err := postingsFormat.FieldsProducer(readState)
			if err != nil {
				core.decRef()
				return nil, fmt.Errorf("creating fields producer: %w", err)
			}
			core.fields = fieldsProducer
		}
	}

	// Initialize TermVectorsReader if there are term vectors
	if fieldInfos.HasTermVectors() {
		termVectorsFormat := codec.TermVectorsFormat()
		if termVectorsFormat != nil {
			tvReader, err := termVectorsFormat.VectorsReader(directory, segmentInfo, fieldInfos, context)
			if err != nil {
				core.decRef()
				return nil, fmt.Errorf("creating term vectors reader: %w", err)
			}
			core.termVectorsReader = tvReader
		}
	}

	// Initialize StoredFieldsReader
	storedFieldsFormat := codec.StoredFieldsFormat()
	if storedFieldsFormat != nil {
		sfReader, err := storedFieldsFormat.FieldsReader(directory, segmentInfo, fieldInfos, context)
		if err != nil {
			core.decRef()
			return nil, fmt.Errorf("creating stored fields reader: %w", err)
		}
		core.storedFieldsReader = sfReader
	}

	return core, nil
}

// IncRef increments the reference count.
// Returns an error if the core is already closed.
func (core *SegmentCoreReaders) IncRef() error {
	for {
		count := core.refCount.Load()
		if count <= 0 {
			return fmt.Errorf("SegmentCoreReaders is already closed")
		}
		if core.refCount.CompareAndSwap(count, count+1) {
			return nil
		}
	}
}

// DecRef decrements the reference count.
// When the count reaches zero, all readers are closed.
func (core *SegmentCoreReaders) DecRef() error {
	if core.refCount.Add(-1) == 0 {
		return core.close()
	}
	return nil
}

// decRef is the internal version that doesn't return error
func (core *SegmentCoreReaders) decRef() {
	if core.refCount.Add(-1) == 0 {
		core.close()
	}
}

// close closes all readers and notifies listeners.
func (core *SegmentCoreReaders) close() error {
	core.mu.Lock()
	defer core.mu.Unlock()

	if core.closed {
		return nil
	}
	core.closed = true

	var lastErr error

	// Close fields producer
	if core.fields != nil {
		if err := core.fields.Close(); err != nil {
			lastErr = err
		}
	}

	// Close term vectors reader
	if core.termVectorsReader != nil {
		if err := core.termVectorsReader.Close(); err != nil {
			lastErr = err
		}
	}

	// Close stored fields reader
	if core.storedFieldsReader != nil {
		if err := core.storedFieldsReader.Close(); err != nil {
			lastErr = err
		}
	}

	// Notify listeners
	for _, listener := range core.closedListeners {
		listener()
	}
	core.closedListeners = nil

	return lastErr
}

// GetRefCount returns the current reference count.
func (core *SegmentCoreReaders) GetRefCount() int32 {
	return core.refCount.Load()
}

// GetFields returns the FieldsProducer (postings reader).
// Returns nil if there are no indexed fields.
func (core *SegmentCoreReaders) GetFields() FieldsProducer {
	return core.fields
}

// GetTermVectorsReader returns the TermVectorsReader.
// Returns nil if there are no term vectors.
func (core *SegmentCoreReaders) GetTermVectorsReader() TermVectorsReader {
	return core.termVectorsReader
}

// GetStoredFieldsReader returns the StoredFieldsReader.
func (core *SegmentCoreReaders) GetStoredFieldsReader() StoredFieldsReader {
	return core.storedFieldsReader
}

// GetFieldInfos returns the FieldInfos for this core.
func (core *SegmentCoreReaders) GetFieldInfos() *FieldInfos {
	return core.fieldInfos
}

// GetSegmentName returns the segment name.
func (core *SegmentCoreReaders) GetSegmentName() string {
	return core.segmentName
}

// GetDirectory returns the directory containing the segment.
func (core *SegmentCoreReaders) GetDirectory() store.Directory {
	return core.directory
}

// AddClosedListener adds a listener to be notified when the core is closed.
func (core *SegmentCoreReaders) AddClosedListener(listener func()) {
	core.mu.Lock()
	defer core.mu.Unlock()
	core.closedListeners = append(core.closedListeners, listener)
}

// IsClosed returns true if the core has been closed.
func (core *SegmentCoreReaders) IsClosed() bool {
	core.mu.Lock()
	defer core.mu.Unlock()
	return core.closed
}