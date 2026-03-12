// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// SegmentInfosFormat handles encoding/decoding of segment metadata.
// This is the Go port of Lucene's org.apache.lucene.codecs.SegmentInfosFormat.
//
// SegmentInfos are stored in files named "segments_N" where N is a
// generation number. The file contains metadata about all segments in the index.
type SegmentInfosFormat interface {
	// Name returns the name of this format.
	Name() string

	// Read reads segment infos from the given directory and file name.
	// Returns the SegmentInfos or an error if reading fails.
	Read(dir store.Directory) (*index.SegmentInfos, error)

	// Write writes segment infos to the given directory.
	// Returns an error if writing fails.
	Write(dir store.Directory, infos *index.SegmentInfos) error
}

// BaseSegmentInfosFormat provides common functionality.
type BaseSegmentInfosFormat struct {
	name string
}

// NewBaseSegmentInfosFormat creates a new BaseSegmentInfosFormat.
func NewBaseSegmentInfosFormat(name string) *BaseSegmentInfosFormat {
	return &BaseSegmentInfosFormat{name: name}
}

// Name returns the format name.
func (f *BaseSegmentInfosFormat) Name() string {
	return f.name
}

// Read reads segment infos (must be implemented by subclasses).
func (f *BaseSegmentInfosFormat) Read(dir store.Directory) (*index.SegmentInfos, error) {
	return nil, fmt.Errorf("Read not implemented")
}

// Write writes segment infos (must be implemented by subclasses).
func (f *BaseSegmentInfosFormat) Write(dir store.Directory, infos *index.SegmentInfos) error {
	return fmt.Errorf("Write not implemented")
}

// Lucene104SegmentInfosFormat is the Lucene 10.4 segment infos format.
//
// File format:
//   - Header: Lucene codec header
//   - Version (Int32)
//   - Counter (Int32) - used for naming new segments
//   - Segment count (Int32)
//   - For each segment:
//   - Segment name (String)
//   - Segment generation (Int64)
//   - Document count (Int32)
//   - Compound file flag (Byte)
//   - Codec name (String)
//   - Has deletion flag (Byte)
//   - If has deletions:
//   - Deletion count (Int32)
//   - Deletion generation (Int64)
//   - Has field infos flag (Byte)
//   - If has field infos:
//   - Field infos generation (Int64)
//   - Has doc values flag (Byte)
//   - If has doc values:
//   - Doc values generation (Int64)
//   - Attributes count (Int32)
//   - For each attribute:
//   - Key (String)
//   - Value (String)
//   - User data count (Int32)
//   - For each user data entry:
//   - Key (String)
//   - Value (String)
//   - Footer: checksum
type Lucene104SegmentInfosFormat struct {
	*BaseSegmentInfosFormat
}

// NewLucene104SegmentInfosFormat creates a new Lucene104SegmentInfosFormat.
func NewLucene104SegmentInfosFormat() *Lucene104SegmentInfosFormat {
	return &Lucene104SegmentInfosFormat{
		BaseSegmentInfosFormat: NewBaseSegmentInfosFormat("Lucene104SegmentInfosFormat"),
	}
}

// Read reads segment infos from the given directory.
func (f *Lucene104SegmentInfosFormat) Read(dir store.Directory) (*index.SegmentInfos, error) {
	// Find the most recent segments file
	files, err := dir.ListAll()
	if err != nil {
		return nil, fmt.Errorf("listing directory: %w", err)
	}

	var maxGen int64 = -1
	var segmentsFile string
	for _, file := range files {
		if len(file) > 9 && file[:9] == "segments_" {
			var gen int64
			if _, err := fmt.Sscanf(file[9:], "%d", &gen); err == nil {
				if gen > maxGen {
					maxGen = gen
					segmentsFile = file
				}
			}
		}
	}

	if maxGen < 0 {
		return nil, fmt.Errorf("no segments file found in directory")
	}

	in, err := dir.OpenInput(segmentsFile, store.IOContextRead)
	if err != nil {
		return nil, fmt.Errorf("opening segments file: %w", err)
	}
	defer in.Close()

	// Read header
	if err := f.readHeader(in); err != nil {
		return nil, fmt.Errorf("reading header: %w", err)
	}

	// Read version
	version, err := store.ReadInt32(in)
	if err != nil {
		return nil, fmt.Errorf("reading version: %w", err)
	}

	// Read counter
	counter, err := store.ReadInt32(in)
	if err != nil {
		return nil, fmt.Errorf("reading counter: %w", err)
	}

	// Read number of segments
	numSegments, err := store.ReadInt32(in)
	if err != nil {
		return nil, fmt.Errorf("reading number of segments: %w", err)
	}

	if numSegments < 0 {
		return nil, fmt.Errorf("invalid number of segments: %d", numSegments)
	}

	sis := index.NewSegmentInfos()
	sis.SetVersion(fmt.Sprintf("%d", version))
	sis.SetCounter(int(counter))
	sis.SetGeneration(maxGen)
	sis.SetLastGeneration(maxGen)

	// Read each segment
	for i := int32(0); i < numSegments; i++ {
		sci, err := f.readSegmentCommitInfo(in, dir)
		if err != nil {
			return nil, fmt.Errorf("reading segment %d: %w", i, err)
		}
		sis.Add(sci)
	}

	// Read user data
	userData, err := f.readUserData(in)
	if err != nil {
		return nil, fmt.Errorf("reading user data: %w", err)
	}
	sis.SetUserData(userData)

	// Read footer
	if err := f.readFooter(in); err != nil {
		return nil, fmt.Errorf("reading footer: %w", err)
	}

	return sis, nil
}

// Write writes segment infos to the given directory.
func (f *Lucene104SegmentInfosFormat) Write(dir store.Directory, infos *index.SegmentInfos) error {
	// Get the next generation
	generation := infos.NextGeneration()
	fileName := index.GetSegmentFileName(generation)

	out, err := dir.CreateOutput(fileName, store.IOContextWrite)
	if err != nil {
		return fmt.Errorf("creating segments file: %w", err)
	}
	defer out.Close()

	// Write header
	if err := f.writeHeader(out); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}

	// Write version
	version := int32(10) // Lucene 10.x
	if err := store.WriteInt32(out, version); err != nil {
		return fmt.Errorf("writing version: %w", err)
	}

	// Write counter
	if err := store.WriteInt32(out, int32(infos.Counter())); err != nil {
		return fmt.Errorf("writing counter: %w", err)
	}

	// Write number of segments
	segments := infos.List()
	if err := store.WriteInt32(out, int32(len(segments))); err != nil {
		return fmt.Errorf("writing number of segments: %w", err)
	}

	// Write each segment
	for _, sci := range segments {
		if err := f.writeSegmentCommitInfo(out, sci); err != nil {
			return fmt.Errorf("writing segment: %w", err)
		}
	}

	// Write user data
	if err := f.writeUserData(out, infos.GetUserData()); err != nil {
		return fmt.Errorf("writing user data: %w", err)
	}

	// Write footer
	if err := f.writeFooter(out); err != nil {
		return fmt.Errorf("writing footer: %w", err)
	}

	// Sync and close
	if err := out.Close(); err != nil {
		return fmt.Errorf("closing output: %w", err)
	}

	// Update last generation
	infos.SetLastGeneration(generation)

	return nil
}

// readHeader reads the file header.
func (f *Lucene104SegmentInfosFormat) readHeader(in store.IndexInput) error {
	// Read magic number
	magic, err := store.ReadUint32(in)
	if err != nil {
		return fmt.Errorf("reading magic: %w", err)
	}
	if magic != 0x534c4366 { // 'fCLS' in little endian (segments file magic)
		return fmt.Errorf("invalid magic number: %x", magic)
	}
	return nil
}

// writeHeader writes the file header.
func (f *Lucene104SegmentInfosFormat) writeHeader(out store.IndexOutput) error {
	// Write magic number
	return store.WriteUint32(out, 0x534c4366)
}

// readFooter reads the file footer.
func (f *Lucene104SegmentInfosFormat) readFooter(in store.IndexInput) error {
	// For now, just read and ignore checksum
	_, err := store.ReadUint32(in)
	if err != nil {
		return fmt.Errorf("reading checksum: %w", err)
	}
	return nil
}

// writeFooter writes the file footer.
func (f *Lucene104SegmentInfosFormat) writeFooter(out store.IndexOutput) error {
	// For now, just write a dummy checksum
	return store.WriteUint32(out, 0)
}

// readSegmentCommitInfo reads a single segment commit info.
func (f *Lucene104SegmentInfosFormat) readSegmentCommitInfo(in store.IndexInput, dir store.Directory) (*index.SegmentCommitInfo, error) {
	// Read segment name
	name, err := store.ReadString(in)
	if err != nil {
		return nil, fmt.Errorf("reading segment name: %w", err)
	}

	// Read generation
	gen, err := store.ReadInt64(in)
	if err != nil {
		return nil, fmt.Errorf("reading generation: %w", err)
	}
	_ = gen // generation might be used for verification

	// Read document count
	docCount, err := store.ReadInt32(in)
	if err != nil {
		return nil, fmt.Errorf("reading doc count: %w", err)
	}

	// Read compound file flag
	isCompoundFileByte, err := in.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("reading compound file flag: %w", err)
	}
	isCompoundFile := isCompoundFileByte != 0

	// Read codec name
	codecName, err := store.ReadString(in)
	if err != nil {
		return nil, fmt.Errorf("reading codec name: %w", err)
	}
	_ = codecName // codec name might be used to load the right codec

	// Create segment info
	segmentInfo := index.NewSegmentInfo(name, int(docCount), dir)
	segmentInfo.SetCompoundFile(isCompoundFile)
	segmentInfo.SetCodec(codecName)

	// Read deletion info
	hasDeletionsByte, err := in.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("reading has deletions flag: %w", err)
	}

	delCount := 0
	delGen := int64(-1)
	if hasDeletionsByte != 0 {
		delCount32, err := store.ReadInt32(in)
		if err != nil {
			return nil, fmt.Errorf("reading deletion count: %w", err)
		}
		delCount = int(delCount32)

		delGen, err = store.ReadInt64(in)
		if err != nil {
			return nil, fmt.Errorf("reading deletion generation: %w", err)
		}
	}

	// Create segment commit info
	sci := index.NewSegmentCommitInfo(segmentInfo, delCount, delGen)

	// Read field infos generation
	hasFieldInfosByte, err := in.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("reading has field infos flag: %w", err)
	}
	if hasFieldInfosByte != 0 {
		fieldInfosGen, err := store.ReadInt64(in)
		if err != nil {
			return nil, fmt.Errorf("reading field infos generation: %w", err)
		}
		sci.SetFieldInfosGen(fieldInfosGen)
	}

	// Read doc values generation
	hasDocValuesByte, err := in.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("reading has doc values flag: %w", err)
	}
	if hasDocValuesByte != 0 {
		docValuesGen, err := store.ReadInt64(in)
		if err != nil {
			return nil, fmt.Errorf("reading doc values generation: %w", err)
		}
		sci.SetDocValuesGen(docValuesGen)
	}

	// Read attributes
	numAttrs, err := store.ReadInt32(in)
	if err != nil {
		return nil, fmt.Errorf("reading number of attributes: %w", err)
	}

	for i := int32(0); i < numAttrs; i++ {
		key, err := store.ReadString(in)
		if err != nil {
			return nil, fmt.Errorf("reading attribute key: %w", err)
		}
		value, err := store.ReadString(in)
		if err != nil {
			return nil, fmt.Errorf("reading attribute value: %w", err)
		}
		sci.SetAttribute(key, value)
	}

	return sci, nil
}

// writeSegmentCommitInfo writes a single segment commit info.
func (f *Lucene104SegmentInfosFormat) writeSegmentCommitInfo(out store.IndexOutput, sci *index.SegmentCommitInfo) error {
	// Write segment name
	if err := store.WriteString(out, sci.Name()); err != nil {
		return fmt.Errorf("writing segment name: %w", err)
	}

	// Write generation
	if err := store.WriteInt64(out, sci.GetGeneration()); err != nil {
		return fmt.Errorf("writing generation: %w", err)
	}

	// Write document count
	if err := store.WriteInt32(out, int32(sci.DocCount())); err != nil {
		return fmt.Errorf("writing doc count: %w", err)
	}

	// Write compound file flag
	isCompoundFile := byte(0)
	if sci.SegmentInfo().IsCompoundFile() {
		isCompoundFile = 1
	}
	if err := out.WriteByte(isCompoundFile); err != nil {
		return fmt.Errorf("writing compound file flag: %w", err)
	}

	// Write codec name
	if err := store.WriteString(out, sci.SegmentInfo().Codec()); err != nil {
		return fmt.Errorf("writing codec name: %w", err)
	}

	// Write deletion info
	hasDeletions := byte(0)
	if sci.HasDeletions() {
		hasDeletions = 1
	}
	if err := out.WriteByte(hasDeletions); err != nil {
		return fmt.Errorf("writing has deletions flag: %w", err)
	}

	if hasDeletions != 0 {
		if err := store.WriteInt32(out, int32(sci.DelCount())); err != nil {
			return fmt.Errorf("writing deletion count: %w", err)
		}
		if err := store.WriteInt64(out, sci.DelGen()); err != nil {
			return fmt.Errorf("writing deletion generation: %w", err)
		}
	}

	// Write field infos generation
	hasFieldInfos := byte(0)
	if sci.HasFieldInfosGen() {
		hasFieldInfos = 1
	}
	if err := out.WriteByte(hasFieldInfos); err != nil {
		return fmt.Errorf("writing has field infos flag: %w", err)
	}

	if hasFieldInfos != 0 {
		if err := store.WriteInt64(out, sci.FieldInfosGen()); err != nil {
			return fmt.Errorf("writing field infos generation: %w", err)
		}
	}

	// Write doc values generation
	hasDocValues := byte(0)
	if sci.HasDocValuesGen() {
		hasDocValues = 1
	}
	if err := out.WriteByte(hasDocValues); err != nil {
		return fmt.Errorf("writing has doc values flag: %w", err)
	}

	if hasDocValues != 0 {
		if err := store.WriteInt64(out, sci.DocValuesGen()); err != nil {
			return fmt.Errorf("writing doc values generation: %w", err)
		}
	}

	// Write attributes
	attrs := sci.GetAttributes()
	if err := store.WriteInt32(out, int32(len(attrs))); err != nil {
		return fmt.Errorf("writing number of attributes: %w", err)
	}
	for key, value := range attrs {
		if err := store.WriteString(out, key); err != nil {
			return fmt.Errorf("writing attribute key: %w", err)
		}
		if err := store.WriteString(out, value); err != nil {
			return fmt.Errorf("writing attribute value: %w", err)
		}
	}

	return nil
}

// readUserData reads user data map.
func (f *Lucene104SegmentInfosFormat) readUserData(in store.IndexInput) (map[string]string, error) {
	count, err := store.ReadInt32(in)
	if err != nil {
		return nil, fmt.Errorf("reading user data count: %w", err)
	}

	userData := make(map[string]string, count)
	for i := int32(0); i < count; i++ {
		key, err := store.ReadString(in)
		if err != nil {
			return nil, fmt.Errorf("reading user data key: %w", err)
		}
		value, err := store.ReadString(in)
		if err != nil {
			return nil, fmt.Errorf("reading user data value: %w", err)
		}
		userData[key] = value
	}

	return userData, nil
}

// writeUserData writes user data map.
func (f *Lucene104SegmentInfosFormat) writeUserData(out store.IndexOutput, userData map[string]string) error {
	if err := store.WriteInt32(out, int32(len(userData))); err != nil {
		return fmt.Errorf("writing user data count: %w", err)
	}

	for key, value := range userData {
		if err := store.WriteString(out, key); err != nil {
			return fmt.Errorf("writing user data key: %w", err)
		}
		if err := store.WriteString(out, value); err != nil {
			return fmt.Errorf("writing user data value: %w", err)
		}
	}

	return nil
}
