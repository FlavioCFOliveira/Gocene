// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package simpletext

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// SimpleTextSegmentInfoFormat writes segment metadata as text.
// Mirrors org.apache.lucene.codecs.simpletext.SimpleTextSegmentInfoFormat.
type SimpleTextSegmentInfoFormat struct{}

// NewSimpleTextSegmentInfoFormat builds the format.
func NewSimpleTextSegmentInfoFormat() *SimpleTextSegmentInfoFormat {
	return &SimpleTextSegmentInfoFormat{}
}

// siExtension is the extension of the segment info file.
// Port of SimpleTextSegmentInfoFormat.SI_EXTENSION (line 73).
const siExtension = "si"

// The line prefixes of the .si file, byte-for-byte ports of the BytesRef
// constants of SimpleTextSegmentInfoFormat (lines 54-71). They are the on-disk
// contract; the trailing spaces are part of it.
var (
	siVersion     = []byte("    version ")
	siMinVersion  = []byte("    min version ")
	siDocCount    = []byte("    number of documents ")
	siUseCompound = []byte("    uses compound file ")
	siHasBlocks   = []byte("    has blocks ")
	siNumDiag     = []byte("    diagnostics ")
	siDiagKey     = []byte("      key ")
	siDiagValue   = []byte("      value ")
	siNumAtt      = []byte("    attributes ")
	siAttKey      = []byte("      key ")
	siAttValue    = []byte("      value ")
	siNumFiles    = []byte("    files ")
	siFile        = []byte("      file ")
	siID          = []byte("    id ")
	siSort        = []byte("    sort ")
	siSortType    = []byte("      type ")
	siSortName    = []byte("      name ")
	siSortBytes   = []byte("      bytes ")
)

func (f *SimpleTextSegmentInfoFormat) Name() string {
	return "SimpleTextSegmentInfoFormat"
}

// readString returns everything after the prefix of the line just read.
// Port of SimpleTextSegmentInfoFormat.readString(int, BytesRefBuilder)
// (line 229).
func (f *SimpleTextSegmentInfoFormat) readString(line, prefix []byte) (string, error) {
	if !bytes.HasPrefix(line, prefix) {
		return "", fmt.Errorf("SimpleTextSegmentInfoFormat: expected %q, got %q", prefix, line)
	}
	return string(line[len(prefix):]), nil
}

func (f *SimpleTextSegmentInfoFormat) Read(dir store.Directory, segmentName string, segmentID []byte, context store.IOContext) (*index.SegmentInfo, error) {
	segFileName := index.SegmentFileName(segmentName, "", siExtension)
	// Java: ChecksumIndexInput input = directory.openChecksumInput(segFileName)
	// (line 82). store.Directory has no OpenChecksumInput; the in-tree
	// rendering is OpenInput + store.NewChecksumIndexInput
	// (codecs/segment_info_format.go:32-36).
	raw, err := dir.OpenInput(segFileName, context)
	if err != nil {
		return nil, err
	}
	in := store.NewChecksumIndexInput(raw)
	defer in.Close()

	readVal := func(prefix []byte) (string, error) {
		line, err := ReadLine(in)
		if err != nil {
			return "", err
		}
		return f.readString(line, prefix)
	}

	// final Version version = Version.parse(...);              (line 87)
	versionStr, err := readVal(siVersion)
	if err != nil {
		return nil, err
	}
	version, err := util.Parse(versionStr)
	if err != nil {
		return nil, fmt.Errorf("unable to parse version string: %w", err)
	}

	// Version minVersion; "null" means absent.                 (lines 93-106)
	minVersionStr, err := readVal(siMinVersion)
	if err != nil {
		return nil, err
	}
	var minVersion *util.Version
	if minVersionStr != "null" {
		minVersion, err = util.Parse(minVersionStr)
		if err != nil {
			return nil, fmt.Errorf("unable to parse version string: %w", err)
		}
	}

	// final int docCount = Integer.parseInt(...);              (line 110)
	docCountStr, err := readVal(siDocCount)
	if err != nil {
		return nil, err
	}
	docCount, err := strconv.Atoi(docCountStr)
	if err != nil {
		return nil, fmt.Errorf("SimpleTextSegmentInfoFormat: number of documents: %w", err)
	}

	// final boolean isCompoundFile = Boolean.parseBoolean(...); (line 114)
	useCompoundStr, err := readVal(siUseCompound)
	if err != nil {
		return nil, err
	}
	isCompoundFile := useCompoundStr == "true"

	// final boolean hasBlocks = Boolean.parseBoolean(...);     (line 119)
	hasBlocksStr, err := readVal(siHasBlocks)
	if err != nil {
		return nil, err
	}
	hasBlocks := hasBlocksStr == "true"

	// int numDiag; Map<String,String> diagnostics;             (lines 123-135)
	numDiagStr, err := readVal(siNumDiag)
	if err != nil {
		return nil, err
	}
	numDiag, err := strconv.Atoi(numDiagStr)
	if err != nil {
		return nil, fmt.Errorf("SimpleTextSegmentInfoFormat: diagnostics: %w", err)
	}
	diagnostics := make(map[string]string, numDiag)
	for i := 0; i < numDiag; i++ {
		key, err := readVal(siDiagKey)
		if err != nil {
			return nil, err
		}
		value, err := readVal(siDiagValue)
		if err != nil {
			return nil, err
		}
		diagnostics[key] = value
	}

	// int numAtt; Map<String,String> attributes;               (lines 139-151)
	numAttStr, err := readVal(siNumAtt)
	if err != nil {
		return nil, err
	}
	numAtt, err := strconv.Atoi(numAttStr)
	if err != nil {
		return nil, fmt.Errorf("SimpleTextSegmentInfoFormat: attributes: %w", err)
	}
	attributes := make(map[string]string, numAtt)
	for i := 0; i < numAtt; i++ {
		key, err := readVal(siAttKey)
		if err != nil {
			return nil, err
		}
		value, err := readVal(siAttValue)
		if err != nil {
			return nil, err
		}
		attributes[key] = value
	}

	// int numFiles; Set<String> files;                         (lines 155-163)
	numFilesStr, err := readVal(siNumFiles)
	if err != nil {
		return nil, err
	}
	numFiles, err := strconv.Atoi(numFilesStr)
	if err != nil {
		return nil, fmt.Errorf("SimpleTextSegmentInfoFormat: files: %w", err)
	}
	files := make([]string, 0, numFiles)
	for i := 0; i < numFiles; i++ {
		fileName, err := readVal(siFile)
		if err != nil {
			return nil, err
		}
		files = append(files, fileName)
	}

	// final byte[] id = SimpleTextUtil.fromBytesRefString(...).bytes; (line 167)
	idStr, err := readVal(siID)
	if err != nil {
		return nil, err
	}
	id, err := FromBytesRefString(idStr)
	if err != nil {
		return nil, err
	}

	// if (!Arrays.equals(segmentID, id)) throw CorruptIndexException (line 169)
	if !bytes.Equal(segmentID, id) {
		return nil, fmt.Errorf("file mismatch, expected: %x, got: %x", segmentID, id)
	}

	// final int numSortFields = Integer.parseInt(...);         (line 180)
	numSortStr, err := readVal(siSort)
	if err != nil {
		return nil, err
	}
	numSortFields, err := strconv.Atoi(numSortStr)
	if err != nil {
		return nil, fmt.Errorf("SimpleTextSegmentInfoFormat: sort: %w", err)
	}
	sortFields := make([]*index.SortField, numSortFields)
	for i := 0; i < numSortFields; i++ {
		// final String provider = readString(SI_SORT_NAME.length, scratch); (line 185)
		provider, err := readVal(siSortName)
		if err != nil {
			return nil, err
		}

		// The SI_SORT_TYPE line is read and discarded: Java only asserts its
		// prefix and never uses the value (lines 187-188).
		if _, err := readVal(siSortType); err != nil {
			return nil, err
		}

		// BytesRef serializedSort = fromBytesRefString(...);   (line 192)
		serializedSortStr, err := readVal(siSortBytes)
		if err != nil {
			return nil, err
		}
		serializedSort, err := FromBytesRefString(serializedSortStr)
		if err != nil {
			return nil, err
		}

		// sortField[i] = SortFieldProvider.forName(provider).readSortField(bytes); (line 197)
		p, err := index.LookupSortFieldProvider(provider)
		if err != nil {
			return nil, err
		}
		value, err := p.ReadSortField(store.NewByteArrayDataInput(serializedSort))
		if err != nil {
			return nil, err
		}
		sortField, ok := value.(*index.SortField)
		if !ok {
			return nil, fmt.Errorf("sort field provider %q returned %T, not a SortField", provider, value)
		}
		sortFields[i] = sortField
	}

	// final Sort indexSort = sortField.length == 0 ? null : new Sort(sortField); (lines 201-206)
	var indexSort *index.Sort
	if len(sortFields) != 0 {
		indexSort = index.NewSort(sortFields...)
	}

	// SimpleTextUtil.checkFooter(input);                       (line 208)
	if err := CheckFooter(in); err != nil {
		return nil, err
	}

	// SegmentInfo info = new SegmentInfo(directory, version, minVersion,
	//     segmentName, docCount, isCompoundFile, hasBlocks, null, diagnostics,
	//     id, attributes, indexSort);
	// info.setFiles(files);                                    (lines 210-224)
	//
	// Gocene renders Java's 12-argument constructor as NewSegmentInfo plus the
	// per-field setters, exactly as Lucene99SegmentInfoFormat.Read does
	// (codecs/segment_info_format.go:242-269). The null codec argument is the
	// constructor's default: no SetCodec call.
	info := index.NewSegmentInfo(segmentName, docCount, dir)
	if err := info.SetID(id); err != nil {
		return nil, err
	}
	info.SetVersion(version.String())
	if minVersion != nil {
		info.SetMinVersion(minVersion.String())
	}
	info.SetHasBlocks(hasBlocks)
	info.SetCompoundFile(isCompoundFile)
	info.SetDiagnostics(diagnostics)
	for k, v := range attributes {
		info.SetAttribute(k, v)
	}
	if indexSort != nil {
		info.SetIndexSort(indexSort)
	}
	info.SetFiles(files)
	return info, nil
}

func (f *SimpleTextSegmentInfoFormat) Write(dir store.Directory, si *index.SegmentInfo, context store.IOContext) error {
	segFileName := index.SegmentFileName(si.Name(), "", siExtension)
	raw, err := dir.CreateOutput(segFileName, context)
	if err != nil {
		return err
	}
	// Java's createOutput already yields a checksumming IndexOutput; Gocene
	// carries the running checksum on store.ChecksumIndexOutput, which the
	// other SimpleText writers wrap the raw output in
	// (simple_text_points_writer.go:74-79).
	out := store.NewChecksumIndexOutput(raw)
	defer out.Close()

	// si.addFile(segFileName);                                 (line 241)
	si.AddFile(segFileName)

	writeVal := func(prefix []byte, value string) error {
		if err := out.WriteBytes(prefix, 0, len(prefix)); err != nil {
			return err
		}
		if err := Write(out, value); err != nil {
			return err
		}
		return WriteNewline(out)
	}

	if err := writeVal(siVersion, si.Version()); err != nil {
		return err
	}

	// if (si.getMinVersion() == null) write "null" else its toString(). (lines 249-253)
	minVersion, hasMinVersion := si.MinVersion()
	if !hasMinVersion {
		minVersion = "null"
	}
	if err := writeVal(siMinVersion, minVersion); err != nil {
		return err
	}

	if err := writeVal(siDocCount, strconv.Itoa(si.MaxDoc())); err != nil {
		return err
	}
	if err := writeVal(siUseCompound, strconv.FormatBool(si.GetUseCompoundFile())); err != nil {
		return err
	}
	if err := writeVal(siHasBlocks, strconv.FormatBool(si.GetHasBlocks())); err != nil {
		return err
	}

	diagnostics := si.GetDiagnostics()
	if err := writeVal(siNumDiag, strconv.Itoa(len(diagnostics))); err != nil {
		return err
	}
	for k, v := range diagnostics {
		if err := writeVal(siDiagKey, k); err != nil {
			return err
		}
		if err := writeVal(siDiagValue, v); err != nil {
			return err
		}
	}

	attributes := si.GetAttributes()
	if err := writeVal(siNumAtt, strconv.Itoa(len(attributes))); err != nil {
		return err
	}
	for k, v := range attributes {
		if err := writeVal(siAttKey, k); err != nil {
			return err
		}
		if err := writeVal(siAttValue, v); err != nil {
			return err
		}
	}

	files := si.Files()
	if err := writeVal(siNumFiles, strconv.Itoa(len(files))); err != nil {
		return err
	}
	for _, fileName := range files {
		if err := writeVal(siFile, fileName); err != nil {
			return err
		}
	}

	// new BytesRef(si.getId()).toString()                      (line 316)
	if err := writeVal(siID, BytesRefString(si.GetID())); err != nil {
		return err
	}

	// Index sort. Mirrors lines 319-344: the number of sort fields, then per
	// field the provider name, the SortField's toString(), and the serialised
	// bytes in BytesRef.toString() form.
	var sortFields []*index.SortField
	if indexSort := si.IndexSort(); indexSort != nil {
		sortFields = indexSort.Fields()
	}
	if err := writeVal(siSort, strconv.Itoa(len(sortFields))); err != nil {
		return err
	}
	for _, sortField := range sortFields {
		// IndexSorter sorter = sortField.getIndexSorter();
		// if (sorter == null) throw new IllegalStateException(...);
		// sorter.getProviderName() is rendered by the index.SortFieldNamer
		// view (index/sort_field_provider.go), as in
		// codecs/segment_info_format.go:298-304.
		var namer any = sortField
		sorter, ok := namer.(index.SortFieldNamer)
		if !ok || sorter.ProviderName() == "" {
			return fmt.Errorf("cannot serialize sort %v: %w", sortField, index.ErrSortFieldNotSerializable)
		}
		if err := writeVal(siSortName, sorter.ProviderName()); err != nil {
			return err
		}
		if err := writeVal(siSortType, sortField.String()); err != nil {
			return err
		}

		b := newBytesRefOutput()
		if err := index.WriteSortField(sortField, b); err != nil {
			return err
		}
		if err := writeVal(siSortBytes, BytesRefString(b.bytes.Bytes()[:b.bytes.Length()])); err != nil {
			return err
		}
	}

	return WriteChecksum(out)
}

// bytesRefOutput is a DataOutput that accumulates into a BytesRefBuilder.
//
// Port of the nested class SimpleTextSegmentInfoFormat.BytesRefOutput
// (line 349), which extends DataOutput and overrides writeByte and writeBytes
// to append to its BytesRefBuilder.
type bytesRefOutput struct {
	*store.BaseDataOutput
	bytes *util.BytesRefBuilder
}

// newBytesRefOutput builds an empty BytesRefOutput.
func newBytesRefOutput() *bytesRefOutput {
	o := &bytesRefOutput{bytes: util.NewBytesRefBuilder()}
	o.BaseDataOutput = store.NewBaseDataOutput(o)
	return o
}

// WriteByte appends b. Port of BytesRefOutput.writeByte (line 354).
func (o *bytesRefOutput) WriteByte(b byte) error {
	o.bytes.AppendByte(b)
	return nil
}

// WriteBytes appends length bytes of b from offset.
// Port of BytesRefOutput.writeBytes (line 359).
func (o *bytesRefOutput) WriteBytes(b []byte, offset, length int) error {
	o.bytes.AppendBytes(b, offset, length)
	return nil
}
