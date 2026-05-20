// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package blocktree

import (
	"fmt"
	"sort"

	bcstore "github.com/FlavioCFOliveira/Gocene/backward_codecs/store"
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// Wire constants — all exported so FieldReader and frame types can reference them.
const (
	// OutputFlagsNumBits is the number of bits reserved for output flags.
	OutputFlagsNumBits = 2
	// OutputFlagsMask masks the two flag bits from an output value.
	OutputFlagsMask = 0x3
	// OutputFlagIsFloor indicates the block is a floor block.
	OutputFlagIsFloor = 0x1
	// OutputFlagHasTerms indicates the block has terms.
	OutputFlagHasTerms = 0x2
)

// File-extension / codec-name constants.
const (
	termsExtension     = "tim"
	termsCodecName     = "BlockTreeTermsDict"
	termsIndexExt      = "tip"
	termsIndexCodec    = "BlockTreeTermsIndex"
	termsMetaExt       = "tmd"
	termsMetaCodecName = "BlockTreeTermsMeta"
)

// Format version constants — exported so tests and frame code can compare.
const (
	// VersionStart is the first supported format version.
	VersionStart = 3

	// VersionMetaLongsRemoved is the version where the long[] + byte[]
	// metadata was replaced with a single byte[].
	VersionMetaLongsRemoved = 4

	// VersionCompressedSuffixes is the version where suffix bytes are
	// compressed. Mirrored by versionCompressedSuffixes (unexported alias
	// used in frame code).
	VersionCompressedSuffixes = 5

	// VersionMetaFile is the version where metadata was moved to its own file.
	VersionMetaFile = 6

	// VersionCurrent is the highest supported format version.
	VersionCurrent = VersionMetaFile
)

// versionCompressedSuffixes is the unexported alias used by frame constructors.
const versionCompressedSuffixes = VersionCompressedSuffixes


// Lucene40BlockTreeTermsReader is a read-only FieldsProducer for the Lucene
// 4.0 block-tree terms dictionary.
//
// Port of
// org.apache.lucene.backward_codecs.lucene40.blocktree.Lucene40BlockTreeTermsReader
// (Lucene 10.4.0).
type Lucene40BlockTreeTermsReader struct {
	termsIn store.IndexInput
	indexIn store.IndexInput

	postingsReader codecs.PostingsReaderBase

	fieldMap  map[string]*FieldReader
	fieldList []string

	segment string
	version int32

	closed bool
}

// NewLucene40BlockTreeTermsReader opens and validates the terms-dictionary
// files for the segment described by state, then reads per-field metadata.
//
// Port of Lucene40BlockTreeTermsReader(PostingsReaderBase, SegmentReadState).
func NewLucene40BlockTreeTermsReader(
	postingsReader codecs.PostingsReaderBase,
	state *codecs.SegmentReadState,
) (*Lucene40BlockTreeTermsReader, error) {
	r := &Lucene40BlockTreeTermsReader{
		postingsReader: postingsReader,
		segment:        state.SegmentInfo.Name(),
	}

	var success bool
	defer func() {
		if !success {
			_ = r.Close()
		}
	}()

	ctx := store.IOContext{Context: store.ContextRead}

	// --- Open terms file (big-endian wrapped) ---
	termsName := codecs.GetSegmentFileName(r.segment, state.SegmentSuffix, termsExtension)
	rawTermsIn, err := bcstore.OpenInput(state.Directory, termsName, ctx)
	if err != nil {
		return nil, fmt.Errorf("blocktree reader: open terms %q: %w", termsName, err)
	}
	r.termsIn = rawTermsIn

	r.version, err = codecs.CheckIndexHeader(
		r.termsIn,
		termsCodecName,
		VersionStart,
		VersionCurrent,
		state.SegmentInfo.GetID(),
		state.SegmentSuffix,
	)
	if err != nil {
		return nil, fmt.Errorf("blocktree reader: check terms header: %w", err)
	}

	// --- Open index file (big-endian wrapped) ---
	indexName := codecs.GetSegmentFileName(r.segment, state.SegmentSuffix, termsIndexExt)
	rawIndexIn, err := bcstore.OpenInput(state.Directory, indexName, ctx)
	if err != nil {
		return nil, fmt.Errorf("blocktree reader: open index %q: %w", indexName, err)
	}
	r.indexIn = rawIndexIn

	if _, err = codecs.CheckIndexHeader(
		r.indexIn,
		termsIndexCodec,
		r.version,
		r.version,
		state.SegmentInfo.GetID(),
		state.SegmentSuffix,
	); err != nil {
		return nil, fmt.Errorf("blocktree reader: check index header: %w", err)
	}

	if r.version < VersionMetaFile {
		if err = postingsReader.Init(r.termsIn, state); err != nil {
			return nil, fmt.Errorf("blocktree reader: postings reader init: %w", err)
		}
		if _, err = codecs.RetrieveChecksum(r.indexIn); err != nil {
			return nil, fmt.Errorf("blocktree reader: retrieve index checksum: %w", err)
		}
		if _, err = codecs.RetrieveChecksum(r.termsIn); err != nil {
			return nil, fmt.Errorf("blocktree reader: retrieve terms checksum: %w", err)
		}
	}

	// --- Read per-field metadata ---
	var fieldMap map[string]*FieldReader
	var indexLength, termsLength int64 = -1, -1

	if r.version >= VersionMetaFile {
		metaName := codecs.GetSegmentFileName(r.segment, state.SegmentSuffix, termsMetaExt)
		rawMetaIn, err2 := bcstore.OpenChecksumInput(state.Directory, metaName, ctx)
		if err2 != nil {
			return nil, fmt.Errorf("blocktree reader: open meta %q: %w", metaName, err2)
		}
		if _, err2 = codecs.CheckIndexHeader(
			rawMetaIn,
			termsMetaCodecName,
			r.version,
			r.version,
			state.SegmentInfo.GetID(),
			state.SegmentSuffix,
		); err2 != nil {
			_ = rawMetaIn.Close()
			return nil, fmt.Errorf("blocktree reader: check meta header: %w", err2)
		}
		if err2 = postingsReader.Init(rawMetaIn, state); err2 != nil {
			_ = rawMetaIn.Close()
			return nil, fmt.Errorf("blocktree reader: postings reader meta init: %w", err2)
		}
		if fieldMap, err = readFields(r, state, rawMetaIn, rawMetaIn, r.version); err != nil {
			_ = rawMetaIn.Close()
			return nil, err
		}
		indexLength, err = rawMetaIn.ReadLong()
		if err != nil {
			_ = rawMetaIn.Close()
			return nil, fmt.Errorf("blocktree reader: read index length: %w", err)
		}
		termsLength, err = rawMetaIn.ReadLong()
		if err != nil {
			_ = rawMetaIn.Close()
			return nil, fmt.Errorf("blocktree reader: read terms length: %w", err)
		}
		if err = checkFooter(rawMetaIn); err != nil {
			_ = rawMetaIn.Close()
			return nil, fmt.Errorf("blocktree reader: check meta footer: %w", err)
		}
		if err = rawMetaIn.Close(); err != nil {
			return nil, fmt.Errorf("blocktree reader: close meta: %w", err)
		}
	} else {
		if err = seekDir(r.termsIn); err != nil {
			return nil, err
		}
		if err = seekDir(r.termsIn); err != nil {
			return nil, err
		}
		if err = seekDir(r.indexIn); err != nil {
			return nil, err
		}
		if fieldMap, err = readFields(r, state, r.indexIn, r.termsIn, r.version); err != nil {
			return nil, err
		}
	}

	// Validate file lengths when we have them (version >= VersionMetaFile).
	if indexLength >= 0 {
		if _, err = codecs.RetrieveChecksum(r.indexIn); err != nil {
			return nil, fmt.Errorf("blocktree reader: retrieve index checksum (meta): %w", err)
		}
	}
	if termsLength >= 0 {
		if _, err = codecs.RetrieveChecksum(r.termsIn); err != nil {
			return nil, fmt.Errorf("blocktree reader: retrieve terms checksum (meta): %w", err)
		}
	}

	// Build sorted field list.
	r.fieldMap = fieldMap
	r.fieldList = make([]string, 0, len(fieldMap))
	for name := range fieldMap {
		r.fieldList = append(r.fieldList, name)
	}
	sort.Strings(r.fieldList)

	success = true
	return r, nil
}

// readFields reads numFields entries from termsMetaIn / indexMetaIn and
// constructs the FieldReader map.
func readFields(
	r *Lucene40BlockTreeTermsReader,
	state *codecs.SegmentReadState,
	indexMetaIn store.IndexInput,
	termsMetaIn store.IndexInput,
	version int32,
) (map[string]*FieldReader, error) {
	numFields, err := store.ReadVInt(termsMetaIn)
	if err != nil {
		return nil, fmt.Errorf("blocktree reader: read numFields: %w", err)
	}
	if numFields < 0 {
		return nil, fmt.Errorf("blocktree reader: invalid numFields: %d", numFields)
	}

	fieldMap := make(map[string]*FieldReader, int(numFields))
	for i := int32(0); i < numFields; i++ {
		field, err2 := store.ReadVInt(termsMetaIn)
		if err2 != nil {
			return nil, fmt.Errorf("blocktree reader: read field number: %w", err2)
		}
		numTerms, err2 := store.ReadVLong(termsMetaIn)
		if err2 != nil {
			return nil, fmt.Errorf("blocktree reader: read numTerms: %w", err2)
		}
		if numTerms <= 0 {
			return nil, fmt.Errorf("blocktree reader: illegal numTerms for field %d: %d", field, numTerms)
		}
		rootCode, err2 := readBytesRef(termsMetaIn)
		if err2 != nil {
			return nil, fmt.Errorf("blocktree reader: read rootCode: %w", err2)
		}
		fieldInfo := state.FieldInfos.GetByNumber(int(field))
		if fieldInfo == nil {
			return nil, fmt.Errorf("blocktree reader: invalid field number: %d", field)
		}
		sumTotalTermFreq, err2 := store.ReadVLong(termsMetaIn)
		if err2 != nil {
			return nil, fmt.Errorf("blocktree reader: read sumTotalTermFreq: %w", err2)
		}
		var sumDocFreq int64
		if fieldInfo.IndexOptions() == index.IndexOptionsDocs {
			sumDocFreq = sumTotalTermFreq
		} else {
			sumDocFreq, err2 = store.ReadVLong(termsMetaIn)
			if err2 != nil {
				return nil, fmt.Errorf("blocktree reader: read sumDocFreq: %w", err2)
			}
		}
		docCount, err2 := store.ReadVInt(termsMetaIn)
		if err2 != nil {
			return nil, fmt.Errorf("blocktree reader: read docCount: %w", err2)
		}
		if version < VersionMetaLongsRemoved {
			longsSize, err3 := store.ReadVInt(termsMetaIn)
			if err3 != nil {
				return nil, fmt.Errorf("blocktree reader: read longsSize: %w", err3)
			}
			if longsSize < 0 {
				return nil, fmt.Errorf(
					"blocktree reader: invalid longsSize for field %s: %d",
					fieldInfo.Name(), longsSize,
				)
			}
		}
		minTerm, err2 := readBytesRef(termsMetaIn)
		if err2 != nil {
			return nil, fmt.Errorf("blocktree reader: read minTerm: %w", err2)
		}
		maxTerm, err2 := readBytesRef(termsMetaIn)
		if err2 != nil {
			return nil, fmt.Errorf("blocktree reader: read maxTerm: %w", err2)
		}
		if int(docCount) < 0 || int(docCount) > state.SegmentInfo.DocCount() {
			return nil, fmt.Errorf(
				"blocktree reader: invalid docCount %d (maxDoc=%d)",
				docCount, state.SegmentInfo.DocCount(),
			)
		}
		if sumDocFreq < int64(docCount) {
			return nil, fmt.Errorf(
				"blocktree reader: invalid sumDocFreq %d < docCount %d",
				sumDocFreq, docCount,
			)
		}
		if sumTotalTermFreq < sumDocFreq {
			return nil, fmt.Errorf(
				"blocktree reader: invalid sumTotalTermFreq %d < sumDocFreq %d",
				sumTotalTermFreq, sumDocFreq,
			)
		}
		indexStartFP, err2 := store.ReadVLong(indexMetaIn)
		if err2 != nil {
			return nil, fmt.Errorf("blocktree reader: read indexStartFP: %w", err2)
		}
		if _, dup := fieldMap[fieldInfo.Name()]; dup {
			return nil, fmt.Errorf("blocktree reader: duplicate field: %s", fieldInfo.Name())
		}
		fr, err2 := newFieldReader(
			r, fieldInfo,
			numTerms, rootCode,
			sumTotalTermFreq, sumDocFreq, int(docCount),
			indexStartFP, indexMetaIn, r.indexIn,
			minTerm, maxTerm,
		)
		if err2 != nil {
			return nil, err2
		}
		fieldMap[fieldInfo.Name()] = fr
	}
	return fieldMap, nil
}

// readBytesRef reads a length-prefixed byte slice from in.
func readBytesRef(in store.IndexInput) (*util.BytesRef, error) {
	n, err := store.ReadVInt(in)
	if err != nil {
		return nil, err
	}
	if n < 0 {
		return nil, fmt.Errorf("blocktree reader: invalid bytes length: %d", n)
	}
	b := make([]byte, int(n))
	if err = in.ReadBytes(b); err != nil {
		return nil, err
	}
	return &util.BytesRef{Bytes: b, Offset: 0, Length: int(n)}, nil
}

// seekDir positions input at the directory-entries offset stored near end of file.
func seekDir(in store.IndexInput) error {
	footerLen := int64(codecs.FooterLength())
	if err := in.SetPosition(in.Length() - footerLen - 8); err != nil {
		return fmt.Errorf("blocktree reader: seekDir seek1: %w", err)
	}
	offset, err := in.ReadLong()
	if err != nil {
		return fmt.Errorf("blocktree reader: seekDir readLong: %w", err)
	}
	if err = in.SetPosition(offset); err != nil {
		return fmt.Errorf("blocktree reader: seekDir seek2: %w", err)
	}
	return nil
}

// Terms returns the FieldReader for field, or nil if not present.
func (r *Lucene40BlockTreeTermsReader) Terms(field string) (index.Terms, error) {
	if r.closed {
		return nil, fmt.Errorf("blocktree reader: already closed")
	}
	fr, ok := r.fieldMap[field]
	if !ok {
		return nil, nil
	}
	return fr, nil
}

// Close releases all resources.
func (r *Lucene40BlockTreeTermsReader) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	var firstErr error
	for _, c := range []interface{ Close() error }{
		r.indexIn,
		r.termsIn,
		r.postingsReader,
	} {
		if c == nil {
			continue
		}
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	r.fieldMap = nil
	return firstErr
}

// Iterator returns a channel-based iterator over sorted field names.
func (r *Lucene40BlockTreeTermsReader) Iterator() []string {
	return r.fieldList
}

// Size returns the number of indexed fields.
func (r *Lucene40BlockTreeTermsReader) Size() int {
	return len(r.fieldMap)
}

// CheckIntegrity verifies the CRC of all owned files and delegates to
// the PostingsReaderBase.
func (r *Lucene40BlockTreeTermsReader) CheckIntegrity() error {
	if _, err := codecs.ChecksumEntireFile(r.indexIn); err != nil {
		return fmt.Errorf("blocktree reader: index integrity: %w", err)
	}
	if _, err := codecs.ChecksumEntireFile(r.termsIn); err != nil {
		return fmt.Errorf("blocktree reader: terms integrity: %w", err)
	}
	return r.postingsReader.CheckIntegrity()
}

// String returns a debug summary.
func (r *Lucene40BlockTreeTermsReader) String() string {
	return fmt.Sprintf("Lucene40BlockTreeTermsReader(fields=%d,delegate=%v)",
		len(r.fieldMap), r.postingsReader)
}


// checksumLike is the minimal surface of EndiannessReverserChecksumIndexInput
// needed for footer validation.  We cannot pass the concrete type to
// codecs.CheckFooter because that function requires *store.ChecksumIndexInput.
type checksumLike interface {
	store.IndexInput
	GetChecksum() uint32
}

// checkFooter validates the codec footer and checksum for a checksumLike
// input.  Mirrors the logic of codecs.CheckFooter.
func checkFooter(in checksumLike) error {
	remaining := in.Length() - in.GetFilePointer()
	const footerLen = 16 // 4 magic + 4 algID + 8 checksum
	if remaining < footerLen {
		return fmt.Errorf("blocktree: misplaced codec footer (truncated?): remaining=%d", remaining)
	}
	if remaining > footerLen {
		return fmt.Errorf("blocktree: misplaced codec footer (extended?): remaining=%d", remaining)
	}
	magic, err := store.ReadInt32(in)
	if err != nil {
		return err
	}
	const footerMagic = int32(^0x3FD76C17)
	if magic != footerMagic {
		return fmt.Errorf("blocktree: codec footer mismatch: actual=%#x expected=%#x", magic, footerMagic)
	}
	alg, err := store.ReadInt32(in)
	if err != nil {
		return err
	}
	if alg != 0 {
		return fmt.Errorf("blocktree: codec footer unknown algorithmID: %d", alg)
	}
	actualChecksum := int64(in.GetChecksum())
	expectedChecksum, err := store.ReadInt64(in)
	if err != nil {
		return err
	}
	if actualChecksum != expectedChecksum {
		return fmt.Errorf("blocktree: checksum failed: actual=%#x expected=%#x",
			actualChecksum, expectedChecksum)
	}
	return nil
}
