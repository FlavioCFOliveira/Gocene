// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// SegmentInfosFormat is an alias of spi.SegmentInfosFormat.
//
// Lifted onto the SPI by rmp #4706. Both Read and Write now carry an
// IOContext to mirror the rest of the codec SPI; codecs implementations
// forward it to the underlying Directory I/O calls.
type SegmentInfosFormat = spi.SegmentInfosFormat

// SegmentInfoFormat is an alias of spi.SegmentInfoFormat.
type SegmentInfoFormat = spi.SegmentInfoFormat

// Lucene104SegmentInfosFormat implements the Lucene 10.4 segment infos format (segments_N).
type Lucene104SegmentInfosFormat struct{}

const (
	sisCodecName = "segments"
	sisVersion   = 10 // Lucene 10.x
)

func NewLucene104SegmentInfosFormat() *Lucene104SegmentInfosFormat {
	return &Lucene104SegmentInfosFormat{}
}

func (f *Lucene104SegmentInfosFormat) Name() string {
	return "Lucene104SegmentInfosFormat"
}

func (f *Lucene104SegmentInfosFormat) Read(dir store.Directory, ctx store.IOContext) (*spi.SegmentInfos, error) {
	files, err := dir.ListAll()
	if err != nil {
		return nil, err
	}

	var maxGen int64 = -1
	var segmentsFile string
	for _, file := range files {
		if len(file) > 9 && file[:9] == "segments_" {
			// Generation numbers are base-36 encoded, matching Lucene's
			// Long.toString(gen, Character.MAX_RADIX).
			if gen, err2 := strconv.ParseInt(file[9:], 36, 64); err2 == nil {
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

	in, err := dir.OpenInput(segmentsFile, ctx)
	if err != nil {
		return nil, err
	}
	checksumIn := store.NewChecksumIndexInput(in)
	defer checksumIn.Close()

	// Check header
	_, err = CheckIndexHeader(checksumIn, sisCodecName, sisVersion, sisVersion, nil, strconv.FormatInt(maxGen, 36))
	if err != nil {
		return nil, err
	}

	// Read Lucene version
	major, err := store.ReadVInt(checksumIn)
	if err != nil {
		return nil, err
	}
	minor, err := store.ReadVInt(checksumIn)
	if err != nil {
		return nil, err
	}
	bugfix, err := store.ReadVInt(checksumIn)
	if err != nil {
		return nil, err
	}

	// Read created major
	createdMajor, err := store.ReadVInt(checksumIn)
	if err != nil {
		return nil, err
	}

	// Read version
	version, err := store.ReadInt64(checksumIn)
	if err != nil {
		return nil, err
	}

	// Read counter
	counter, err := store.ReadVLong(checksumIn)
	if err != nil {
		return nil, err
	}

	// Read segment count
	numSegments, err := store.ReadInt32(checksumIn)
	if err != nil {
		return nil, err
	}

	if numSegments < 0 {
		return nil, fmt.Errorf("invalid number of segments: %d", numSegments)
	}

	// Read min segment version if any
	if numSegments > 0 {
		_, _ = store.ReadVInt(checksumIn) // major
		_, _ = store.ReadVInt(checksumIn) // minor
		_, _ = store.ReadVInt(checksumIn) // bugfix
	}

	sis := spi.NewSegmentInfos()
	sis.SetLuceneVersion(fmt.Sprintf("%d.%d.%d", major, minor, bugfix))
	sis.SetIndexCreatedVersionMajor(createdMajor)
	sis.SetVersion(version)
	sis.SetCounter(counter)
	sis.SetGeneration(maxGen)
	sis.SetLastGeneration(maxGen)

	for i := int32(0); i < numSegments; i++ {
		sci, err := f.readSegmentCommitInfo(checksumIn, dir)
		if err != nil {
			return nil, err
		}
		sis.Add(sci)
	}

	userData, err := store.ReadMapOfStrings(checksumIn)
	if err != nil {
		return nil, err
	}
	sis.SetUserData(userData)

	_, err = CheckFooter(checksumIn)
	if err != nil {
		return nil, err
	}

	return sis, nil
}

func (f *Lucene104SegmentInfosFormat) readSegmentCommitInfo(in store.IndexInput, dir store.Directory) (*spi.SegmentCommitInfo, error) {
	name, err := store.ReadString(in)
	if err != nil {
		return nil, err
	}

	id, err := in.ReadBytesN(16)
	if err != nil {
		return nil, err
	}

	codecName, err := store.ReadString(in)
	if err != nil {
		return nil, err
	}

	delGen, err := store.ReadInt64(in)
	if err != nil {
		return nil, err
	}

	delCount, err := store.ReadInt32(in)
	if err != nil {
		return nil, err
	}

	fieldInfosGen, err := store.ReadInt64(in)
	if err != nil {
		return nil, err
	}

	docValuesGen, err := store.ReadInt64(in)
	if err != nil {
		return nil, err
	}

	softDelCount, err := store.ReadInt32(in)
	if err != nil {
		return nil, err
	}

	hasSciID, err := in.ReadByte()
	if err != nil {
		return nil, err
	}
	var sciID []byte
	if hasSciID == 1 {
		sciID, err = in.ReadBytesN(16)
		if err != nil {
			return nil, err
		}
	}

	fieldInfosFiles, err := store.ReadSetOfStrings(in)
	if err != nil {
		return nil, err
	}

	docValuesUpdatesFiles, err := store.ReadMapOfIntToSetOfStrings(in)
	if err != nil {
		return nil, err
	}

	// For now, we don't have SegmentInfo fully populated from .si file here
	// In Lucene, it's loaded lazily or passed in.
	// We'll create a placeholder SegmentInfo.
	si := schema.NewSegmentInfo(name, 0, dir)
	si.SetID(id)
	si.SetCodec(codecName)

	sci := spi.NewSegmentCommitInfo(si, int(delCount), delGen)
	sci.SetFieldInfosGen(fieldInfosGen)
	sci.SetDocValuesGen(docValuesGen)
	sci.SetSoftDelCount(int(softDelCount))
	sci.SetID(sciID)
	sci.SetFieldInfosFiles(fieldInfosFiles)
	sci.SetDocValuesUpdatesFiles(docValuesUpdatesFiles)

	return sci, nil
}

func (f *Lucene104SegmentInfosFormat) Write(dir store.Directory, infos *spi.SegmentInfos, ctx store.IOContext) error {
	generation := infos.NextGeneration()
	fileName := spi.GetSegmentFileName(generation)

	out, err := dir.CreateOutput(fileName, ctx)
	if err != nil {
		return err
	}
	checksumOut := store.NewChecksumIndexOutput(out)
	defer checksumOut.Close()

	// Random ID for segments_N header
	id := make([]byte, 16)
	// In a real implementation, we should probably use a proper random source

	err = WriteIndexHeader(checksumOut, sisCodecName, sisVersion, id, strconv.FormatInt(generation, 36))
	if err != nil {
		return err
	}

	// Write Lucene version
	var major, minor, bugfix int32
	fmt.Sscanf(infos.LuceneVersion(), "%d.%d.%d", &major, &minor, &bugfix)
	store.WriteVInt(checksumOut, major)
	store.WriteVInt(checksumOut, minor)
	store.WriteVInt(checksumOut, bugfix)

	// Write created major
	store.WriteVInt(checksumOut, infos.IndexCreatedVersionMajor())

	// Write version
	store.WriteInt64(checksumOut, infos.Version())

	// Write counter
	store.WriteVLong(checksumOut, infos.Counter())

	// Write segment count
	segments := infos.List()
	store.WriteInt32(checksumOut, int32(len(segments)))

	// Write min segment version if any
	if len(segments) > 0 {
		// Just write current version as min version for now
		store.WriteVInt(checksumOut, major)
		store.WriteVInt(checksumOut, minor)
		store.WriteVInt(checksumOut, bugfix)
	}

	for _, sci := range segments {
		err = f.writeSegmentCommitInfo(checksumOut, sci)
		if err != nil {
			return err
		}
	}

	store.WriteMapOfStrings(checksumOut, infos.GetUserData())

	err = WriteFooter(checksumOut)
	if err != nil {
		return err
	}

	infos.SetLastGeneration(generation)
	return nil
}

func (f *Lucene104SegmentInfosFormat) writeSegmentCommitInfo(out store.IndexOutput, sci *spi.SegmentCommitInfo) error {
	store.WriteString(out, sci.Name())
	out.WriteBytes(sci.SegmentInfo().GetID())
	store.WriteString(out, sci.SegmentInfo().Codec())
	store.WriteInt64(out, sci.DelGen())
	store.WriteInt32(out, int32(sci.DelCount()))
	store.WriteInt64(out, sci.FieldInfosGen())
	store.WriteInt64(out, sci.DocValuesGen())
	store.WriteInt32(out, int32(sci.SoftDelCount()))

	sciID := sci.GetID()
	if len(sciID) == 16 {
		out.WriteByte(1)
		out.WriteBytes(sciID)
	} else {
		out.WriteByte(0)
	}

	store.WriteSetOfStrings(out, sci.FieldInfosFiles())
	store.WriteMapOfIntToSetOfStrings(out, sci.DocValuesUpdatesFiles())

	return nil
}

// Lucene99SegmentInfoFormat implements Lucene 9.9/10.4 segment info format (.si).
type Lucene99SegmentInfoFormat struct{}

func NewLucene99SegmentInfoFormat() *Lucene99SegmentInfoFormat {
	return &Lucene99SegmentInfoFormat{}
}

const (
	siFileCodecName = "Lucene90SegmentInfo"
	siFileVersion   = 0
)

func (f *Lucene99SegmentInfoFormat) Read(dir store.Directory, segmentName string, segmentID []byte, context store.IOContext) (*index.SegmentInfo, error) {
	fileName := GetSegmentFileName(segmentName, "", "si")
	in, err := dir.OpenInput(fileName, context)
	if err != nil {
		return nil, err
	}
	checksumIn := store.NewChecksumIndexInput(in)
	defer checksumIn.Close()

	_, err = CheckIndexHeader(checksumIn, siFileCodecName, siFileVersion, siFileVersion, segmentID, "")
	if err != nil {
		return nil, err
	}

	// Version fields use Java's DataOutput.writeInt (little-endian), not CodecUtil.writeBEInt.
	major, err := store.ReadInt32LE(checksumIn)
	if err != nil {
		return nil, err
	}
	minor, err := store.ReadInt32LE(checksumIn)
	if err != nil {
		return nil, err
	}
	bugfix, err := store.ReadInt32LE(checksumIn)
	if err != nil {
		return nil, err
	}
	luceneVersion := fmt.Sprintf("%d.%d.%d", major, minor, bugfix)

	hasMinVersion, err := checksumIn.ReadByte()
	if err != nil {
		return nil, err
	}
	var minVersion string
	switch hasMinVersion {
	case 0:
		// no minVersion
	case 1:
		minMajor, err := store.ReadInt32LE(checksumIn)
		if err != nil {
			return nil, err
		}
		minMinor, err := store.ReadInt32LE(checksumIn)
		if err != nil {
			return nil, err
		}
		minBugfix, err := store.ReadInt32LE(checksumIn)
		if err != nil {
			return nil, err
		}
		minVersion = fmt.Sprintf("%d.%d.%d", minMajor, minMinor, minBugfix)
	default:
		// Mirrors Lucene99SegmentInfoFormat: any value other than 0/1 is corrupt.
		return nil, fmt.Errorf("illegal hasMinVersion byte value: %d", hasMinVersion)
	}

	docCount, err := store.ReadInt32LE(checksumIn)
	if err != nil {
		return nil, err
	}

	isCompoundFileByte, err := checksumIn.ReadByte()
	if err != nil {
		return nil, err
	}
	// Lucene encodes this byte as SegmentInfo.YES (1) for compound and
	// SegmentInfo.NO (-1, i.e. 255 unsigned) for non-compound; the read is
	// `readByte() == SegmentInfo.YES` (Lucene90SegmentInfoFormat). A `!= 0`
	// test would misread every non-compound segment (255) as compound.
	isCompoundFile := isCompoundFileByte == 1

	// hasBlocks: Lucene reads `readByte() == SegmentInfo.YES`, so any non-1
	// byte (including the 255 "NO" sentinel) means false.
	hasBlocksByte, err := checksumIn.ReadByte()
	if err != nil {
		return nil, err
	}
	hasBlocks := hasBlocksByte == 1

	diagnostics, err := store.ReadMapOfStrings(checksumIn)
	if err != nil {
		return nil, err
	}

	files, err := store.ReadSetOfStrings(checksumIn)
	if err != nil {
		return nil, err
	}

	attributes, err := store.ReadMapOfStrings(checksumIn)
	if err != nil {
		return nil, err
	}

	// Index sort (numSortFields + per-field SortField), decoded in lock-step
	// with the index-package .si writer via index.ReadSegmentInfoSort (rmp
	// #4789). Keeping the two .si readers byte-aligned is what lets a segment
	// written by IndexWriter.writeSegmentInfo be reopened through the codec
	// SegmentInfoFormat at directory_reader.go.
	indexSort, err := index.ReadSegmentInfoSort(checksumIn)
	if err != nil {
		return nil, fmt.Errorf("index sort: %w", err)
	}

	_, err = CheckFooter(checksumIn)
	if err != nil {
		return nil, err
	}

	si := index.NewSegmentInfo(segmentName, int(docCount), dir)
	si.SetID(segmentID)
	si.SetVersion(luceneVersion)
	if minVersion != "" {
		si.SetMinVersion(minVersion)
	}
	si.SetHasBlocks(hasBlocks)
	si.SetCompoundFile(isCompoundFile)
	si.SetDiagnostics(diagnostics)
	fileList := make([]string, 0, len(files))
	for f := range files {
		fileList = append(fileList, f)
	}
	si.SetFiles(fileList)
	for k, v := range attributes {
		si.SetAttribute(k, v)
	}
	if indexSort != nil {
		si.SetIndexSort(indexSort)
	}

	return si, nil
}

func (f *Lucene99SegmentInfoFormat) Write(dir store.Directory, info *index.SegmentInfo, context store.IOContext) error {
	fileName := GetSegmentFileName(info.Name(), "", "si")
	out, err := dir.CreateOutput(fileName, context)
	if err != nil {
		return err
	}
	checksumOut := store.NewChecksumIndexOutput(out)
	defer checksumOut.Close()

	if err := WriteIndexHeader(checksumOut, siFileCodecName, siFileVersion, info.GetID(), ""); err != nil {
		return err
	}

	// Payload fields mirror Lucene99SegmentInfoFormat.writeSegmentInfo, which
	// uses DataOutput.writeInt (little-endian). Only the CodecUtil header/footer
	// framing is big-endian, so payload ints must use the LE helpers.
	major, minor, bugfix := parseVersion(info.Version())
	if err := store.WriteInt32LE(checksumOut, major); err != nil {
		return err
	}
	if err := store.WriteInt32LE(checksumOut, minor); err != nil {
		return err
	}
	if err := store.WriteInt32LE(checksumOut, bugfix); err != nil {
		return err
	}

	// hasMinVersion sentinel + optional minVersion ints, mirroring
	// Lucene99SegmentInfoFormat.writeSegmentInfo: writeByte(1) + 3 LE ints when
	// SegmentInfo.getMinVersion() != null, otherwise writeByte(0). (rmp #4784)
	if minVer, ok := info.MinVersion(); ok {
		if err := checksumOut.WriteByte(1); err != nil {
			return err
		}
		minMajor, minMinor, minBugfix := parseVersion(minVer)
		if err := store.WriteInt32LE(checksumOut, minMajor); err != nil {
			return err
		}
		if err := store.WriteInt32LE(checksumOut, minMinor); err != nil {
			return err
		}
		if err := store.WriteInt32LE(checksumOut, minBugfix); err != nil {
			return err
		}
	} else {
		if err := checksumOut.WriteByte(0); err != nil {
			return err
		}
	}

	if err := store.WriteInt32LE(checksumOut, int32(info.DocCount())); err != nil {
		return err
	}

	// isCompoundFile is written via Java's (byte) cast of SegmentInfo.YES (1)
	// / SegmentInfo.NO (-1). (byte)(-1) serializes as 0xFF == 255, so the
	// "not compound" sentinel is byte 255, matching Lucene exactly.
	isCompoundFile := byte(255)
	if info.IsCompoundFile() {
		isCompoundFile = 1
	}
	if err := checksumOut.WriteByte(isCompoundFile); err != nil {
		return err
	}

	// hasBlocks byte. Lucene writes (byte)(getHasBlocks() ? YES(1) : NO(-1)),
	// so false serialises to 0xFF == 255 (matching the isCompoundFile sentinel),
	// not literal 0. The reader compares the byte against YES. (rmp #4784)
	hasBlocks := byte(255)
	if info.HasBlocks() {
		hasBlocks = 1
	}
	if err := checksumOut.WriteByte(hasBlocks); err != nil {
		return err
	}

	if err := store.WriteMapOfStrings(checksumOut, info.GetDiagnostics()); err != nil {
		return err
	}

	files := make(map[string]struct{}, len(info.Files()))
	for _, f := range info.Files() {
		files[f] = struct{}{}
	}
	if err := store.WriteSetOfStrings(checksumOut, files); err != nil {
		return err
	}

	if err := store.WriteMapOfStrings(checksumOut, info.GetAttributes()); err != nil {
		return err
	}

	// Index sort: numSortFields followed by each SortField, byte-faithful to
	// Lucene90SegmentInfoFormat.write (rmp #4789).
	if err := index.WriteSegmentInfoSort(checksumOut, info.IndexSort()); err != nil {
		return fmt.Errorf("write index sort: %w", err)
	}

	return WriteFooter(checksumOut)
}

func (f *Lucene99SegmentInfoFormat) Name() string {
	return "Lucene99SegmentInfoFormat"
}

func parseVersion(v string) (int32, int32, int32) {
	var major, minor, bugfix int32
	fmt.Sscanf(v, "%d.%d.%d", &major, &minor, &bugfix)
	return major, minor, bugfix
}
