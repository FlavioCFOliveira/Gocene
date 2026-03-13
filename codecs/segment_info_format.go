// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// SegmentInfosFormat handles encoding/decoding of segment metadata (segments_N).
// This is the Go port of Lucene's org.apache.lucene.index.SegmentInfos.
type SegmentInfosFormat interface {
	// Name returns the name of this format.
	Name() string

	// Read reads segment infos from the given directory.
	Read(dir store.Directory) (*index.SegmentInfos, error)

	// Write writes segment infos to the given directory.
	Write(dir store.Directory, infos *index.SegmentInfos) error
}

// SegmentInfoFormat handles encoding/decoding of a single segment's metadata (.si file).
// This is the Go port of Lucene's org.apache.lucene.codecs.SegmentInfoFormat.
type SegmentInfoFormat interface {
	// Read reads segment info from the given directory.
	Read(dir store.Directory, segmentName string, segmentID []byte, context store.IOContext) (*index.SegmentInfo, error)

	// Write writes segment info to the given directory.
	Write(dir store.Directory, info *index.SegmentInfo, context store.IOContext) error
}

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

func (f *Lucene104SegmentInfosFormat) Read(dir store.Directory) (*index.SegmentInfos, error) {
	files, err := dir.ListAll()
	if err != nil {
		return nil, err
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
		return nil, err
	}
	checksumIn := store.NewChecksumIndexInput(in)
	defer checksumIn.Close()

	// Check header
	_, err = CheckIndexHeader(checksumIn, sisCodecName, sisVersion, sisVersion, nil, fmt.Sprintf("%d", maxGen))
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

	sis := index.NewSegmentInfos()
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

func (f *Lucene104SegmentInfosFormat) readSegmentCommitInfo(in store.IndexInput, dir store.Directory) (*index.SegmentCommitInfo, error) {
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
	si := index.NewSegmentInfo(name, 0, dir)
	si.SetID(id)
	si.SetCodec(codecName)

	sci := index.NewSegmentCommitInfo(si, int(delCount), delGen)
	sci.SetFieldInfosGen(fieldInfosGen)
	sci.SetDocValuesGen(docValuesGen)
	sci.SetSoftDelCount(int(softDelCount))
	sci.SetID(sciID)
	sci.SetFieldInfosFiles(fieldInfosFiles)
	sci.SetDocValuesUpdatesFiles(docValuesUpdatesFiles)

	return sci, nil
}

func (f *Lucene104SegmentInfosFormat) Write(dir store.Directory, infos *index.SegmentInfos) error {
	generation := infos.NextGeneration()
	fileName := index.GetSegmentFileName(generation)

	out, err := dir.CreateOutput(fileName, store.IOContextWrite)
	if err != nil {
		return err
	}
	checksumOut := store.NewChecksumIndexOutput(out)
	defer checksumOut.Close()

	// Random ID for segments_N header
	id := make([]byte, 16)
	// In a real implementation, we should probably use a proper random source

	err = WriteIndexHeader(checksumOut, sisCodecName, sisVersion, id, fmt.Sprintf("%d", generation))
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

func (f *Lucene104SegmentInfosFormat) writeSegmentCommitInfo(out store.IndexOutput, sci *index.SegmentCommitInfo) error {
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

	major, err := store.ReadInt32(checksumIn)
	if err != nil {
		return nil, err
	}
	minor, err := store.ReadInt32(checksumIn)
	if err != nil {
		return nil, err
	}
	bugfix, err := store.ReadInt32(checksumIn)
	if err != nil {
		return nil, err
	}
	luceneVersion := fmt.Sprintf("%d.%d.%d", major, minor, bugfix)

	hasMinVersion, err := checksumIn.ReadByte()
	if err != nil {
		return nil, err
	}
	if hasMinVersion != 0 {
		_, err = store.ReadInt32(checksumIn) // minMajor
		if err != nil {
			return nil, err
		}
		_, err = store.ReadInt32(checksumIn) // minMinor
		if err != nil {
			return nil, err
		}
		_, err = store.ReadInt32(checksumIn) // minBugfix
		if err != nil {
			return nil, err
		}
	}

	docCount, err := store.ReadInt32(checksumIn)
	if err != nil {
		return nil, err
	}

	isCompoundFileByte, err := checksumIn.ReadByte()
	if err != nil {
		return nil, err
	}
	isCompoundFile := isCompoundFileByte != 0

	_, err = checksumIn.ReadByte() // hasBlocks
	if err != nil {
		return nil, err
	}

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

	// Index Sort
	numSortFields, err := store.ReadVInt(checksumIn)
	if err != nil {
		return nil, err
	}
	if numSortFields > 0 {
		return nil, fmt.Errorf("index sort not yet supported in SegmentInfoFormat")
	}

	_, err = CheckFooter(checksumIn)
	if err != nil {
		return nil, err
	}

	si := index.NewSegmentInfo(segmentName, int(docCount), dir)
	si.SetID(segmentID)
	si.SetVersion(luceneVersion)
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

	err = WriteIndexHeader(checksumOut, siFileCodecName, siFileVersion, info.GetID(), "")
	if err != nil {
		return err
	}

	major, minor, bugfix := parseVersion(info.Version())
	store.WriteInt32(checksumOut, major)
	store.WriteInt32(checksumOut, minor)
	store.WriteInt32(checksumOut, bugfix)

	checksumOut.WriteByte(0) // hasMinVersion = false for now

	store.WriteInt32(checksumOut, int32(info.DocCount()))

	isCompoundFile := byte(255) // -1 in Java signed byte
	if info.IsCompoundFile() {
		isCompoundFile = 1
	}
	checksumOut.WriteByte(isCompoundFile)

	checksumOut.WriteByte(0) // hasBlocks = false

	store.WriteMapOfStrings(checksumOut, info.GetDiagnostics())

	files := make(map[string]struct{})
	for _, f := range info.Files() {
		files[f] = struct{}{}
	}
	store.WriteSetOfStrings(checksumOut, files)

	store.WriteMapOfStrings(checksumOut, info.GetAttributes())

	store.WriteVInt(checksumOut, 0) // numSortFields = 0

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
