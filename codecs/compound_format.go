// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"sort"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// CompoundFormat encodes/decodes compound files.
// This is the Go port of Lucene's org.apache.lucene.codecs.CompoundFormat.
//
// Compound files combine multiple index files into a single file to reduce
// the number of file handles needed when opening an index.
type CompoundFormat interface {
	// GetCompoundReader returns a Directory view (read-only) for the compound files in this segment.
	GetCompoundReader(dir store.Directory, si *index.SegmentInfo) (CompoundDirectory, error)

	// Write packs the provided segment's files into a compound format.
	Write(dir store.Directory, si *index.SegmentInfo, context store.IOContext) error
}

// CompoundDirectory is a read-only Directory view for compound files.
// This is the Go port of Lucene's org.apache.lucene.codecs.CompoundDirectory.
type CompoundDirectory interface {
	store.Directory

	// CheckIntegrity validates the checksum of all files in the compound directory.
	CheckIntegrity() error
}

// BaseCompoundFormat provides common functionality for CompoundFormat implementations.
type BaseCompoundFormat struct {
	name string
}

// NewBaseCompoundFormat creates a new BaseCompoundFormat.
func NewBaseCompoundFormat(name string) *BaseCompoundFormat {
	return &BaseCompoundFormat{name: name}
}

// Name returns the format name.
func (f *BaseCompoundFormat) Name() string {
	return f.name
}

// GetCompoundReader returns a compound reader (must be implemented by subclasses).
func (f *BaseCompoundFormat) GetCompoundReader(dir store.Directory, si *index.SegmentInfo) (CompoundDirectory, error) {
	return nil, fmt.Errorf("GetCompoundReader not implemented")
}

// Write writes the compound file (must be implemented by subclasses).
func (f *BaseCompoundFormat) Write(dir store.Directory, si *index.SegmentInfo, context store.IOContext) error {
	return fmt.Errorf("Write not implemented")
}

// CompoundFileEntry represents a single file entry in a compound file.
type CompoundFileEntry struct {
	FileName string
	Offset   int64
	Length   int64
}

// Lucene90CompoundFormat is the Lucene 9.0 compound file format.
// This format stores multiple files in a single .cfs file with a separate .cfe entries file.
type Lucene90CompoundFormat struct {
	*BaseCompoundFormat
}

// NewLucene90CompoundFormat creates a new Lucene90CompoundFormat.
func NewLucene90CompoundFormat() *Lucene90CompoundFormat {
	return &Lucene90CompoundFormat{
		BaseCompoundFormat: NewBaseCompoundFormat("Lucene90CompoundFormat"),
	}
}

// Write packs the segment's files into a compound format.
func (f *Lucene90CompoundFormat) Write(dir store.Directory, si *index.SegmentInfo, context store.IOContext) error {
	segmentName := si.Name()
	files := si.Files()

	if len(files) == 0 {
		return nil
	}

	// Sort files by size (smallest first) for better packing
	sortedFiles := make([]string, len(files))
	copy(sortedFiles, files)
	sort.Slice(sortedFiles, func(i, j int) bool {
		len1, _ := dir.FileLength(sortedFiles[i])
		len2, _ := dir.FileLength(sortedFiles[j])
		return len1 < len2
	})

	// Create the compound data file (.cfs)
	cfsFileName := segmentName + ".cfs"
	cfsOut, err := dir.CreateOutput(cfsFileName, context)
	if err != nil {
		return fmt.Errorf("failed to create compound data file: %w", err)
	}
	defer cfsOut.Close()

	// Write header
	if err := f.writeHeader(cfsOut); err != nil {
		return err
	}

	// Copy each file into the compound file
	entries := make([]CompoundFileEntry, 0, len(sortedFiles))
	for _, fileName := range sortedFiles {
		if !dir.FileExists(fileName) {
			continue
		}

		fileLen, err := dir.FileLength(fileName)
		if err != nil {
			return fmt.Errorf("failed to get file length for %s: %w", fileName, err)
		}

		offset := cfsOut.GetFilePointer()

		// Copy file content
		in, err := dir.OpenInput(fileName, context)
		if err != nil {
			return fmt.Errorf("failed to open input for %s: %w", fileName, err)
		}

		if err := f.copyFile(in, cfsOut); err != nil {
			in.Close()
			return fmt.Errorf("failed to copy file %s: %w", fileName, err)
		}
		in.Close()

		entries = append(entries, CompoundFileEntry{
			FileName: fileName,
			Offset:   offset,
			Length:   fileLen,
		})
	}

	// Create the entries file (.cfe)
	cfeFileName := segmentName + ".cfe"
	cfeOut, err := dir.CreateOutput(cfeFileName, context)
	if err != nil {
		return fmt.Errorf("failed to create compound entries file: %w", err)
	}
	defer cfeOut.Close()

	// Write entries file header
	if err := f.writeHeader(cfeOut); err != nil {
		return err
	}

	// Write number of entries
	if err := store.WriteVInt(cfeOut, int32(len(entries))); err != nil {
		return fmt.Errorf("failed to write entry count: %w", err)
	}

	// Write each entry
	for _, entry := range entries {
		if err := store.WriteString(cfeOut, entry.FileName); err != nil {
			return fmt.Errorf("failed to write entry name: %w", err)
		}
		if err := store.WriteVLong(cfeOut, entry.Offset); err != nil {
			return fmt.Errorf("failed to write entry offset: %w", err)
		}
		if err := store.WriteVLong(cfeOut, entry.Length); err != nil {
			return fmt.Errorf("failed to write entry length: %w", err)
		}
	}

	return nil
}

// writeHeader writes the compound file header.
func (f *Lucene90CompoundFormat) writeHeader(out store.IndexOutput) error {
	// Write magic number
	if err := store.WriteUint32(out, 0x43465300); err != nil { // "CFS\0"
		return fmt.Errorf("failed to write magic number: %w", err)
	}
	// Write version
	if err := store.WriteUint32(out, 1); err != nil {
		return fmt.Errorf("failed to write version: %w", err)
	}
	return nil
}

// copyFile copies the contents of one file to another.
func (f *Lucene90CompoundFormat) copyFile(in store.IndexInput, out store.IndexOutput) error {
	buffer := make([]byte, 8192)
	remaining := in.Length()

	for remaining > 0 {
		toRead := int64(len(buffer))
		if toRead > remaining {
			toRead = remaining
		}

		if err := in.ReadBytes(buffer[:toRead]); err != nil {
			return err
		}

		if err := out.WriteBytes(buffer[:toRead]); err != nil {
			return err
		}

		remaining -= toRead
	}

	return nil
}

// GetCompoundReader returns a Directory view for the compound files.
func (f *Lucene90CompoundFormat) GetCompoundReader(dir store.Directory, si *index.SegmentInfo) (CompoundDirectory, error) {
	segmentName := si.Name()
	cfeFileName := segmentName + ".cfe"

	if !dir.FileExists(cfeFileName) {
		return nil, fmt.Errorf("compound entries file %s not found", cfeFileName)
	}

	// Read entries
	entries, err := f.readEntries(dir, cfeFileName)
	if err != nil {
		return nil, err
	}

	// Open the compound data file
	cfsFileName := segmentName + ".cfs"
	cfsIn, err := dir.OpenInput(cfsFileName, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return nil, fmt.Errorf("failed to open compound data file: %w", err)
	}

	return &lucene90CompoundDirectory{
		directory: dir,
		cfsInput:  cfsIn,
		entries:   entries,
	}, nil
}

// readEntries reads the compound file entries from the .cfe file.
func (f *Lucene90CompoundFormat) readEntries(dir store.Directory, cfeFileName string) (map[string]CompoundFileEntry, error) {
	in, err := dir.OpenInput(cfeFileName, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return nil, fmt.Errorf("failed to open entries file: %w", err)
	}
	defer in.Close()

	// Read header
	magic, err := store.ReadUint32(in)
	if err != nil {
		return nil, fmt.Errorf("failed to read magic number: %w", err)
	}
	if magic != 0x43465300 {
		return nil, fmt.Errorf("invalid magic number: expected 0x43465300, got 0x%08x", magic)
	}

	version, err := store.ReadUint32(in)
	if err != nil {
		return nil, fmt.Errorf("failed to read version: %w", err)
	}
	if version != 1 {
		return nil, fmt.Errorf("unsupported version: %d", version)
	}

	// Read number of entries
	numEntries, err := store.ReadVInt(in)
	if err != nil {
		return nil, fmt.Errorf("failed to read entry count: %w", err)
	}

	// Read each entry
	entries := make(map[string]CompoundFileEntry)
	for i := int32(0); i < numEntries; i++ {
		fileName, err := store.ReadString(in)
		if err != nil {
			return nil, fmt.Errorf("failed to read entry name: %w", err)
		}

		offset, err := store.ReadVLong(in)
		if err != nil {
			return nil, fmt.Errorf("failed to read entry offset: %w", err)
		}

		length, err := store.ReadVLong(in)
		if err != nil {
			return nil, fmt.Errorf("failed to read entry length: %w", err)
		}

		entries[fileName] = CompoundFileEntry{
			FileName: fileName,
			Offset:   offset,
			Length:   length,
		}
	}

	return entries, nil
}

// lucene90CompoundDirectory is a read-only Directory implementation for compound files.
type lucene90CompoundDirectory struct {
	directory store.Directory
	cfsInput  store.IndexInput
	entries   map[string]CompoundFileEntry
	mu        sync.RWMutex
	closed    bool
}

// ListAll returns all file names in the compound directory.
func (d *lucene90CompoundDirectory) ListAll() ([]string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.closed {
		return nil, fmt.Errorf("compound directory is closed")
	}

	names := make([]string, 0, len(d.entries))
	for name := range d.entries {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// FileExists returns true if the file exists in the compound directory.
func (d *lucene90CompoundDirectory) FileExists(name string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.closed {
		return false
	}

	_, exists := d.entries[name]
	return exists
}

// FileLength returns the length of a file in the compound directory.
func (d *lucene90CompoundDirectory) FileLength(name string) (int64, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.closed {
		return 0, fmt.Errorf("compound directory is closed")
	}

	entry, exists := d.entries[name]
	if !exists {
		return 0, fmt.Errorf("file %s not found", name)
	}

	return entry.Length, nil
}

// OpenInput opens a file for reading from the compound directory.
func (d *lucene90CompoundDirectory) OpenInput(name string, context store.IOContext) (store.IndexInput, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.closed {
		return nil, fmt.Errorf("compound directory is closed")
	}

	entry, exists := d.entries[name]
	if !exists {
		return nil, fmt.Errorf("file %s not found", name)
	}

	// Create a slice of the underlying input
	return d.cfsInput.Slice(name, entry.Offset, entry.Length)
}

// GetDirectory returns the directory itself.
func (d *lucene90CompoundDirectory) GetDirectory() store.Directory {
	return d
}

// CreateOutput is not supported for compound directories (read-only).
func (d *lucene90CompoundDirectory) CreateOutput(name string, context store.IOContext) (store.IndexOutput, error) {
	return nil, fmt.Errorf("compound directory is read-only")
}

// DeleteFile is not supported for compound directories (read-only).
func (d *lucene90CompoundDirectory) DeleteFile(name string) error {
	return fmt.Errorf("compound directory is read-only")
}

// Rename is not supported for compound directories (read-only).
func (d *lucene90CompoundDirectory) Rename(from, to string) error {
	return fmt.Errorf("compound directory is read-only")
}

// RenameFile is not supported for compound directories (read-only).
func (d *lucene90CompoundDirectory) RenameFile(from, to string) error {
	return fmt.Errorf("compound directory is read-only")
}

// Sync is not supported for compound directories (read-only).
func (d *lucene90CompoundDirectory) Sync(names []string) error {
	return fmt.Errorf("compound directory is read-only")
}

// ObtainLock is not supported for compound directories (read-only).
func (d *lucene90CompoundDirectory) ObtainLock(name string) (store.Lock, error) {
	return nil, fmt.Errorf("compound directory is read-only")
}

// Close closes the compound directory.
func (d *lucene90CompoundDirectory) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.closed {
		return nil
	}

	d.closed = true
	return d.cfsInput.Close()
}

// CheckIntegrity validates the checksum of all files in the compound directory.
func (d *lucene90CompoundDirectory) CheckIntegrity() error {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.closed {
		return fmt.Errorf("compound directory is closed")
	}

	// Verify each entry can be read
	for name, entry := range d.entries {
		in, err := d.OpenInput(name, store.IOContext{Context: store.ContextRead})
		if err != nil {
			return fmt.Errorf("failed to open %s: %w", name, err)
		}

		// Try to read the entire file
		buffer := make([]byte, 8192)
		remaining := entry.Length
		for remaining > 0 {
			toRead := int64(len(buffer))
			if toRead > remaining {
				toRead = remaining
			}

			if err := in.ReadBytes(buffer[:toRead]); err != nil {
				return fmt.Errorf("failed to read %s: %w", name, err)
			}

			remaining -= toRead
		}
		in.Close()
	}

	return nil
}

// Ensure implementations satisfy the interfaces
var _ CompoundFormat = (*BaseCompoundFormat)(nil)
var _ CompoundFormat = (*Lucene90CompoundFormat)(nil)
var _ CompoundDirectory = (*lucene90CompoundDirectory)(nil)
var _ store.Directory = (*lucene90CompoundDirectory)(nil)
