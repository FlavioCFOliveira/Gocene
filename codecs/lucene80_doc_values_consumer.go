// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"slices"

	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

type lucene80DocValuesConsumer struct {
	mode         Mode
	data         IndexOutput
	meta         IndexOutput
	maxDoc       int
	state        SegmentWriteState
	termsDictBuf []byte
}

func NewLucene80DocValuesConsumer(
	state SegmentWriteState,
	dataCodec, dataExtension, metaCodec, metaExtension string,
	mode Mode) (DocValuesConsumer, error) {

	var data, meta IndexOutput
	var success bool

	defer func() {
		if !success {
			if data != nil {
				data.Close()
			}
			if meta != nil {
				meta.Close()
			}
		}
	}()

	dataName := IndexFileNames.segmentFileName(state.segmentInfo.name, state.segmentSuffix, dataExtension)
	data, err := state.directory.CreateOutput(dataName, state.context)
	if err != nil {
		return nil, err
	}

	err = CodecUtil.WriteIndexHeader(
		data,
		dataCodec,
		VersionCurrent,
		state.segmentInfo.getId(),
		state.segmentSuffix)
	if err != nil {
		return nil, err
	}

	metaName := IndexFileNames.segmentFileName(state.segmentInfo.name, state.segmentSuffix, metaExtension)
	meta, err = state.directory.CreateOutput(metaName, state.context)
	if err != nil {
		return nil, err
	}

	err = CodecUtil.WriteIndexHeader(
		meta,
		metaCodec,
		VersionCurrent,
		state.segmentInfo.getId(),
		state.segmentSuffix)
	if err != nil {
		return nil, err
	}

	var termsDictBuf []byte
	if mode == ModeBestCompression {
		termsDictBuf = make([]byte, 1<<14)
	}

	success = true
	return &lucene80DocValuesConsumer{
		mode:         mode,
		data:         data,
		meta:         meta,
		maxDoc:       state.segmentInfo.maxDoc(),
		state:        state,
		termsDictBuf: termsDictBuf,
	}, nil
}

func (c *lucene80DocValuesConsumer) Close() error {
	var err error
	if c.meta != nil {
		if err = c.meta.WriteInt(-1); err != nil {
			return err
		}
		if err = CodecUtil.WriteFooter(c.meta); err != nil {
			return err
		}
	}
	if c.data != nil {
		if err = CodecUtil.WriteFooter(c.data); err != nil {
			return err
		}
	}

	if c.meta != nil {
		c.meta.Close()
	}
	if c.data != nil {
		c.data.Close()
	}
	return err
}

type minMaxTracker struct {
	min, max, numValues, spaceInBits int64
}

func newMinMaxTracker() *minMaxTracker {
	return &minMaxTracker{}
}

func (t *minMaxTracker) reset() {
	t.min = math.MaxInt64
	t.max = math.MinInt64
	t.numValues = 0
}

func (t *minMaxTracker) update(v int64) {
	if v < t.min {
		t.min = v
	}
	if v > t.max {
		t.max = v
	}
	t.numValues++
}

func (t *minMaxTracker) finish() {
	if t.max > t.min {
		t.spaceInBits += int64(packed.UnsignedBitsRequired(uint64(t.max-t.min))) * t.numValues
	}
}

func (t *minMaxTracker) nextBlock() {
	t.finish()
	t.reset()
}

func (c *lucene80DocValuesConsumer) AddNumericField(field FieldInfo, valuesProducer DocValuesProducer) error {
	c.meta.WriteInt(field.number)
	c.meta.WriteByte(NumericType)

	// We need to get the values from the producer
	// Note: In the Java version, it uses a singleton for the values.
	// We can just get the numeric values directly.
	values := valuesProducer.GetNumeric(field)

	numDocsWithValue, numValues, err := c.writeNumericValues(field, values)
	if err != nil {
		return err
	}

	// The Java version returns the stats, but we just wrote them to the meta file inside writeNumericValues.
	_ = numDocsWithValue
	_ = numValues

	return nil
}

func (c *lucene80DocValuesConsumer) writeNumericValues(field FieldInfo, values NumericDocValues) (int, int64, error) {
	numDocsWithValue := 0
	minMax := newMinMaxTracker()
	blockMinMax := newMinMaxTracker()
	var gcd int64 = 0
	var uniqueValues map[int64]struct{}

	for doc := values.NextDoc(); doc != dvNoMoreDocs; doc = values.NextDoc() {
		for i := 0; i < values.DocValueCount(); i++ {
			v := values.NextValue()

			if gcd != 1 {
				if v < math.MinInt64/2 || v > math.MaxInt64/2 {
					gcd = 1
				} else if minMax.numValues != 0 {
					gcd = gcdFunc(gcd, v-minMax.min)
				}
			}

			minMax.update(v)
			blockMinMax.update(v)
			if blockMinMax.numValues == NumericBlockSize {
				blockMinMax.nextBlock()
			}

			if uniqueValues == nil {
				uniqueValues = make(map[int64]struct{})
			}
			uniqueValues[v] = struct{}{}
			if len(uniqueValues) > 256 {
				uniqueValues = nil
			}
		}
		numDocsWithValue++
	}

	minMax.finish()
	blockMinMax.finish()

	numValues := minMax.numValues
	min := minMax.min
	max := minMax.max

	if numDocsWithValue == 0 {
		c.meta.WriteLong(-2)
		c.meta.WriteLong(0)
		c.meta.WriteShort(-1)
		c.meta.WriteByte(-1)
	} else if numDocsWithValue == c.maxDoc {
		c.meta.WriteLong(-1)
		c.meta.WriteLong(0)
		c.meta.WriteShort(-1)
		c.meta.WriteByte(-1)
	} else {
		offset := c.data.getFilePointer()
		c.meta.WriteLong(offset)

		// Write the BitSet (IndexedDISI)
		// Assuming IndexedDISI.WriteBitSet exists in package codecs
		jumpTableEntryCount := WriteBitSet(values, c.data, 4) // DEFAULT_DENSE_RANK_POWER = 4
		c.meta.WriteLong(c.data.getFilePointer() - offset)
		c.meta.WriteShort(jumpTableEntryCount)
		c.meta.WriteByte(4)
	}

	c.meta.WriteLong(numValues)

	var numBitsPerValue int
	var doBlocks bool = false
	var encode map[int64]int

	if min >= max {
		numBitsPerValue = 0
		c.meta.WriteInt(-1)
	} else {
		if uniqueValues != nil && len(uniqueValues) > 1 &&
			packed.UnsignedBitsRequired(uint64(len(uniqueValues)-1)) <
				packed.UnsignedBitsRequired(uint64((max-min)/gcd)) {

			numBitsPerValue = packed.UnsignedBitsRequired(uint64(len(uniqueValues) - 1))

			sortedUnique := make([]int64, 0, len(uniqueValues))
			for v := range uniqueValues {
				sortedUnique = append(sortedUnique, v)
			}
			// Sort the unique values
			sortInt64s(sortedUnique)

			c.meta.WriteInt(len(sortedUnique))
			for _, v := range sortedUnique {
				c.meta.WriteLong(v)
			}

			encode = make(map[int64]int)
			for i, v := range sortedUnique {
				encode[v] = i
			}
			min = 0
			gcd = 1
		} else {
			uniqueValues = nil
			doBlocks = minMax.spaceInBits > 0 && float64(blockMinMax.spaceInBits)/float64(minMax.spaceInBits) <= 0.9
			if doBlocks {
				numBitsPerValue = 0xFF
				c.meta.WriteInt(-2 - NumericBlockShift)
			} else {
				numBitsPerValue = packed.UnsignedBitsRequired(uint64((max - min) / gcd))
				if gcd == 1 && min > 0 &&
					packed.UnsignedBitsRequired(uint64(max)) == packed.UnsignedBitsRequired(uint64(max-min)) {
					min = 0
				}
				c.meta.WriteInt(-1)
			}
		}
	}

	c.meta.WriteByte(byte(numBitsPerValue))
	c.meta.WriteLong(min)
	c.meta.WriteLong(gcd)

	startOffset := c.data.getFilePointer()
	c.meta.WriteLong(startOffset)

	var jumpTableOffset int64 = -1
	if doBlocks {
		jumpTableOffset = c.writeNumericValuesMultipleBlocks(values, gcd)
	} else if numBitsPerValue != 0 {
		c.writeNumericValuesSingleBlock(values, numValues, numBitsPerValue, min, gcd, encode)
	}

	c.meta.WriteLong(c.data.getFilePointer() - startOffset)
	c.meta.WriteLong(jumpTableOffset)

	return numDocsWithValue, int(numValues), nil
}

func (c *lucene80DocValuesConsumer) writeNumericValuesSingleBlock(
	values NumericDocValues,
	numValues int64,
	numBitsPerValue int,
	min, gcd int64,
	encode map[int64]int) error {

	writer := packed.NewDirectWriter(c.data, numValues, numBitsPerValue)
	for doc := values.NextDoc(); doc != dvNoMoreDocs; doc = values.NextDoc() {
		for i := 0; i < values.DocValueCount(); i++ {
			v := values.NextValue()
			if encode == nil {
				writer.Add((v - min) / gcd)
			} else {
				writer.Add(int64(encode[v]))
			}
		}
	}
	return writer.Finish()
}

func (c *lucene80DocValuesConsumer) writeNumericValuesMultipleBlocks(values NumericDocValues, gcd int64) int64 {
	offsets := make([]int64, 0, 8)
	buffer := make([]int64, NumericBlockSize)
	upTo := 0

	for doc := values.NextDoc(); doc != dvNoMoreDocs; doc = values.NextDoc() {
		for i := 0; i < values.DocValueCount(); i++ {
			buffer[upTo] = values.NextValue()
			upTo++
			if upTo == NumericBlockSize {
				offsets = append(offsets, c.data.getFilePointer())
				c.writeNumericBlock(buffer, NumericBlockSize, gcd)
				upTo = 0
			}
		}
	}
	if upTo > 0 {
		offsets = append(offsets, c.data.getFilePointer())
		c.writeNumericBlock(buffer, upTo, gcd)
	}

	offsetsOrigo := c.data.getFilePointer()
	for _, off := range offsets {
		c.data.WriteLong(off)
	}
	c.data.WriteLong(offsetsOrigo)
	return offsetsOrigo
}

func (c *lucene80DocValuesConsumer) writeNumericBlock(values []int64, length int, gcd int64) {
	min := values[0]
	max := values[0]
	for i := 1; i < length; i++ {
		v := values[i]
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	if min == max {
		c.data.WriteByte(0)
		c.data.WriteLong(min)
	} else {
		bitsPerValue := packed.UnsignedBitsRequired(uint64((max - min) / gcd))

		// We use a temporary buffer for the block
		buf := new(bytes.Buffer)
		writer := packed.NewDirectWriter(buf, int64(length), bitsPerValue)
		for i := 0; i < length; i++ {
			writer.Add((values[i] - min) / gcd)
		}
		writer.Finish()

		c.data.WriteByte(byte(bitsPerValue))
		c.data.WriteLong(min)
		c.data.WriteInt(buf.Len())
		c.data.WriteBytes(buf.Bytes())
	}
}

type compressedBinaryBlockWriter struct {
	consumer                   *lucene80DocValuesConsumer
	ht                         *packed.FastCompressionHashTable
	uncompressedBlockLength    int
	maxUncompressedBlockLength int
	numDocsInCurrentBlock      int
	docLengths                 []int
	block                      []byte
	totalChunks                int
	maxPointer                 int64
	blockAddressesStart        int64
	tempBinaryOffsets          IndexOutput
}

func (c *lucene80DocValuesConsumer) newCompressedBinaryBlockWriter() (*compressedBinaryBlockWriter, error) {
	state := c.state
	tempBinaryOffsets, err := state.directory.CreateTempOutput(state.segmentInfo.name, "binary_pointers", state.context)
	if err != nil {
		return nil, err
	}

	var success bool
	defer func() {
		if !success {
			tempBinaryOffsets.Close()
		}
	}()

	err = CodecUtil.WriteIndexHeader(
		tempBinaryOffsets,
		MetaCodec+"FilePointers",
		VersionCurrent,
		state.segmentInfo.getId(),
		state.segmentSuffix)
	if err != nil {
		return nil, err
	}

	blockAddressesStart := c.data.getFilePointer()
	success = true

	return &compressedBinaryBlockWriter{
		consumer:            c,
		ht:                  packed.NewFastCompressionHashTable(),
		docLengths:          make([]int, BinaryDocsPerCompressedBlock),
		blockAddressesStart: blockAddressesStart,
		tempBinaryOffsets:   tempBinaryOffsets,
	}, nil
}

func (w *compressedBinaryBlockWriter) addDoc(doc int, v BytesRef) error {
	w.docLengths[w.numDocsInCurrentBlock] = v.length
	w.block = append(w.block, v.bytes[v.offset:v.offset+v.length]...)
	w.uncompressedBlockLength += v.length
	w.numDocsInCurrentBlock++
	if w.numDocsInCurrentBlock == BinaryDocsPerCompressedBlock {
		if err := w.flushData(); err != nil {
			return err
		}
	}
	return nil
}

func (w *compressedBinaryBlockWriter) flushData() error {
	if w.numDocsInCurrentBlock == 0 {
		return nil
	}

	w.totalChunks++
	thisBlockStartPointer := w.consumer.data.getFilePointer()

	allLengthsSame := true
	for i := 1; i < BinaryDocsPerCompressedBlock; i++ {
		if w.docLengths[i] != w.docLengths[i-1] {
			allLengthsSame = false
			break
		}
	}

	if allLengthsSame {
		onlyOneLength := (w.docLengths[0] << 1) | 1
		w.consumer.data.WriteVInt(onlyOneLength)
	} else {
		for i := 0; i < BinaryDocsPerCompressedBlock; i++ {
			if i == 0 {
				multipleLengths := (w.docLengths[0] << 1)
				w.consumer.data.WriteVInt(multipleLengths)
			} else {
				w.consumer.data.WriteVInt(w.docLengths[i])
			}
		}
	}

	if w.uncompressedBlockLength > w.maxUncompressedBlockLength {
		w.maxUncompressedBlockLength = w.uncompressedBlockLength
	}

	// LZ4 compression
	compressedLen, err := packed.LZ4Compress(w.block, w.consumer.data, w.ht)
	if err != nil {
		return err
	}

	w.numDocsInCurrentBlock = 0
	for i := range w.docLengths {
		w.docLengths[i] = 0
	}
	w.uncompressedBlockLength = 0
	w.maxPointer = w.consumer.data.getFilePointer()
	w.tempBinaryOffsets.WriteVLong(w.maxPointer - thisBlockStartPointer)

	return nil
}

func (w *compressedBinaryBlockWriter) writeMetaData() error {
	if w.totalChunks == 0 {
		return nil
	}

	startDMW := w.consumer.data.getFilePointer()
	w.consumer.meta.WriteLong(startDMW)
	w.consumer.meta.WriteVInt(w.totalChunks)
	w.consumer.meta.WriteVInt(BinaryBlockShift)
	w.consumer.meta.WriteVInt(w.maxUncompressedBlockLength)
	w.consumer.meta.WriteVInt(DirectMonotonicBlockShift)

	if err := CodecUtil.WriteFooter(w.tempBinaryOffsets); err != nil {
		return err
	}
	w.tempBinaryOffsets.Close()

	var filePointersIn IndexInput
	var err error
	filePointersIn, err = w.consumer.state.directory.OpenChecksumInput(w.tempBinaryOffsets.getName(), w.consumer.state.context)
	if err != nil {
		return err
	}
	defer filePointersIn.Close()

	err = CodecUtil.CheckIndexHeader(
		filePointersIn,
		MetaCodec+"FilePointers",
		VersionCurrent,
		VersionCurrent,
		w.consumer.state.segmentInfo.getId(),
		w.consumer.state.segmentSuffix)
	if err != nil {
		return err
	}

	var priorE error
	defer func() {
		if priorE != nil {
			CodecUtil.CheckFooter(filePointersIn, priorE)
		}
	}()

	writer := packed.NewDirectMonotonicWriter(w.consumer.meta, filePointersIn, int64(w.totalChunks), DirectMonotonicBlockShift)
	fp := w.blockAddressesStart
	for i := 0; i < w.totalChunks; i++ {
		writer.Add(fp)
		fp += filePointersIn.ReadVLong()
	}
	if w.maxPointer < fp {
		return fmt.Errorf("file pointers don't add up (%d vs expected %d)", fp, w.maxPointer)
	}
	if err := writer.Finish(); err != nil {
		return err
	}

	w.consumer.meta.WriteLong(w.consumer.data.getFilePointer() - startDMW)
	return nil
}

func (w *compressedBinaryBlockWriter) Close() error {
	if w.tempBinaryOffsets != nil {
		w.tempBinaryOffsets.Close()
		w.consumer.state.directory.DeleteFile(w.tempBinaryOffsets.getName())
	}
	return nil
}

func (c *lucene80DocValuesConsumer) AddBinaryField(field FieldInfo, valuesProducer DocValuesProducer) error {
	field.PutAttribute(ModeKey, c.mode.String())
	c.meta.WriteInt(field.number)
	c.meta.WriteByte(BinaryType)

	switch c.mode {
	case ModeBestSpeed:
		return c.doAddUncompressedBinaryField(field, valuesProducer)
	case ModeBestCompression:
		return c.doAddCompressedBinaryField(field, valuesProducer)
	default:
		return fmt.Errorf("invalid mode: %v", c.mode)
	}
}

func (c *lucene80DocValuesConsumer) doAddUncompressedBinaryField(field FieldInfo, valuesProducer DocValuesProducer) error {
	values := valuesProducer.GetBinary(field)
	start := c.data.getFilePointer()
	c.meta.WriteLong(start)

	numDocsWithField := 0
	minLength := math.MaxInt32
	maxLength := 0

	for doc := values.NextDoc(); doc != dvNoMoreDocs; doc = values.NextDoc() {
		numDocsWithField++
		v := values.BinaryValue()
		length := v.length
		c.data.WriteBytes(v.bytes[v.offset : v.offset+v.length])
		if length < minLength {
			minLength = length
		}
		if length > maxLength {
			maxLength = length
		}
	}

	c.meta.WriteLong(c.data.getFilePointer() - start)

	if numDocsWithField == 0 {
		c.meta.WriteLong(-2)
		c.meta.WriteLong(0)
		c.meta.WriteShort(-1)
		c.meta.WriteByte(-1)
	} else if numDocsWithField == c.maxDoc {
		c.meta.WriteLong(-1)
		c.meta.WriteLong(0)
		c.meta.WriteShort(-1)
		c.meta.WriteByte(-1)
	} else {
		offset := c.data.getFilePointer()
		c.meta.WriteLong(offset)
		jumpTableEntryCount := WriteBitSet(values, c.data, 4)
		c.meta.WriteLong(c.data.getFilePointer() - offset)
		c.meta.WriteShort(jumpTableEntryCount)
		c.meta.WriteByte(4)
	}

	c.meta.WriteInt(numDocsWithField)
	c.meta.WriteInt(minLength)
	c.meta.WriteInt(maxLength)

	if maxLength > minLength {
		startAddr := c.data.getFilePointer()
		c.meta.WriteLong(startAddr)
		c.meta.WriteVInt(DirectMonotonicBlockShift)

		writer := packed.NewDirectMonotonicWriter(c.meta, c.data, int64(numDocsWithField+1), DirectMonotonicBlockShift)
		var addr int64 = 0
		writer.Add(addr)

		values = valuesProducer.GetBinary(field)
		for doc := values.NextDoc(); doc != dvNoMoreDocs; doc = values.NextDoc() {
			addr += int64(values.BinaryValue().length)
			writer.Add(addr)
		}
		if err := writer.Finish(); err != nil {
			return err
		}
		c.meta.WriteLong(c.data.getFilePointer() - startAddr)
	}

	return nil
}

func (c *lucene80DocValuesConsumer) doAddCompressedBinaryField(field FieldInfo, valuesProducer DocValuesProducer) error {
	blockWriter, err := c.newCompressedBinaryBlockWriter()
	if err != nil {
		return err
	}
	defer blockWriter.Close()

	values := valuesProducer.GetBinary(field)
	start := c.data.getFilePointer()
	c.meta.WriteLong(start)

	numDocsWithField := 0
	minLength := math.MaxInt32
	maxLength := 0

	for doc := values.NextDoc(); doc != dvNoMoreDocs; doc = values.NextDoc() {
		numDocsWithField++
		v := values.BinaryValue()
		if err := blockWriter.addDoc(doc, v); err != nil {
			return err
		}
		length := v.length
		if length < minLength {
			minLength = length
		}
		if length > maxLength {
			maxLength = length
		}
	}

	if err := blockWriter.flushData(); err != nil {
		return err
	}

	c.meta.WriteLong(c.data.getFilePointer() - start)

	if numDocsWithField == 0 {
		c.meta.WriteLong(-2)
		c.meta.WriteLong(0)
		c.meta.WriteShort(-1)
		c.meta.WriteByte(-1)
	} else if numDocsWithField == c.maxDoc {
		c.meta.WriteLong(-1)
		c.meta.WriteLong(0)
		c.meta.WriteShort(-1)
		c.meta.WriteByte(-1)
	} else {
		offset := c.data.getFilePointer()
		c.meta.WriteLong(offset)
		values = valuesProducer.GetBinary(field)
		jumpTableEntryCount := WriteBitSet(values, c.data, 4)
		c.meta.WriteLong(c.data.getFilePointer() - offset)
		c.meta.WriteShort(jumpTableEntryCount)
		c.meta.WriteByte(4)
	}

	c.meta.WriteInt(numDocsWithField)
	c.meta.WriteInt(minLength)
	c.meta.WriteInt(maxLength)

	if err := blockWriter.writeMetaData(); err != nil {
		return err
	}

	return nil
}

func (c *lucene80DocValuesConsumer) AddSortedField(field FieldInfo, valuesProducer DocValuesProducer) error {
	c.meta.WriteInt(field.number)
	c.meta.WriteByte(SortedType)
	return c.doAddSortedField(field, valuesProducer)
}

func (c *lucene80DocValuesConsumer) doAddSortedField(field FieldInfo, valuesProducer DocValuesProducer) error {
	values := valuesProducer.GetSorted(field)
	numDocsWithField := 0
	for doc := values.NextDoc(); doc != dvNoMoreDocs; doc = values.NextDoc() {
		numDocsWithField++
	}

	if numDocsWithField == 0 {
		c.meta.WriteLong(-2)
		c.meta.WriteLong(0)
		c.meta.WriteShort(-1)
		c.meta.WriteByte(-1)
	} else if numDocsWithField == c.maxDoc {
		c.meta.WriteLong(-1)
		c.meta.WriteLong(0)
		c.meta.WriteShort(-1)
		c.meta.WriteByte(-1)
	} else {
		offset := c.data.getFilePointer()
		c.meta.WriteLong(offset)
		values = valuesProducer.GetSorted(field)
		jumpTableEntryCount := WriteBitSet(values, c.data, 4)
		c.meta.WriteLong(c.data.getFilePointer() - offset)
		c.meta.WriteShort(jumpTableEntryCount)
		c.meta.WriteByte(4)
	}

	c.meta.WriteInt(numDocsWithField)
	if values.GetValueCount() <= 1 {
		c.meta.WriteByte(0)
		c.meta.WriteLong(0)
		c.meta.WriteLong(0)
	} else {
		numberOfBitsPerOrd := packed.UnsignedBitsRequired(uint64(values.GetValueCount() - 1))
		c.meta.WriteByte(byte(numberOfBitsPerOrd))
		start := c.data.getFilePointer()
		c.meta.WriteLong(start)

		writer := packed.NewDirectWriter(c.data, int64(numDocsWithField), numberOfBitsPerOrd)
		values = valuesProducer.GetSorted(field)
		for doc := values.NextDoc(); doc != dvNoMoreDocs; doc = values.NextDoc() {
			writer.Add(int64(values.OrdValue()))
		}
		if err := writer.Finish(); err != nil {
			return err
		}
		c.meta.WriteLong(c.data.getFilePointer() - start)
	}

	// Wrap sorted set for terms dict
	return c.addTermsDict(values)
}

func (c *lucene80DocValuesConsumer) addTermsDict(values SortedSetDocValues) error {
	size := values.GetValueCount()
	c.meta.WriteVLong(size)

	compress := c.mode == ModeBestCompression && size > TermsDictBlockCompressionThreshold
	var code, blockMask, shift int
	if compress {
		code = TermsDictBlockLZ4Code
		blockMask = TermsDictBlockLZ4Mask
		shift = TermsDictBlockLZ4Shift
	} else {
		code = TermsDictBlockShift
		shift = TermsDictBlockShift
		blockMask = TermsDictBlockMask
	}

	c.meta.WriteInt(code)
	c.meta.WriteInt(DirectMonotonicBlockShift)

	addressBuf := new(bytes.Buffer)
	// Use a dummy IndexOutput for the writer to write into addressBuf
	addressOutput := &bufferIndexOutput{buf: addressBuf}

	numBlocks := int64(uint64(size+int64(blockMask)) >> uint(shift))
	writer := packed.NewDirectMonotonicWriter(c.meta, addressOutput, numBlocks, DirectMonotonicBlockShift)

	var previous BytesRef
	var ord int64 = 0
	start := c.data.getFilePointer()
	maxLength, maxBlockLength := 0, 0
	iterator := values.TermsEnum()

	var ht *packed.FastCompressionHashTable
	var bufferedOutput *bytes.Buffer
	if compress {
		ht = packed.NewFastCompressionHashTable()
		bufferedOutput = new(bytes.Buffer)
	}

	for term := iterator.Next(); term != nil; term = iterator.Next() {
		if (ord & int64(blockMask)) == 0 {
			if compress && bufferedOutput.Len() > 0 {
				blockLen := c.compressAndGetTermsDictBlockLength(bufferedOutput, ht)
				if blockLen > maxBlockLength {
					maxBlockLength = blockLen
				}
				bufferedOutput.Reset()
			}

			writer.Add(c.data.getFilePointer() - start)
			c.data.WriteVInt(term.length)
			c.data.WriteBytes(term.bytes[term.offset : term.offset+term.length])
		} else {
			prefixLen := bytesDifference(previous, term)
			suffixLen := term.length - prefixLen

			var blockOut io.Writer
			if compress {
				blockOut = bufferedOutput
			} else {
				blockOut = c.data
			}

			token := byte(minInt(prefixLen, 15) | (minInt(15, suffixLen-1) << 4))
			blockOut.Write([]byte{token})

			if prefixLen >= 15 {
				writeVInt(blockOut, prefixLen-15)
			}
			if suffixLen >= 16 {
				writeVInt(blockOut, suffixLen-16)
			}
			blockOut.Write(term.bytes[term.offset+prefixLen : term.offset+prefixLen+suffixLen])
		}
		if term.length > maxLength {
			maxLength = term.length
		}
		previous = term
		ord++
	}

	if compress && bufferedOutput.Len() > 0 {
		blockLen := c.compressAndGetTermsDictBlockLength(bufferedOutput, ht)
		if blockLen > maxBlockLength {
			maxBlockLength = blockLen
		}
	}

	if err := writer.Finish(); err != nil {
		return err
	}

	c.meta.WriteInt(maxLength)
	if compress {
		c.meta.WriteInt(maxBlockLength)
	}
	c.meta.WriteLong(start)
	c.meta.WriteLong(c.data.getFilePointer() - start)

	startAddr := c.data.getFilePointer()
	c.data.WriteBytes(addressBuf.Bytes())
	c.meta.WriteLong(startAddr)
	c.meta.WriteLong(c.data.getFilePointer() - startAddr)

	return c.writeTermsIndex(values)
}

func (c *lucene80DocValuesConsumer) compressAndGetTermsDictBlockLength(buf *bytes.Buffer, ht *packed.FastCompressionHashTable) int {
	uncompressedLength := buf.Len()
	c.data.WriteVInt(uncompressedLength)
	before := c.data.getFilePointer()

	compressedLen, err := packed.LZ4Compress(buf.Bytes(), c.data, ht)
	if err != nil {
		panic(err)
	}

	compressedLength := int(c.data.getFilePointer() - before)
	if uncompressedLength > compressedLength {
		return uncompressedLength
	}
	return compressedLength
}

func (c *lucene80DocValuesConsumer) writeTermsIndex(values SortedSetDocValues) error {
	size := values.GetValueCount()
	c.meta.WriteInt(TermsDictReverseIndexShift)
	start := c.data.getFilePointer()

	numBlocks := 1 + int64(uint64(size+int64(TermsDictReverseIndexMask))>>uint(TermsDictReverseIndexShift))

	addressBuf := new(bytes.Buffer)
	addressOutput := &bufferIndexOutput{buf: addressBuf}

	writer := packed.NewDirectMonotonicWriter(c.meta, addressOutput, numBlocks, DirectMonotonicBlockShift)

	iterator := values.TermsEnum()
	var previous BytesRef
	var offset int64 = 0
	var ord int64 = 0

	for term := iterator.Next(); term != nil; term = iterator.Next() {
		if (ord & int64(TermsDictReverseIndexMask)) == 0 {
			writer.Add(offset)
			var sortKeyLen int
			if ord == 0 {
				sortKeyLen = 0
			} else {
				sortKeyLen = sortKeyLength(previous, term)
			}
			offset += int64(sortKeyLen)
			c.data.WriteBytes(term.bytes[term.offset : term.offset+sortKeyLen])
		} else if (ord & int64(TermsDictReverseIndexMask)) == int64(TermsDictReverseIndexMask) {
			previous = term
		}
		ord++
	}
	writer.Add(offset)
	if err := writer.Finish(); err != nil {
		return err
	}

	c.meta.WriteLong(start)
	c.meta.WriteLong(c.data.getFilePointer() - start)

	startAddr := c.data.getFilePointer()
	c.data.WriteBytes(addressBuf.Bytes())
	c.meta.WriteLong(startAddr)
	c.meta.WriteLong(c.data.getFilePointer() - startAddr)

	return nil
}

func (c *lucene80DocValuesConsumer) AddSortedNumericField(field FieldInfo, valuesProducer DocValuesProducer) error {
	c.meta.WriteInt(field.number)
	c.meta.WriteByte(SortedNumericType)

	values := valuesProducer.GetSortedNumeric(field)
	numDocsWithValue, numValues, err := c.writeNumericValues(field, values)
	if err != nil {
		return err
	}

	c.meta.WriteInt(numDocsWithValue)
	if numValues > int64(numDocsWithValue) {
		start := c.data.getFilePointer()
		c.meta.WriteLong(start)
		c.meta.WriteVInt(DirectMonotonicBlockShift)

		writer := packed.NewDirectMonotonicWriter(c.meta, c.data, int64(numDocsWithValue+1), DirectMonotonicBlockShift)
		var addr int64 = 0
		writer.Add(addr)

		values = valuesProducer.GetSortedNumeric(field)
		for doc := values.NextDoc(); doc != dvNoMoreDocs; doc = values.NextDoc() {
			addr += int64(values.DocValueCount())
			writer.Add(addr)
		}
		if err := writer.Finish(); err != nil {
			return err
		}
		c.meta.WriteLong(c.data.getFilePointer() - start)
	}

	return nil
}

func (c *lucene80DocValuesConsumer) AddSortedSetField(field FieldInfo, valuesProducer DocValuesProducer) error {
	c.meta.WriteInt(field.number)
	c.meta.WriteByte(SortedSetType)

	values := valuesProducer.GetSortedSet(field)
	numDocsWithField := 0
	var numOrds int64 = 0
	for doc := values.NextDoc(); doc != dvNoMoreDocs; doc = values.NextDoc() {
		numDocsWithField++
		numOrds += int64(values.DocValueCount())
	}

	if numDocsWithField == int(numOrds) {
		c.meta.WriteByte(0) // singleValued
		// We need to wrap it as sorted
		return c.doAddSortedField(field, valuesProducer)
	}

	c.meta.WriteByte(1) // multiValued

	if numDocsWithField == 0 {
		c.meta.WriteLong(-2)
		c.meta.WriteLong(0)
		c.meta.WriteShort(-1)
		c.meta.WriteByte(-1)
	} else if numDocsWithField == c.maxDoc {
		c.meta.WriteLong(-1)
		c.meta.WriteLong(0)
		c.meta.WriteShort(-1)
		c.meta.WriteByte(-1)
	} else {
		offset := c.data.getFilePointer()
		c.meta.WriteLong(offset)
		values = valuesProducer.GetSortedSet(field)
		jumpTableEntryCount := WriteBitSet(values, c.data, 4)
		c.meta.WriteLong(c.data.getFilePointer() - offset)
		c.meta.WriteShort(jumpTableEntryCount)
		c.meta.WriteByte(4)
	}

	numberOfBitsPerOrd := packed.UnsignedBitsRequired(uint64(values.GetValueCount() - 1))
	c.meta.WriteByte(byte(numberOfBitsPerOrd))
	start := c.data.getFilePointer()
	c.meta.WriteLong(start)

	writer := packed.NewDirectWriter(c.data, numOrds, numberOfBitsPerOrd)
	values = valuesProducer.GetSortedSet(field)
	for doc := values.NextDoc(); doc != dvNoMoreDocs; doc = values.NextDoc() {
		for i := 0; i < values.DocValueCount(); i++ {
			writer.Add(int64(values.NextOrd()))
		}
	}
	if err := writer.Finish(); err != nil {
		return err
	}
	c.meta.WriteLong(c.data.getFilePointer() - start)

	c.meta.WriteInt(numDocsWithField)
	startAddr := c.data.getFilePointer()
	c.meta.WriteLong(startAddr)
	c.meta.WriteVInt(DirectMonotonicBlockShift)

	addressesWriter := packed.NewDirectMonotonicWriter(c.meta, c.data, int64(numDocsWithField+1), DirectMonotonicBlockShift)
	var addr int64 = 0
	addressesWriter.Add(addr)
	values = valuesProducer.GetSortedSet(field)
	for doc := values.NextDoc(); doc != dvNoMoreDocs; doc = values.NextDoc() {
		addr += int64(values.DocValueCount())
		addressesWriter.Add(addr)
	}
	if err := addressesWriter.Finish(); err != nil {
		return err
	}
	c.meta.WriteLong(c.data.getFilePointer() - startAddr)

	return c.addTermsDict(values)
}

// Helpers

func gcdFunc(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func bytesDifference(a, b BytesRef) int {
	if a == nil {
		return 0
	}
	n := minInt(a.length, b.length)
	for i := 0; i < n; i++ {
		if a.bytes[a.offset+i] != b.bytes[b.offset+i] {
			return i
		}
	}
	return n
}

func sortKeyLength(a, b BytesRef) int {
	// Mimics Lucene's StringHelper.sortKeyLength
	n := minInt(a.length, b.length)
	for i := 0; i < n; i++ {
		if a.bytes[a.offset+i] != b.bytes[b.offset+i] {
			return i + 1
		}
	}
	return n
}

func writeVInt(w io.Writer, v int) {
	for v >= 0x80 {
		w.Write([]byte{byte(v | 0x80)})
		v >>= 7
	}
	w.Write([]byte{byte(v)})
}

type bufferIndexOutput struct {
	buf *bytes.Buffer
}

func (b *bufferIndexOutput) WriteByte(v byte) error { return b.buf.WriteByte(v) }
func (b *bufferIndexOutput) WriteInt(v int) error {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(v))
	_, err := b.buf.Write(buf)
	return err
}
func (b *bufferIndexOutput) WriteLong(v int64) error {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(v))
	_, err := b.buf.Write(buf)
	return err
}
func (b *bufferIndexOutput) WriteShort(v int16) error {
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, uint16(v))
	_, err := b.buf.Write(buf)
	return err
}
func (b *bufferIndexOutput) WriteByteValue(v byte) error { return b.buf.WriteByte(v) }
func (b *bufferIndexOutput) WriteVInt(v int) error {
	for v >= 0x80 {
		b.buf.WriteByte(byte(v | 0x80))
		v >>= 7
	}
	b.buf.WriteByte(byte(v))
	return nil
}
func (b *bufferIndexOutput) WriteVLong(v int64) error {
	for v >= 0x80 {
		b.buf.WriteByte(byte(v | 0x80))
		v >>= 7
	}
	b.buf.WriteByte(byte(v))
	return nil
}
func (b *bufferIndexOutput) WriteBytes(p []byte) error {
	_, err := b.buf.Write(p)
	return err
}
func (b *bufferIndexOutput) Close() error          { return nil }
func (b *bufferIndexOutput) getFilePointer() int64 { return int64(b.buf.Len()) }

// Need to import encoding/binary for bufferIndexOutput
