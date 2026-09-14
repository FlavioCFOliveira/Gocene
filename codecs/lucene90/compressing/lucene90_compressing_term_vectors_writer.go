// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package compressing

import (
	"errors"
	"fmt"
	"math"
	"sort"

	gcodecs "github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/codecs/compressing"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// Constants of Lucene90CompressingTermVectorsWriter (Apache Lucene 10.5.0),
// shared with Lucene90CompressingTermVectorsReader.
const (
	// vectorsExtension is VECTORS_EXTENSION: the extension of the vectors
	// data file.
	vectorsExtension = "tvd"

	// vectorsIndexExtension is VECTORS_INDEX_EXTENSION: the extension of the
	// vectors index file.
	vectorsIndexExtension = "tvx"

	// vectorsMetaExtension is VECTORS_META_EXTENSION: the extension of the
	// vectors meta file.
	vectorsMetaExtension = "tvm"

	// vectorsIndexCodecName is VECTORS_INDEX_CODEC_NAME: the codec name of
	// the vectors index file.
	vectorsIndexCodecName = "Lucene90TermVectorsIndex"

	// termVectorsVersionStart is VERSION_START.
	termVectorsVersionStart = int32(0)

	// termVectorsVersionCurrent is VERSION_CURRENT.
	termVectorsVersionCurrent = termVectorsVersionStart

	// termVectorsMetaVersionStart is META_VERSION_START.
	termVectorsMetaVersionStart = int32(0)

	// termVectorsPackedBlockSize is PACKED_BLOCK_SIZE.
	termVectorsPackedBlockSize = 64

	// termVectorsPositions is POSITIONS.
	termVectorsPositions = 0x01

	// termVectorsOffsets is OFFSETS.
	termVectorsOffsets = 0x02

	// termVectorsPayloads is PAYLOADS.
	termVectorsPayloads = 0x04

	// termVectorsBulkMergeEnabledSysprop is BULK_MERGE_ENABLED_SYSPROP.
	termVectorsBulkMergeEnabledSysprop = "org.apache.lucene.codecs.lucene90.compressing.Lucene90CompressingTermVectorsWriter.enableBulkMerge"
)

// termVectorsFlagsBits is FLAGS_BITS =
// DirectWriter.bitsRequired(POSITIONS | OFFSETS | PAYLOADS).
var termVectorsFlagsBits = packed.DirectWriterBitsRequired(int64(termVectorsPositions | termVectorsOffsets | termVectorsPayloads))

// termVectorsBulkMergeEnabled is BULK_MERGE_ENABLED. Java reads the
// termVectorsBulkMergeEnabledSysprop system property with a default of
// "true"; a Go process has no JVM system properties, so the default applies.
var termVectorsBulkMergeEnabled = true

// termVectorsDocData is the Go rendering of the inner class
// Lucene90CompressingTermVectorsWriter.DocData.
type termVectorsDocData struct {
	numFields                    int
	fields                       []*termVectorsFieldData
	posStart, offStart, payStart int
}

func newTermVectorsDocData(numFields, posStart, offStart, payStart int) *termVectorsDocData {
	return &termVectorsDocData{
		numFields: numFields,
		fields:    make([]*termVectorsFieldData, 0, numFields),
		posStart:  posStart,
		offStart:  offStart,
		payStart:  payStart,
	}
}

func (d *termVectorsDocData) addField(fieldNum, numTerms int, positions, offsets, payloads bool) *termVectorsFieldData {
	var field *termVectorsFieldData
	if len(d.fields) == 0 {
		field = newTermVectorsFieldData(fieldNum, numTerms, positions, offsets, payloads, d.posStart, d.offStart, d.payStart)
	} else {
		last := d.fields[len(d.fields)-1]
		posStart := last.posStart
		if last.hasPositions {
			posStart += last.totalPositions
		}
		offStart := last.offStart
		if last.hasOffsets {
			offStart += last.totalPositions
		}
		payStart := last.payStart
		if last.hasPayloads {
			payStart += last.totalPositions
		}
		field = newTermVectorsFieldData(fieldNum, numTerms, positions, offsets, payloads, posStart, offStart, payStart)
	}
	d.fields = append(d.fields, field)
	return field
}

// termVectorsFieldData is the Go rendering of the inner class
// Lucene90CompressingTermVectorsWriter.FieldData.
type termVectorsFieldData struct {
	hasPositions, hasOffsets, hasPayloads bool
	fieldNum, flags, numTerms             int
	freqs, prefixLengths, suffixLengths   []int32
	posStart, offStart, payStart          int
	totalPositions                        int
	ord                                   int
}

func newTermVectorsFieldData(fieldNum, numTerms int, positions, offsets, payloads bool, posStart, offStart, payStart int) *termVectorsFieldData {
	flags := 0
	if positions {
		flags |= termVectorsPositions
	}
	if offsets {
		flags |= termVectorsOffsets
	}
	if payloads {
		flags |= termVectorsPayloads
	}
	return &termVectorsFieldData{
		fieldNum:       fieldNum,
		numTerms:       numTerms,
		hasPositions:   positions,
		hasOffsets:     offsets,
		hasPayloads:    payloads,
		flags:          flags,
		freqs:          make([]int32, numTerms),
		prefixLengths:  make([]int32, numTerms),
		suffixLengths:  make([]int32, numTerms),
		posStart:       posStart,
		offStart:       offStart,
		payStart:       payStart,
		totalPositions: 0,
		ord:            0,
	}
}

func (fd *termVectorsFieldData) addTerm(freq, prefixLength, suffixLength int) {
	fd.freqs[fd.ord] = int32(freq)
	fd.prefixLengths[fd.ord] = int32(prefixLength)
	fd.suffixLengths[fd.ord] = int32(suffixLength)
	fd.ord++
}

// addPosition writes into the enclosing writer's buffers, as the non-static
// Java inner class does.
func (fd *termVectorsFieldData) addPosition(w *Lucene90CompressingTermVectorsWriter, position, startOffset, length, payloadLength int32) {
	if fd.hasPositions {
		if fd.posStart+fd.totalPositions == len(w.positionsBuf) {
			w.positionsBuf = util.GrowInt32(w.positionsBuf, 1+len(w.positionsBuf))
		}
		w.positionsBuf[fd.posStart+fd.totalPositions] = position
	}
	if fd.hasOffsets {
		if fd.offStart+fd.totalPositions == len(w.startOffsetsBuf) {
			newLength := util.Oversize(fd.offStart+fd.totalPositions, 4)
			w.startOffsetsBuf = util.GrowExactInt32(w.startOffsetsBuf, newLength)
			w.lengthsBuf = util.GrowExactInt32(w.lengthsBuf, newLength)
		}
		w.startOffsetsBuf[fd.offStart+fd.totalPositions] = startOffset
		w.lengthsBuf[fd.offStart+fd.totalPositions] = length
	}
	if fd.hasPayloads {
		if fd.payStart+fd.totalPositions == len(w.payloadLengthsBuf) {
			w.payloadLengthsBuf = util.GrowInt32(w.payloadLengthsBuf, 1+len(w.payloadLengthsBuf))
		}
		w.payloadLengthsBuf[fd.payStart+fd.totalPositions] = payloadLength
	}
	fd.totalPositions++
}

// Lucene90CompressingTermVectorsWriter is the TermVectorsWriter for
// Lucene90CompressingTermVectorsFormat.
//
// This is the Go port of
// org.apache.lucene.codecs.lucene90.compressing.Lucene90CompressingTermVectorsWriter
// of Apache Lucene 10.5.0.
type Lucene90CompressingTermVectorsWriter struct {
	segment                     string
	indexWriter                 *FieldsIndexWriter
	metaStream, vectorsStream   store.IndexOutput
	compressionMode             compressing.CompressionMode
	compressor                  compressing.Compressor
	chunkSize                   int
	numChunks                   int64 // number of chunks
	numDirtyChunks              int64 // number of incomplete compressed blocks written
	numDirtyDocs                int64 // cumulative number of docs in incomplete chunks
	numDocs                     int   // total number of docs seen
	pendingDocs                 []*termVectorsDocData
	curDoc                      *termVectorsDocData   // current document
	curField                    *termVectorsFieldData // current field
	lastTerm                    *util.BytesRef
	positionsBuf                []int32
	startOffsetsBuf, lengthsBuf []int32
	payloadLengthsBuf           []int32
	termSuffixes                *store.ByteBuffersDataOutput // buffered term suffixes
	payloadBytes                *store.ByteBuffersDataOutput // buffered term payloads
	writer                      *packed.BlockPackedWriter
	maxDocsPerChunk             int // hard limit on number of docs per chunk
	scratchBuffer               *store.ByteBuffersDataOutput

	// base carries the concrete members of the abstract
	// org.apache.lucene.codecs.TermVectorsWriter (addAllDocVectors).
	base gcodecs.TermVectorsWriterHelper
}

// newLucene90CompressingTermVectorsWriter is the sole constructor of
// Lucene90CompressingTermVectorsWriter.
func newLucene90CompressingTermVectorsWriter(
	directory store.Directory,
	si *index.SegmentInfo,
	segmentSuffix string,
	context store.IOContext,
	formatName string,
	compressionMode compressing.CompressionMode,
	chunkSize, maxDocsPerChunk, blockShift int,
) (*Lucene90CompressingTermVectorsWriter, error) {
	// assert directory != null;
	w := &Lucene90CompressingTermVectorsWriter{
		segment:         si.Name(),
		compressionMode: compressionMode,
		compressor:      compressionMode.NewCompressor(),
		chunkSize:       chunkSize,
		maxDocsPerChunk: maxDocsPerChunk,
		numDocs:         0,
		pendingDocs:     make([]*termVectorsDocData, 0),
		termSuffixes:    store.NewByteBuffersDataOutput(),
		payloadBytes:    store.NewByteBuffersDataOutput(),
		lastTerm:        &util.BytesRef{Bytes: make([]byte, util.Oversize(30, 1))},
		scratchBuffer:   store.NewByteBuffersDataOutput(),
	}

	success := false
	defer func() {
		if !success {
			// IOUtils.closeWhileHandlingException(metaStream, vectorsStream,
			// indexWriter, indexWriter): the construction failure is the
			// error reported, close failures are suppressed.
			if w.metaStream != nil {
				_ = w.metaStream.Close() // closeWhileHandlingException
			}
			if w.vectorsStream != nil {
				_ = w.vectorsStream.Close() // closeWhileHandlingException
			}
			if w.indexWriter != nil {
				_ = w.indexWriter.Close() // closeWhileHandlingException
			}
		}
	}()

	metaName := store.SegmentFileName(w.segment, segmentSuffix, vectorsMetaExtension)
	rawMeta, err := directory.CreateOutput(metaName, context)
	if err != nil {
		return nil, err
	}
	w.metaStream = store.NewChecksumIndexOutput(rawMeta)
	if err := gcodecs.WriteIndexHeader(
		w.metaStream, vectorsIndexCodecName+"Meta", termVectorsVersionCurrent, si.GetID(), segmentSuffix); err != nil {
		return nil, err
	}
	// assert CodecUtil.indexHeaderLength(VECTORS_INDEX_CODEC_NAME + "Meta", segmentSuffix)
	//     == metaStream.getFilePointer();

	vectorsName := store.SegmentFileName(w.segment, segmentSuffix, vectorsExtension)
	rawVectors, err := directory.CreateOutput(vectorsName, context)
	if err != nil {
		return nil, err
	}
	w.vectorsStream = store.NewChecksumIndexOutput(rawVectors)
	if err := gcodecs.WriteIndexHeader(
		w.vectorsStream, formatName, termVectorsVersionCurrent, si.GetID(), segmentSuffix); err != nil {
		return nil, err
	}
	// assert CodecUtil.indexHeaderLength(formatName, segmentSuffix)
	//     == vectorsStream.getFilePointer();

	w.indexWriter, err = NewFieldsIndexWriter(
		directory,
		w.segment,
		segmentSuffix,
		vectorsIndexExtension,
		vectorsIndexCodecName,
		si.GetID(),
		blockShift,
		context)
	if err != nil {
		return nil, err
	}

	if err := w.metaStream.WriteVInt(int32(packed.VersionCurrent)); err != nil {
		return nil, err
	}
	if err := w.metaStream.WriteVInt(int32(chunkSize)); err != nil {
		return nil, err
	}
	w.writer, err = packed.NewBlockPackedWriter(w.vectorsStream, termVectorsPackedBlockSize)
	if err != nil {
		return nil, err
	}

	w.positionsBuf = make([]int32, 1024)
	w.startOffsetsBuf = make([]int32, 1024)
	w.lengthsBuf = make([]int32, 1024)
	w.payloadLengthsBuf = make([]int32, 1024)

	success = true
	return w, nil
}

func (w *Lucene90CompressingTermVectorsWriter) addDocData(numVectorFields int) *termVectorsDocData {
	var last *termVectorsFieldData
	for i := len(w.pendingDocs) - 1; i >= 0; i-- {
		doc := w.pendingDocs[i]
		if len(doc.fields) != 0 {
			last = doc.fields[len(doc.fields)-1]
			break
		}
	}
	var doc *termVectorsDocData
	if last == nil {
		doc = newTermVectorsDocData(numVectorFields, 0, 0, 0)
	} else {
		posStart := last.posStart
		if last.hasPositions {
			posStart += last.totalPositions
		}
		offStart := last.offStart
		if last.hasOffsets {
			offStart += last.totalPositions
		}
		payStart := last.payStart
		if last.hasPayloads {
			payStart += last.totalPositions
		}
		doc = newTermVectorsDocData(numVectorFields, posStart, offStart, payStart)
	}
	w.pendingDocs = append(w.pendingDocs, doc)
	return doc
}

// Close mirrors Lucene90CompressingTermVectorsWriter.close():
// IOUtils.close(metaStream, vectorsStream, indexWriter), which closes every
// resource and rethrows the first failure with the later ones suppressed.
func (w *Lucene90CompressingTermVectorsWriter) Close() error {
	var errs []error
	if w.metaStream != nil {
		if err := w.metaStream.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if w.vectorsStream != nil {
		if err := w.vectorsStream.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if w.indexWriter != nil {
		if err := w.indexWriter.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	w.metaStream = nil
	w.vectorsStream = nil
	w.indexWriter = nil
	return errors.Join(errs...)
}

// StartDocument mirrors Lucene90CompressingTermVectorsWriter.startDocument(int).
func (w *Lucene90CompressingTermVectorsWriter) StartDocument(numVectorFields int) error {
	w.curDoc = w.addDocData(numVectorFields)
	return nil
}

// FinishDocument mirrors Lucene90CompressingTermVectorsWriter.finishDocument().
func (w *Lucene90CompressingTermVectorsWriter) FinishDocument() error {
	// append the payload bytes of the doc after its terms
	if err := w.payloadBytes.CopyTo(w.termSuffixes); err != nil {
		return err
	}
	w.payloadBytes.Reset()
	w.numDocs++
	if w.triggerFlush() {
		if err := w.flush(false); err != nil {
			return err
		}
	}
	w.curDoc = nil
	return nil
}

// StartField mirrors Lucene90CompressingTermVectorsWriter.startField.
func (w *Lucene90CompressingTermVectorsWriter) StartField(info *index.FieldInfo, numTerms int, positions, offsets, payloads bool) error {
	w.curField = w.curDoc.addField(info.Number(), numTerms, positions, offsets, payloads)
	w.lastTerm.Length = 0
	return nil
}

// FinishField mirrors Lucene90CompressingTermVectorsWriter.finishField().
func (w *Lucene90CompressingTermVectorsWriter) FinishField() error {
	w.curField = nil
	return nil
}

// StartTerm mirrors Lucene90CompressingTermVectorsWriter.startTerm(BytesRef, int).
func (w *Lucene90CompressingTermVectorsWriter) StartTerm(term []byte, freq int) error {
	// assert freq >= 1;
	var prefix int
	if w.lastTerm.Length == 0 {
		// no previous term: no bytes to write
		prefix = 0
	} else {
		var err error
		prefix, err = util.BytesDifference(w.lastTerm, util.NewBytesRef(term))
		if err != nil {
			return err
		}
	}
	w.curField.addTerm(freq, prefix, len(term)-prefix)
	if err := w.termSuffixes.WriteBytes(term, prefix, len(term)-prefix); err != nil {
		return err
	}
	// copy last term
	if len(w.lastTerm.Bytes) < len(term) {
		w.lastTerm.Bytes = make([]byte, util.Oversize(len(term), 1))
	}
	w.lastTerm.Offset = 0
	w.lastTerm.Length = len(term)
	copy(w.lastTerm.Bytes, term)
	return nil
}

// FinishTerm is the default TermVectorsWriter.finishTerm(), which does
// nothing; Lucene90CompressingTermVectorsWriter does not override it.
func (w *Lucene90CompressingTermVectorsWriter) FinishTerm() error {
	return nil
}

// AddPosition mirrors Lucene90CompressingTermVectorsWriter.addPosition.
func (w *Lucene90CompressingTermVectorsWriter) AddPosition(position, startOffset, endOffset int, payload []byte) error {
	// assert curField.flags != 0;
	payloadLength := 0
	if payload != nil {
		payloadLength = len(payload)
	}
	w.curField.addPosition(w, int32(position), int32(startOffset), int32(endOffset-startOffset), int32(payloadLength))
	if w.curField.hasPayloads && payload != nil {
		if err := w.payloadBytes.WriteBytes(payload, 0, len(payload)); err != nil {
			return err
		}
	}
	return nil
}

func (w *Lucene90CompressingTermVectorsWriter) triggerFlush() bool {
	return w.termSuffixes.Size() >= int64(w.chunkSize) || len(w.pendingDocs) >= w.maxDocsPerChunk
}

func (w *Lucene90CompressingTermVectorsWriter) flush(force bool) error {
	// assert force != triggerFlush();
	chunkDocs := len(w.pendingDocs)
	// assert chunkDocs > 0 : chunkDocs;
	w.numChunks++
	if force {
		w.numDirtyChunks++ // incomplete: we had to force this flush
		w.numDirtyDocs += int64(len(w.pendingDocs))
	}
	// write the index file
	if err := w.indexWriter.WriteIndex(chunkDocs, w.vectorsStream.GetFilePointer()); err != nil {
		return err
	}

	docBase := w.numDocs - chunkDocs
	if err := w.vectorsStream.WriteVInt(int32(docBase)); err != nil {
		return err
	}
	dirtyBit := 0
	if force {
		dirtyBit = 1
	}
	if err := w.vectorsStream.WriteVInt(int32((chunkDocs << 1) | dirtyBit)); err != nil {
		return err
	}

	// total number of fields of the chunk
	totalFields, err := w.flushNumFields(chunkDocs)
	if err != nil {
		return err
	}

	if totalFields > 0 {
		// unique field numbers (sorted)
		fieldNums, err := w.flushFieldNums()
		if err != nil {
			return err
		}
		// offsets in the array of unique field numbers
		if err := w.flushFields(totalFields, fieldNums); err != nil {
			return err
		}
		// flags (does the field have positions, offsets, payloads?)
		if err := w.flushFlags(totalFields, fieldNums); err != nil {
			return err
		}
		// number of terms of each field
		if err := w.flushNumTerms(totalFields); err != nil {
			return err
		}
		// prefix and suffix lengths for each field
		if err := w.flushTermLengths(); err != nil {
			return err
		}
		// term freqs - 1 (because termFreq is always >=1) for each term
		if err := w.flushTermFreqs(); err != nil {
			return err
		}
		// positions for all terms, when enabled
		if err := w.flushPositions(); err != nil {
			return err
		}
		// offsets for all terms, when enabled
		if err := w.flushOffsets(fieldNums); err != nil {
			return err
		}
		// payload lengths for all terms, when enabled
		if err := w.flushPayloadLengths(); err != nil {
			return err
		}

		// compress terms and payloads and write them to the output
		content := store.NewByteBuffersDataInput(w.termSuffixes.ToArrayCopy())
		if err := w.compressor.Compress(content, w.vectorsStream); err != nil {
			return err
		}
	}

	// reset
	w.pendingDocs = w.pendingDocs[:0]
	w.curDoc = nil
	w.curField = nil
	w.termSuffixes.Reset()
	return nil
}

func (w *Lucene90CompressingTermVectorsWriter) flushNumFields(chunkDocs int) (int, error) {
	if chunkDocs == 1 {
		numFields := w.pendingDocs[0].numFields
		if err := w.vectorsStream.WriteVInt(int32(numFields)); err != nil {
			return 0, err
		}
		return numFields, nil
	}
	w.writer.Reset(w.vectorsStream)
	totalFields := 0
	for _, dd := range w.pendingDocs {
		if err := w.writer.Add(int64(dd.numFields)); err != nil {
			return 0, err
		}
		totalFields += dd.numFields
	}
	if err := w.writer.Finish(); err != nil {
		return 0, err
	}
	return totalFields, nil
}

func (w *Lucene90CompressingTermVectorsWriter) flushFieldNums() ([]int, error) {
	fieldNumsSet := make(map[int]struct{})
	for _, dd := range w.pendingDocs {
		for _, fd := range dd.fields {
			fieldNumsSet[fd.fieldNum] = struct{}{}
		}
	}
	fieldNums := make([]int, 0, len(fieldNumsSet))
	for fieldNum := range fieldNumsSet {
		fieldNums = append(fieldNums, fieldNum)
	}
	sort.Ints(fieldNums)

	numDistinctFields := len(fieldNums)
	// assert numDistinctFields > 0;
	bitsRequired := packed.BitsRequired(int64(fieldNums[numDistinctFields-1]))
	token := (min(numDistinctFields-1, 0x07) << 5) | bitsRequired
	if err := w.vectorsStream.WriteByte(byte(token)); err != nil {
		return nil, err
	}
	if numDistinctFields-1 >= 0x07 {
		if err := w.vectorsStream.WriteVInt(int32(numDistinctFields - 1 - 0x07)); err != nil {
			return nil, err
		}
	}
	writer, err := packed.GetWriterNoHeader(w.vectorsStream, packed.FormatPacked, numDistinctFields, bitsRequired, 1)
	if err != nil {
		return nil, err
	}
	for _, fieldNum := range fieldNums {
		if err := writer.Add(int64(fieldNum)); err != nil {
			return nil, err
		}
	}
	if err := writer.Finish(); err != nil {
		return nil, err
	}
	return fieldNums, nil
}

func (w *Lucene90CompressingTermVectorsWriter) flushFields(totalFields int, fieldNums []int) error {
	w.scratchBuffer.Reset()
	writer, err := packed.GetDirectWriter(
		w.scratchBuffer, int64(totalFields), packed.DirectWriterBitsRequired(int64(len(fieldNums)-1)))
	if err != nil {
		return err
	}
	for _, dd := range w.pendingDocs {
		for _, fd := range dd.fields {
			fieldNumIndex := sort.SearchInts(fieldNums, fd.fieldNum)
			// assert fieldNumIndex >= 0;
			if err := writer.Add(int64(fieldNumIndex)); err != nil {
				return err
			}
		}
	}
	if err := writer.Finish(); err != nil {
		return err
	}
	if err := w.vectorsStream.WriteVLong(w.scratchBuffer.Size()); err != nil {
		return err
	}
	return w.scratchBuffer.CopyTo(w.vectorsStream)
}

func (w *Lucene90CompressingTermVectorsWriter) flushFlags(totalFields int, fieldNums []int) error {
	// check if fields always have the same flags
	nonChangingFlags := true
	fieldFlags := make([]int, len(fieldNums))
	for i := range fieldFlags {
		fieldFlags[i] = -1
	}
outer:
	for _, dd := range w.pendingDocs {
		for _, fd := range dd.fields {
			fieldNumOff := sort.SearchInts(fieldNums, fd.fieldNum)
			// assert fieldNumOff >= 0;
			if fieldFlags[fieldNumOff] == -1 {
				fieldFlags[fieldNumOff] = fd.flags
			} else if fieldFlags[fieldNumOff] != fd.flags {
				nonChangingFlags = false
				break outer
			}
		}
	}

	if nonChangingFlags {
		// write one flag per field num
		if err := w.vectorsStream.WriteVInt(0); err != nil {
			return err
		}
		w.scratchBuffer.Reset()
		writer, err := packed.GetDirectWriter(w.scratchBuffer, int64(len(fieldFlags)), termVectorsFlagsBits)
		if err != nil {
			return err
		}
		for _, flags := range fieldFlags {
			// assert flags >= 0;
			if err := writer.Add(int64(flags)); err != nil {
				return err
			}
		}
		if err := writer.Finish(); err != nil {
			return err
		}
		size, err := toIntExact(w.scratchBuffer.Size())
		if err != nil {
			return err
		}
		if err := w.vectorsStream.WriteVInt(size); err != nil {
			return err
		}
		return w.scratchBuffer.CopyTo(w.vectorsStream)
	}

	// write one flag for every field instance
	if err := w.vectorsStream.WriteVInt(1); err != nil {
		return err
	}
	w.scratchBuffer.Reset()
	writer, err := packed.GetDirectWriter(w.scratchBuffer, int64(totalFields), termVectorsFlagsBits)
	if err != nil {
		return err
	}
	for _, dd := range w.pendingDocs {
		for _, fd := range dd.fields {
			if err := writer.Add(int64(fd.flags)); err != nil {
				return err
			}
		}
	}
	if err := writer.Finish(); err != nil {
		return err
	}
	size, err := toIntExact(w.scratchBuffer.Size())
	if err != nil {
		return err
	}
	if err := w.vectorsStream.WriteVInt(size); err != nil {
		return err
	}
	return w.scratchBuffer.CopyTo(w.vectorsStream)
}

func (w *Lucene90CompressingTermVectorsWriter) flushNumTerms(totalFields int) error {
	maxNumTerms := 0
	for _, dd := range w.pendingDocs {
		for _, fd := range dd.fields {
			maxNumTerms |= fd.numTerms
		}
	}
	bitsRequired := packed.DirectWriterBitsRequired(int64(maxNumTerms))
	if err := w.vectorsStream.WriteVInt(int32(bitsRequired)); err != nil {
		return err
	}
	w.scratchBuffer.Reset()
	writer, err := packed.GetDirectWriter(w.scratchBuffer, int64(totalFields), bitsRequired)
	if err != nil {
		return err
	}
	for _, dd := range w.pendingDocs {
		for _, fd := range dd.fields {
			if err := writer.Add(int64(fd.numTerms)); err != nil {
				return err
			}
		}
	}
	if err := writer.Finish(); err != nil {
		return err
	}
	size, err := toIntExact(w.scratchBuffer.Size())
	if err != nil {
		return err
	}
	if err := w.vectorsStream.WriteVInt(size); err != nil {
		return err
	}
	return w.scratchBuffer.CopyTo(w.vectorsStream)
}

func (w *Lucene90CompressingTermVectorsWriter) flushTermLengths() error {
	w.writer.Reset(w.vectorsStream)
	for _, dd := range w.pendingDocs {
		for _, fd := range dd.fields {
			for i := 0; i < fd.numTerms; i++ {
				if err := w.writer.Add(int64(fd.prefixLengths[i])); err != nil {
					return err
				}
			}
		}
	}
	if err := w.writer.Finish(); err != nil {
		return err
	}
	w.writer.Reset(w.vectorsStream)
	for _, dd := range w.pendingDocs {
		for _, fd := range dd.fields {
			for i := 0; i < fd.numTerms; i++ {
				if err := w.writer.Add(int64(fd.suffixLengths[i])); err != nil {
					return err
				}
			}
		}
	}
	return w.writer.Finish()
}

func (w *Lucene90CompressingTermVectorsWriter) flushTermFreqs() error {
	w.writer.Reset(w.vectorsStream)
	for _, dd := range w.pendingDocs {
		for _, fd := range dd.fields {
			for i := 0; i < fd.numTerms; i++ {
				if err := w.writer.Add(int64(fd.freqs[i] - 1)); err != nil {
					return err
				}
			}
		}
	}
	return w.writer.Finish()
}

func (w *Lucene90CompressingTermVectorsWriter) flushPositions() error {
	w.writer.Reset(w.vectorsStream)
	for _, dd := range w.pendingDocs {
		for _, fd := range dd.fields {
			if fd.hasPositions {
				pos := 0
				for i := 0; i < fd.numTerms; i++ {
					previousPosition := int32(0)
					for j := 0; j < int(fd.freqs[i]); j++ {
						position := w.positionsBuf[fd.posStart+pos]
						pos++
						if err := w.writer.Add(int64(position - previousPosition)); err != nil {
							return err
						}
						previousPosition = position
					}
				}
				// assert pos == fd.totalPositions;
			}
		}
	}
	return w.writer.Finish()
}

func (w *Lucene90CompressingTermVectorsWriter) flushOffsets(fieldNums []int) error {
	hasOffsets := false
	sumPos := make([]int64, len(fieldNums))
	sumOffsets := make([]int64, len(fieldNums))
	for _, dd := range w.pendingDocs {
		for _, fd := range dd.fields {
			hasOffsets = hasOffsets || fd.hasOffsets
			if fd.hasOffsets && fd.hasPositions {
				fieldNumOff := sort.SearchInts(fieldNums, fd.fieldNum)
				pos := 0
				for i := 0; i < fd.numTerms; i++ {
					sumPos[fieldNumOff] += int64(w.positionsBuf[fd.posStart+int(fd.freqs[i])-1+pos])
					sumOffsets[fieldNumOff] += int64(w.startOffsetsBuf[fd.offStart+int(fd.freqs[i])-1+pos])
					pos += int(fd.freqs[i])
				}
				// assert pos == fd.totalPositions;
			}
		}
	}

	if !hasOffsets {
		// nothing to do
		return nil
	}

	charsPerTerm := make([]float32, len(fieldNums))
	for i := range fieldNums {
		if sumPos[i] <= 0 || sumOffsets[i] <= 0 {
			charsPerTerm[i] = 0
		} else {
			charsPerTerm[i] = float32(float64(sumOffsets[i]) / float64(sumPos[i]))
		}
	}

	// start offsets
	for i := range fieldNums {
		if err := w.vectorsStream.WriteInt(int32(math.Float32bits(charsPerTerm[i]))); err != nil {
			return err
		}
	}

	w.writer.Reset(w.vectorsStream)
	for _, dd := range w.pendingDocs {
		for _, fd := range dd.fields {
			if (fd.flags & termVectorsOffsets) != 0 {
				fieldNumOff := sort.SearchInts(fieldNums, fd.fieldNum)
				cpt := charsPerTerm[fieldNumOff]
				pos := 0
				for i := 0; i < fd.numTerms; i++ {
					previousPos := int32(0)
					previousOff := int32(0)
					for j := 0; j < int(fd.freqs[i]); j++ {
						position := int32(0)
						if fd.hasPositions {
							position = w.positionsBuf[fd.posStart+pos]
						}
						startOffset := w.startOffsetsBuf[fd.offStart+pos]
						if err := w.writer.Add(int64(startOffset - previousOff - floatToInt(cpt*float32(position-previousPos)))); err != nil {
							return err
						}
						previousPos = position
						previousOff = startOffset
						pos++
					}
				}
			}
		}
	}
	if err := w.writer.Finish(); err != nil {
		return err
	}

	// lengths
	w.writer.Reset(w.vectorsStream)
	for _, dd := range w.pendingDocs {
		for _, fd := range dd.fields {
			if (fd.flags & termVectorsOffsets) != 0 {
				pos := 0
				for i := 0; i < fd.numTerms; i++ {
					for j := 0; j < int(fd.freqs[i]); j++ {
						length := w.lengthsBuf[fd.offStart+pos] - fd.prefixLengths[i] - fd.suffixLengths[i]
						pos++
						if err := w.writer.Add(int64(length)); err != nil {
							return err
						}
					}
				}
				// assert pos == fd.totalPositions;
			}
		}
	}
	return w.writer.Finish()
}

func (w *Lucene90CompressingTermVectorsWriter) flushPayloadLengths() error {
	w.writer.Reset(w.vectorsStream)
	for _, dd := range w.pendingDocs {
		for _, fd := range dd.fields {
			if fd.hasPayloads {
				for i := 0; i < fd.totalPositions; i++ {
					if err := w.writer.Add(int64(w.payloadLengthsBuf[fd.payStart+i])); err != nil {
						return err
					}
				}
			}
		}
	}
	return w.writer.Finish()
}

// Finish mirrors Lucene90CompressingTermVectorsWriter.finish(int).
func (w *Lucene90CompressingTermVectorsWriter) Finish(numDocs int) error {
	if len(w.pendingDocs) > 0 {
		if err := w.flush(true); err != nil {
			return err
		}
	}
	if numDocs != w.numDocs {
		return fmt.Errorf("Wrote %d docs, finish called with numDocs=%d", w.numDocs, numDocs)
	}
	if err := w.indexWriter.Finish(numDocs, w.vectorsStream.GetFilePointer(), w.metaStream); err != nil {
		return err
	}
	if err := w.metaStream.WriteVLong(w.numChunks); err != nil {
		return err
	}
	if err := w.metaStream.WriteVLong(w.numDirtyChunks); err != nil {
		return err
	}
	if err := w.metaStream.WriteVLong(w.numDirtyDocs); err != nil {
		return err
	}
	if err := store.WriteFooter(w.metaStream); err != nil {
		return err
	}
	return store.WriteFooter(w.vectorsStream)
}

// AddProx mirrors Lucene90CompressingTermVectorsWriter.addProx(int, DataInput,
// DataInput), which overrides TermVectorsWriter.addProx.
func (w *Lucene90CompressingTermVectorsWriter) AddProx(numProx int, positions, offsets store.DataInput) error {
	// assert (curField.hasPositions) == (positions != null);
	// assert (curField.hasOffsets) == (offsets != null);

	if w.curField.hasPositions {
		posStart := w.curField.posStart + w.curField.totalPositions
		if posStart+numProx > len(w.positionsBuf) {
			w.positionsBuf = util.GrowInt32(w.positionsBuf, posStart+numProx)
		}
		position := int32(0)
		if w.curField.hasPayloads {
			payStart := w.curField.payStart + w.curField.totalPositions
			if payStart+numProx > len(w.payloadLengthsBuf) {
				w.payloadLengthsBuf = util.GrowInt32(w.payloadLengthsBuf, payStart+numProx)
			}
			for i := 0; i < numProx; i++ {
				code, err := positions.ReadVInt()
				if err != nil {
					return err
				}
				if (code & 1) != 0 {
					// This position has a payload
					payloadLength, err := positions.ReadVInt()
					if err != nil {
						return err
					}
					w.payloadLengthsBuf[payStart+i] = payloadLength
					if err := w.payloadBytes.CopyBytes(positions, int64(payloadLength)); err != nil {
						return err
					}
				} else {
					w.payloadLengthsBuf[payStart+i] = 0
				}
				position += int32(uint32(code) >> 1)
				w.positionsBuf[posStart+i] = position
			}
		} else {
			for i := 0; i < numProx; i++ {
				code, err := positions.ReadVInt()
				if err != nil {
					return err
				}
				position += int32(uint32(code) >> 1)
				w.positionsBuf[posStart+i] = position
			}
		}
	}

	if w.curField.hasOffsets {
		offStart := w.curField.offStart + w.curField.totalPositions
		if offStart+numProx > len(w.startOffsetsBuf) {
			newLength := util.Oversize(offStart+numProx, 4)
			w.startOffsetsBuf = util.GrowExactInt32(w.startOffsetsBuf, newLength)
			w.lengthsBuf = util.GrowExactInt32(w.lengthsBuf, newLength)
		}
		lastOffset := int32(0)
		for i := 0; i < numProx; i++ {
			delta, err := offsets.ReadVInt()
			if err != nil {
				return err
			}
			startOffset := lastOffset + delta
			length, err := offsets.ReadVInt()
			if err != nil {
				return err
			}
			endOffset := startOffset + length
			lastOffset = endOffset
			w.startOffsetsBuf[offStart+i] = startOffset
			w.lengthsBuf[offStart+i] = endOffset - startOffset
		}
	}

	w.curField.totalPositions += numProx
	return nil
}

func (w *Lucene90CompressingTermVectorsWriter) copyChunks(
	mergeState *index.MergeState,
	sub *compressingTermVectorsSub,
	fromDocID, toDocID int,
) error {
	reader := mergeState.TermVectorsReaders[sub.readerIndex].(*Lucene90CompressingTermVectorsReader)
	// assert reader.getVersion() == VERSION_CURRENT;
	// assert reader.getChunkSize() == chunkSize;
	// assert reader.getCompressionMode() == compressionMode;
	// assert !tooDirty(reader);
	// assert mergeState.liveDocs[sub.readerIndex] == null;

	docID := fromDocID
	idx := reader.getIndexReader()

	// copy docs that belong to the previous chunk
	for docID < toDocID && reader.isLoaded(docID) {
		vectors, err := reader.Get(docID)
		if err != nil {
			return err
		}
		docID++
		if err := w.base.AddAllDocVectors(w, vectors, mergeState); err != nil {
			return err
		}
	}
	if docID >= toDocID {
		return nil
	}
	// copy chunks
	fromPointer, err := fieldsIndexStartPointer(idx, docID)
	if err != nil {
		return err
	}
	var toPointer int64
	if toDocID == sub.maxDoc {
		toPointer = reader.getMaxPointer()
	} else {
		toPointer, err = fieldsIndexStartPointer(idx, toDocID)
		if err != nil {
			return err
		}
	}
	if fromPointer < toPointer {
		// flush any pending chunks
		if len(w.pendingDocs) > 0 {
			if err := w.flush(true); err != nil {
				return err
			}
		}
		rawDocs := reader.getVectorsStream()
		if err := rawDocs.SetPosition(fromPointer); err != nil {
			return err
		}
		for {
			// iterate over each chunk. we use the vectors index to find chunk boundaries,
			// read the docstart + doccount from the chunk header (we write a new header, since doc
			// numbers will change),
			// and just copy the bytes directly.
			// read header
			base, err := rawDocs.ReadVInt()
			if err != nil {
				return err
			}
			if int(base) != docID {
				return index.NewCorruptIndexException(
					fmt.Sprintf("invalid state: base=%d, docID=%d", base, docID), fmt.Sprint(rawDocs))
			}

			code, err := rawDocs.ReadVInt()
			if err != nil {
				return err
			}
			bufferedDocs := int(uint32(code) >> 1)

			// write a new index entry and new header for this chunk.
			if err := w.indexWriter.WriteIndex(bufferedDocs, w.vectorsStream.GetFilePointer()); err != nil {
				return err
			}
			if err := w.vectorsStream.WriteVInt(int32(w.numDocs)); err != nil { // rebase
				return err
			}
			if err := w.vectorsStream.WriteVInt(code); err != nil {
				return err
			}
			docID += bufferedDocs
			w.numDocs += bufferedDocs
			if docID > toDocID {
				return index.NewCorruptIndexException(
					fmt.Sprintf("invalid state: base=%d, count=%d, toDocID=%d", base, bufferedDocs, toDocID),
					fmt.Sprint(rawDocs))
			}

			// copy bytes until the next chunk boundary (or end of chunk data).
			// using the stored fields index for this isn't the most efficient, but fast enough
			// and is a source of redundancy for detecting bad things.
			var end int64
			if docID == sub.maxDoc {
				end = reader.getMaxPointer()
			} else {
				end, err = fieldsIndexStartPointer(idx, docID)
				if err != nil {
					return err
				}
			}
			if err := w.vectorsStream.CopyBytes(rawDocs, end-rawDocs.GetFilePointer()); err != nil {
				return err
			}
			w.numChunks++
			dirtyChunk := (code & 1) != 0
			if dirtyChunk {
				w.numDirtyChunks++
				w.numDirtyDocs += int64(bufferedDocs)
			}
			fromPointer = end
			if !(fromPointer < toPointer) {
				break
			}
		}
	}

	// copy leftover docs that don't form a complete chunk
	// assert reader.isLoaded(docID) == false;
	for docID < toDocID {
		vectors, err := reader.Get(docID)
		if err != nil {
			return err
		}
		docID++
		if err := w.base.AddAllDocVectors(w, vectors, mergeState); err != nil {
			return err
		}
	}
	return nil
}

// Merge mirrors Lucene90CompressingTermVectorsWriter.merge(MergeState), which
// overrides TermVectorsWriter.merge.
func (w *Lucene90CompressingTermVectorsWriter) Merge(mergeState *index.MergeState) (int, error) {
	numReaders := len(mergeState.TermVectorsReaders)
	matchingReaders := compressing.NewMatchingReaders(&compressing.MergeState{
		MaxDocs:         mergeState.MaxDocs,
		FieldInfos:      mergeState.FieldInfos,
		MergeFieldInfos: mergeState.MergeFieldInfos,
	})
	subs := make([]index.DocIDMergerSub, 0, numReaders)
	for i := 0; i < numReaders; i++ {
		reader := mergeState.TermVectorsReaders[i]
		if reader != nil {
			if err := mergeState.CheckAborted(); err != nil {
				return 0, err
			}
			if err := reader.CheckIntegrity(); err != nil {
				return 0, err
			}
		}
		bulkMerge, err := w.canPerformBulkMerge(mergeState, matchingReaders, i)
		if err != nil {
			return 0, err
		}
		subs = append(subs, newCompressingTermVectorsSub(mergeState, bulkMerge, i))
	}
	docCount := 0
	docIDMerger, err := index.NewDocIDMerger(subs, 0, mergeState.NeedsIndexSort)
	if err != nil {
		return 0, err
	}
	next := func() (*compressingTermVectorsSub, error) {
		s, err := docIDMerger.Next()
		if err != nil || s == nil {
			return nil, err
		}
		return s.(*compressingTermVectorsSub), nil
	}
	sub, err := next()
	if err != nil {
		return 0, err
	}
	for sub != nil {
		// assert sub.mappedDocID == docCount : sub.mappedDocID + " != " + docCount;
		if sub.canPerformBulkMerge {
			fromDocID := sub.docID
			toDocID := fromDocID
			current := sub
			for {
				sub, err = next()
				if err != nil {
					return 0, err
				}
				if sub != current {
					break
				}
				toDocID++
				// assert sub.docID == toDocID;
			}
			toDocID++ // exclusive bound
			if err := w.copyChunks(mergeState, current, fromDocID, toDocID); err != nil {
				return 0, err
			}
			docCount += toDocID - fromDocID
		} else {
			reader := mergeState.TermVectorsReaders[sub.readerIndex]
			var vectors index.Fields
			if reader != nil {
				vectors, err = reader.Get(sub.docID)
				if err != nil {
					return 0, err
				}
			}
			if err := w.base.AddAllDocVectors(w, vectors, mergeState); err != nil {
				return 0, err
			}
			docCount++
			sub, err = next()
			if err != nil {
				return 0, err
			}
		}
	}
	if err := w.Finish(docCount); err != nil {
		return 0, err
	}
	return docCount, nil
}

// tooDirty reports whether a segment is considered dirty: only if it has
// enough dirty docs to make a full block AND more than 1% blocks are dirty.
// Mirrors Lucene90CompressingTermVectorsWriter.tooDirty.
func (w *Lucene90CompressingTermVectorsWriter) tooDirty(candidate *Lucene90CompressingTermVectorsReader) (bool, error) {
	numDirtyDocs, err := candidate.getNumDirtyDocs()
	if err != nil {
		return false, err
	}
	if numDirtyDocs <= int64(w.maxDocsPerChunk) {
		return false, nil
	}
	numDirtyChunks, err := candidate.getNumDirtyChunks()
	if err != nil {
		return false, err
	}
	numChunks, err := candidate.getNumChunks()
	if err != nil {
		return false, err
	}
	return numDirtyChunks*100 > numChunks, nil
}

func (w *Lucene90CompressingTermVectorsWriter) canPerformBulkMerge(
	mergeState *index.MergeState, matchingReaders *compressing.MatchingReaders, readerIndex int,
) (bool, error) {
	reader, ok := mergeState.TermVectorsReaders[readerIndex].(*Lucene90CompressingTermVectorsReader)
	if !ok {
		return false, nil
	}
	if !(termVectorsBulkMergeEnabled &&
		matchingReaders.MatchingReaders[readerIndex] &&
		reader.getCompressionMode() == w.compressionMode &&
		reader.getChunkSize() == w.chunkSize &&
		reader.getVersion() == termVectorsVersionCurrent &&
		reader.getPackedIntsVersion() == packed.VersionCurrent &&
		mergeState.LiveDocs[readerIndex] == nil) {
		return false, nil
	}
	dirty, err := w.tooDirty(reader)
	if err != nil {
		return false, err
	}
	return !dirty, nil
}

// compressingTermVectorsSub is the Go rendering of the private static nested
// class Lucene90CompressingTermVectorsWriter.CompressingTermVectorsSub, a
// DocIDMerger.Sub.
type compressingTermVectorsSub struct {
	docMap              index.DocMap
	maxDoc              int
	readerIndex         int
	canPerformBulkMerge bool
	docID               int
}

func newCompressingTermVectorsSub(mergeState *index.MergeState, canPerformBulkMerge bool, readerIndex int) *compressingTermVectorsSub {
	return &compressingTermVectorsSub{
		docMap:              mergeState.DocMaps[readerIndex],
		maxDoc:              mergeState.MaxDocs[readerIndex],
		readerIndex:         readerIndex,
		canPerformBulkMerge: canPerformBulkMerge,
		docID:               -1,
	}
}

func (s *compressingTermVectorsSub) MappedDocID() int {
	return s.docMap.Get(s.docID)
}

func (s *compressingTermVectorsSub) NextDoc() (int, error) {
	s.docID++
	if s.docID == s.maxDoc {
		return index.NO_MORE_DOCS, nil
	}
	return s.docID, nil
}

func (s *compressingTermVectorsSub) NextMappedDoc() (int, error) {
	for {
		doc, err := s.NextDoc()
		if err != nil {
			return 0, err
		}
		if doc == index.NO_MORE_DOCS {
			return index.NO_MORE_DOCS, nil
		}
		if mapped := s.docMap.Get(doc); mapped != -1 {
			return mapped, nil
		}
	}
}

// RamBytesUsed mirrors Lucene90CompressingTermVectorsWriter.ramBytesUsed().
func (w *Lucene90CompressingTermVectorsWriter) RamBytesUsed() int64 {
	return int64(len(w.positionsBuf)) +
		int64(len(w.startOffsetsBuf)) +
		int64(len(w.lengthsBuf)) +
		int64(len(w.payloadLengthsBuf)) +
		w.termSuffixes.RamBytesUsed() +
		w.payloadBytes.RamBytesUsed() +
		int64(len(w.lastTerm.Bytes)) +
		w.scratchBuffer.RamBytesUsed()
}

// GetChildResources mirrors
// Lucene90CompressingTermVectorsWriter.getChildResources().
func (w *Lucene90CompressingTermVectorsWriter) GetChildResources() []util.Accountable {
	return []util.Accountable{w.termSuffixes, w.payloadBytes}
}

// toIntExact renders Math.toIntExact(long), which throws ArithmeticException
// when the value does not fit an int.
func toIntExact(value int64) (int32, error) {
	if value < math.MinInt32 || value > math.MaxInt32 {
		return 0, errors.New("integer overflow")
	}
	return int32(value), nil
}

// floatToInt renders Java's narrowing primitive conversion (int) of a float
// (JLS 5.1.3): NaN converts to 0, values beyond the int range saturate, and
// every other value is rounded toward zero.
func floatToInt(f float32) int32 {
	switch {
	case f != f:
		return 0
	case f >= math.MaxInt32:
		return math.MaxInt32
	case f <= math.MinInt32:
		return math.MinInt32
	default:
		return int32(f)
	}
}

var _ gcodecs.TermVectorsWriter = (*Lucene90CompressingTermVectorsWriter)(nil)
