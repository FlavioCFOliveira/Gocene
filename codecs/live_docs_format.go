// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// LiveDocsFormat handles encoding/decoding of live docs (deleted documents).
// This is the Go port of Lucene's org.apache.lucene.codecs.LiveDocsFormat.
//
// Live docs are stored in files like _X.liv and contain a bitset indicating
// which documents are still "live" (not deleted) in the index.
type LiveDocsFormat interface {
	// Name returns the name of this format.
	Name() string

	// NewLiveDocs returns a new FixedBitSet for tracking live documents.
	NewLiveDocs(numDocs int) (*util.FixedBitSet, error)

	// ReadLiveDocs reads the live docs from the directory.
	ReadLiveDocs(dir store.Directory, segmentInfo *index.SegmentInfo) (util.Bits, error)

	// WriteLiveDocs writes the live docs to the directory.
	WriteLiveDocs(bits util.Bits, dir store.Directory, segmentInfo *index.SegmentInfo) error

	// Files returns the files used by this format for the given segment.
	Files(segmentInfo *index.SegmentInfo) []string
}

// BaseLiveDocsFormat provides common functionality for LiveDocsFormat implementations.
type BaseLiveDocsFormat struct {
	name string
}

// NewBaseLiveDocsFormat creates a new BaseLiveDocsFormat.
func NewBaseLiveDocsFormat(name string) *BaseLiveDocsFormat {
	return &BaseLiveDocsFormat{name: name}
}

// Name returns the format name.
func (f *BaseLiveDocsFormat) Name() string {
	return f.name
}

// NewLiveDocs returns a new FixedBitSet (must be implemented by subclasses).
func (f *BaseLiveDocsFormat) NewLiveDocs(numDocs int) (*util.FixedBitSet, error) {
	return nil, fmt.Errorf("NewLiveDocs not implemented")
}

// ReadLiveDocs reads the live docs (must be implemented by subclasses).
func (f *BaseLiveDocsFormat) ReadLiveDocs(dir store.Directory, segmentInfo *index.SegmentInfo) (util.Bits, error) {
	return nil, fmt.Errorf("ReadLiveDocs not implemented")
}

// WriteLiveDocs writes the live docs (must be implemented by subclasses).
func (f *BaseLiveDocsFormat) WriteLiveDocs(bits util.Bits, dir store.Directory, segmentInfo *index.SegmentInfo) error {
	return fmt.Errorf("WriteLiveDocs not implemented")
}

// Files returns the files used by this format (must be implemented by subclasses).
func (f *BaseLiveDocsFormat) Files(segmentInfo *index.SegmentInfo) []string {
	return nil
}

// Lucene90LiveDocsFormat is the Lucene 9.0 live docs format.
// This format stores live docs as a bitset in a .liv file.
type Lucene90LiveDocsFormat struct {
	*BaseLiveDocsFormat
}

// NewLucene90LiveDocsFormat creates a new Lucene90LiveDocsFormat.
func NewLucene90LiveDocsFormat() *Lucene90LiveDocsFormat {
	return &Lucene90LiveDocsFormat{
		BaseLiveDocsFormat: NewBaseLiveDocsFormat("Lucene90LiveDocsFormat"),
	}
}

// NewLiveDocs returns a new FixedBitSet for tracking live documents.
func (f *Lucene90LiveDocsFormat) NewLiveDocs(numDocs int) (*util.FixedBitSet, error) {
	return util.NewFixedBitSet(numDocs)
}

// ReadLiveDocs reads the live docs from the directory.
func (f *Lucene90LiveDocsFormat) ReadLiveDocs(dir store.Directory, segmentInfo *index.SegmentInfo) (util.Bits, error) {
	fileName := segmentInfo.Name() + ".liv"

	if !dir.FileExists(fileName) {
		// No live docs file means all documents are live
		return nil, nil
	}

	in, err := dir.OpenInput(fileName, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return nil, fmt.Errorf("failed to open live docs file: %w", err)
	}
	defer in.Close()

	// Read header
	magic, err := store.ReadUint32(in)
	if err != nil {
		return nil, fmt.Errorf("failed to read magic number: %w", err)
	}
	if magic != 0x4C495600 { // "LIV\0"
		return nil, fmt.Errorf("invalid magic number: expected 0x4C495600, got 0x%08x", magic)
	}

	version, err := store.ReadUint32(in)
	if err != nil {
		return nil, fmt.Errorf("failed to read version: %w", err)
	}
	if version != 1 {
		return nil, fmt.Errorf("unsupported version: %d", version)
	}

	// Read number of docs
	numDocs, err := store.ReadVInt(in)
	if err != nil {
		return nil, fmt.Errorf("failed to read doc count: %w", err)
	}

	// Read the bitset
	bits, err := util.NewFixedBitSet(int(numDocs))
	if err != nil {
		return nil, fmt.Errorf("failed to create bitset: %w", err)
	}
	numLongs := (int(numDocs) + 63) / 64

	for i := 0; i < numLongs; i++ {
		val, err := store.ReadInt64(in)
		if err != nil {
			return nil, fmt.Errorf("failed to read bitset value at index %d: %w", i, err)
		}
		// Set bits from the long value
		for j := 0; j < 64 && i*64+j < int(numDocs); j++ {
			if val&(1<<j) != 0 {
				bits.Set(i*64 + j)
			}
		}
	}

	return bits, nil
}

// WriteLiveDocs writes the live docs to the directory.
func (f *Lucene90LiveDocsFormat) WriteLiveDocs(bits util.Bits, dir store.Directory, segmentInfo *index.SegmentInfo) error {
	if bits == nil {
		return nil
	}

	fileName := segmentInfo.Name() + ".liv"

	out, err := dir.CreateOutput(fileName, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		return fmt.Errorf("failed to create live docs file: %w", err)
	}
	defer out.Close()

	// Write header
	if err := store.WriteUint32(out, 0x4C495600); err != nil { // "LIV\0"
		return fmt.Errorf("failed to write magic number: %w", err)
	}

	if err := store.WriteUint32(out, 1); err != nil { // Version
		return fmt.Errorf("failed to write version: %w", err)
	}

	// Write number of docs
	numDocs := bits.Length()
	if err := store.WriteVInt(out, int32(numDocs)); err != nil {
		return fmt.Errorf("failed to write doc count: %w", err)
	}

	// Write the bitset as longs
	numLongs := (numDocs + 63) / 64
	for i := 0; i < numLongs; i++ {
		var val int64
		for j := 0; j < 64 && i*64+j < numDocs; j++ {
			if bits.Get(i*64 + j) {
				val |= 1 << j
			}
		}
		if err := store.WriteInt64(out, val); err != nil {
			return fmt.Errorf("failed to write bitset value at index %d: %w", i, err)
		}
	}

	return nil
}

// Files returns the files used by this format for the given segment.
func (f *Lucene90LiveDocsFormat) Files(segmentInfo *index.SegmentInfo) []string {
	return []string{segmentInfo.Name() + ".liv"}
}

// LiveDocsReader provides read access to live docs.
type LiveDocsReader struct {
	format      LiveDocsFormat
	directory   store.Directory
	segmentInfo *index.SegmentInfo
	liveDocs    util.Bits
	mu          sync.RWMutex
}

// NewLiveDocsReader creates a new LiveDocsReader.
func NewLiveDocsReader(format LiveDocsFormat, dir store.Directory, segmentInfo *index.SegmentInfo) (*LiveDocsReader, error) {
	reader := &LiveDocsReader{
		format:      format,
		directory:   dir,
		segmentInfo: segmentInfo,
	}

	liveDocs, err := format.ReadLiveDocs(dir, segmentInfo)
	if err != nil {
		return nil, err
	}
	reader.liveDocs = liveDocs

	return reader, nil
}

// IsLive returns true if the document is live (not deleted).
func (r *LiveDocsReader) IsLive(docID int) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.liveDocs == nil {
		return true
	}

	if docID < 0 || docID >= r.liveDocs.Length() {
		return false
	}

	return r.liveDocs.Get(docID)
}

// NumDocs returns the number of documents in the live docs.
func (r *LiveDocsReader) NumDocs() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.liveDocs == nil {
		return r.segmentInfo.DocCount()
	}

	return r.liveDocs.Length()
}

// LiveDocsWriter provides write access to live docs.
type LiveDocsWriter struct {
	format      LiveDocsFormat
	directory   store.Directory
	segmentInfo *index.SegmentInfo
	liveDocs    *util.FixedBitSet
	mu          sync.Mutex
}

// NewLiveDocsWriter creates a new LiveDocsWriter.
func NewLiveDocsWriter(format LiveDocsFormat, dir store.Directory, segmentInfo *index.SegmentInfo) (*LiveDocsWriter, error) {
	numDocs := segmentInfo.DocCount()
	liveDocs, err := format.NewLiveDocs(numDocs)
	if err != nil {
		return nil, err
	}

	// Initialize all docs as live
	for i := 0; i < numDocs; i++ {
		liveDocs.Set(i)
	}

	return &LiveDocsWriter{
		format:      format,
		directory:   dir,
		segmentInfo: segmentInfo,
		liveDocs:    liveDocs,
	}, nil
}

// DeleteDocument marks a document as deleted (not live).
func (w *LiveDocsWriter) DeleteDocument(docID int) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if docID < 0 || docID >= w.liveDocs.Length() {
		return fmt.Errorf("document ID %d out of range [0, %d)", docID, w.liveDocs.Length())
	}

	w.liveDocs.Clear(docID)
	return nil
}

// IsLive returns true if the document is live (not deleted).
func (w *LiveDocsWriter) IsLive(docID int) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	if docID < 0 || docID >= w.liveDocs.Length() {
		return false
	}

	return w.liveDocs.Get(docID)
}

// Commit writes the live docs to disk.
func (w *LiveDocsWriter) Commit() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.format.WriteLiveDocs(w.liveDocs, w.directory, w.segmentInfo)
}

// Ensure implementations satisfy the interfaces
var _ LiveDocsFormat = (*BaseLiveDocsFormat)(nil)
var _ LiveDocsFormat = (*Lucene90LiveDocsFormat)(nil)
