// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene95

import (
	"errors"
	"fmt"
	"io"

	bcstore "github.com/FlavioCFOliveira/Gocene/backward_codecs/store"
	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// Lucene 9.5 HNSW format file-descriptor constants.
const (
	lucene95MetaCodecName        = "Lucene95HnswVectorsFormatMeta"
	lucene95VectorDataCodecName  = "Lucene95HnswVectorsFormatData"
	lucene95VectorIndexCodecName = "Lucene95HnswVectorsFormatIndex"
	lucene95MetaExtension        = "vem"
	lucene95VectorDataExtension  = "vec"
	lucene95VectorIndexExtension = "vex"
	lucene95VersionStart         = int32(0)
	lucene95VersionCurrent       = int32(1)
)

// lucene95FieldEntry holds all per-field metadata read from the .vem file.
//
// The key structural differences from Lucene94's FieldEntry are:
//   - M and numLevels are VInt-encoded (not int32).
//   - HNSW neighbour-offset addressing uses a DirectMonotonicReader.Meta
//     (offsetsMeta) stored inline, rather than the derived graphOffsetsByLevel
//     approach used by Lucene94.
//
// Port of the FieldEntry record in Lucene95HnswVectorsReader.java.
type lucene95FieldEntry struct {
	vectorEncoding     index.VectorEncoding
	similarityFunction index.VectorSimilarityFunction

	vectorDataOffset  int64
	vectorDataLength  int64
	vectorIndexOffset int64
	vectorIndexLength int64

	dimension int
	size      int

	// OrdToDoc DISI configuration (mirrors OrdToDocDISIReaderConfiguration).
	// docsWithFieldOffset special values: -1 = dense, -2 = empty, other = sparse.
	docsWithFieldOffset int64
	docsWithFieldLength int64
	jumpTableEntryCount int16
	denseRankPower      int8

	// DirectMonotonic ordToDoc mapping (sparse case only).
	addressesOffset int64
	blockShift      int
	meta            *packed.DirectMonotonicMeta
	addressesLength int64

	// HNSW graph topology.
	maxConn      int
	numLevels    int
	nodesByLevel [][]int32

	// HNSW neighbour-offset addressing (DirectMonotonic over all nodes across levels).
	// Non-nil only when numberOfOffsets > 0.
	offsetsOffset     int64
	offsetsBlockShift int
	offsetsMeta       *packed.DirectMonotonicMeta
	offsetsLength     int64
	numberOfOffsets   int64
}

// Lucene95HnswVectorsReader reads float32 and byte vector values together with
// the associated HNSW graph from segments written in the Lucene 9.5 format.
//
// Compared to Lucene94HnswVectorsReader, this reader changes the HNSW neighbour
// offset storage to use DirectMonotonicReader instead of the derived
// graphOffsetsByLevel approach.
//
// Port of org.apache.lucene.backward_codecs.lucene95.Lucene95HnswVectorsReader.
type Lucene95HnswVectorsReader struct {
	fields     map[int]*lucene95FieldEntry
	vectorData store.IndexInput
	vectorIdx  store.IndexInput
	fieldInfos *index.FieldInfos
	closed     bool
}

// NewLucene95HnswVectorsReader opens the .vem / .vec / .vex files for state and
// returns a ready-to-use Lucene95HnswVectorsReader.
func NewLucene95HnswVectorsReader(state *index.SegmentReadState) (*Lucene95HnswVectorsReader, error) {
	r := &Lucene95HnswVectorsReader{
		fields:     make(map[int]*lucene95FieldEntry),
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

	r.vectorData, err = openLucene95DataInput(state, versionMeta,
		lucene95VectorDataExtension, lucene95VectorDataCodecName)
	if err != nil {
		return nil, err
	}

	r.vectorIdx, err = openLucene95DataInput(state, versionMeta,
		lucene95VectorIndexExtension, lucene95VectorIndexCodecName)
	if err != nil {
		return nil, err
	}

	ok = true
	return r, nil
}

// readMetadata reads the .vem file and populates r.fields.
func (r *Lucene95HnswVectorsReader) readMetadata(state *index.SegmentReadState) (int32, error) {
	metaName := index.SegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, lucene95MetaExtension)

	meta, err := bcstore.OpenChecksumInput(state.Directory, metaName,
		store.IOContext{Context: store.ContextReadOnce})
	if err != nil {
		return 0, fmt.Errorf("lucene95 vectors: open meta %q: %w", metaName, err)
	}
	defer func() { _ = meta.Close() }()

	version, err := codecs.CheckIndexHeader(
		meta,
		lucene95MetaCodecName,
		lucene95VersionStart,
		lucene95VersionCurrent,
		state.SegmentInfo.GetID(),
		state.SegmentSuffix,
	)
	if err != nil {
		return 0, fmt.Errorf("lucene95 vectors meta header: %w", err)
	}

	if err = r.readFields(meta, state.FieldInfos); err != nil {
		return 0, err
	}

	if err = checkLucene95Footer(meta); err != nil {
		return 0, fmt.Errorf("lucene95 vectors meta footer: %w", err)
	}
	return version, nil
}

// checkLucene95Footer validates the codec footer on an
// EndiannessReverserChecksumIndexInput.
func checkLucene95Footer(in *bcstore.EndiannessReverserChecksumIndexInput) error {
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
func (r *Lucene95HnswVectorsReader) readFields(in store.DataInput, infos *index.FieldInfos) error {
	for {
		fieldNumber, err := store.ReadInt32(in)
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
		entry, err := readLucene95FieldEntry(in, fi)
		if err != nil {
			return err
		}
		if err = validateLucene95FieldEntry(fi, entry); err != nil {
			return err
		}
		r.fields[fi.Number()] = entry
	}
}

// readLucene95FieldEntry deserialises one field entry from the metadata stream.
//
// Wire layout mirrors FieldEntry.create() in Lucene95HnswVectorsReader.java:
//
//	[int]   vectorEncoding ordinal
//	[int]   similarityFunction ordinal
//	[vlong] vectorDataOffset
//	[vlong] vectorDataLength
//	[vlong] vectorIndexOffset
//	[vlong] vectorIndexLength
//	[vint]  dimension
//	[int]   size
//	OrdToDocDISIReaderConfiguration fields (fromStoredMeta)
//	[vint]  M
//	[vint]  numLevels
//	For each level > 0:
//	  [vint]     numNodesOnLevel
//	  array[vint] delta-encoded ordinals
//	If numberOfOffsets > 0:
//	  [long]  offsetsOffset
//	  [vint]  offsetsBlockShift
//	  DirectMonotonicReader.Meta
//	  [long]  offsetsLength
func readLucene95FieldEntry(in store.DataInput, fi *index.FieldInfo) (*lucene95FieldEntry, error) {
	// vector encoding
	encID, err := store.ReadInt32(in)
	if err != nil {
		return nil, fmt.Errorf("readFieldEntry %q encoding: %w", fi.Name(), err)
	}
	enc := index.VectorEncoding(encID)
	if enc != fi.VectorEncoding() {
		return nil, fmt.Errorf("readFieldEntry %q: encoding mismatch %v != %v",
			fi.Name(), enc, fi.VectorEncoding())
	}

	// similarity function
	simID, err := store.ReadInt32(in)
	if err != nil {
		return nil, fmt.Errorf("readFieldEntry %q similarity: %w", fi.Name(), err)
	}
	simFn := index.VectorSimilarityFunction(simID)
	if simFn != fi.VectorSimilarityFunction() {
		return nil, fmt.Errorf("readFieldEntry %q: similarity mismatch %v != %v",
			fi.Name(), simFn, fi.VectorSimilarityFunction())
	}

	e := &lucene95FieldEntry{vectorEncoding: enc, similarityFunction: simFn}

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

	dim, err := store.ReadVInt(in)
	if err != nil {
		return nil, fmt.Errorf("readFieldEntry %q dimension: %w", fi.Name(), err)
	}
	e.dimension = int(dim)

	sz, err := store.ReadInt32(in)
	if err != nil {
		return nil, fmt.Errorf("readFieldEntry %q size: %w", fi.Name(), err)
	}
	e.size = int(sz)

	// OrdToDocDISIReaderConfiguration.fromStoredMeta fields
	docsOff, err := store.ReadInt64(in)
	if err != nil {
		return nil, fmt.Errorf("readFieldEntry %q docsWithFieldOffset: %w", fi.Name(), err)
	}
	e.docsWithFieldOffset = docsOff

	docsLen, err := store.ReadInt64(in)
	if err != nil {
		return nil, fmt.Errorf("readFieldEntry %q docsWithFieldLength: %w", fi.Name(), err)
	}
	e.docsWithFieldLength = docsLen

	jt, err := store.ReadInt16(in)
	if err != nil {
		return nil, fmt.Errorf("readFieldEntry %q jumpTableEntryCount: %w", fi.Name(), err)
	}
	e.jumpTableEntryCount = jt

	drp, err := in.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("readFieldEntry %q denseRankPower: %w", fi.Name(), err)
	}
	e.denseRankPower = int8(drp)

	// sparse ordToDoc mapping (condition: docsWithFieldOffset > -1)
	if e.docsWithFieldOffset > -1 {
		addrOff, err2 := store.ReadInt64(in)
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

		addrLen, err2 := store.ReadInt64(in)
		if err2 != nil {
			return nil, fmt.Errorf("readFieldEntry %q addressesLength: %w", fi.Name(), err2)
		}
		e.addressesLength = addrLen
	}

	// HNSW graph topology (VInt-encoded in Lucene95, unlike Lucene94's int32)
	m, err := store.ReadVInt(in)
	if err != nil {
		return nil, fmt.Errorf("readFieldEntry %q M: %w", fi.Name(), err)
	}
	e.maxConn = int(m)

	nl, err := store.ReadVInt(in)
	if err != nil {
		return nil, fmt.Errorf("readFieldEntry %q numLevels: %w", fi.Name(), err)
	}
	e.numLevels = int(nl)

	e.nodesByLevel = make([][]int32, e.numLevels)
	var numberOfOffsets int64
	for level := 0; level < e.numLevels; level++ {
		if level > 0 {
			cnt, err2 := store.ReadVInt(in)
			if err2 != nil {
				return nil, fmt.Errorf("readFieldEntry %q nodesByLevel[%d] count: %w",
					fi.Name(), level, err2)
			}
			numNodesOnLevel := int(cnt)
			numberOfOffsets += int64(numNodesOnLevel)
			nodes := make([]int32, numNodesOnLevel)
			if numNodesOnLevel > 0 {
				first, err2 := store.ReadVInt(in)
				if err2 != nil {
					return nil, fmt.Errorf("readFieldEntry %q nodesByLevel[%d][0]: %w",
						fi.Name(), level, err2)
				}
				nodes[0] = first
				for i := 1; i < numNodesOnLevel; i++ {
					delta, err2 := store.ReadVInt(in)
					if err2 != nil {
						return nil, fmt.Errorf("readFieldEntry %q nodesByLevel[%d][%d]: %w",
							fi.Name(), level, i, err2)
					}
					nodes[i] = nodes[i-1] + delta
				}
			}
			e.nodesByLevel[level] = nodes
		} else {
			// level 0: all nodes, count is implicit (= size)
			numberOfOffsets += int64(e.size)
		}
	}
	e.numberOfOffsets = numberOfOffsets

	// HNSW neighbour-offset addressing (only if there are nodes)
	if numberOfOffsets > 0 {
		offOff, err2 := store.ReadInt64(in)
		if err2 != nil {
			return nil, fmt.Errorf("readFieldEntry %q offsetsOffset: %w", fi.Name(), err2)
		}
		e.offsetsOffset = offOff

		obShift, err2 := store.ReadVInt(in)
		if err2 != nil {
			return nil, fmt.Errorf("readFieldEntry %q offsetsBlockShift: %w", fi.Name(), err2)
		}
		e.offsetsBlockShift = int(obShift)

		e.offsetsMeta, err2 = packed.LoadDirectMonotonicMeta(in, numberOfOffsets, e.offsetsBlockShift)
		if err2 != nil {
			return nil, fmt.Errorf("readFieldEntry %q offsetsMeta: %w", fi.Name(), err2)
		}

		offLen, err2 := store.ReadInt64(in)
		if err2 != nil {
			return nil, fmt.Errorf("readFieldEntry %q offsetsLength: %w", fi.Name(), err2)
		}
		e.offsetsLength = offLen
	}

	return e, nil
}

// validateLucene95FieldEntry checks metadata self-consistency against FieldInfo.
func validateLucene95FieldEntry(fi *index.FieldInfo, e *lucene95FieldEntry) error {
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

// openLucene95DataInput opens a .vec or .vex file, validates header, and retrieves
// the trailing checksum offset.
func openLucene95DataInput(
	state *index.SegmentReadState,
	versionMeta int32,
	ext, codecName string,
) (store.IndexInput, error) {
	name := index.SegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, ext)
	in, err := state.Directory.OpenInput(name, store.IOContextRead)
	if err != nil {
		return nil, fmt.Errorf("lucene95 vectors: open %q: %w", name, err)
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
		lucene95VersionStart,
		lucene95VersionCurrent,
		state.SegmentInfo.GetID(),
		state.SegmentSuffix,
	)
	if err != nil {
		return nil, fmt.Errorf("lucene95 vectors %q header: %w", name, err)
	}
	if version != versionMeta {
		return nil, fmt.Errorf("lucene95 vectors %q: version mismatch meta=%d data=%d",
			name, versionMeta, version)
	}
	if _, err = codecs.RetrieveChecksum(in); err != nil {
		return nil, fmt.Errorf("lucene95 vectors %q checksum: %w", name, err)
	}

	ok = true
	return in, nil
}

// CheckIntegrity verifies the checksums of the .vec and .vex files.
func (r *Lucene95HnswVectorsReader) CheckIntegrity() error {
	if r.closed {
		return errors.New("lucene95 vectors reader: already closed")
	}
	if _, err := codecs.ChecksumEntireFile(r.vectorData); err != nil {
		return fmt.Errorf("lucene95 vectors checkIntegrity .vec: %w", err)
	}
	if _, err := codecs.ChecksumEntireFile(r.vectorIdx); err != nil {
		return fmt.Errorf("lucene95 vectors checkIntegrity .vex: %w", err)
	}
	return nil
}

// getFieldEntryOrThrow returns the field entry by name, or an error if missing.
func (r *Lucene95HnswVectorsReader) getFieldEntryOrThrow(field string) (*lucene95FieldEntry, error) {
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
func (r *Lucene95HnswVectorsReader) GetFieldEntry(field string, enc index.VectorEncoding) (*lucene95FieldEntry, error) {
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
func (r *Lucene95HnswVectorsReader) VectorDataInput(e *lucene95FieldEntry) (store.IndexInput, error) {
	return r.vectorData.Slice("vector-data", e.vectorDataOffset, e.vectorDataLength)
}

// VectorIndexInput returns a slice of the .vex file for the given field.
func (r *Lucene95HnswVectorsReader) VectorIndexInput(e *lucene95FieldEntry) (store.IndexInput, error) {
	return r.vectorIdx.Slice("graph-data", e.vectorIndexOffset, e.vectorIndexLength)
}

// Close releases resources held by this reader.
func (r *Lucene95HnswVectorsReader) Close() error {
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

// ensure Lucene95HnswVectorsReader satisfies io.Closer at compile time.
var _ io.Closer = (*Lucene95HnswVectorsReader)(nil)
