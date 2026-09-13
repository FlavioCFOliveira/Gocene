// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"sync"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/internal/util"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util/compress"
)

// CompressingStoredFieldsFormat is a StoredFieldsFormat that compresses documents
// in chunks and stores them in compressed form.
type CompressingStoredFieldsFormat struct {
	*BaseStoredFieldsFormat
	compressionMode CompressionMode
	chunkSize       int
	maxDocsPerChunk int
}

func NewCompressingStoredFieldsFormat(mode CompressionMode, chunkSize, maxDocsPerChunk int) *CompressingStoredFieldsFormat {
	return &CompressingStoredFieldsFormat{
		BaseStoredFieldsFormat: NewBaseStoredFieldsFormat("CompressingStoredFieldsFormat"),
		compressionMode:        mode,
		chunkSize:              chunkSize,
		maxDocsPerChunk:        maxDocsPerChunk,
	}
}

func (f *CompressingStoredFieldsFormat) FieldsReader(dir store.Directory, segmentInfo *index.SegmentInfo, fieldInfos *index.FieldInfos, context store.IOContext) (spi.StoredFieldsReader, error) {
	return NewCompressingStoredFieldsReader(dir, segmentInfo, fieldInfos, f.compressionMode, f.chunkSize, f.maxDocsPerChunk)
}

func (f *CompressingStoredFieldsFormat) FieldsWriter(dir store.Directory, segmentInfo *index.SegmentInfo, context store.IOContext) (spi.StoredFieldsWriter, error) {
	return nil, fmt.Errorf("old formats can't be used for writing")
}

type CompressionMode int

const (
	CompressionModeLZ4Fast CompressionMode = iota
	CompressionModeLZ4High
	CompressionModeDeflate
)

func (m CompressionMode) decompressor() func([]byte, int) ([]byte, error) {
	switch m {
	case CompressionModeLZ4Fast, CompressionModeLZ4High:
		return lz4Decompress
	case CompressionModeDeflate:
		return deflateDecompress
	default:
		return lz4Decompress
	}
}

// lz4Decompress decompresses an LZ4 block produced by Apache Lucene.
//
// It is the adapter that gives this package's []byte-shaped decompressor
// signature access to the faithful port of org.apache.lucene.util.compress.LZ4
// in util/compress. The reference is the anonymous LZ4_DECOMPRESSOR held by
// org.apache.lucene.codecs.compressing.CompressionMode (Lucene 10.5.0,
// CompressionMode.java:118-141), which is the Decompressor returned by both
// CompressionMode.FAST and CompressionMode.FAST_DECOMPRESSION:
//
//	public void decompress(DataInput in, int originalLength, int offset,
//	                       int length, BytesRef bytes) throws IOException {
//	  assert offset + length <= originalLength;
//	  // add 7 padding bytes, this is not necessary but can help decompression run faster
//	  if (bytes.bytes.length < originalLength + 7) {
//	    bytes.bytes = new byte[ArrayUtil.oversize(originalLength + 7, 1)];
//	  }
//	  final int decompressedLength = LZ4.decompress(in, offset + length, bytes.bytes, 0);
//	  if (decompressedLength > originalLength) {
//	    throw new CorruptIndexException(
//	        "Corrupted: lengths mismatch: " + decompressedLength + " > " + originalLength, in);
//	  }
//	  bytes.offset = offset;
//	  bytes.length = length;
//	}
//
// This entry point carries the whole-block case (Java offset == 0 and
// length == originalLength), so Java's LZ4.decompress(in, offset + length, ...)
// becomes LZ4Decompress(in, uncompressedLen, ...). The 7 padding bytes and the
// "lengths mismatch" guard are reproduced exactly.
//
// LZ4.decompress is DataInput-based in Java and so is its Go port, so the
// []byte block is wrapped in a store.ByteArrayDataInput rather than the LZ4
// decoder being written a second time against a slice.
func lz4Decompress(data []byte, uncompressedLen int) ([]byte, error) {
	dest := make([]byte, uncompressedLen+lz4DecompressorPadding)
	in := store.NewByteArrayDataInput(data)
	decompressedLength, err := compress.LZ4Decompress(in, uncompressedLen, dest, 0)
	if err != nil {
		return nil, err
	}
	if decompressedLength > uncompressedLen {
		return nil, fmt.Errorf("Corrupted: lengths mismatch: %d > %d", decompressedLength, uncompressedLen)
	}
	return dest[:decompressedLength], nil
}

func deflateDecompress(data []byte, uncompressedLen int) ([]byte, error) {
	buf := bytes.NewReader(data)
	r, err := zlib.NewReader(buf)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	result := make([]byte, uncompressedLen)
	n, err := io.ReadFull(r, result)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	if n != uncompressedLen {
		return nil, fmt.Errorf("decompressed length mismatch: expected %d, got %d", uncompressedLen, n)
	}
	return result, nil
}

type CompressingStoredFieldsReader struct {
	directory         store.Directory
	segmentInfo       *index.SegmentInfo
	fieldInfos        *index.FieldInfos
	compressionMode   CompressionMode
	chunkSize         int
	packedIntsVersion int
	fieldsStream      store.IndexInput
	indexReader       FieldsIndex
	maxPointer        int64
	numDocs           int
	closed            bool
	mu                sync.Mutex
	state             *blockState
}

type blockState struct {
	docBase      int
	chunkDocs    int
	sliced       bool
	offsets      []int64
	numFields    []int
	startPointer int64
}

func NewCompressingStoredFieldsReader(dir store.Directory, si *index.SegmentInfo, fn *index.FieldInfos, mode CompressionMode, chunkSize, maxDocsPerChunk int) (*CompressingStoredFieldsReader, error) {
	r := &CompressingStoredFieldsReader{
		directory:       dir,
		segmentInfo:     si,
		fieldInfos:      fn,
		compressionMode: mode,
		chunkSize:       chunkSize,
		numDocs:         si.MaxDoc(),
	}

	if err := r.load(); err != nil {
		return nil, err
	}

	return r, nil
}

func (r *CompressingStoredFieldsReader) load() error {
	segment := r.segmentInfo.Name()
	fieldsStreamFN := segment + ".fdt"

	fieldsStream, err := r.directory.OpenInput(fieldsStreamFN, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return err
	}
	r.fieldsStream = fieldsStream

	version, err := store.CheckIndexHeader(fieldsStream, "CompressingStoredFieldsFormat", 1, 4, r.segmentInfo.ID(), "")
	if err != nil {
		return err
	}

	var metaIn store.IndexInput
	if version >= 2 {
		metaStreamFN := segment + ".fdm"
		metaIn, err = r.directory.OpenInput(metaStreamFN, store.IOContext{Context: store.ContextRead})
		if err != nil {
			return err
		}
		defer metaIn.Close()

		if _, err := store.CheckIndexHeader(metaIn, "Lucene85FieldsIndexMeta", 0, version, r.segmentInfo.ID(), ""); err != nil {
			return err
		}
	}

	if version >= 3 {
		r.chunkSize, _ = store.ReadVInt(metaIn)
		r.packedIntsVersion, _ = store.ReadVInt(metaIn)
	} else {
		r.chunkSize, _ = store.ReadVInt(fieldsStream)
		r.packedIntsVersion, _ = store.ReadVInt(fieldsStream)
	}

	// Index reading
	indexFileName := segment + ".fdx"
	indexReader, err := NewFieldsIndexReader(r.directory, segment, "", "fdx", "Lucene85FieldsIndex", r.segmentInfo.ID(), metaIn)
	if err != nil {
		return err
	}
	r.indexReader = indexReader
	r.maxPointer = indexReader.MaxPointer()

	return nil
}

func (r *CompressingStoredFieldsReader) VisitDocument(docID int, visitor spi.StoredFieldVisitor) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return fmt.Errorf("reader is closed")
	}

	if r.state == nil || !r.state.contains(docID) {
		r.fieldsStream.SetPosition(r.indexReader.GetStartPointer(docID))
		if err := r.resetState(docID); err != nil {
			return err
		}
	}

	return r.visit(docID, visitor)
}

func (r *CompressingStoredFieldsReader) resetState(docID int) error {
	docBase, _ := store.ReadVInt(r.fieldsStream)
	token, _ := store.ReadVInt(r.fieldsStream)

	chunkDocs := token >> 2 // simplified
	sliced := (token & 1) != 0

	r.state = &blockState{
		docBase:   docBase,
		chunkDocs: chunkDocs,
		sliced:    sliced,
		offsets:   make([]int64, chunkDocs+1),
		numFields: make([]int, chunkDocs),
	}

	if chunkDocs == 1 {
		nf, _ := store.ReadVInt(r.fieldsStream)
		r.state.numFields[0] = nf
		off, _ := store.ReadVInt(r.fieldsStream)
		r.state.offsets[1] = int64(off)
	} else {
		bitsPerFields, _ := store.ReadVInt(r.fieldsStream)
		if bitsPerFields == 0 {
			val, _ := store.ReadVInt(r.fieldsStream)
			for i := 0; i < chunkDocs; i++ {
				r.state.numFields[i] = val
			}
		} else {
			decoder := util.NewPackedIntsDecoder(readBytes(r.fieldsStream), bitsPerFields)
			for i := 0; i < chunkDocs; i++ {
				val, _ := decoder.Next()
				r.state.numFields[i] = int(val)
			}
		}

		bitsPerLength, _ := store.ReadVInt(r.fieldsStream)
		if bitsPerLength == 0 {
			val, _ := store.ReadVInt(r.fieldsStream)
			for i := 0; i < chunkDocs; i++ {
				r.state.offsets[i+1] = int64(i+1) * int64(val)
			}
		} else {
			decoder := util.NewPackedIntsDecoder(readBytes(r.fieldsStream), bitsPerLength)
			for i := 0; i < chunkDocs; i++ {
				val, _ := decoder.Next()
				r.state.offsets[i+1] = r.state.offsets[i] + val
			}
		}
	}

	r.state.startPointer = r.fieldsStream.GetPosition()
	return nil
}

func (r *CompressingStoredFieldsReader) visit(docID int, visitor spi.StoredFieldVisitor) error {
	index := docID - r.state.docBase
	offset := r.state.offsets[index]
	length := r.state.offsets[index+1] - offset

	// Decompression
	r.fieldsStream.SetPosition(r.state.startPointer)
	decomp := r.compressionMode.decompressor()

	// Simplified: read whole chunk and then slice
	// In production, handle sliced chunks properly
	data, err := readBytes(r.fieldsStream)
	if err != nil {
		return err
	}

	uncompressed, err := decomp(data, 0) // simplified
	if err != nil {
		return err
	}

	docData := uncompressed[offset : offset+length]

	for i := 0; i < r.state.numFields[index]; i++ {
		if len(docData) == 0 {
			break
		}

		infoAndBits, _ := docData.ReadVLong() // Need to wrap []byte as store.IndexInput
		// ... handle fields ...
	}

	return nil
}

func (r *CompressingStoredFieldsReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	return r.fieldsStream.Close()
}

func (s *blockState) contains(docID int) bool {
	return docID >= s.docBase && docID < s.docBase+s.chunkDocs
}

func readBytes(in store.IndexInput) ([]byte, error) {
	// Helper to read remaining bytes
	var buf bytes.Buffer
	tmp := make([]byte, 1024)
	for {
		n, err := in.ReadBytes(tmp, 0, len(tmp))
		if n > 0 {
			buf.Write(tmp[:n])
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

// CheckIntegrity validates the checksums of the fields index and of the entire
// fields stream.
//
// Port of
// org.apache.lucene.codecs.lucene90.compressing.Lucene90CompressingStoredFieldsReader#checkIntegrity
// (Lucene 10.5.0):
//
//	indexReader.checkIntegrity();
//	CodecUtil.checksumEntireFile(fieldsStream);
func (r *CompressingStoredFieldsReader) CheckIntegrity() error {
	if err := r.indexReader.CheckIntegrity(); err != nil {
		return err
	}
	_, err := ChecksumEntireFile(r.fieldsStream)
	return err
}
