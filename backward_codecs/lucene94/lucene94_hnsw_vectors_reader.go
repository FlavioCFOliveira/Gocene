// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene94

import (
	"errors"
	"fmt"
	"io"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// Lucene 9.4 HNSW format file-descriptor constants.
const (
	lucene94MetaCodecName        = "lucene94HnswVectorsFormatMeta"
	lucene94VectorDataCodecName  = "lucene94HnswVectorsFormatData"
	lucene94VectorIndexCodecName = "lucene94HnswVectorsFormatIndex"
	lucene94MetaExtension        = "vem"
	lucene94VectorDataExtension  = "vec"
	lucene94VectorIndexExtension = "vex"
	lucene94VersionStart         = int32(0)
	lucene94VersionCurrent       = int32(1)
)

// lucene94FieldEntry holds all per-field metadata read from the .vem file.
// Port of the FieldEntry record in Lucene94HnswVectorsReader.java.
type lucene94FieldEntry struct {
	vectorEncoding     index.VectorEncoding
	similarityFunction index.VectorSimilarityFunction

	vectorDataOffset  int64
	vectorDataLength  int64
	vectorIndexOffset int64
	vectorIndexLength int64

	dimension int
	size      int

	// docsWithField encoding (mirrors IndexedDISI sentinel convention):
	//   -1 = dense, -2 = empty, other = sparse
	docsWithFieldOffset int64
	docsWithFieldLength int64
	jumpTableEntryCount int16
	denseRankPower      int8

	// DirectMonotonic ordToDoc mapping (sparse case only)
	addressesOffset int64
	blockShift      int
	meta            *packed.DirectMonotonicMeta
	addressesLength int64

	// HNSW graph topology
	maxConn             int
	numLevels           int
	nodesByLevel        [][]int32
	graphOffsetsByLevel []int64
}

// Lucene94HnswVectorsReader reads float32 and byte vector values together with
// the associated HNSW graph from segments written in the Lucene 9.4 format.
//
// Compared to Lucene92HnswVectorsReader, this reader adds support for byte
// (BYTE) vector encoding in addition to FLOAT32.
//
// Port of org.apache.lucene.backward_codecs.lucene94.Lucene94HnswVectorsReader.
type Lucene94HnswVectorsReader struct {
	fields     map[int]*lucene94FieldEntry
	vectorData store.IndexInput
	vectorIdx  store.IndexInput
	fieldInfos *index.FieldInfos
	closed     bool
}

// NewLucene94HnswVectorsReader opens the .vem / .vec / .vex files for state and
// returns a ready-to-use Lucene94HnswVectorsReader.
func NewLucene94HnswVectorsReader(state *index.SegmentReadState) (*Lucene94HnswVectorsReader, error) {
	r := &Lucene94HnswVectorsReader{
		fields:     make(map[int]*lucene94FieldEntry),
		fieldInfos: state.FieldInfos,
	}

	versionMeta, err := r.readMetadata(state)
	if err != nil {
		return nil, err
	}

	ok := false
	defer func() {
		if !ok {
			_ = r.Close()
		}
	}()

	r.vectorData, err = openLucene94DataInput(state, versionMeta,
		lucene94VectorDataExtension, lucene94VectorDataCodecName)
	if err != nil {
		return nil, err
	}

	r.vectorIdx, err = openLucene94DataInput(state, versionMeta,
		lucene94VectorIndexExtension, lucene94VectorIndexCodecName)
	if err != nil {
		return nil, err
	}

	ok = true
	return r, nil
}

// readMetadata reads the .vem file and populates r.fields.
func (r *Lucene94HnswVectorsReader) readMetadata(state *index.SegmentReadState) (int32, error) {
	metaName := index.SegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, lucene94MetaExtension)

	// Lucene94HnswVectorsReader reads the .vem metadata via a plain
	// ChecksumIndexInput (Directory.openChecksumInput in Lucene): payload
	// fields are little-endian because Lucene 9.0+ DataOutput is LE. The
	// EndiannessReverser wrappers are reserved for Lucene <= 8.x codecs and
	// MUST NOT be applied here, otherwise the payload integers decode with the
	// wrong byte order. CodecUtil header/footer framing remains big-endian and
	// is read through the raw store.ReadInt32/ReadInt64 helpers.
	rawMeta, err := state.Directory.OpenInput(metaName,
		store.IOContext{Context: store.ContextReadOnce})
	if err != nil {
		return 0, fmt.Errorf("lucene94 vectors: open meta %q: %w", metaName, err)
	}
	meta := store.NewChecksumIndexInput(rawMeta)
	defer func() { _ = meta.Close() }()

	version, err := codecs.CheckIndexHeader(
		meta,
		lucene94MetaCodecName,
		lucene94VersionStart,
		lucene94VersionCurrent,
		state.SegmentInfo.GetID(),
		state.SegmentSuffix,
	)
	if err != nil {
		return 0, fmt.Errorf("lucene94 vectors meta header: %w", err)
	}

	if err = r.readFields(meta, state.FieldInfos); err != nil {
		return 0, err
	}

	if err = checkLucene94Footer(meta); err != nil {
		return 0, fmt.Errorf("lucene94 vectors meta footer: %w", err)
	}
	return version, nil
}

// checkLucene94Footer validates the codec footer. The footer framing (magic,
// algorithmID, CRC) is big-endian, so it is read with the raw store.ReadInt32/
// ReadInt64 helpers regardless of the little-endian payload body.
func checkLucene94Footer(in *store.ChecksumIndexInput) error {
	remaining := in.Length() - in.GetFilePointer()
	const footerLen = 16
	if remaining < footerLen {
		return fmt.Errorf("misplaced codec footer: remaining=%d (too short)", remaining)
	}
	if remaining > footerLen {
		return fmt.Errorf("misplaced codec footer: remaining=%d (too long)", remaining)
	}

	magic, err := store.ReadInt32(in)
	if err != nil {
		return err
	}
	const footerMagic = int32(^0x3FD76C17)
	if magic != footerMagic {
		return fmt.Errorf("codec footer magic mismatch: got %x", magic)
	}

	algID, err := store.ReadInt32(in)
	if err != nil {
		return err
	}
	if algID != 0 {
		return fmt.Errorf("codec footer unknown algorithmID %d", algID)
	}

	actualChecksum := int64(in.GetChecksum())
	expectedChecksum, err := store.ReadInt64(in)
	if err != nil {
		return err
	}
	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum failed: actual=%x expected=%x", actualChecksum, expectedChecksum)
	}
	return nil
}

// readFields reads per-field entries from the metadata stream until it sees -1.
func (r *Lucene94HnswVectorsReader) readFields(in store.DataInput, infos *index.FieldInfos) error {
	for {
		fieldNumber, err := in.ReadInt()
		if err != nil {
			return fmt.Errorf("readFields: %w", err)
		}
		if fieldNumber == -1 {
			return nil
		}
		fi := infos.GetByNumber(int(fieldNumber))
		if fi == nil {
			return fmt.Errorf("readFields: invalid field number %d", fieldNumber)
		}
		entry, err := readLucene94FieldEntry(in, fi)
		if err != nil {
			return err
		}
		if err = validateLucene94FieldEntry(fi, entry); err != nil {
			return err
		}
		r.fields[fi.Number()] = entry
	}
}

// readLucene94FieldEntry deserialises one field entry from the metadata stream.
func readLucene94FieldEntry(in store.DataInput, fi *index.FieldInfo) (*lucene94FieldEntry, error) {
	// vector encoding (added in Lucene 9.4 vs 9.2)
	encID, err := in.ReadInt()
	if err != nil {
		return nil, fmt.Errorf("readFieldEntry %q encoding: %w", fi.Name(), err)
	}
	enc := index.VectorEncoding(encID)
	if enc != fi.VectorEncoding() {
		return nil, fmt.Errorf("readFieldEntry %q: encoding mismatch %v != %v",
			fi.Name(), enc, fi.VectorEncoding())
	}

	// similarity function
	simID, err := in.ReadInt()
	if err != nil {
		return nil, fmt.Errorf("readFieldEntry %q similarity: %w", fi.Name(), err)
	}
	simFn := index.VectorSimilarityFunction(simID)
	if simFn != fi.VectorSimilarityFunction() {
		return nil, fmt.Errorf("readFieldEntry %q: similarity mismatch %v != %v",
			fi.Name(), simFn, fi.VectorSimilarityFunction())
	}

	e := &lucene94FieldEntry{vectorEncoding: enc, similarityFunction: simFn}

	if e.vectorDataOffset, err = store.ReadVLong(in); err != nil {
		return nil, fmt.Errorf("readFieldEntry %q vectorDataOffset: %w", fi.Name(), err)
	}
	if e.vectorDataLength, err = store.ReadVLong(in); err != nil {
		return nil, fmt.Errorf("readFieldEntry %q vectorDataLength: %w", fi.Name(), err)
	}
	if e.vectorIndexOffset, err = store.ReadVLong(in); err != nil {
		return nil, fmt.Errorf("readFieldEntry %q vectorIndexOffset: %w", fi.Name(), err)
	}
	if e.vectorIndexLength, err = store.ReadVLong(in); err != nil {
		return nil, fmt.Errorf("readFieldEntry %q vectorIndexLength: %w", fi.Name(), err)
	}

	dim, err := in.ReadInt()
	if err != nil {
		return nil, fmt.Errorf("readFieldEntry %q dimension: %w", fi.Name(), err)
	}
	e.dimension = int(dim)

	sz, err := in.ReadInt()
	if err != nil {
		return nil, fmt.Errorf("readFieldEntry %q size: %w", fi.Name(), err)
	}
	e.size = int(sz)

	docsOff, err := in.ReadLong()
	if err != nil {
		return nil, fmt.Errorf("readFieldEntry %q docsWithFieldOffset: %w", fi.Name(), err)
	}
	e.docsWithFieldOffset = docsOff

	docsLen, err := in.ReadLong()
	if err != nil {
		return nil, fmt.Errorf("readFieldEntry %q docsWithFieldLength: %w", fi.Name(), err)
	}
	e.docsWithFieldLength = docsLen

	jt, err := in.ReadShort()
	if err != nil {
		return nil, fmt.Errorf("readFieldEntry %q jumpTableEntryCount: %w", fi.Name(), err)
	}
	e.jumpTableEntryCount = jt

	drp, err := in.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("readFieldEntry %q denseRankPower: %w", fi.Name(), err)
	}
	e.denseRankPower = int8(drp)

	// sparse ordToDoc mapping
	if e.docsWithFieldOffset != -1 && e.docsWithFieldOffset != -2 {
		addrOff, err2 := in.ReadLong()
		if err2 != nil {
			return nil, fmt.Errorf("readFieldEntry %q addressesOffset: %w", fi.Name(), err2)
		}
		e.addressesOffset = addrOff

		bs, err2 := store.ReadVInt(in)
		if err2 != nil {
			return nil, fmt.Errorf("readFieldEntry %q blockShift: %w", fi.Name(), err2)
		}
		e.blockShift = int(bs)

		e.meta, err2 = packed.LoadDirectMonotonicMeta(in, int64(e.size), e.blockShift)
		if err2 != nil {
			return nil, fmt.Errorf("readFieldEntry %q DirectMonotonicMeta: %w", fi.Name(), err2)
		}

		addrLen, err2 := in.ReadLong()
		if err2 != nil {
			return nil, fmt.Errorf("readFieldEntry %q addressesLength: %w", fi.Name(), err2)
		}
		e.addressesLength = addrLen
	}

	// HNSW graph topology
	m32, err := in.ReadInt()
	if err != nil {
		return nil, fmt.Errorf("readFieldEntry %q M: %w", fi.Name(), err)
	}
	e.maxConn = int(m32)

	nl, err := in.ReadInt()
	if err != nil {
		return nil, fmt.Errorf("readFieldEntry %q numLevels: %w", fi.Name(), err)
	}
	e.numLevels = int(nl)

	e.nodesByLevel = make([][]int32, e.numLevels)
	for level := 0; level < e.numLevels; level++ {
		cnt, err2 := in.ReadInt()
		if err2 != nil {
			return nil, fmt.Errorf("readFieldEntry %q nodesByLevel[%d] count: %w", fi.Name(), level, err2)
		}
		if level == 0 {
			e.nodesByLevel[0] = nil
		} else {
			nodes := make([]int32, cnt)
			for i := int32(0); i < cnt; i++ {
				n, err3 := in.ReadInt()
				if err3 != nil {
					return nil, fmt.Errorf("readFieldEntry %q nodesByLevel[%d][%d]: %w", fi.Name(), level, i, err3)
				}
				nodes[i] = n
			}
			e.nodesByLevel[level] = nodes
		}
	}

	// derive graphOffsetsByLevel
	e.graphOffsetsByLevel = make([]int64, e.numLevels)
	bytesLevel0 := int64(1+2*e.maxConn) * 4
	bytesOther := int64(1+e.maxConn) * 4
	for level := 0; level < e.numLevels; level++ {
		switch level {
		case 0:
			e.graphOffsetsByLevel[0] = 0
		case 1:
			e.graphOffsetsByLevel[1] = bytesLevel0 * int64(e.size)
		default:
			prevCount := int64(len(e.nodesByLevel[level-1]))
			e.graphOffsetsByLevel[level] = e.graphOffsetsByLevel[level-1] + bytesOther*prevCount
		}
	}

	return e, nil
}

// validateLucene94FieldEntry checks metadata self-consistency against FieldInfo.
func validateLucene94FieldEntry(fi *index.FieldInfo, e *lucene94FieldEntry) error {
	if fi.VectorDimension() != e.dimension {
		return fmt.Errorf("field %q: vector dimension mismatch %d != %d",
			fi.Name(), fi.VectorDimension(), e.dimension)
	}
	var byteSize int64
	switch e.vectorEncoding {
	case index.VectorEncodingByte:
		byteSize = 1
	default: // FLOAT32
		byteSize = 4
	}
	wantBytes := int64(e.size) * int64(e.dimension) * byteSize
	if wantBytes != e.vectorDataLength {
		return fmt.Errorf("field %q: vectorDataLength %d != size*dim*byteSize = %d",
			fi.Name(), e.vectorDataLength, wantBytes)
	}
	return nil
}

// openLucene94DataInput opens a .vec or .vex file, validates header, and retrieves
// the trailing checksum offset.
func openLucene94DataInput(
	state *index.SegmentReadState,
	versionMeta int32,
	ext, codecName string,
) (store.IndexInput, error) {
	name := index.SegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, ext)
	in, err := state.Directory.OpenInput(name, store.IOContextRead)
	if err != nil {
		return nil, fmt.Errorf("lucene94 vectors: open %q: %w", name, err)
	}

	ok := false
	defer func() {
		if !ok {
			_ = in.Close()
		}
	}()

	version, err := codecs.CheckIndexHeader(
		in,
		codecName,
		lucene94VersionStart,
		lucene94VersionCurrent,
		state.SegmentInfo.GetID(),
		state.SegmentSuffix,
	)
	if err != nil {
		return nil, fmt.Errorf("lucene94 vectors %q header: %w", name, err)
	}
	if version != versionMeta {
		return nil, fmt.Errorf("lucene94 vectors %q: version mismatch meta=%d data=%d",
			name, versionMeta, version)
	}
	if _, err = codecs.RetrieveChecksum(in); err != nil {
		return nil, fmt.Errorf("lucene94 vectors %q checksum: %w", name, err)
	}

	ok = true
	return in, nil
}

// CheckIntegrity verifies the checksums of the .vec and .vex files.
func (r *Lucene94HnswVectorsReader) CheckIntegrity() error {
	if r.closed {
		return errors.New("lucene94 vectors reader: already closed")
	}
	if _, err := codecs.ChecksumEntireFile(r.vectorData); err != nil {
		return fmt.Errorf("lucene94 vectors checkIntegrity .vec: %w", err)
	}
	if _, err := codecs.ChecksumEntireFile(r.vectorIdx); err != nil {
		return fmt.Errorf("lucene94 vectors checkIntegrity .vex: %w", err)
	}
	return nil
}

// getFieldEntryOrThrow returns the field entry by name, or an error if missing.
func (r *Lucene94HnswVectorsReader) getFieldEntryOrThrow(field string) (*lucene94FieldEntry, error) {
	fi := r.fieldInfos.GetByName(field)
	if fi == nil {
		return nil, fmt.Errorf("field %q not found", field)
	}
	e, ok := r.fields[fi.Number()]
	if !ok {
		return nil, fmt.Errorf("field %q not found in vector index", field)
	}
	return e, nil
}

// GetFieldEntry returns the field entry, verifying the expected vector encoding.
func (r *Lucene94HnswVectorsReader) GetFieldEntry(field string, enc index.VectorEncoding) (*lucene94FieldEntry, error) {
	e, err := r.getFieldEntryOrThrow(field)
	if err != nil {
		return nil, err
	}
	if e.vectorEncoding != enc {
		return nil, fmt.Errorf("field %q: encoding mismatch: got %v, want %v",
			field, e.vectorEncoding, enc)
	}
	return e, nil
}

// VectorDataInput returns a slice of the .vec file for the given field.
func (r *Lucene94HnswVectorsReader) VectorDataInput(e *lucene94FieldEntry) (store.IndexInput, error) {
	return r.vectorData.Slice("vector-data", e.vectorDataOffset, e.vectorDataLength)
}

// VectorIndexInput returns a slice of the .vex file for the given field.
func (r *Lucene94HnswVectorsReader) VectorIndexInput(e *lucene94FieldEntry) (store.IndexInput, error) {
	return r.vectorIdx.Slice("graph-data", e.vectorIndexOffset, e.vectorIndexLength)
}

// Close releases resources held by this reader.
func (r *Lucene94HnswVectorsReader) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	var errs []error
	if r.vectorData != nil {
		if err := r.vectorData.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if r.vectorIdx != nil {
		if err := r.vectorIdx.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// ensure Lucene94HnswVectorsReader satisfies io.Closer at compile time.
var _ io.Closer = (*Lucene94HnswVectorsReader)(nil)
