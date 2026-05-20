// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package lucene92

import (
	"errors"
	"fmt"
	"io"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	bcstore "github.com/FlavioCFOliveira/Gocene/backward_codecs/store"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// lucene92FieldEntry holds all per-field metadata read from the .vem file.
// Port of the FieldEntry record in Lucene92HnswVectorsReader.java.
type lucene92FieldEntry struct {
	similarityFunction index.VectorSimilarityFunction

	vectorDataOffset  int64
	vectorDataLength  int64
	vectorIndexOffset int64
	vectorIndexLength int64

	dimension int
	size      int

	// docsWithField encoding (mirrors IndexedDISI sentinel convention):
	//   -1 = dense (all documents have a value)
	//   -2 = empty (no documents have a value)
	//   other = sparse
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
	maxConn  int // M parameter
	numLevels int
	nodesByLevel        [][]int32
	// graphOffsetsByLevel[l] is the byte offset in .vex where level l begins
	graphOffsetsByLevel []int64
}

// Lucene92HnswVectorsReader reads float32 vector values and the associated HNSW
// graph from segments written in the Lucene 9.2 format.
//
// This is a read-only backward-compatibility reader; write operations are not
// supported.
//
// Port of org.apache.lucene.backward_codecs.lucene92.Lucene92HnswVectorsReader.
type Lucene92HnswVectorsReader struct {
	fields     map[int]*lucene92FieldEntry // keyed by field number
	vectorData store.IndexInput
	vectorIdx  store.IndexInput
	fieldInfos *index.FieldInfos
	closed     bool
}

// NewLucene92HnswVectorsReader opens the .vem / .vec / .vex files for state and
// returns a ready-to-use Lucene92HnswVectorsReader.
func NewLucene92HnswVectorsReader(state *index.SegmentReadState) (*Lucene92HnswVectorsReader, error) {
	r := &Lucene92HnswVectorsReader{
		fields:     make(map[int]*lucene92FieldEntry),
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

	r.vectorData, err = openLucene92DataInput(state, versionMeta,
		lucene92VectorDataExtension, lucene92VectorDataCodecName)
	if err != nil {
		return nil, err
	}

	r.vectorIdx, err = openLucene92DataInput(state, versionMeta,
		lucene92VectorIndexExtension, lucene92VectorIndexCodecName)
	if err != nil {
		return nil, err
	}

	ok = true
	return r, nil
}

// readMetadata reads the .vem file and populates r.fields.
func (r *Lucene92HnswVectorsReader) readMetadata(state *index.SegmentReadState) (int32, error) {
	metaName := index.SegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, lucene92MetaExtension)

	meta, err := bcstore.OpenChecksumInput(state.Directory, metaName,
		store.IOContext{Context: store.ContextReadOnce})
	if err != nil {
		return 0, fmt.Errorf("lucene92 vectors: open meta %q: %w", metaName, err)
	}
	defer func() { _ = meta.Close() }()

	version, err := codecs.CheckIndexHeader(
		meta,
		lucene92MetaCodecName,
		lucene92VersionStart,
		lucene92VersionCurrent,
		state.SegmentInfo.GetID(),
		state.SegmentSuffix,
	)
	if err != nil {
		return 0, fmt.Errorf("lucene92 vectors meta header: %w", err)
	}

	if err = r.readFields(meta, state.FieldInfos); err != nil {
		return 0, err
	}

	if err = checkLucene92Footer(meta); err != nil {
		return 0, fmt.Errorf("lucene92 vectors meta footer: %w", err)
	}
	return version, nil
}

// checkLucene92Footer validates the codec footer of an
// EndiannessReverserChecksumIndexInput (which is not a *store.ChecksumIndexInput,
// so it cannot be passed to codecs.CheckFooter directly).
func checkLucene92Footer(in *bcstore.EndiannessReverserChecksumIndexInput) error {
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
func (r *Lucene92HnswVectorsReader) readFields(in store.DataInput, infos *index.FieldInfos) error {
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
		entry, err := readLucene92FieldEntry(in, fi)
		if err != nil {
			return err
		}
		if err = validateLucene92FieldEntry(fi, entry); err != nil {
			return err
		}
		r.fields[fi.Number()] = entry
	}
}

// readLucene92FieldEntry deserialises one field entry from the metadata stream.
func readLucene92FieldEntry(in store.DataInput, fi *index.FieldInfo) (*lucene92FieldEntry, error) {
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

	e := &lucene92FieldEntry{similarityFunction: simFn}

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

	dim, err := store.ReadInt32(in)
	if err != nil {
		return nil, fmt.Errorf("readFieldEntry %q dimension: %w", fi.Name(), err)
	}
	e.dimension = int(dim)

	sz, err := store.ReadInt32(in)
	if err != nil {
		return nil, fmt.Errorf("readFieldEntry %q size: %w", fi.Name(), err)
	}
	e.size = int(sz)

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

	// sparse ordToDoc mapping
	if e.docsWithFieldOffset != -1 && e.docsWithFieldOffset != -2 {
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

	// HNSW graph topology
	m32, err := store.ReadInt32(in)
	if err != nil {
		return nil, fmt.Errorf("readFieldEntry %q M: %w", fi.Name(), err)
	}
	e.maxConn = int(m32)

	nl, err := store.ReadInt32(in)
	if err != nil {
		return nil, fmt.Errorf("readFieldEntry %q numLevels: %w", fi.Name(), err)
	}
	e.numLevels = int(nl)

	e.nodesByLevel = make([][]int32, e.numLevels)
	for level := 0; level < e.numLevels; level++ {
		cnt, err2 := store.ReadInt32(in)
		if err2 != nil {
			return nil, fmt.Errorf("readFieldEntry %q nodesByLevel[%d] count: %w", fi.Name(), level, err2)
		}
		if level == 0 {
			// level 0 contains all nodes — not stored explicitly
			e.nodesByLevel[0] = nil
		} else {
			nodes := make([]int32, cnt)
			for i := int32(0); i < cnt; i++ {
				n, err3 := store.ReadInt32(in)
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
	bytesLevel0 := int64(1+2*e.maxConn) * 4 // (1 + M*2) * sizeof(int32) for level 0
	bytesOther := int64(1+e.maxConn) * 4    // (1 + M) * sizeof(int32) for levels > 0
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

// validateLucene92FieldEntry checks that the on-disk metadata is self-consistent
// with the in-memory FieldInfo.
func validateLucene92FieldEntry(fi *index.FieldInfo, e *lucene92FieldEntry) error {
	if fi.VectorDimension() != e.dimension {
		return fmt.Errorf("field %q: vector dimension mismatch %d != %d",
			fi.Name(), fi.VectorDimension(), e.dimension)
	}
	wantBytes := int64(e.size) * int64(e.dimension) * 4 // float32 = 4 bytes
	if wantBytes != e.vectorDataLength {
		return fmt.Errorf("field %q: vectorDataLength %d != size*dim*4 = %d",
			fi.Name(), e.vectorDataLength, wantBytes)
	}
	return nil
}

// openLucene92DataInput opens a .vec or .vex file for the given segment, verifies
// the codec header and retrieves the trailing checksum.
func openLucene92DataInput(
	state *index.SegmentReadState,
	versionMeta int32,
	ext, codecName string,
) (store.IndexInput, error) {
	name := index.SegmentFileName(state.SegmentInfo.Name(), state.SegmentSuffix, ext)
	in, err := state.Directory.OpenInput(name, store.IOContextRead)
	if err != nil {
		return nil, fmt.Errorf("lucene92 vectors: open %q: %w", name, err)
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
		lucene92VersionStart,
		lucene92VersionCurrent,
		state.SegmentInfo.GetID(),
		state.SegmentSuffix,
	)
	if err != nil {
		return nil, fmt.Errorf("lucene92 vectors %q header: %w", name, err)
	}
	if version != versionMeta {
		return nil, fmt.Errorf("lucene92 vectors %q: version mismatch meta=%d data=%d",
			name, versionMeta, version)
	}
	if _, err = codecs.RetrieveChecksum(in); err != nil {
		return nil, fmt.Errorf("lucene92 vectors %q checksum: %w", name, err)
	}

	ok = true
	return in, nil
}

// CheckIntegrity verifies the checksums of the .vec and .vex files.
func (r *Lucene92HnswVectorsReader) CheckIntegrity() error {
	if r.closed {
		return errors.New("lucene92 vectors reader: already closed")
	}
	if _, err := codecs.ChecksumEntireFile(r.vectorData); err != nil {
		return fmt.Errorf("lucene92 vectors checkIntegrity .vec: %w", err)
	}
	if _, err := codecs.ChecksumEntireFile(r.vectorIdx); err != nil {
		return fmt.Errorf("lucene92 vectors checkIntegrity .vex: %w", err)
	}
	return nil
}

// GetFieldEntry returns the field entry for the named field, or an error if not found.
func (r *Lucene92HnswVectorsReader) GetFieldEntry(field string) (*lucene92FieldEntry, error) {
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

// VectorDataInput returns a slice of the .vec file covering the given field's data.
func (r *Lucene92HnswVectorsReader) VectorDataInput(e *lucene92FieldEntry) (store.IndexInput, error) {
	return r.vectorData.Slice("vector-data", e.vectorDataOffset, e.vectorDataLength)
}

// VectorIndexInput returns a slice of the .vex file covering the given field's graph.
func (r *Lucene92HnswVectorsReader) VectorIndexInput(e *lucene92FieldEntry) (store.IndexInput, error) {
	return r.vectorIdx.Slice("graph-data", e.vectorIndexOffset, e.vectorIndexLength)
}

// Close releases resources held by this reader.
func (r *Lucene92HnswVectorsReader) Close() error {
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

// ensure Lucene92HnswVectorsReader satisfies io.Closer at compile time.
var _ io.Closer = (*Lucene92HnswVectorsReader)(nil)
