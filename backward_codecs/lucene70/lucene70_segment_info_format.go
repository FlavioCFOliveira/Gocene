// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene70

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	bcstore "github.com/FlavioCFOliveira/Gocene/backward_codecs/store"
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	gstore "github.com/FlavioCFOliveira/Gocene/store"
)

const (
	lucene70SIExtension    = "si"
	lucene70SICodecName    = "Lucene70SegmentInfo"
	lucene70SIVersionStart = int32(0)
	lucene70SIVersionEnd   = int32(0)

	// footerMagic mirrors CodecUtil.FOOTER_MAGIC.
	lucene70FooterMagic = int32(^0x3FD76C17)
)

// Lucene70SegmentInfoFormat implements the Lucene 7.0 segment info format.
//
// .si file layout:
//
//	Header, SegVersion(3×Int32), hasMinVersion(Int8),
//	[minVersion(3×Int32)], docCount(Int32), isCompoundFile(Int8),
//	Diagnostics(Map<String,String>), Files(Set<String>),
//	Attributes(Map<String,String>), numSortFields(VInt),
//	[sortFields…], Footer
//
// Write always returns an error — old formats are read-only.
//
// Port of org.apache.lucene.backward_codecs.lucene70.Lucene70SegmentInfoFormat
// (Lucene 10.4.0).
type Lucene70SegmentInfoFormat struct{}

// NewLucene70SegmentInfoFormat creates a Lucene70SegmentInfoFormat.
func NewLucene70SegmentInfoFormat() *Lucene70SegmentInfoFormat {
	return &Lucene70SegmentInfoFormat{}
}

// Read reads a Lucene 7.0 .si file and returns the corresponding SegmentInfo.
//
// Port of Lucene70SegmentInfoFormat.read(Directory, String, byte[], IOContext).
func (f *Lucene70SegmentInfoFormat) Read(
	dir gstore.Directory,
	segmentName string,
	segmentID []byte,
	context gstore.IOContext,
) (*index.SegmentInfo, error) {
	fileName := codecs.GetSegmentFileName(segmentName, "", lucene70SIExtension)
	in, err := bcstore.OpenChecksumInput(dir, fileName, context)
	if err != nil {
		return nil, fmt.Errorf("lucene70 segment info: open %q: %w", fileName, err)
	}
	defer in.Close()

	var parseErr error
	var si *index.SegmentInfo

	_, parseErr = codecs.CheckIndexHeader(
		in,
		lucene70SICodecName,
		lucene70SIVersionStart,
		lucene70SIVersionEnd,
		segmentID,
		"",
	)
	if parseErr == nil {
		si, parseErr = readSegmentInfo70(in, dir, segmentName, segmentID)
	}

	if footerErr := checkLucene70SIFooter(in); footerErr != nil {
		if parseErr != nil {
			return nil, parseErr
		}
		return nil, footerErr
	}
	if parseErr != nil {
		return nil, parseErr
	}
	return si, nil
}

// Write writes a Lucene 7.0 .si file.
//
// Port of Lucene70SegmentInfoFormat.write (Lucene 10.4.0). Production old
// segment-info formats are read-only; Lucene provides the test-only writer as
// Lucene70RWSegmentInfoFormat.
func (f *Lucene70SegmentInfoFormat) Write(
	dir gstore.Directory,
	si *index.SegmentInfo,
	ctx gstore.IOContext,
) error {
	return fmt.Errorf("lucene70 segment info: old formats cannot be used for writing")
}

// parseSegmentVersion parses a version string such as "10.4.0" into its
// three numeric components. Mirrors index.parseSegmentVersion.
func parseSegmentVersion(v string) (major, minor, bugfix int32) {
	parts := strings.Split(v, ".")
	if len(parts) >= 1 {
		if m, err := strconv.Atoi(parts[0]); err == nil {
			major = int32(m)
		}
	}
	if len(parts) >= 2 {
		if m, err := strconv.Atoi(parts[1]); err == nil {
			minor = int32(m)
		}
	}
	if len(parts) >= 3 {
		if m, err := strconv.Atoi(parts[2]); err == nil {
			bugfix = int32(m)
		}
	}
	return
}

// compile-time assertion
var _ codecs.SegmentInfoFormat = (*Lucene70SegmentInfoFormat)(nil)

// readSegmentInfo70 deserialises the body of a Lucene 7.0 .si file.
func readSegmentInfo70(
	in *bcstore.EndiannessReverserChecksumIndexInput,
	dir gstore.Directory,
	segmentName string,
	segmentID []byte,
) (*index.SegmentInfo, error) {
	// SegVersion: major.minor.bugfix as three Int32.
	major, err := in.ReadInt()
	if err != nil {
		return nil, fmt.Errorf("lucene70 segment info: version major: %w", err)
	}
	minor, err := in.ReadInt()
	if err != nil {
		return nil, fmt.Errorf("lucene70 segment info: version minor: %w", err)
	}
	bugfix, err := in.ReadInt()
	if err != nil {
		return nil, fmt.Errorf("lucene70 segment info: version bugfix: %w", err)
	}
	luceneVersion := fmt.Sprintf("%d.%d.%d", major, minor, bugfix)

	// hasMinVersion: 0=absent, 1=present.
	hasMinVersion, err := in.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("lucene70 segment info: hasMinVersion: %w", err)
	}
	switch hasMinVersion {
	case 0:
		// minVersion absent — skip
	case 1:
		if _, err := in.ReadInt(); err != nil { // minMajor
			return nil, fmt.Errorf("lucene70 segment info: minVersion major: %w", err)
		}
		if _, err := in.ReadInt(); err != nil { // minMinor
			return nil, fmt.Errorf("lucene70 segment info: minVersion minor: %w", err)
		}
		if _, err := in.ReadInt(); err != nil { // minBugfix
			return nil, fmt.Errorf("lucene70 segment info: minVersion bugfix: %w", err)
		}
	default:
		return nil, fmt.Errorf("lucene70 segment info: illegal hasMinVersion value %d", hasMinVersion)
	}

	// docCount as Int32.
	docCountI32, err := in.ReadInt()
	if err != nil {
		return nil, fmt.Errorf("lucene70 segment info: docCount: %w", err)
	}
	docCount := int(docCountI32)
	if docCount < 0 {
		return nil, fmt.Errorf("lucene70 segment info: invalid docCount %d", docCount)
	}

	// isCompoundFile: -1=no, 1=yes (Java: SegmentInfo.YES=1).
	isCompoundFileByte, err := in.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("lucene70 segment info: isCompoundFile: %w", err)
	}
	isCompoundFile := int8(isCompoundFileByte) == 1

	// Diagnostics, Files, Attributes.
	diagnostics, err := gstore.ReadMapOfStrings(in)
	if err != nil {
		return nil, fmt.Errorf("lucene70 segment info: diagnostics: %w", err)
	}
	files, err := gstore.ReadSetOfStrings(in)
	if err != nil {
		return nil, fmt.Errorf("lucene70 segment info: files: %w", err)
	}
	attributes, err := gstore.ReadMapOfStrings(in)
	if err != nil {
		return nil, fmt.Errorf("lucene70 segment info: attributes: %w", err)
	}

	// IndexSort.
	indexSort, err := readIndexSort70(in)
	if err != nil {
		return nil, fmt.Errorf("lucene70 segment info: index sort: %w", err)
	}

	si := index.NewSegmentInfo(segmentName, docCount, dir)
	if err := si.SetID(segmentID); err != nil {
		return nil, fmt.Errorf("lucene70 segment info: set ID: %w", err)
	}
	si.SetVersion(luceneVersion)
	si.SetCompoundFile(isCompoundFile)
	si.SetDiagnostics(diagnostics)
	for k, v := range attributes {
		si.SetAttribute(k, v)
	}
	fileList := make([]string, 0, len(files))
	for f := range files {
		fileList = append(fileList, f)
	}
	si.SetFiles(fileList)
	if indexSort != nil {
		si.SetIndexSort(indexSort)
	}
	return si, nil
}

// readIndexSort70 reads the optional IndexSort from a Lucene 7.0 .si stream.
func readIndexSort70(in *bcstore.EndiannessReverserChecksumIndexInput) (*index.Sort, error) {
	numSortFieldsI32, err := gstore.ReadVInt(in)
	if err != nil {
		return nil, fmt.Errorf("numSortFields: %w", err)
	}
	numSortFields := int(numSortFieldsI32)
	if numSortFields < 0 {
		return nil, fmt.Errorf("invalid index sort field count: %d", numSortFields)
	}
	if numSortFields == 0 {
		return nil, nil
	}

	sortFields := make([]index.SortField, numSortFields)
	for i := 0; i < numSortFields; i++ {
		sf, err := readSortField70(in)
		if err != nil {
			return nil, fmt.Errorf("sort field %d: %w", i, err)
		}
		sortFields[i] = sf
	}
	return index.NewSort(sortFields...), nil
}

// readSortField70 reads one SortField from the stream.
//
// Encoding:
//
//	fieldName (String)
//	sortTypeID (VInt): 0=STRING 1=LONG 2=INT 3=DOUBLE 4=FLOAT
//	                   5=SortedSet 6=SortedNumeric
//	reverse (Int8): 0=reversed, 1=natural
//	missingValue (Int8 flag, then type-specific value)
func readSortField70(in *bcstore.EndiannessReverserChecksumIndexInput) (index.SortField, error) {
	fieldName, err := in.ReadString()
	if err != nil {
		return index.SortField{}, fmt.Errorf("field name: %w", err)
	}
	sortTypeIDI32, err := gstore.ReadVInt(in)
	if err != nil {
		return index.SortField{}, fmt.Errorf("sortTypeID: %w", err)
	}
	sortTypeID := int(sortTypeIDI32)

	var sortType index.SortType
	var selector string // for SortedSet / SortedNumeric

	switch sortTypeID {
	case 0:
		sortType = index.SortTypeString
	case 1:
		sortType = index.SortTypeLong
	case 2:
		sortType = index.SortTypeInt
	case 3:
		sortType = index.SortTypeDouble
	case 4:
		sortType = index.SortTypeFloat
	case 5:
		// SortedSetSortField: selector follows.
		sortType = index.SortTypeString
		sel, err := in.ReadByte()
		if err != nil {
			return index.SortField{}, fmt.Errorf("SortedSet selector: %w", err)
		}
		switch sel {
		case 0:
			selector = "min"
		case 1:
			selector = "max"
		case 2:
			selector = "middle_min"
		case 3:
			selector = "middle_max"
		default:
			return index.SortField{}, fmt.Errorf("invalid SortedSetSelector ID: %d", sel)
		}
	case 6:
		// SortedNumericSortField: numeric type + selector follow.
		numType, err := in.ReadByte()
		if err != nil {
			return index.SortField{}, fmt.Errorf("SortedNumeric type: %w", err)
		}
		switch numType {
		case 0:
			sortType = index.SortTypeLong
		case 1:
			sortType = index.SortTypeInt
		case 2:
			sortType = index.SortTypeDouble
		case 3:
			sortType = index.SortTypeFloat
		default:
			return index.SortField{}, fmt.Errorf("invalid SortedNumericSortField type ID: %d", numType)
		}
		numSel, err := in.ReadByte()
		if err != nil {
			return index.SortField{}, fmt.Errorf("SortedNumeric selector: %w", err)
		}
		switch numSel {
		case 0:
			selector = "min"
		case 1:
			selector = "max"
		default:
			return index.SortField{}, fmt.Errorf("invalid SortedNumericSelector ID: %d", numSel)
		}
	default:
		return index.SortField{}, fmt.Errorf("invalid index sort field type ID: %d", sortTypeID)
	}

	// reverse byte: 0=descending, 1=ascending.
	reverseByte, err := in.ReadByte()
	if err != nil {
		return index.SortField{}, fmt.Errorf("reverse: %w", err)
	}
	var reverse bool
	switch reverseByte {
	case 0:
		reverse = true
	case 1:
		reverse = false
	default:
		return index.SortField{}, fmt.Errorf("invalid index sort reverse: %d", reverseByte)
	}

	// missingValue flag.
	missingFlag, err := in.ReadByte()
	if err != nil {
		return index.SortField{}, fmt.Errorf("missingValue flag: %w", err)
	}
	var missingValue interface{}
	if missingFlag != 0 {
		switch sortType {
		case index.SortTypeString:
			switch missingFlag {
			case 1:
				missingValue = "LAST"
			case 2:
				missingValue = "FIRST"
			default:
				return index.SortField{}, fmt.Errorf("invalid STRING missing value flag: %d", missingFlag)
			}
		case index.SortTypeLong:
			if missingFlag != 1 {
				return index.SortField{}, fmt.Errorf("invalid LONG missing value flag: %d", missingFlag)
			}
			v, err := in.ReadLong()
			if err != nil {
				return index.SortField{}, fmt.Errorf("LONG missing value: %w", err)
			}
			missingValue = v
		case index.SortTypeInt:
			if missingFlag != 1 {
				return index.SortField{}, fmt.Errorf("invalid INT missing value flag: %d", missingFlag)
			}
			v, err := in.ReadInt()
			if err != nil {
				return index.SortField{}, fmt.Errorf("INT missing value: %w", err)
			}
			missingValue = v
		case index.SortTypeDouble:
			if missingFlag != 1 {
				return index.SortField{}, fmt.Errorf("invalid DOUBLE missing value flag: %d", missingFlag)
			}
			v, err := in.ReadLong()
			if err != nil {
				return index.SortField{}, fmt.Errorf("DOUBLE missing value: %w", err)
			}
			missingValue = math.Float64frombits(uint64(v))
		case index.SortTypeFloat:
			if missingFlag != 1 {
				return index.SortField{}, fmt.Errorf("invalid FLOAT missing value flag: %d", missingFlag)
			}
			v, err := in.ReadInt()
			if err != nil {
				return index.SortField{}, fmt.Errorf("FLOAT missing value: %w", err)
			}
			missingValue = math.Float32frombits(uint32(v))
		}
	}

	sf := index.NewSortField(fieldName, sortType)
	sf.SetReverse(reverse)
	if missingValue != nil {
		sf.SetMissingValue(missingValue)
	}
	_ = selector // selector is stored internally in the Java SortField subclass;
	// Gocene's SortField struct does not expose selector construction publicly yet.
	return sf, nil
}

// checkLucene70SIFooter validates the codec footer of a Lucene 7.0 .si file.
// The footer layout (big-endian, 16 bytes) is:
//
//	magic (Int32) | algorithmID (Int32) | checksum (Int64)
func checkLucene70SIFooter(in *bcstore.EndiannessReverserChecksumIndexInput) error {
	remaining := in.Length() - in.GetFilePointer()
	const footerLen = 16
	if remaining < footerLen {
		return fmt.Errorf("lucene70 segment info: misplaced footer (too short): remaining=%d", remaining)
	}
	if remaining > footerLen {
		return fmt.Errorf("lucene70 segment info: misplaced footer (too long): remaining=%d", remaining)
	}
	magic, err := gstore.ReadInt32(in)
	if err != nil {
		return fmt.Errorf("lucene70 segment info: footer magic: %w", err)
	}
	if magic != lucene70FooterMagic {
		return fmt.Errorf("lucene70 segment info: footer magic mismatch: got %x want %x", magic, lucene70FooterMagic)
	}
	algID, err := gstore.ReadInt32(in)
	if err != nil {
		return fmt.Errorf("lucene70 segment info: footer algorithmID: %w", err)
	}
	if algID != 0 {
		return fmt.Errorf("lucene70 segment info: unknown algorithmID: %d", algID)
	}
	actualChecksum := int64(in.GetChecksum())
	expected, err := gstore.ReadInt64(in)
	if err != nil {
		return fmt.Errorf("lucene70 segment info: footer checksum: %w", err)
	}
	if actualChecksum != expected {
		return fmt.Errorf("lucene70 segment info: checksum mismatch: actual=%x expected=%x",
			actualChecksum, expected)
	}
	return nil
}
