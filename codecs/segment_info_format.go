// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/index"
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

	// Read version. BE long: CodecUtil.readBELong (SegmentInfos.java:372).
	version, err := store.ReadBELong(checksumIn)
	if err != nil {
		return nil, err
	}

	// Read counter
	counter, err := checksumIn.ReadVLong()
	if err != nil {
		return nil, err
	}

	// Read segment count. BE int: CodecUtil.readBEInt (SegmentInfos.java:374).
	numSegments, err := store.ReadBEInt(checksumIn)
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

	userData, err := checksumIn.ReadMapOfStrings()
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

	// delGen / delCount / fieldInfosGen / dvGen / softDelCount are big-endian:
	// CodecUtil.readBELong / readBEInt (SegmentInfos.java:400-409).
	delGen, err := store.ReadBELong(in)
	if err != nil {
		return nil, err
	}

	delCount, err := store.ReadBEInt(in)
	if err != nil {
		return nil, err
	}

	fieldInfosGen, err := store.ReadBELong(in)
	if err != nil {
		return nil, err
	}

	docValuesGen, err := store.ReadBELong(in)
	if err != nil {
		return nil, err
	}

	softDelCount, err := store.ReadBEInt(in)
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

	fieldInfosFiles, err := in.ReadSetOfStrings()
	if err != nil {
		return nil, err
	}

	docValuesUpdatesFiles, err := readDocValuesUpdatesFiles(in)
	if err != nil {
		return nil, err
	}

	// For now, we don't have SegmentInfo fully populated from .si file here
	// In Lucene, it's loaded lazily or passed in.
	// We'll create a placeholder SegmentInfo.
	si := spi.NewSegmentInfo(name, 0, dir)
	si.SetID(id)
	si.SetCodecName(codecName)

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
	tempFileName := "pending_" + fileName

	err := func() error {
		out, err := dir.CreateOutput(tempFileName, ctx)
		if err != nil {
			return err
		}
		checksumOut := store.NewChecksumIndexOutput(out)
		defer checksumOut.Close()

		// Random ID for segments_N header
		id := make([]byte, 16)
		// In a real implementation, we should probably use a proper random source

		if err := WriteIndexHeader(checksumOut, sisCodecName, sisVersion, id, strconv.FormatInt(generation, 36)); err != nil {
			return err
		}

		// Write Lucene version
		var major, minor, bugfix int32
		fmt.Sscanf(infos.LuceneVersion(), "%d.%d.%d", &major, &minor, &bugfix)
		checksumOut.WriteVInt(major)
		checksumOut.WriteVInt(minor)
		checksumOut.WriteVInt(bugfix)

		// Write created major
		checksumOut.WriteVInt(infos.IndexCreatedVersionMajor())

		// Write version. BE long: CodecUtil.writeBELong (SegmentInfos.java).
		store.WriteBELong(checksumOut, infos.Version())

		// Write counter
		checksumOut.WriteVLong(infos.Counter())

		// Write segment count. BE int: CodecUtil.writeBEInt (SegmentInfos.java).
		segments := infos.List()
		store.WriteBEInt(checksumOut, int32(len(segments)))

		// Write min segment version if any
		if len(segments) > 0 {
			// Just write current version as min version for now
			checksumOut.WriteVInt(major)
			checksumOut.WriteVInt(minor)
			checksumOut.WriteVInt(bugfix)
		}

		for _, sci := range segments {
			if err := f.writeSegmentCommitInfo(checksumOut, sci); err != nil {
				return err
			}
		}

		checksumOut.WriteMapOfStrings(infos.GetUserData())

		if err := WriteFooter(checksumOut); err != nil {
			return err
		}
		return nil
	}()

	if err != nil {
		_ = dir.DeleteFile(tempFileName)
		return err
	}

	if err := dir.Rename(tempFileName, fileName); err != nil {
		_ = dir.DeleteFile(tempFileName)
		return err
	}

	infos.SetLastGeneration(generation)
	return nil
}

func (f *Lucene104SegmentInfosFormat) writeSegmentCommitInfo(out store.IndexOutput, sci *spi.SegmentCommitInfo) error {
	store.WriteString(out, sci.Name())
	out.WriteBytes(sci.SegmentInfo().GetID(), 0, len(sci.SegmentInfo().GetID()))
	store.WriteString(out, sci.SegmentInfo().CodecName())
	// SegmentInfos.java:658-682: every one of these is written big-endian via
	// CodecUtil.writeBELong / CodecUtil.writeBEInt.
	store.WriteBELong(out, sci.DelGen())
	store.WriteBEInt(out, int32(sci.DelCount()))
	store.WriteBELong(out, sci.FieldInfosGen())
	store.WriteBELong(out, sci.DocValuesGen())
	store.WriteBEInt(out, int32(sci.SoftDelCount()))

	sciID := sci.GetID()
	if len(sciID) == 16 {
		out.WriteByte(1)
		out.WriteBytes(sciID, 0, len(sciID))
	} else {
		out.WriteByte(0)
	}

	out.WriteSetOfStrings(sci.FieldInfosFiles())
	if err := writeDocValuesUpdatesFiles(out, sci.DocValuesUpdatesFiles()); err != nil {
		return err
	}

	return nil
}

// readDocValuesUpdatesFiles reads the docValuesUpdatesFiles map exactly as
// SegmentInfos.java:441-449 writes it: the entry count as a BIG-endian int32
// (CodecUtil.readBEInt), then per entry the field number as a BIG-endian int32
// followed by readSetOfStrings. Lucene never uses a vInt here.
func readDocValuesUpdatesFiles(in store.IndexInput) (map[int]map[string]struct{}, error) {
	numDVFields, err := store.ReadBEInt(in)
	if err != nil {
		return nil, err
	}
	if numDVFields == 0 {
		return map[int]map[string]struct{}{}, nil
	}
	m := make(map[int]map[string]struct{}, int(numDVFields))
	for i := int32(0); i < numDVFields; i++ {
		key, err := store.ReadBEInt(in)
		if err != nil {
			return nil, err
		}
		files, err := in.ReadSetOfStrings()
		if err != nil {
			return nil, err
		}
		set := make(map[string]struct{}, len(files))
		for _, f := range files {
			set[f] = struct{}{}
		}
		m[int(key)] = set
	}
	return m, nil
}

// writeDocValuesUpdatesFiles writes the docValuesUpdatesFiles map exactly as
// SegmentInfos.java:696-701 does: the entry count as a BIG-endian int32
// (CodecUtil.writeBEInt), then per entry the field number as a BIG-endian int32
// followed by writeSetOfStrings. Java iterates entrySet(), which is unordered;
// the keys are not sorted.
func writeDocValuesUpdatesFiles(out store.IndexOutput, m map[int]map[string]struct{}) error {
	if err := store.WriteBEInt(out, int32(len(m))); err != nil {
		return err
	}
	for k, v := range m {
		if err := store.WriteBEInt(out, int32(k)); err != nil {
			return err
		}
		files := make([]string, 0, len(v))
		for f := range v {
			files = append(files, f)
		}
		if err := out.WriteSetOfStrings(files); err != nil {
			return err
		}
	}
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
	major, err := checksumIn.ReadInt()
	if err != nil {
		return nil, err
	}
	minor, err := checksumIn.ReadInt()
	if err != nil {
		return nil, err
	}
	bugfix, err := checksumIn.ReadInt()
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
		minMajor, err := checksumIn.ReadInt()
		if err != nil {
			return nil, err
		}
		minMinor, err := checksumIn.ReadInt()
		if err != nil {
			return nil, err
		}
		minBugfix, err := checksumIn.ReadInt()
		if err != nil {
			return nil, err
		}
		minVersion = fmt.Sprintf("%d.%d.%d", minMajor, minMinor, minBugfix)
	default:
		// Mirrors Lucene99SegmentInfoFormat: any value other than 0/1 is corrupt.
		return nil, fmt.Errorf("illegal hasMinVersion byte value: %d", hasMinVersion)
	}

	docCount, err := checksumIn.ReadInt()
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

	diagnostics, err := checksumIn.ReadMapOfStrings()
	if err != nil {
		return nil, err
	}

	files, err := checksumIn.ReadSetOfStrings()
	if err != nil {
		return nil, err
	}

	attributes, err := checksumIn.ReadMapOfStrings()
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
	if err := checksumOut.WriteInt(major); err != nil {
		return err
	}
	if err := checksumOut.WriteInt(minor); err != nil {
		return err
	}
	if err := checksumOut.WriteInt(bugfix); err != nil {
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
		if err := checksumOut.WriteInt(minMajor); err != nil {
			return err
		}
		if err := checksumOut.WriteInt(minMinor); err != nil {
			return err
		}
		if err := checksumOut.WriteInt(minBugfix); err != nil {
			return err
		}
	} else {
		if err := checksumOut.WriteByte(0); err != nil {
			return err
		}
	}

	if err := checksumOut.WriteInt(int32(info.DocCount())); err != nil {
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

	if err := checksumOut.WriteMapOfStrings(info.GetDiagnostics()); err != nil {
		return err
	}

	files := make(map[string]struct{}, len(info.Files()))
	for _, f := range info.Files() {
		files[f] = struct{}{}
	}
	if err := checksumOut.WriteSetOfStrings(files); err != nil {
		return err
	}

	if err := checksumOut.WriteMapOfStrings(info.GetAttributes()); err != nil {
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
