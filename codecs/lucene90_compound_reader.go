// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Portions adapted from Apache Lucene 10.4.0:
//
//   Licensed to the Apache Software Foundation (ASF) under one or more
//   contributor license agreements. See the NOTICE file distributed with
//   this work for additional information regarding copyright ownership.
//   The ASF licenses this file to You under the Apache License, Version
//   2.0 (the "License"); you may not use this file except in compliance
//   with the License. You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
//   implied. See the License for the specific language governing
//   permissions and limitations under the License.

package codecs

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Lucene90CompoundReader is a read-only Directory view over the .cfs / .cfe
// pair produced by Lucene90CompoundFormat.Write. It mirrors
// org.apache.lucene.codecs.lucene90.Lucene90CompoundReader from Apache
// Lucene 10.4.0:
//
//   - the .cfe file is parsed eagerly on open (entries map + footer check);
//   - the .cfs file is opened once, its index header is validated against
//     the segment id and the version negotiated from the .cfe header, the
//     trailing footer structure is sanity-checked, and its physical length
//     is asserted against the sum of the largest entry offset+length plus
//     the codec footer size (so a truncation of 16 bytes is still caught);
//   - OpenInput / FileLength strip the leading "_segName" prefix from the
//     requested name, matching IndexFileNames.stripSegmentName;
//   - ListAll returns each entry name with the segment name prepended;
//   - CheckIntegrity calls ChecksumEntireFile on the .cfs handle.
//
// The directory is read-only: every mutating Directory method returns
// ErrReadOnlyCompoundDirectory.
type Lucene90CompoundReader struct {
	directory   store.Directory
	segmentName string
	entries     map[string]CompoundFileEntry
	handle      store.IndexInput
	version     int32

	closed atomic.Bool
	mu     sync.RWMutex
}

// ErrReadOnlyCompoundDirectory is returned by every mutating Directory
// method on Lucene90CompoundReader.
var ErrReadOnlyCompoundDirectory = errors.New("lucene90 compound: directory is read-only")

// NewLucene90CompoundReader opens the compound segment (si.Name + .cfs /
// .cfe) inside dir and returns a CompoundDirectory view.
//
// Mirrors Lucene90CompoundReader's constructor: read entries first (the
// .cfe header carries the format version), then open the .cfs and use the
// same version when validating its index header. Verifies the .cfs has
// the expected physical length (largest offset+length + footerLength),
// catching simple truncations. On error the .cfs handle is closed before
// returning.
func NewLucene90CompoundReader(dir store.Directory, si *index.SegmentInfo) (*Lucene90CompoundReader, error) {
	segmentName := si.Name()
	dataName := GetSegmentFileName(segmentName, "", Lucene90CompoundDataExtension)
	entriesName := GetSegmentFileName(segmentName, "", Lucene90CompoundEntriesExtension)

	entries, version, err := readCompoundEntries(dir, entriesName, si.GetID())
	if err != nil {
		return nil, err
	}

	expectedLength := expectedDataLength(entries)

	handle, err := dir.OpenInput(dataName, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return nil, fmt.Errorf("lucene90 compound: open data file %q: %w", dataName, err)
	}

	r := &Lucene90CompoundReader{
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

	// Validate the .cfs index header with the exact version negotiated by
	// the .cfe (min == max == version). Java passes the entries version as
	// both bounds.
	if _, err := CheckIndexHeader(handle, Lucene90CompoundDataCodec, version, version, si.GetID(), ""); err != nil {
		return nil, fmt.Errorf("lucene90 compound: validate %q header: %w", dataName, err)
	}

	// Validate the footer structure without re-reading the whole file: cheap
	// and catches truncated tails. Mirrors Java's CodecUtil.retrieveChecksum.
	if _, err := RetrieveChecksum(handle); err != nil {
		return nil, fmt.Errorf("lucene90 compound: retrieve checksum of %q: %w", dataName, err)
	}

	// Length sanity check. Catches "strip 16 trailing bytes" attacks that
	// retrieveChecksum alone misses because validateFooter is happy with
	// any well-formed footer pattern at length-16.
	if handle.Length() != expectedLength {
		return nil, fmt.Errorf("lucene90 compound: %q length should be %d bytes, but is %d instead",
			dataName, expectedLength, handle.Length())
	}

	success = true
	return r, nil
}

// readCompoundEntries reads the .cfe file, returns the (name → entry) map
// and the format version stamped in its index header. The version is
// reused to validate the .cfs header.
func readCompoundEntries(dir store.Directory, entriesName string, expectedSegmentID []byte) (map[string]CompoundFileEntry, int32, error) {
	in, err := dir.OpenInput(entriesName, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return nil, 0, fmt.Errorf("lucene90 compound: open entries file %q: %w", entriesName, err)
	}
	defer in.Close()

	csIn := store.NewChecksumIndexInput(in)
	version, err := CheckIndexHeader(csIn, Lucene90CompoundEntriesCodec, Lucene90CompoundVersionStart, Lucene90CompoundVersionCurrent, expectedSegmentID, "")
	if err != nil {
		return nil, 0, fmt.Errorf("lucene90 compound: validate entries header: %w", err)
	}

	numEntries, err := store.ReadVInt(csIn)
	if err != nil {
		return nil, 0, fmt.Errorf("lucene90 compound: read entry count: %w", err)
	}
	mapping := make(map[string]CompoundFileEntry, numEntries)
	for i := int32(0); i < numEntries; i++ {
		name, err := store.ReadString(csIn)
		if err != nil {
			return nil, 0, fmt.Errorf("lucene90 compound: read entry name [%d]: %w", i, err)
		}
		if _, dup := mapping[name]; dup {
			return nil, 0, fmt.Errorf("lucene90 compound: duplicate cfs entry id=%q", name)
		}
		// offset/length are written by Lucene90CompoundFormat with
		// entries.writeLong (little-endian); read them with the matching LE
		// helper. See the writer in compound_format.go.
		off, err := store.ReadInt64LE(csIn)
		if err != nil {
			return nil, 0, fmt.Errorf("lucene90 compound: read entry offset [%d]: %w", i, err)
		}
		length, err := store.ReadInt64LE(csIn)
		if err != nil {
			return nil, 0, fmt.Errorf("lucene90 compound: read entry length [%d]: %w", i, err)
		}
		mapping[name] = CompoundFileEntry{FileName: name, Offset: off, Length: length}
	}
	if _, err := CheckFooter(csIn); err != nil {
		return nil, 0, fmt.Errorf("lucene90 compound: validate entries footer: %w", err)
	}
	return mapping, version, nil
}

// expectedDataLength returns max(offset+length) over entries plus the
// codec footer length. If the entries map is empty we still expect at
// least an index header followed by the footer, mirroring Java's fallback
// to indexHeaderLength(DATA_CODEC, "").
func expectedDataLength(entries map[string]CompoundFileEntry) int64 {
	var maxEnd int64
	for _, e := range entries {
		if end := e.Offset + e.Length; end > maxEnd {
			maxEnd = end
		}
	}
	if maxEnd == 0 {
		maxEnd = int64(IndexHeaderLength(Lucene90CompoundDataCodec, ""))
	}
	return maxEnd + int64(FooterLength())
}

// ensureOpen reports an error if the reader has been closed.
func (r *Lucene90CompoundReader) ensureOpen() error {
	if r.closed.Load() {
		return fmt.Errorf("lucene90 compound: reader is closed")
	}
	return nil
}

// ListAll returns the names of every embedded file, each prefixed with
// the segment name. Mirrors Lucene90CompoundReader.listAll, which iterates
// the entries map and rewrites each name as segmentName+entryName.
func (r *Lucene90CompoundReader) ListAll() ([]string, error) {
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

// FileExists reports whether the named entry exists. The leading
// "_segName" prefix is stripped via IndexFileNames.StripSegmentName so the
// caller may pass either form (Java has no public fileExists on
// Directory, but the in-tree Directory interface exposes it; matching
// behaviour to OpenInput keeps callers consistent).
func (r *Lucene90CompoundReader) FileExists(name string) bool {
	if r.closed.Load() {
		return false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.entries[index.StripSegmentName(name)]
	return ok
}

// FileLength returns the embedded length of the named file. Mirrors
// Lucene90CompoundReader.fileLength which strips the segment name prefix
// before the lookup.
func (r *Lucene90CompoundReader) FileLength(name string) (int64, error) {
	if err := r.ensureOpen(); err != nil {
		return 0, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	id := index.StripSegmentName(name)
	e, ok := r.entries[id]
	if !ok {
		return 0, fmt.Errorf("lucene90 compound: %q not found", name)
	}
	return e.Length, nil
}

// OpenInput returns a sliced IndexInput over the named file. Mirrors
// Lucene90CompoundReader.openInput: the leading "_segName" prefix is
// stripped before looking the entry up, and the slice is taken on the
// shared .cfs handle (Lucene's IndexInput.slice is documented as
// thread-safe; Gocene's store.IndexInput.Slice follows the same
// contract).
func (r *Lucene90CompoundReader) OpenInput(name string, _ store.IOContext) (store.IndexInput, error) {
	if err := r.ensureOpen(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	id := index.StripSegmentName(name)
	e, ok := r.entries[id]
	if !ok {
		dataFileName := GetSegmentFileName(r.segmentName, "", Lucene90CompoundDataExtension)
		return nil, fmt.Errorf("lucene90 compound: no sub-file with id %q found in compound file %q (fileName=%q)",
			id, dataFileName, name)
	}
	return r.handle.Slice(name, e.Offset, e.Length)
}

// CheckIntegrity validates the .cfs file by re-running the codec footer
// over every byte. Mirrors Lucene90CompoundReader.checkIntegrity →
// CodecUtil.checksumEntireFile.
func (r *Lucene90CompoundReader) CheckIntegrity() error {
	if err := r.ensureOpen(); err != nil {
		return err
	}
	if _, err := ChecksumEntireFile(r.handle); err != nil {
		return fmt.Errorf("lucene90 compound: checksum entire file: %w", err)
	}
	return nil
}

// Close releases the underlying .cfs handle. Idempotent: a second call
// returns nil. Mirrors Lucene's IOUtils.close(handle).
func (r *Lucene90CompoundReader) Close() error {
	if !r.closed.CompareAndSwap(false, true) {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.handle.Close()
}

// GetDirectory returns the reader itself (Lucene's CompoundDirectory is a
// Directory; the GetDirectory shim exists in Gocene to bridge Java's
// FilterDirectory.getDelegate pattern).
func (r *Lucene90CompoundReader) GetDirectory() store.Directory { return r }

// String mirrors Lucene90CompoundReader.toString.
func (r *Lucene90CompoundReader) String() string {
	return fmt.Sprintf("CompoundFileDirectory(segment=%q in dir=%v)", r.segmentName, r.directory)
}

// --- Mutating Directory methods: all return ErrReadOnlyCompoundDirectory.

// CreateOutput is not supported on a compound reader.
func (r *Lucene90CompoundReader) CreateOutput(_ string, _ store.IOContext) (store.IndexOutput, error) {
	return nil, ErrReadOnlyCompoundDirectory
}

// DeleteFile is not supported on a compound reader.
func (r *Lucene90CompoundReader) DeleteFile(_ string) error {
	return ErrReadOnlyCompoundDirectory
}

// ObtainLock is not supported on a compound reader.
func (r *Lucene90CompoundReader) ObtainLock(_ string) (store.Lock, error) {
	return nil, ErrReadOnlyCompoundDirectory
}

// Compile-time assertions.
var (
	_ CompoundDirectory = (*Lucene90CompoundReader)(nil)
	_ store.Directory   = (*Lucene90CompoundReader)(nil)
)
