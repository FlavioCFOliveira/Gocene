// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// SegmentInfoFormat is an alias of spi.SegmentInfoFormat.
type SegmentInfoFormat = spi.SegmentInfoFormat

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

	// Index sort. Mirrors Lucene99SegmentInfoFormat.parseSegmentInfo:
	//
	//	int numSortFields = input.readVInt();
	//	if (numSortFields > 0) {
	//	  for (...) { String name = input.readString();
	//	              sortFields[i] = SortFieldProvider.forName(name).readSortField(input); }
	//	  indexSort = new Sort(sortFields);
	//	} else if (numSortFields < 0) {
	//	  throw new CorruptIndexException("invalid index sort field count: " + numSortFields, input);
	//	} else { indexSort = null; }
	numSortFields, err := checksumIn.ReadVInt()
	if err != nil {
		return nil, err
	}
	var indexSort *index.Sort
	if numSortFields > 0 {
		sortFields := make([]*index.SortField, numSortFields)
		for i := range sortFields {
			providerName, err := checksumIn.ReadString()
			if err != nil {
				return nil, err
			}
			provider, err := index.LookupSortFieldProvider(providerName)
			if err != nil {
				return nil, err
			}
			value, err := provider.ReadSortField(checksumIn)
			if err != nil {
				return nil, err
			}
			sortField, ok := value.(*index.SortField)
			if !ok {
				return nil, fmt.Errorf("sort field provider %q returned %T, not a SortField", providerName, value)
			}
			sortFields[i] = sortField
		}
		indexSort = index.NewSort(sortFields...)
	} else if numSortFields < 0 {
		return nil, fmt.Errorf("invalid index sort field count: %d", numSortFields)
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
	si.SetFiles(files)
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
	if info.GetHasBlocks() {
		hasBlocks = 1
	}
	if err := checksumOut.WriteByte(hasBlocks); err != nil {
		return err
	}

	if err := checksumOut.WriteMapOfStrings(info.GetDiagnostics()); err != nil {
		return err
	}

	if err := checksumOut.WriteSetOfStrings(info.Files()); err != nil {
		return err
	}

	if err := checksumOut.WriteMapOfStrings(info.GetAttributes()); err != nil {
		return err
	}

	// Index sort. Mirrors Lucene99SegmentInfoFormat.writeSegmentInfo:
	//
	//	int numSortFields = indexSort == null ? 0 : indexSort.getSort().length;
	//	output.writeVInt(numSortFields);
	//	for (...) {
	//	  IndexSorter sorter = sortField.getIndexSorter();
	//	  if (sorter == null) throw new IllegalArgumentException("cannot serialize SortField " + sortField);
	//	  output.writeString(sorter.getProviderName());
	//	  SortFieldProvider.write(sortField, output);
	//	}
	//
	// sortField.getIndexSorter().getProviderName() is rendered by the
	// index.SortFieldNamer view (see index/sort_field_provider.go).
	var sortFields []*index.SortField
	if indexSort := info.IndexSort(); indexSort != nil {
		sortFields = indexSort.Fields()
	}
	if err := checksumOut.WriteVInt(int32(len(sortFields))); err != nil {
		return err
	}
	for _, sortField := range sortFields {
		var namer any = sortField
		sorter, ok := namer.(index.SortFieldNamer)
		if !ok || sorter.ProviderName() == "" {
			return fmt.Errorf("%w: %v", index.ErrSortFieldNotSerializable, sortField)
		}
		if err := checksumOut.WriteString(sorter.ProviderName()); err != nil {
			return err
		}
		if err := index.WriteSortField(sortField, checksumOut); err != nil {
			return err
		}
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
