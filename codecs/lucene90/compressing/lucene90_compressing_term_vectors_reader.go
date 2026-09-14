// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package compressing

import (
	"errors"
	"fmt"
	"math"

	gcodecs "github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/codecs/compressing"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/automaton"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

const (
	// termVectorsPrefetchCacheSize is PREFETCH_CACHE_SIZE.
	termVectorsPrefetchCacheSize = 1 << 4
	// termVectorsPrefetchCacheMask is PREFETCH_CACHE_MASK.
	termVectorsPrefetchCacheMask = termVectorsPrefetchCacheSize - 1
)

// errTVTermsEnumUnsupported renders the UnsupportedOperationException that
// TVTermsEnum.seekExact(long) and TVTermsEnum.ord() throw.
var errTVTermsEnumUnsupported = errors.New("UnsupportedOperationException")

// termVectorsBlockState is the Go rendering of the private record
// Lucene90CompressingTermVectorsReader.BlockState.
type termVectorsBlockState struct {
	startPointer int64
	docBase      int
	chunkDocs    int
}

// Lucene90CompressingTermVectorsReader is the TermVectorsReader for
// Lucene90CompressingTermVectorsFormat.
//
// This is the Go port of
// org.apache.lucene.codecs.lucene90.compressing.Lucene90CompressingTermVectorsReader
// of Apache Lucene 10.5.0.
type Lucene90CompressingTermVectorsReader struct {
	fieldInfos        *index.FieldInfos
	indexReader       fieldsIndex
	vectorsStream     store.IndexInput
	version           int32
	packedIntsVersion int
	compressionMode   compressing.CompressionMode
	decompressor      compressing.Decompressor
	chunkSize         int
	numDocs           int
	closed            bool
	reader            *packed.BlockPackedReaderIterator
	numChunks         int64 // number of written blocks
	numDirtyChunks    int64 // number of incomplete compressed blocks written
	numDirtyDocs      int64 // cumulative number of docs in incomplete chunks
	maxPointer        int64 // end of the data section
	blockState        termVectorsBlockState

	// Cache of recently prefetched block IDs. This helps reduce chances of
	// prefetching the same block multiple times, which is otherwise likely due
	// to index sorting or recursive graph bisection clustering similar
	// documents together. NOTE: this cache must be small since it's fully
	// scanned.
	prefetchedBlockIDCache      []int64
	prefetchedBlockIDCacheIndex int
}

func newTermVectorsPrefetchedBlockIDCache() []int64 {
	cache := make([]int64, termVectorsPrefetchCacheSize)
	for i := range cache {
		cache[i] = -1
	}
	return cache
}

// newLucene90CompressingTermVectorsReaderFrom is the private constructor
// Lucene90CompressingTermVectorsReader(Lucene90CompressingTermVectorsReader)
// used by clone.
func newLucene90CompressingTermVectorsReaderFrom(reader *Lucene90CompressingTermVectorsReader) (*Lucene90CompressingTermVectorsReader, error) {
	indexReader, err := reader.indexReader.clone()
	if err != nil {
		return nil, err
	}
	r := &Lucene90CompressingTermVectorsReader{
		fieldInfos:             reader.fieldInfos,
		vectorsStream:          reader.vectorsStream.Clone(),
		indexReader:            indexReader,
		packedIntsVersion:      reader.packedIntsVersion,
		compressionMode:        reader.compressionMode,
		decompressor:           reader.decompressor.Clone(),
		chunkSize:              reader.chunkSize,
		numDocs:                reader.numDocs,
		version:                reader.version,
		numChunks:              reader.numChunks,
		numDirtyChunks:         reader.numDirtyChunks,
		numDirtyDocs:           reader.numDirtyDocs,
		maxPointer:             reader.maxPointer,
		blockState:             termVectorsBlockState{startPointer: -1, docBase: -1, chunkDocs: 0},
		prefetchedBlockIDCache: newTermVectorsPrefetchedBlockIDCache(),
		closed:                 false,
	}
	r.reader, err = packed.NewBlockPackedReaderIterator(r.vectorsStream, r.packedIntsVersion, termVectorsPackedBlockSize, 0)
	if err != nil {
		return nil, err
	}
	return r, nil
}

// newLucene90CompressingTermVectorsReader is the public constructor
// Lucene90CompressingTermVectorsReader(Directory, SegmentInfo, String,
// FieldInfos, IOContext, String, CompressionMode).
func newLucene90CompressingTermVectorsReader(
	d store.Directory,
	si *index.SegmentInfo,
	segmentSuffix string,
	fn *index.FieldInfos,
	context store.IOContext,
	formatName string,
	compressionMode compressing.CompressionMode,
) (*Lucene90CompressingTermVectorsReader, error) {
	r := &Lucene90CompressingTermVectorsReader{
		compressionMode: compressionMode,
		fieldInfos:      fn,
		numDocs:         si.DocCount(),
		blockState:      termVectorsBlockState{startPointer: -1, docBase: -1, chunkDocs: 0},
	}
	segment := si.Name()
	success := false
	var metaIn *store.ChecksumIndexInput
	defer func() {
		if !success {
			// IOUtils.closeWhileHandlingException(this, metaIn): the
			// construction failure is the error reported, close failures are
			// suppressed.
			_ = r.Close() // closeWhileHandlingException
			if metaIn != nil {
				_ = metaIn.Close() // closeWhileHandlingException
			}
		}
	}()

	err := func() error {
		// Open the data file
		vectorsStreamFN := store.SegmentFileName(segment, segmentSuffix, vectorsExtension)
		vectorsStream, err := d.OpenInput(vectorsStreamFN, context.WithHints(spi.FileTypeData, spi.DataAccessRandom))
		if err != nil {
			return err
		}
		r.vectorsStream = vectorsStream
		r.version, err = gcodecs.CheckIndexHeader(
			r.vectorsStream, formatName, termVectorsVersionStart, termVectorsVersionCurrent, si.GetID(), segmentSuffix)
		if err != nil {
			return err
		}
		// assert CodecUtil.indexHeaderLength(formatName, segmentSuffix)
		//     == vectorsStream.getFilePointer();

		metaStreamFN := store.SegmentFileName(segment, segmentSuffix, vectorsMetaExtension)
		rawMeta, err := d.OpenInput(metaStreamFN, store.IOContextReadOnce)
		if err != nil {
			return err
		}
		metaIn = store.NewChecksumIndexInput(rawMeta)
		if _, err := gcodecs.CheckIndexHeader(
			metaIn,
			vectorsIndexCodecName+"Meta",
			termVectorsMetaVersionStart,
			r.version,
			si.GetID(),
			segmentSuffix); err != nil {
			return err
		}

		packedIntsVersion, err := metaIn.ReadVInt()
		if err != nil {
			return err
		}
		r.packedIntsVersion = int(packedIntsVersion)
		chunkSize, err := metaIn.ReadVInt()
		if err != nil {
			return err
		}
		r.chunkSize = int(chunkSize)

		// NOTE: data file is too costly to verify checksum against all the bytes on open,
		// but for now we at least verify proper structure of the checksum footer: which looks
		// for FOOTER_MAGIC + algorithmID. This is cheap and can detect some forms of corruption
		// such as file truncation.
		if _, err := gcodecs.RetrieveChecksum(r.vectorsStream); err != nil {
			return err
		}

		fieldsIndexReader, err := newFieldsIndexReader(
			d,
			si.Name(),
			segmentSuffix,
			vectorsIndexExtension,
			vectorsIndexCodecName,
			si.GetID(),
			metaIn,
			context)
		if err != nil {
			return err
		}
		r.indexReader = fieldsIndexReader
		r.maxPointer = fieldsIndexReader.getMaxPointer()

		if r.numChunks, err = metaIn.ReadVLong(); err != nil {
			return err
		}
		if r.numDirtyChunks, err = metaIn.ReadVLong(); err != nil {
			return err
		}
		if r.numDirtyDocs, err = metaIn.ReadVLong(); err != nil {
			return err
		}
		if r.numChunks < r.numDirtyChunks {
			return index.NewCorruptIndexException(
				fmt.Sprintf("Cannot have more dirty chunks than chunks: numChunks=%d, numDirtyChunks=%d",
					r.numChunks, r.numDirtyChunks),
				fmt.Sprint(metaIn))
		}
		if (r.numDirtyChunks == 0) != (r.numDirtyDocs == 0) {
			return index.NewCorruptIndexException(
				fmt.Sprintf("Cannot have dirty chunks without dirty docs or vice-versa: numDirtyChunks=%d, numDirtyDocs=%d",
					r.numDirtyChunks, r.numDirtyDocs),
				fmt.Sprint(metaIn))
		}
		if r.numDirtyDocs < r.numDirtyChunks {
			return index.NewCorruptIndexException(
				fmt.Sprintf("Cannot have more dirty chunks than documents within dirty chunks: numDirtyChunks=%d, numDirtyDocs=%d",
					r.numDirtyChunks, r.numDirtyDocs),
				fmt.Sprint(metaIn))
		}

		r.decompressor = compressionMode.NewDecompressor()
		r.reader, err = packed.NewBlockPackedReaderIterator(r.vectorsStream, r.packedIntsVersion, termVectorsPackedBlockSize, 0)
		if err != nil {
			return err
		}

		if _, err := store.CheckFooter(metaIn); err != nil {
			return err
		}
		closeErr := metaIn.Close()
		metaIn = nil
		if closeErr != nil {
			return closeErr
		}
		r.prefetchedBlockIDCache = newTermVectorsPrefetchedBlockIDCache()
		return nil
	}()
	if err != nil {
		if metaIn != nil {
			// CodecUtil.checkFooter(metaIn, t) rethrows t, annotated with the
			// result of verifying the meta file's checksum.
			return nil, store.CheckFooterWithPriorError(metaIn, err)
		}
		return nil, err
	}
	success = true
	return r, nil
}

func (r *Lucene90CompressingTermVectorsReader) getCompressionMode() compressing.CompressionMode {
	return r.compressionMode
}

func (r *Lucene90CompressingTermVectorsReader) getChunkSize() int {
	return r.chunkSize
}

func (r *Lucene90CompressingTermVectorsReader) getPackedIntsVersion() int {
	return r.packedIntsVersion
}

func (r *Lucene90CompressingTermVectorsReader) getVersion() int32 {
	return r.version
}

func (r *Lucene90CompressingTermVectorsReader) getIndexReader() fieldsIndex {
	return r.indexReader
}

func (r *Lucene90CompressingTermVectorsReader) getVectorsStream() store.IndexInput {
	return r.vectorsStream
}

func (r *Lucene90CompressingTermVectorsReader) getMaxPointer() int64 {
	return r.maxPointer
}

func (r *Lucene90CompressingTermVectorsReader) getNumDirtyDocs() (int64, error) {
	if r.version != termVectorsVersionCurrent {
		return 0, errors.New("getNumDirtyDocs should only ever get called when the reader is on the current version")
	}
	// assert numDirtyDocs >= 0;
	return r.numDirtyDocs, nil
}

func (r *Lucene90CompressingTermVectorsReader) getNumDirtyChunks() (int64, error) {
	if r.version != termVectorsVersionCurrent {
		return 0, errors.New("getNumDirtyChunks should only ever get called when the reader is on the current version")
	}
	// assert numDirtyChunks >= 0;
	return r.numDirtyChunks, nil
}

func (r *Lucene90CompressingTermVectorsReader) getNumChunks() (int64, error) {
	if r.version != termVectorsVersionCurrent {
		return 0, errors.New("getNumChunks should only ever get called when the reader is on the current version")
	}
	// assert numChunks >= 0;
	return r.numChunks, nil
}

func (r *Lucene90CompressingTermVectorsReader) getNumDocs() int {
	return r.numDocs
}

func (r *Lucene90CompressingTermVectorsReader) ensureOpen() error {
	if r.closed {
		return store.NewAlreadyClosedException("this FieldsReader is closed", nil)
	}
	return nil
}

// Close mirrors Lucene90CompressingTermVectorsReader.close():
// IOUtils.close(indexReader, vectorsStream) once.
func (r *Lucene90CompressingTermVectorsReader) Close() error {
	if !r.closed {
		var errs []error
		if r.indexReader != nil {
			if err := r.indexReader.Close(); err != nil {
				errs = append(errs, err)
			}
		}
		if r.vectorsStream != nil {
			if err := r.vectorsStream.Close(); err != nil {
				errs = append(errs, err)
			}
		}
		if err := errors.Join(errs...); err != nil {
			return err
		}
		r.closed = true
	}
	return nil
}

// Clone mirrors Lucene90CompressingTermVectorsReader.clone().
func (r *Lucene90CompressingTermVectorsReader) Clone() (gcodecs.TermVectorsReader, error) {
	return newLucene90CompressingTermVectorsReaderFrom(r)
}

// GetMergeInstance mirrors
// Lucene90CompressingTermVectorsReader.getMergeInstance().
func (r *Lucene90CompressingTermVectorsReader) GetMergeInstance() (gcodecs.TermVectorsReader, error) {
	return newLucene90CompressingTermVectorsReaderFrom(r)
}

// termVectorsSlice mirrors the private static
// Lucene90CompressingTermVectorsReader.slice(IndexInput).
func termVectorsSlice(in store.IndexInput) (packed.RandomAccessInput, error) {
	length, err := in.ReadVInt()
	if err != nil {
		return nil, err
	}
	bytes := make([]byte, length)
	if err := in.ReadBytes(bytes, 0, int(length)); err != nil {
		return nil, err
	}
	return store.NewByteArrayRandomAccessInput(bytes), nil
}

func (r *Lucene90CompressingTermVectorsReader) isLoaded(docID int) bool {
	return r.blockState.docBase <= docID && docID < r.blockState.docBase+r.blockState.chunkDocs
}

// Prefetch mirrors Lucene90CompressingTermVectorsReader.prefetch(int).
func (r *Lucene90CompressingTermVectorsReader) Prefetch(docID int) error {
	blockID, err := r.indexReader.getBlockID(docID)
	if err != nil {
		return err
	}

	for _, prefetchedBlockID := range r.prefetchedBlockIDCache {
		if prefetchedBlockID == blockID {
			return nil
		}
	}

	blockStartPointer, err := r.indexReader.getBlockStartPointer(blockID)
	if err != nil {
		return err
	}
	blockLength, err := r.indexReader.getBlockLength(blockID)
	if err != nil {
		return err
	}
	// vectorsStream.prefetch(blockStartPointer, blockLength). IndexInput.prefetch
	// is a no-op by default; inputs that implement it are asked to prefetch.
	if prefetcher, ok := r.vectorsStream.(interface {
		Prefetch(offset int64, length int64) error
	}); ok {
		if err := prefetcher.Prefetch(blockStartPointer, blockLength); err != nil {
			return err
		}
	}

	r.prefetchedBlockIDCache[r.prefetchedBlockIDCacheIndex&termVectorsPrefetchCacheMask] = blockID
	r.prefetchedBlockIDCacheIndex++
	return nil
}

// longValuesGetInt returns (int) values.get(index).
func longValuesGetInt(values packed.LongValues, index int) (int, error) {
	v, err := values.Get(int64(index))
	if err != nil {
		return 0, err
	}
	return int(int32(v)), nil
}

// Get mirrors Lucene90CompressingTermVectorsReader.get(int). It returns nil
// when the document has no term vectors.
func (r *Lucene90CompressingTermVectorsReader) Get(doc int) (index.Fields, error) {
	if err := r.ensureOpen(); err != nil {
		return nil, err
	}

	// seek to the right place
	var startPointer int64
	if r.isLoaded(doc) {
		startPointer = r.blockState.startPointer // avoid searching the start pointer
	} else {
		var err error
		startPointer, err = fieldsIndexStartPointer(r.indexReader, doc)
		if err != nil {
			return nil, err
		}
	}
	if err := r.vectorsStream.SetPosition(startPointer); err != nil {
		return nil, err
	}

	// decode
	// - docBase: first doc ID of the chunk
	// - chunkDocs: number of docs of the chunk
	docBaseV, err := r.vectorsStream.ReadVInt()
	if err != nil {
		return nil, err
	}
	docBase := int(docBaseV)
	chunkDocsV, err := r.vectorsStream.ReadVInt()
	if err != nil {
		return nil, err
	}
	chunkDocs := int(uint32(chunkDocsV) >> 1)
	if doc < docBase || doc >= docBase+chunkDocs || docBase+chunkDocs > r.numDocs {
		return nil, index.NewCorruptIndexException(
			fmt.Sprintf("docBase=%d,chunkDocs=%d,doc=%d", docBase, chunkDocs, doc), fmt.Sprint(r.vectorsStream))
	}
	r.blockState = termVectorsBlockState{startPointer: startPointer, docBase: docBase, chunkDocs: chunkDocs}

	var skip int        // number of fields to skip
	var numFields int   // number of fields of the document we're looking for
	var totalFields int // total number of fields of the chunk (sum for all docs)
	if chunkDocs == 1 {
		skip = 0
		v, err := r.vectorsStream.ReadVInt()
		if err != nil {
			return nil, err
		}
		numFields = int(v)
		totalFields = numFields
	} else {
		r.reader.Reset(r.vectorsStream, int64(chunkDocs))
		sum := 0
		for i := docBase; i < doc; i++ {
			v, err := r.reader.Next()
			if err != nil {
				return nil, err
			}
			sum += int(v)
		}
		skip = sum
		v, err := r.reader.Next()
		if err != nil {
			return nil, err
		}
		numFields = int(v)
		sum += numFields
		for i := doc + 1; i < docBase+chunkDocs; i++ {
			v, err := r.reader.Next()
			if err != nil {
				return nil, err
			}
			sum += int(v)
		}
		totalFields = sum
	}

	if numFields == 0 {
		// no vectors
		return nil, nil
	}

	// read field numbers that have term vectors
	var fieldNums []int
	{
		tokenByte, err := r.vectorsStream.ReadByte()
		if err != nil {
			return nil, err
		}
		token := int(tokenByte) & 0xFF
		// assert token != 0; // means no term vectors, cannot happen since we checked for numFields == 0
		bitsPerFieldNum := token & 0x1F
		totalDistinctFields := token >> 5
		if totalDistinctFields == 0x07 {
			v, err := r.vectorsStream.ReadVInt()
			if err != nil {
				return nil, err
			}
			totalDistinctFields += int(v)
		}
		totalDistinctFields++
		it, err := packed.GetReaderIteratorNoHeader(
			r.vectorsStream,
			packed.FormatPacked,
			r.packedIntsVersion,
			totalDistinctFields,
			bitsPerFieldNum,
			1)
		if err != nil {
			return nil, err
		}
		fieldNums = make([]int, totalDistinctFields)
		for i := 0; i < totalDistinctFields; i++ {
			v, err := it.Next()
			if err != nil {
				return nil, err
			}
			fieldNums[i] = int(v)
		}
	}

	// read field numbers and flags
	fieldNumOffs := make([]int, numFields)
	var flags packed.LongValues
	{
		bitsPerOff := packed.DirectWriterBitsRequired(int64(len(fieldNums) - 1))
		allFieldNumOffsSlice, err := termVectorsSlice(r.vectorsStream)
		if err != nil {
			return nil, err
		}
		allFieldNumOffs, err := packed.GetDirectReader(allFieldNumOffsSlice, bitsPerOff)
		if err != nil {
			return nil, err
		}
		switchValue, err := r.vectorsStream.ReadVInt()
		if err != nil {
			return nil, err
		}
		switch switchValue {
		case 0:
			fieldFlagsSlice, err := termVectorsSlice(r.vectorsStream)
			if err != nil {
				return nil, err
			}
			fieldFlags, err := packed.GetDirectReader(fieldFlagsSlice, termVectorsFlagsBits)
			if err != nil {
				return nil, err
			}
			out := store.NewByteBuffersDataOutput()
			writer, err := packed.GetDirectWriter(out, int64(totalFields), termVectorsFlagsBits)
			if err != nil {
				return nil, err
			}
			for i := 0; i < totalFields; i++ {
				fieldNumOff, err := longValuesGetInt(allFieldNumOffs, i)
				if err != nil {
					return nil, err
				}
				// assert fieldNumOff >= 0 && fieldNumOff < fieldNums.length;
				fl, err := fieldFlags.Get(int64(fieldNumOff))
				if err != nil {
					return nil, err
				}
				if err := writer.Add(fl); err != nil {
					return nil, err
				}
			}
			if err := writer.Finish(); err != nil {
				return nil, err
			}
			flags, err = packed.GetDirectReader(store.NewByteArrayRandomAccessInput(out.ToArrayCopy()), termVectorsFlagsBits)
			if err != nil {
				return nil, err
			}
		case 1:
			flagsSlice, err := termVectorsSlice(r.vectorsStream)
			if err != nil {
				return nil, err
			}
			flags, err = packed.GetDirectReader(flagsSlice, termVectorsFlagsBits)
			if err != nil {
				return nil, err
			}
		default:
			// throw new AssertionError();
			return nil, fmt.Errorf("AssertionError: unexpected flags encoding %d", switchValue)
		}
		for i := 0; i < numFields; i++ {
			fieldNumOffs[i], err = longValuesGetInt(allFieldNumOffs, skip+i)
			if err != nil {
				return nil, err
			}
		}
	}

	// number of terms per field for all fields
	var numTerms packed.LongValues
	var totalTerms int
	{
		bitsRequired, err := r.vectorsStream.ReadVInt()
		if err != nil {
			return nil, err
		}
		numTermsSlice, err := termVectorsSlice(r.vectorsStream)
		if err != nil {
			return nil, err
		}
		numTerms, err = packed.GetDirectReader(numTermsSlice, int(bitsRequired))
		if err != nil {
			return nil, err
		}
		sum := 0
		for i := 0; i < totalFields; i++ {
			v, err := longValuesGetInt(numTerms, i)
			if err != nil {
				return nil, err
			}
			sum += v
		}
		totalTerms = sum
	}

	// term lengths
	docOff, docLen, totalLen := 0, 0, 0
	fieldLengths := make([]int, numFields)
	prefixLengths := make([][]int32, numFields)
	suffixLengths := make([][]int32, numFields)
	{
		r.reader.Reset(r.vectorsStream, int64(totalTerms))
		// skip
		toSkip := 0
		for i := 0; i < skip; i++ {
			v, err := longValuesGetInt(numTerms, i)
			if err != nil {
				return nil, err
			}
			toSkip += v
		}
		if err := r.reader.Skip(int64(toSkip)); err != nil {
			return nil, err
		}
		// read prefix lengths
		for i := 0; i < numFields; i++ {
			termCount, err := longValuesGetInt(numTerms, skip+i)
			if err != nil {
				return nil, err
			}
			fieldPrefixLengths := make([]int32, termCount)
			prefixLengths[i] = fieldPrefixLengths
			for j := 0; j < termCount; {
				next, err := r.reader.NextN(termCount - j)
				if err != nil {
					return nil, err
				}
				for k := 0; k < next.Length; k++ {
					fieldPrefixLengths[j] = int32(next.Longs[next.Offset+k])
					j++
				}
			}
		}
		if err := r.reader.Skip(int64(totalTerms) - r.reader.Ord()); err != nil {
			return nil, err
		}

		r.reader.Reset(r.vectorsStream, int64(totalTerms))
		// skip
		for i := 0; i < skip; i++ {
			count, err := longValuesGetInt(numTerms, i)
			if err != nil {
				return nil, err
			}
			for j := 0; j < count; j++ {
				v, err := r.reader.Next()
				if err != nil {
					return nil, err
				}
				docOff += int(v)
			}
		}
		for i := 0; i < numFields; i++ {
			termCount, err := longValuesGetInt(numTerms, skip+i)
			if err != nil {
				return nil, err
			}
			fieldSuffixLengths := make([]int32, termCount)
			suffixLengths[i] = fieldSuffixLengths
			for j := 0; j < termCount; {
				next, err := r.reader.NextN(termCount - j)
				if err != nil {
					return nil, err
				}
				for k := 0; k < next.Length; k++ {
					fieldSuffixLengths[j] = int32(next.Longs[next.Offset+k])
					j++
				}
			}
			fieldLengths[i] = termVectorsSum(suffixLengths[i])
			docLen += fieldLengths[i]
		}
		totalLen = docOff + docLen
		for i := skip + numFields; i < totalFields; i++ {
			count, err := longValuesGetInt(numTerms, i)
			if err != nil {
				return nil, err
			}
			for j := 0; j < count; j++ {
				v, err := r.reader.Next()
				if err != nil {
					return nil, err
				}
				totalLen += int(v)
			}
		}
	}

	// term freqs
	termFreqs := make([]int32, totalTerms)
	{
		r.reader.Reset(r.vectorsStream, int64(totalTerms))
		for i := 0; i < totalTerms; {
			next, err := r.reader.NextN(totalTerms - i)
			if err != nil {
				return nil, err
			}
			for k := 0; k < next.Length; k++ {
				termFreqs[i] = 1 + int32(next.Longs[next.Offset+k])
				i++
			}
		}
	}

	// total number of positions, offsets and payloads
	totalPositions, totalOffsets, totalPayloads := 0, 0, 0
	for i, termIndex := 0, 0; i < totalFields; i++ {
		f, err := longValuesGetInt(flags, i)
		if err != nil {
			return nil, err
		}
		termCount, err := longValuesGetInt(numTerms, i)
		if err != nil {
			return nil, err
		}
		for j := 0; j < termCount; j++ {
			freq := int(termFreqs[termIndex])
			termIndex++
			if (f & termVectorsPositions) != 0 {
				totalPositions += freq
			}
			if (f & termVectorsOffsets) != 0 {
				totalOffsets += freq
			}
			if (f & termVectorsPayloads) != 0 {
				totalPayloads += freq
			}
		}
		// assert i != totalFields - 1 || termIndex == totalTerms : termIndex + " " + totalTerms;
	}

	positionIndex, err := r.positionIndex(skip, numFields, numTerms, termFreqs)
	if err != nil {
		return nil, err
	}
	var positions, startOffsets, lengths [][]int32
	if totalPositions > 0 {
		positions, err = r.readPositions(
			skip, numFields, flags, numTerms, termFreqs, termVectorsPositions, totalPositions, positionIndex)
		if err != nil {
			return nil, err
		}
	} else {
		positions = make([][]int32, numFields)
	}

	if totalOffsets > 0 {
		// average number of chars per term
		charsPerTerm := make([]float32, len(fieldNums))
		for i := range charsPerTerm {
			bits, err := r.vectorsStream.ReadInt()
			if err != nil {
				return nil, err
			}
			charsPerTerm[i] = math.Float32frombits(uint32(bits))
		}
		startOffsets, err = r.readPositions(
			skip, numFields, flags, numTerms, termFreqs, termVectorsOffsets, totalOffsets, positionIndex)
		if err != nil {
			return nil, err
		}
		lengths, err = r.readPositions(
			skip, numFields, flags, numTerms, termFreqs, termVectorsOffsets, totalOffsets, positionIndex)
		if err != nil {
			return nil, err
		}

		for i := 0; i < numFields; i++ {
			fStartOffsets := startOffsets[i]
			fPositions := positions[i]
			// patch offsets from positions
			if fStartOffsets != nil && fPositions != nil {
				fieldCharsPerTerm := charsPerTerm[fieldNumOffs[i]]
				for j := 0; j < len(startOffsets[i]); j++ {
					fStartOffsets[j] += floatToInt(fieldCharsPerTerm * float32(fPositions[j]))
				}
			}
			if fStartOffsets != nil {
				fPrefixLengths := prefixLengths[i]
				fSuffixLengths := suffixLengths[i]
				fLengths := lengths[i]
				end, err := longValuesGetInt(numTerms, skip+i)
				if err != nil {
					return nil, err
				}
				for j := 0; j < end; j++ {
					// delta-decode start offsets and  patch lengths using term lengths
					termLength := fPrefixLengths[j] + fSuffixLengths[j]
					lengths[i][positionIndex[i][j]] += termLength
					for k := positionIndex[i][j] + 1; k < positionIndex[i][j+1]; k++ {
						fStartOffsets[k] += fStartOffsets[k-1]
						fLengths[k] += termLength
					}
				}
			}
		}
	} else {
		startOffsets = make([][]int32, numFields)
		lengths = startOffsets
	}
	if totalPositions > 0 {
		// delta-decode positions
		for i := 0; i < numFields; i++ {
			fPositions := positions[i]
			fpositionIndex := positionIndex[i]
			if fPositions != nil {
				end, err := longValuesGetInt(numTerms, skip+i)
				if err != nil {
					return nil, err
				}
				for j := 0; j < end; j++ {
					// delta-decode start offsets
					for k := fpositionIndex[j] + 1; k < fpositionIndex[j+1]; k++ {
						fPositions[k] += fPositions[k-1]
					}
				}
			}
		}
	}

	// payload lengths
	payloadIndex := make([][]int, numFields)
	totalPayloadLength := 0
	payloadOff := 0
	payloadLen := 0
	if totalPayloads > 0 {
		r.reader.Reset(r.vectorsStream, int64(totalPayloads))
		// skip
		termIndex := 0
		for i := 0; i < skip; i++ {
			f, err := longValuesGetInt(flags, i)
			if err != nil {
				return nil, err
			}
			termCount, err := longValuesGetInt(numTerms, i)
			if err != nil {
				return nil, err
			}
			if (f & termVectorsPayloads) != 0 {
				for j := 0; j < termCount; j++ {
					freq := int(termFreqs[termIndex+j])
					for k := 0; k < freq; k++ {
						l, err := r.reader.Next()
						if err != nil {
							return nil, err
						}
						payloadOff += int(int32(l))
					}
				}
			}
			termIndex += termCount
		}
		totalPayloadLength = payloadOff
		// read doc payload lengths
		for i := 0; i < numFields; i++ {
			f, err := longValuesGetInt(flags, skip+i)
			if err != nil {
				return nil, err
			}
			termCount, err := longValuesGetInt(numTerms, skip+i)
			if err != nil {
				return nil, err
			}
			if (f & termVectorsPayloads) != 0 {
				totalFreq := positionIndex[i][termCount]
				payloadIndex[i] = make([]int, totalFreq+1)
				posIdx := 0
				payloadIndex[i][posIdx] = payloadLen
				for j := 0; j < termCount; j++ {
					freq := int(termFreqs[termIndex+j])
					for k := 0; k < freq; k++ {
						payloadLength, err := r.reader.Next()
						if err != nil {
							return nil, err
						}
						payloadLen += int(int32(payloadLength))
						payloadIndex[i][posIdx+1] = payloadLen
						posIdx++
					}
				}
				// assert posIdx == totalFreq;
			}
			termIndex += termCount
		}
		totalPayloadLength += payloadLen
		for i := skip + numFields; i < totalFields; i++ {
			f, err := longValuesGetInt(flags, i)
			if err != nil {
				return nil, err
			}
			termCount, err := longValuesGetInt(numTerms, i)
			if err != nil {
				return nil, err
			}
			if (f & termVectorsPayloads) != 0 {
				for j := 0; j < termCount; j++ {
					freq := int(termFreqs[termIndex+j])
					for k := 0; k < freq; k++ {
						v, err := r.reader.Next()
						if err != nil {
							return nil, err
						}
						totalPayloadLength += int(v)
					}
				}
			}
			termIndex += termCount
		}
		// assert termIndex == totalTerms : termIndex + " " + totalTerms;
	}

	// decompress data
	suffixBytes := &util.BytesRef{}
	if err := r.decompressor.Decompress(
		r.vectorsStream,
		totalLen+totalPayloadLength,
		docOff+payloadOff,
		docLen+payloadLen,
		suffixBytes); err != nil {
		return nil, err
	}
	suffixBytes.Length = docLen
	payloadBytes := &util.BytesRef{Bytes: suffixBytes.Bytes, Offset: suffixBytes.Offset + docLen, Length: payloadLen}

	fieldFlags := make([]int, numFields)
	for i := 0; i < numFields; i++ {
		if fieldFlags[i], err = longValuesGetInt(flags, skip+i); err != nil {
			return nil, err
		}
	}

	fieldNumTerms := make([]int, numFields)
	for i := 0; i < numFields; i++ {
		if fieldNumTerms[i], err = longValuesGetInt(numTerms, skip+i); err != nil {
			return nil, err
		}
	}

	fieldTermFreqs := make([][]int32, numFields)
	{
		termIdx := 0
		for i := 0; i < skip; i++ {
			count, err := longValuesGetInt(numTerms, i)
			if err != nil {
				return nil, err
			}
			termIdx += count
		}
		for i := 0; i < numFields; i++ {
			termCount, err := longValuesGetInt(numTerms, skip+i)
			if err != nil {
				return nil, err
			}
			fieldTermFreqs[i] = make([]int32, termCount)
			for j := 0; j < termCount; j++ {
				fieldTermFreqs[i][j] = termFreqs[termIdx]
				termIdx++
			}
		}
	}

	// assert sum(fieldLengths) == docLen : sum(fieldLengths) + " != " + docLen;

	return &termVectorsTVFields{
		fieldInfos:    r.fieldInfos,
		fieldNums:     fieldNums,
		fieldFlags:    fieldFlags,
		fieldNumOffs:  fieldNumOffs,
		numTerms:      fieldNumTerms,
		fieldLengths:  fieldLengths,
		prefixLengths: prefixLengths,
		suffixLengths: suffixLengths,
		termFreqs:     fieldTermFreqs,
		positionIndex: positionIndex,
		positions:     positions,
		startOffsets:  startOffsets,
		lengths:       lengths,
		payloadBytes:  payloadBytes,
		payloadIndex:  payloadIndex,
		suffixBytes:   suffixBytes,
	}, nil
}

// GetField mirrors the final TermVectors.get(int doc, String field) that
// Lucene90CompressingTermVectorsReader inherits.
func (r *Lucene90CompressingTermVectorsReader) GetField(doc int, field string) (index.Terms, error) {
	vectors, err := r.Get(doc)
	if err != nil {
		return nil, err
	}
	if vectors == nil {
		return nil, nil
	}
	return vectors.Terms(field)
}

// positionIndex returns field -> term index -> position index. Mirrors
// Lucene90CompressingTermVectorsReader.positionIndex.
func (r *Lucene90CompressingTermVectorsReader) positionIndex(
	skip, numFields int, numTerms packed.LongValues, termFreqs []int32,
) ([][]int, error) {
	positionIndex := make([][]int, numFields)
	termIndex := 0
	for i := 0; i < skip; i++ {
		termCount, err := longValuesGetInt(numTerms, i)
		if err != nil {
			return nil, err
		}
		termIndex += termCount
	}
	for i := 0; i < numFields; i++ {
		termCount, err := longValuesGetInt(numTerms, skip+i)
		if err != nil {
			return nil, err
		}
		positionIndex[i] = make([]int, termCount+1)
		for j := 0; j < termCount; j++ {
			freq := int(termFreqs[termIndex+j])
			positionIndex[i][j+1] = positionIndex[i][j] + freq
		}
		termIndex += termCount
	}
	return positionIndex, nil
}

// readPositions mirrors Lucene90CompressingTermVectorsReader.readPositions.
func (r *Lucene90CompressingTermVectorsReader) readPositions(
	skip, numFields int,
	flags, numTerms packed.LongValues,
	termFreqs []int32,
	flag int,
	totalPositions int,
	positionIndex [][]int,
) ([][]int32, error) {
	positions := make([][]int32, numFields)
	r.reader.Reset(r.vectorsStream, int64(totalPositions))
	// skip
	toSkip := 0
	termIndex := 0
	for i := 0; i < skip; i++ {
		f, err := longValuesGetInt(flags, i)
		if err != nil {
			return nil, err
		}
		termCount, err := longValuesGetInt(numTerms, i)
		if err != nil {
			return nil, err
		}
		if (f & flag) != 0 {
			for j := 0; j < termCount; j++ {
				freq := int(termFreqs[termIndex+j])
				toSkip += freq
			}
		}
		termIndex += termCount
	}
	if err := r.reader.Skip(int64(toSkip)); err != nil {
		return nil, err
	}
	// read doc positions
	for i := 0; i < numFields; i++ {
		f, err := longValuesGetInt(flags, skip+i)
		if err != nil {
			return nil, err
		}
		termCount, err := longValuesGetInt(numTerms, skip+i)
		if err != nil {
			return nil, err
		}
		if (f & flag) != 0 {
			totalFreq := positionIndex[i][termCount]
			fieldPositions := make([]int32, totalFreq)
			positions[i] = fieldPositions
			for j := 0; j < totalFreq; {
				nextPositions, err := r.reader.NextN(totalFreq - j)
				if err != nil {
					return nil, err
				}
				for k := 0; k < nextPositions.Length; k++ {
					fieldPositions[j] = int32(nextPositions.Longs[nextPositions.Offset+k])
					j++
				}
			}
		}
		termIndex += termCount
	}
	if err := r.reader.Skip(int64(totalPositions) - r.reader.Ord()); err != nil {
		return nil, err
	}
	return positions, nil
}

// termVectorsTVFields is the Go rendering of the inner class
// Lucene90CompressingTermVectorsReader.TVFields.
type termVectorsTVFields struct {
	fieldInfos                                                  *index.FieldInfos
	fieldNums, fieldFlags, fieldNumOffs, numTerms, fieldLengths []int
	prefixLengths, suffixLengths, termFreqs                     [][]int32
	positionIndex                                               [][]int
	positions, startOffsets, lengths                            [][]int32
	payloadIndex                                                [][]int
	suffixBytes, payloadBytes                                   *util.BytesRef
}

// Iterator mirrors TVFields.iterator().
func (f *termVectorsTVFields) Iterator() (index.FieldIterator, error) {
	return &termVectorsTVFieldsIterator{fields: f}, nil
}

// termVectorsTVFieldsIterator is the anonymous Iterator<String> of
// TVFields.iterator(). Following the spi FieldIterator contract, Next returns
// the empty string once the fields are exhausted.
type termVectorsTVFieldsIterator struct {
	fields *termVectorsTVFields
	i      int
}

func (it *termVectorsTVFieldsIterator) HasNext() bool {
	return it.i < len(it.fields.fieldNumOffs)
}

func (it *termVectorsTVFieldsIterator) Next() (string, error) {
	if !it.HasNext() {
		return "", nil
	}
	fieldNum := it.fields.fieldNums[it.fields.fieldNumOffs[it.i]]
	it.i++
	fieldInfo := it.fields.fieldInfos.FieldInfoByNumber(fieldNum)
	if fieldInfo == nil {
		return "", fmt.Errorf("no FieldInfo for field number %d", fieldNum)
	}
	return fieldInfo.Name(), nil
}

// Terms mirrors TVFields.terms(String).
func (f *termVectorsTVFields) Terms(field string) (index.Terms, error) {
	fieldInfo := f.fieldInfos.FieldInfo(field)
	if fieldInfo == nil {
		return nil, nil
	}
	idx := -1
	for i := 0; i < len(f.fieldNumOffs); i++ {
		if f.fieldNums[f.fieldNumOffs[i]] == fieldInfo.Number() {
			idx = i
			break
		}
	}

	if idx == -1 || f.numTerms[idx] == 0 {
		// no term
		return nil, nil
	}
	fieldOff, fieldLen := 0, -1
	for i := 0; i < len(f.fieldNumOffs); i++ {
		if i < idx {
			fieldOff += f.fieldLengths[i]
		} else {
			fieldLen = f.fieldLengths[i]
			break
		}
	}
	// assert fieldLen >= 0;
	return newTermVectorsTVTerms(
		field,
		f.numTerms[idx],
		f.fieldFlags[idx],
		f.prefixLengths[idx],
		f.suffixLengths[idx],
		f.termFreqs[idx],
		f.positionIndex[idx],
		f.positions[idx],
		f.startOffsets[idx],
		f.lengths[idx],
		f.payloadIndex[idx],
		f.payloadBytes,
		&util.BytesRef{Bytes: f.suffixBytes.Bytes, Offset: f.suffixBytes.Offset + fieldOff, Length: fieldLen}), nil
}

// Size mirrors TVFields.size().
func (f *termVectorsTVFields) Size() int {
	return len(f.fieldNumOffs)
}

// termVectorsTVTerms is the Go rendering of the private static nested class
// Lucene90CompressingTermVectorsReader.TVTerms.
type termVectorsTVTerms struct {
	field                                   string
	numTerms, flags                         int
	totalTermFreq                           int64
	prefixLengths, suffixLengths, termFreqs []int32
	positionIndex                           []int
	positions, startOffsets, lengths        []int32
	payloadIndex                            []int
	termBytes, payloadBytes                 *util.BytesRef
}

func newTermVectorsTVTerms(
	field string,
	numTerms, flags int,
	prefixLengths, suffixLengths, termFreqs []int32,
	positionIndex []int,
	positions, startOffsets, lengths []int32,
	payloadIndex []int,
	payloadBytes, termBytes *util.BytesRef,
) *termVectorsTVTerms {
	t := &termVectorsTVTerms{
		field:         field,
		numTerms:      numTerms,
		flags:         flags,
		prefixLengths: prefixLengths,
		suffixLengths: suffixLengths,
		termFreqs:     termFreqs,
		positionIndex: positionIndex,
		positions:     positions,
		startOffsets:  startOffsets,
		lengths:       lengths,
		payloadIndex:  payloadIndex,
		payloadBytes:  payloadBytes,
		termBytes:     termBytes,
	}
	ttf := int64(0)
	for _, tf := range termFreqs {
		ttf += int64(tf)
	}
	t.totalTermFreq = ttf
	return t
}

// Field returns the name of the field these terms belong to.
func (t *termVectorsTVTerms) Field() string {
	return t.field
}

func (t *termVectorsTVTerms) iterator() *termVectorsTVTermsEnum {
	termsEnum := newTermVectorsTVTermsEnum()
	termsEnum.reset(
		t.field,
		t.numTerms,
		t.flags,
		t.prefixLengths,
		t.suffixLengths,
		t.termFreqs,
		t.positionIndex,
		t.positions,
		t.startOffsets,
		t.lengths,
		t.payloadIndex,
		t.payloadBytes,
		store.NewByteArrayDataInputWithOffset(t.termBytes.Bytes, t.termBytes.Offset, t.termBytes.Length))
	return termsEnum
}

// Iterator mirrors TVTerms.iterator().
func (t *termVectorsTVTerms) Iterator() (index.TermsEnum, error) {
	return t.iterator(), nil
}

// GetIteratorWithSeek returns a TermsEnum positioned at or after seekTerm, or
// nil when no term is on or after it.
func (t *termVectorsTVTerms) GetIteratorWithSeek(seekTerm *index.Term) (index.TermsEnum, error) {
	termsEnum := t.iterator()
	if seekTerm == nil {
		return termsEnum, nil
	}
	term, err := termsEnum.SeekCeil(seekTerm)
	if err != nil {
		return nil, err
	}
	if term == nil {
		return nil, nil
	}
	return termsEnum, nil
}

// Intersect is the default Terms.intersect(CompiledAutomaton, BytesRef) that
// TVTerms inherits.
func (t *termVectorsTVTerms) Intersect(compiled *automaton.CompiledAutomaton, startTerm *index.Term) (index.TermsEnum, error) {
	termsEnum := t.iterator()
	if compiled.Type != automaton.AutomatonTypeNormal {
		return nil, errors.New("please use CompiledAutomaton.getTermsEnum instead")
	}
	automatonTermsEnum := index.NewAutomatonTermsEnum(termsEnum, compiled)
	if startTerm != nil {
		automatonTermsEnum.SetInitialSeekTerm(startTerm)
	}
	return automatonTermsEnum, nil
}

// GetPostingsReader returns the postings of termText, or nil when the term is
// absent.
func (t *termVectorsTVTerms) GetPostingsReader(termText string, flags int) (index.PostingsEnum, error) {
	termsEnum := t.iterator()
	found, err := termsEnum.SeekExact(index.NewTerm(t.field, termText))
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	return termsEnum.Postings(flags)
}

// Size mirrors TVTerms.size().
func (t *termVectorsTVTerms) Size() int64 {
	return int64(t.numTerms)
}

// GetSumTotalTermFreq mirrors TVTerms.getSumTotalTermFreq().
func (t *termVectorsTVTerms) GetSumTotalTermFreq() (int64, error) {
	return t.totalTermFreq, nil
}

// GetSumDocFreq mirrors TVTerms.getSumDocFreq().
func (t *termVectorsTVTerms) GetSumDocFreq() (int64, error) {
	return int64(t.numTerms), nil
}

// GetDocCount mirrors TVTerms.getDocCount().
func (t *termVectorsTVTerms) GetDocCount() (int, error) {
	return 1, nil
}

// HasFreqs mirrors TVTerms.hasFreqs().
func (t *termVectorsTVTerms) HasFreqs() bool {
	return true
}

// HasOffsets mirrors TVTerms.hasOffsets().
func (t *termVectorsTVTerms) HasOffsets() bool {
	return (t.flags & termVectorsOffsets) != 0
}

// HasPositions mirrors TVTerms.hasPositions().
func (t *termVectorsTVTerms) HasPositions() bool {
	return (t.flags & termVectorsPositions) != 0
}

// HasPayloads mirrors TVTerms.hasPayloads().
func (t *termVectorsTVTerms) HasPayloads() bool {
	return (t.flags & termVectorsPayloads) != 0
}

// GetMin is the default Terms.getMin() that TVTerms inherits:
// iterator().next().
func (t *termVectorsTVTerms) GetMin() (*index.Term, error) {
	return t.iterator().Next()
}

// GetMax is the default Terms.getMax() that TVTerms inherits.
func (t *termVectorsTVTerms) GetMax() (*index.Term, error) {
	size := t.Size()

	if size == 0 {
		// empty: only possible from a FilteredTermsEnum...
		return nil, nil
	} else if size >= 0 {
		// try to seek-by-ord
		iterator := t.iterator()
		if err := iterator.SeekExactOrd(size - 1); err == nil {
			return iterator.Term(), nil
		} else if !errors.Is(err, errTVTermsEnumUnsupported) {
			return nil, err
		}
		// ok
	}

	// otherwise: binary search
	iterator := t.iterator()
	v, err := iterator.Next()
	if err != nil {
		return nil, err
	}
	if v == nil {
		// empty: only possible from a FilteredTermsEnum...
		return v, nil
	}

	scratch := []byte{0}

	// Iterates over digits:
	for {
		low := 0
		high := 256

		// Binary search current digit to find the highest
		// digit before END:
		for low != high {
			mid := int(uint(low+high) >> 1)
			scratch[len(scratch)-1] = byte(mid)
			term, err := iterator.SeekCeil(index.NewTermFromBytes(t.field, scratch))
			if err != nil {
				return nil, err
			}
			if term == nil {
				// Scratch was too high
				if mid == 0 {
					scratch = scratch[:len(scratch)-1]
					return index.NewTermFromBytes(t.field, scratch), nil
				}
				high = mid
			} else {
				// Scratch was too low; there is at least one term
				// still after it:
				if low == mid {
					break
				}
				low = mid
			}
		}

		// Recurse to next digit:
		scratch = append(scratch, 0)
	}
}

// termVectorsTVTermsEnum is the Go rendering of the private static nested
// class Lucene90CompressingTermVectorsReader.TVTermsEnum, a BaseTermsEnum.
type termVectorsTVTermsEnum struct {
	spi.TermsEnumBase

	field                                   string
	numTerms, startPos, ord                 int
	prefixLengths, suffixLengths, termFreqs []int32
	positionIndex                           []int
	positions, startOffsets, lengths        []int32
	payloadIndex                            []int
	in                                      *store.ByteArrayDataInput
	payloads                                *util.BytesRef
	term                                    *util.BytesRef
}

func newTermVectorsTVTermsEnum() *termVectorsTVTermsEnum {
	return &termVectorsTVTermsEnum{term: &util.BytesRef{Bytes: make([]byte, 16)}}
}

func (e *termVectorsTVTermsEnum) reset(
	field string,
	numTerms, flags int,
	prefixLengths, suffixLengths, termFreqs []int32,
	positionIndex []int,
	positions, startOffsets, lengths []int32,
	payloadIndex []int,
	payloads *util.BytesRef,
	in *store.ByteArrayDataInput,
) {
	e.field = field
	e.numTerms = numTerms
	e.prefixLengths = prefixLengths
	e.suffixLengths = suffixLengths
	e.termFreqs = termFreqs
	e.positionIndex = positionIndex
	e.positions = positions
	e.startOffsets = startOffsets
	e.lengths = lengths
	e.payloadIndex = payloadIndex
	e.payloads = payloads
	e.in = in
	e.startPos = in.GetPosition()
	e.rewind()
}

// rewind is the no-argument TVTermsEnum.reset() overload.
func (e *termVectorsTVTermsEnum) rewind() {
	e.term.Length = 0
	e.in.SetPosition(e.startPos)
	e.ord = -1
}

// Next mirrors TVTermsEnum.next().
func (e *termVectorsTVTermsEnum) Next() (*index.Term, error) {
	if e.ord == e.numTerms-1 {
		return nil, nil
	}
	// assert ord < numTerms;
	e.ord++

	// read term
	e.term.Offset = 0
	e.term.Length = int(e.prefixLengths[e.ord] + e.suffixLengths[e.ord])
	if e.term.Length > len(e.term.Bytes) {
		e.term.Bytes = util.GrowByte(e.term.Bytes, e.term.Length)
	}
	if err := e.in.ReadBytes(e.term.Bytes, int(e.prefixLengths[e.ord]), int(e.suffixLengths[e.ord])); err != nil {
		return nil, err
	}

	return e.Term(), nil
}

// SeekCeil mirrors TVTermsEnum.seekCeil(BytesRef). It returns the term the
// enum is positioned on (SeekStatus.FOUND or NOT_FOUND), or nil at
// SeekStatus.END.
func (e *termVectorsTVTermsEnum) SeekCeil(text *index.Term) (*index.Term, error) {
	target := text.BytesValue()
	if e.ord < e.numTerms && e.ord >= 0 {
		cmp := util.BytesRefCompare(e.term, target)
		if cmp == 0 {
			return e.Term(), nil
		} else if cmp > 0 {
			e.rewind()
		}
	}
	// linear scan
	for {
		term, err := e.Next()
		if err != nil {
			return nil, err
		}
		if term == nil {
			return nil, nil
		}
		cmp := util.BytesRefCompare(e.term, target)
		if cmp > 0 {
			return term, nil
		} else if cmp == 0 {
			return term, nil
		}
	}
}

// SeekExact is the default BaseTermsEnum.seekExact(BytesRef) that TVTermsEnum
// inherits: seekCeil(text) == SeekStatus.FOUND.
func (e *termVectorsTVTermsEnum) SeekExact(text *index.Term) (bool, error) {
	term, err := e.SeekCeil(text)
	if err != nil {
		return false, err
	}
	return term != nil && util.BytesRefCompare(e.term, text.BytesValue()) == 0, nil
}

// SeekExactOrd mirrors TVTermsEnum.seekExact(long), which throws
// UnsupportedOperationException.
func (e *termVectorsTVTermsEnum) SeekExactOrd(ord int64) error {
	return errTVTermsEnumUnsupported
}

// Term mirrors TVTermsEnum.term().
func (e *termVectorsTVTermsEnum) Term() *index.Term {
	return index.NewTermFromBytesRef(e.field, e.term)
}

// Ord mirrors TVTermsEnum.ord(), which throws UnsupportedOperationException.
// Ord carries no error in the TermsEnum contract, so the exception is raised
// as a panic.
func (e *termVectorsTVTermsEnum) Ord() int64 {
	panic(errTVTermsEnumUnsupported)
}

// DocFreq mirrors TVTermsEnum.docFreq().
func (e *termVectorsTVTermsEnum) DocFreq() (int, error) {
	return 1, nil
}

// TotalTermFreq mirrors TVTermsEnum.totalTermFreq().
func (e *termVectorsTVTermsEnum) TotalTermFreq() (int64, error) {
	return int64(e.termFreqs[e.ord]), nil
}

// Postings mirrors TVTermsEnum.postings(PostingsEnum, int); the spi TermsEnum
// has no reuse parameter.
func (e *termVectorsTVTermsEnum) Postings(flags int) (index.PostingsEnum, error) {
	docsEnum := newTermVectorsTVPostingsEnum()
	docsEnum.reset(
		e.termFreqs[e.ord],
		e.positionIndex[e.ord],
		e.positions,
		e.startOffsets,
		e.lengths,
		e.payloads,
		e.payloadIndex)
	return docsEnum, nil
}

// PostingsWithLiveDocs returns Postings(flags): term vectors describe a single
// document, so there is no live-docs filtering to apply.
func (e *termVectorsTVTermsEnum) PostingsWithLiveDocs(liveDocs util.Bits, flags int) (index.PostingsEnum, error) {
	return e.Postings(flags)
}

// Impacts mirrors TVTermsEnum.impacts(int).
func (e *termVectorsTVTermsEnum) Impacts(flags int) (index.ImpactsEnum, error) {
	delegate, err := e.Postings(index.PostingsFlagFreqs)
	if err != nil {
		return nil, err
	}
	return index.NewSlowImpactsEnum(delegate), nil
}

// termVectorsTVPostingsEnum is the Go rendering of the private static nested
// class Lucene90CompressingTermVectorsReader.TVPostingsEnum.
type termVectorsTVPostingsEnum struct {
	doc               int
	termFreq          int
	positionIndex     int
	positions         []int32
	startOffsets      []int32
	lengths           []int32
	payload           *util.BytesRef
	payloadIndex      []int
	basePayloadOffset int
	i                 int
}

func newTermVectorsTVPostingsEnum() *termVectorsTVPostingsEnum {
	return &termVectorsTVPostingsEnum{doc: -1, payload: &util.BytesRef{}}
}

func (p *termVectorsTVPostingsEnum) reset(
	freq int32,
	positionIndex int,
	positions, startOffsets, lengths []int32,
	payloads *util.BytesRef,
	payloadIndex []int,
) {
	p.termFreq = int(freq)
	p.positionIndex = positionIndex
	p.positions = positions
	p.startOffsets = startOffsets
	p.lengths = lengths
	p.basePayloadOffset = payloads.Offset
	p.payload.Bytes = payloads.Bytes
	p.payload.Offset = 0
	p.payload.Length = 0
	p.payloadIndex = payloadIndex

	p.doc = -1
	p.i = -1
}

func (p *termVectorsTVPostingsEnum) checkDoc() error {
	if p.doc == index.NO_MORE_DOCS {
		return errors.New("DocsEnum exhausted")
	} else if p.doc == -1 {
		return errors.New("DocsEnum not started")
	}
	return nil
}

func (p *termVectorsTVPostingsEnum) checkPosition() error {
	if err := p.checkDoc(); err != nil {
		return err
	}
	if p.i < 0 {
		return errors.New("Position enum not started")
	} else if p.i >= p.termFreq {
		return errors.New("Read past last position")
	}
	return nil
}

// NextPosition mirrors TVPostingsEnum.nextPosition().
func (p *termVectorsTVPostingsEnum) NextPosition() (int, error) {
	if p.doc != 0 {
		// throw new IllegalStateException();
		return 0, errors.New("IllegalStateException")
	} else if p.i >= p.termFreq-1 {
		return 0, errors.New("Read past last position")
	}

	p.i++

	if p.payloadIndex != nil {
		p.payload.Offset = p.basePayloadOffset + p.payloadIndex[p.positionIndex+p.i]
		p.payload.Length = p.payloadIndex[p.positionIndex+p.i+1] - p.payloadIndex[p.positionIndex+p.i]
	}

	if p.positions == nil {
		return -1, nil
	}
	return int(p.positions[p.positionIndex+p.i]), nil
}

// StartOffset mirrors TVPostingsEnum.startOffset().
func (p *termVectorsTVPostingsEnum) StartOffset() (int, error) {
	if err := p.checkPosition(); err != nil {
		return 0, err
	}
	if p.startOffsets == nil {
		return -1, nil
	}
	return int(p.startOffsets[p.positionIndex+p.i]), nil
}

// EndOffset mirrors TVPostingsEnum.endOffset().
func (p *termVectorsTVPostingsEnum) EndOffset() (int, error) {
	if err := p.checkPosition(); err != nil {
		return 0, err
	}
	if p.startOffsets == nil {
		return -1, nil
	}
	return int(p.startOffsets[p.positionIndex+p.i] + p.lengths[p.positionIndex+p.i]), nil
}

// GetPayload mirrors TVPostingsEnum.getPayload().
func (p *termVectorsTVPostingsEnum) GetPayload() ([]byte, error) {
	if err := p.checkPosition(); err != nil {
		return nil, err
	}
	if p.payloadIndex == nil || p.payload.Length == 0 {
		return nil, nil
	}
	return p.payload.Bytes[p.payload.Offset : p.payload.Offset+p.payload.Length], nil
}

// Freq mirrors TVPostingsEnum.freq().
func (p *termVectorsTVPostingsEnum) Freq() (int, error) {
	if err := p.checkDoc(); err != nil {
		return 0, err
	}
	return p.termFreq, nil
}

// DocID mirrors TVPostingsEnum.docID().
func (p *termVectorsTVPostingsEnum) DocID() int {
	return p.doc
}

// NextDoc mirrors TVPostingsEnum.nextDoc().
func (p *termVectorsTVPostingsEnum) NextDoc() (int, error) {
	if p.doc == -1 {
		p.doc = 0
		return p.doc, nil
	}
	p.doc = index.NO_MORE_DOCS
	return p.doc, nil
}

// Advance mirrors TVPostingsEnum.advance(int): slowAdvance(target).
func (p *termVectorsTVPostingsEnum) Advance(target int) (int, error) {
	return util.SlowAdvance(p, target)
}

// Cost mirrors TVPostingsEnum.cost().
func (p *termVectorsTVPostingsEnum) Cost() int64 {
	return 1
}

// DocIDRunEnd carries the default body of DocIdSetIterator.docIDRunEnd(),
// which TVPostingsEnum does not override.
func (p *termVectorsTVPostingsEnum) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(p)
}

// IntoBitSet carries the default body of
// DocIdSetIterator.intoBitSet(int, FixedBitSet, int), which TVPostingsEnum does
// not override.
func (p *termVectorsTVPostingsEnum) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(p, upTo, bitSet, offset)
}

func termVectorsSum(arr []int32) int {
	sum := int32(0)
	for _, el := range arr {
		sum += el
	}
	return int(sum)
}

// CheckIntegrity mirrors Lucene90CompressingTermVectorsReader.checkIntegrity().
func (r *Lucene90CompressingTermVectorsReader) CheckIntegrity() error {
	if err := r.indexReader.checkIntegrity(); err != nil {
		return err
	}
	_, err := gcodecs.ChecksumEntireFile(r.vectorsStream)
	return err
}

// String mirrors Lucene90CompressingTermVectorsReader.toString().
func (r *Lucene90CompressingTermVectorsReader) String() string {
	return fmt.Sprintf("Lucene90CompressingTermVectorsReader(mode=%v,chunksize=%d)", r.compressionMode, r.chunkSize)
}

var _ gcodecs.TermVectorsReader = (*Lucene90CompressingTermVectorsReader)(nil)
