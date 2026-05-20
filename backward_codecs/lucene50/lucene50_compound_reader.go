// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene50

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"

	bcstore "github.com/FlavioCFOliveira/Gocene/backward_codecs/store"
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Lucene50 compound-format constants.
const (
	// compoundDataExtension is the file extension for compound data (.cfs).
	compoundDataExtension = "cfs"

	// compoundEntriesExtension is the file extension for compound entries (.cfe).
	compoundEntriesExtension = "cfe"

	// compoundDataCodec is the codec name in the .cfs index header.
	compoundDataCodec = "Lucene50CompoundData"

	// compoundEntryCodec is the codec name in the .cfe index header.
	compoundEntryCodec = "Lucene50CompoundEntries"

	// compoundVersionStart is the minimum supported format version.
	compoundVersionStart int32 = 0

	// compoundVersionCurrent is the current format version.
	compoundVersionCurrent int32 = compoundVersionStart
)

// compoundFileEntry records the byte range of a single embedded file in the
// compound .cfs data stream.
type compoundFileEntry struct {
	offset int64
	length int64
}

// Lucene50CompoundReader is a read-only Directory view over the .cfs / .cfe
// pair produced by the Lucene 5.0 compound format.
//
// Port of
// org.apache.lucene.backward_codecs.lucene50.Lucene50CompoundReader
// (Lucene 10.4.0).
type Lucene50CompoundReader struct {
	directory   store.Directory
	segmentName string
	entries     map[string]compoundFileEntry
	handle      store.IndexInput
	version     int32

	closed atomic.Bool
	mu     sync.RWMutex
}

// checksumLike50 is the minimal interface needed to validate the codec footer
// of an EndiannessReverserChecksumIndexInput. codecs.CheckFooter requires
// *store.ChecksumIndexInput, which is incompatible with the BE-swapping
// wrapper, so we duplicate the validation locally (same pattern as
// backward_codecs/lucene40/blocktree).
type checksumLike50 interface {
	store.IndexInput
	GetChecksum() uint32
}

// checkFooter50 validates the codec footer and checksum for a checksumLike50
// input. Mirrors the logic of codecs.CheckFooter.
func checkFooter50(in checksumLike50) error {
	remaining := in.Length() - in.GetFilePointer()
	const footerLen = 16 // 4 magic + 4 algID + 8 checksum
	if remaining < footerLen {
		return fmt.Errorf("lucene50 compound: misplaced codec footer (truncated?): remaining=%d", remaining)
	}
	if remaining > footerLen {
		return fmt.Errorf("lucene50 compound: misplaced codec footer (extended?): remaining=%d", remaining)
	}
	magic, err := store.ReadInt32(in)
	if err != nil {
		return err
	}
	const footerMagic = int32(^0x3FD76C17)
	if magic != footerMagic {
		return fmt.Errorf("lucene50 compound: codec footer mismatch: actual=%#x expected=%#x", magic, footerMagic)
	}
	alg, err := store.ReadInt32(in)
	if err != nil {
		return err
	}
	if alg != 0 {
		return fmt.Errorf("lucene50 compound: codec footer unknown algorithmID: %d", alg)
	}
	actualChecksum := int64(in.GetChecksum())
	expectedChecksum, err := store.ReadInt64(in)
	if err != nil {
		return err
	}
	if actualChecksum != expectedChecksum {
		return fmt.Errorf("lucene50 compound: codec footer checksum mismatch: actual=%d expected=%d",
			actualChecksum, expectedChecksum)
	}
	return nil
}

// NewLucene50CompoundReader opens the compound segment (si.Name + .cfs /
// .cfe) inside dir and returns a CompoundDirectory view.
//
// Port of Lucene50CompoundReader(Directory, SegmentInfo).
func NewLucene50CompoundReader(dir store.Directory, si *index.SegmentInfo) (*Lucene50CompoundReader, error) {
	segmentName := si.Name()
	dataFileName := codecs.GetSegmentFileName(segmentName, "", compoundDataExtension)
	entriesFileName := codecs.GetSegmentFileName(segmentName, "", compoundEntriesExtension)

	entries, version, err := readLucene50Entries(si.GetID(), dir, entriesFileName)
	if err != nil {
		return nil, err
	}

	// Compute the expected data-file length:
	// indexHeaderLength(DATA_CODEC,"") + sum(entry.length) + footerLength.
	expectedLength := int64(codecs.IndexHeaderLength(compoundDataCodec, ""))
	for _, e := range entries {
		expectedLength += e.length
	}
	expectedLength += int64(codecs.FooterLength())

	handle, err := bcstore.OpenInput(dir, dataFileName, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return nil, fmt.Errorf("lucene50 compound: open data file %q: %w", dataFileName, err)
	}

	r := &Lucene50CompoundReader{
		directory:   dir,
		segmentName: segmentName,
		entries:     entries,
		handle:      handle,
		version:     version,
	}

	success := false
	defer func() {
		if !success {
			_ = handle.Close()
		}
	}()

	// Validate the .cfs index header. The version from the .cfe is used as
	// both min and max (same semantics as Java).
	if _, err := codecs.CheckIndexHeader(handle, compoundDataCodec, version, version, si.GetID(), ""); err != nil {
		return nil, fmt.Errorf("lucene50 compound: validate %q header: %w", dataFileName, err)
	}

	// Sanity-check the footer structure without reading all bytes.
	if _, err := codecs.RetrieveChecksum(handle); err != nil {
		return nil, fmt.Errorf("lucene50 compound: retrieve checksum of %q: %w", dataFileName, err)
	}

	// Length sanity check: catches simple truncations.
	if handle.Length() != expectedLength {
		return nil, fmt.Errorf("lucene50 compound: %q length should be %d bytes, but is %d instead",
			dataFileName, expectedLength, handle.Length())
	}

	success = true
	return r, nil
}

// readLucene50Entries reads the .cfe entries file and returns the entry map
// and the format version stamped in its index header.
func readLucene50Entries(
	segmentID []byte,
	dir store.Directory,
	entriesFileName string,
) (map[string]compoundFileEntry, int32, error) {
	in, err := bcstore.OpenChecksumInput(dir, entriesFileName, store.IOContext{Context: store.ContextReadOnce})
	if err != nil {
		return nil, 0, fmt.Errorf("lucene50 compound: open entries file %q: %w", entriesFileName, err)
	}
	defer func() { _ = in.Close() }()

	var priorErr error
	var mapping map[string]compoundFileEntry
	var version int32

	version, err = codecs.CheckIndexHeader(in, compoundEntryCodec, compoundVersionStart, compoundVersionCurrent, segmentID, "")
	if err != nil {
		priorErr = err
	} else {
		mapping, err = readLucene50Mapping(in)
		if err != nil {
			priorErr = err
		}
	}

	if ferr := checkFooter50(in); ferr != nil {
		if priorErr == nil {
			return nil, 0, ferr
		}
		return nil, 0, fmt.Errorf("%w; additionally: %v", priorErr, ferr)
	}
	if priorErr != nil {
		return nil, 0, priorErr
	}
	return mapping, version, nil
}

// readLucene50Mapping reads the per-file entries from the .cfe stream.
func readLucene50Mapping(in store.IndexInput) (map[string]compoundFileEntry, error) {
	numEntriesI, err := store.ReadVInt(in)
	if err != nil {
		return nil, fmt.Errorf("lucene50 compound: read entry count: %w", err)
	}
	numEntries := int(numEntriesI)
	mapping := make(map[string]compoundFileEntry, numEntries)
	for i := 0; i < numEntries; i++ {
		id, err := store.ReadString(in)
		if err != nil {
			return nil, fmt.Errorf("lucene50 compound: read entry name [%d]: %w", i, err)
		}
		if _, dup := mapping[id]; dup {
			return nil, fmt.Errorf("lucene50 compound: duplicate cfs entry id=%q", id)
		}
		offset, err := in.ReadLong()
		if err != nil {
			return nil, fmt.Errorf("lucene50 compound: read entry offset [%d]: %w", i, err)
		}
		length, err := in.ReadLong()
		if err != nil {
			return nil, fmt.Errorf("lucene50 compound: read entry length [%d]: %w", i, err)
		}
		mapping[id] = compoundFileEntry{offset: offset, length: length}
	}
	return mapping, nil
}

// ensureOpen returns an error if the reader has been closed.
func (r *Lucene50CompoundReader) ensureOpen() error {
	if r.closed.Load() {
		return fmt.Errorf("lucene50 compound: reader is closed")
	}
	return nil
}

// Close releases the underlying .cfs handle. Idempotent.
func (r *Lucene50CompoundReader) Close() error {
	if !r.closed.CompareAndSwap(false, true) {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.handle.Close()
}

// OpenInput returns a sliced IndexInput over the named embedded file.
//
// Port of Lucene50CompoundReader.openInput(String, IOContext).
func (r *Lucene50CompoundReader) OpenInput(name string, _ store.IOContext) (store.IndexInput, error) {
	if err := r.ensureOpen(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	id := index.StripSegmentName(name)
	e, ok := r.entries[id]
	if !ok {
		dataFileName := codecs.GetSegmentFileName(r.segmentName, "", compoundDataExtension)
		keys := make([]string, 0, len(r.entries))
		for k := range r.entries {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return nil, fmt.Errorf("lucene50 compound: no sub-file with id %q found in compound file %q (fileName=%q, files=%v)",
			id, dataFileName, name, keys)
	}
	return r.handle.Slice(name, e.offset, e.length)
}

// ListAll returns the names of every embedded file, each prefixed with the
// segment name.
//
// Port of Lucene50CompoundReader.listAll().
func (r *Lucene50CompoundReader) ListAll() ([]string, error) {
	if err := r.ensureOpen(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.entries))
	for name := range r.entries {
		out = append(out, r.segmentName+name)
	}
	sort.Strings(out)
	return out, nil
}

// FileLength returns the embedded length of the named file.
//
// Port of Lucene50CompoundReader.fileLength(String).
func (r *Lucene50CompoundReader) FileLength(name string) (int64, error) {
	if err := r.ensureOpen(); err != nil {
		return 0, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	id := index.StripSegmentName(name)
	e, ok := r.entries[id]
	if !ok {
		return 0, fmt.Errorf("lucene50 compound: %q not found", name)
	}
	return e.length, nil
}

// CheckIntegrity validates the .cfs file by running a full checksum.
//
// Port of Lucene50CompoundReader.checkIntegrity().
func (r *Lucene50CompoundReader) CheckIntegrity() error {
	if err := r.ensureOpen(); err != nil {
		return err
	}
	if _, err := codecs.ChecksumEntireFile(r.handle); err != nil {
		return fmt.Errorf("lucene50 compound: checksum entire file: %w", err)
	}
	return nil
}

// GetPendingDeletions returns an empty set (compound readers have no pending
// deletions).
func (r *Lucene50CompoundReader) GetPendingDeletions() ([]string, error) {
	return nil, nil
}

// String mirrors Lucene50CompoundReader.toString.
func (r *Lucene50CompoundReader) String() string {
	return fmt.Sprintf("CompoundFileDirectory(segment=%q in dir=%v)", r.segmentName, r.directory)
}

// FileExists reports whether the named entry exists in the compound file.
func (r *Lucene50CompoundReader) FileExists(name string) bool {
	if r.closed.Load() {
		return false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.entries[index.StripSegmentName(name)]
	return ok
}

// GetDirectory returns the reader itself.
func (r *Lucene50CompoundReader) GetDirectory() store.Directory { return r }

// CreateOutput is not supported on a compound reader.
func (r *Lucene50CompoundReader) CreateOutput(_ string, _ store.IOContext) (store.IndexOutput, error) {
	return nil, errReadOnlyCompound50
}

// DeleteFile is not supported on a compound reader.
func (r *Lucene50CompoundReader) DeleteFile(_ string) error {
	return errReadOnlyCompound50
}

// ObtainLock is not supported on a compound reader.
func (r *Lucene50CompoundReader) ObtainLock(_ string) (store.Lock, error) {
	return nil, errReadOnlyCompound50
}

// errReadOnlyCompound50 is returned by every mutating Directory method.
var errReadOnlyCompound50 = fmt.Errorf("lucene50 compound: directory is read-only")

// compile-time assertion
var _ codecs.CompoundDirectory = (*Lucene50CompoundReader)(nil)
