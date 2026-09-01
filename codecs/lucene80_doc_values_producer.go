// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"io"
	"math"

	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

type lucene80DocValuesProducer struct {
	numerics     map[int]*numericEntry
	binaries     map[int]*binaryEntry
	sorted       map[int]*sortedEntry
	sortedSets   map[int]*sortedSetEntry
	sortedNumerics map[int]*sortedNumericEntry
	data         IndexInput
	maxDoc       int
	version      int
}

func NewLucene80DocValuesProducer(
	state SegmentReadState,
	dataCodec, dataExtension, metaCodec, metaExtension string) (DocValuesProducer, error) {

	metaName := IndexFileNames.segmentFileName(state.segmentInfo.name, state.segmentSuffix, metaExtension)
	maxDoc := state.segmentInfo.maxDoc()

	var version int
	var err error

	// We use a temporary variable for the meta reader to close it after reading fields
	meta, err := state.directory.OpenChecksumInput(metaName, state.context)
	if err != nil {
		return nil, err
	}
	defer meta.Close()

	version, err = CodecUtil.CheckIndexHeader(
		meta,
		metaCodec,
		VersionStart,
		VersionCurrent,
		state.segmentInfo.getId(),
		state.segmentSuffix)
	if err != nil {
		return nil, err
	}

	producer := &lucene80DocValuesProducer{
		numerics:       make(map[int]*numericEntry),
		binaries:       make(map[int]*binaryEntry),
		sorted:         make(map[int]*sortedEntry),
		sortedSets:     make(map[int]*sortedSetEntry),
		sortedNumerics: make(map[int]*sortedNumericEntry),
		maxDoc:         maxDoc,
		version:        version,
	}

	if err := producer.readFields(state.segmentInfo.name, meta, state.fieldInfos); err != nil {
		return nil, err
	}

	dataName := IndexFileNames.segmentFileName(state.segmentInfo.name, state.segmentSuffix, dataExtension)
	data, err := state.directory.OpenInput(dataName, state.context)
	if err != nil {
		return nil, err
	}
	producer.data = data

	if err := CodecUtil.CheckIndexHeader(
		data,
		dataCodec,
		VersionStart,
		VersionCurrent,
		state.segmentInfo.getId(),
		state.segmentSuffix); err != nil {
		data.Close()
		return nil, err
	}

	if err := CodecUtil.RetrieveChecksum(data); err != nil {
		data.Close()
		return nil, err
	}

	return producer, nil
}

func (p *lucene80DocValuesProducer) readFields(segmentName string, meta IndexInput, infos FieldInfos) error {
	for {
		fieldNumber, err := meta.ReadInt()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if fieldNumber == -1 {
			break
		}

		info := infos.FieldInfo(fieldNumber)
		if info == nil {
			return fmt.Errorf("invalid field number: %d", fieldNumber)
		}

		dataType, err := meta.ReadByte()
		if err != nil {
			return err
		}

		switch dataType {
		case NumericType:
			entry, err := p.readNumeric(meta)
			if err != nil {
				return err
			}
			p.numerics[info.number] = entry
		case BinaryType:
			compressed, err := p.readBinaryMeta(meta, info, segmentName)
			if err != nil {
				return err
			}
			entry, err := p.readBinary(meta, compressed)
			if err != nil {
				return err
			}
			p.binaries[info.number] = entry
		case SortedType:
			entry, err := p.readSorted(meta)
			if err != nil {
				return err
			}
			p.sorted[info.number] = entry
		case SortedSetType:
			entry, err := p.readSortedSet(meta)
			if err != nil {
				return err
			}
			p.sortedSets[info.number] = entry
		case SortedNumericType:
			entry, err := p.readSortedNumeric(meta)
			if err != nil {
				return err
			}
			p.sortedNumerics[info.number] = entry
		default:
			return fmt.Errorf("invalid type: %d", dataType)
		}
	}
	return nil
}

func (p *lucene80DocValuesProducer) readNumeric(meta IndexInput) (*numericEntry, error) {
	entry := &numericEntry{}
	entry.docsWithFieldOffset = meta.ReadLong()
	entry.docsWithFieldLength = meta.ReadLong()
	entry.jumpTableEntryCount = meta.ReadShort()
	entry.denseRankPower = meta.ReadByte()
	entry.numValues = meta.ReadLong()

	tableSize := meta.ReadInt()
	if tableSize > 256 {
		return nil, fmt.Errorf("invalid table size: %d", tableSize)
	}
	if tableSize >= 0 {
		entry.table = make([]int64, tableSize)
		for i := 0; i < tableSize; i++ {
			entry.table[i] = meta.ReadLong()
		}
	}

	if tableSize < -1 {
		entry.blockShift = -2 - tableSize
	} else {
		entry.blockShift = -1
	}

	entry.bitsPerValue = meta.ReadByte()
	entry.minValue = meta.ReadLong()
	entry.gcd = meta.ReadLong()
	entry.valuesOffset = meta.ReadLong()
	entry.valuesLength = meta.ReadLong()
	entry.valueJumpTableOffset = meta.ReadLong()

	return entry, nil
}

func (p *lucene80DocValuesProducer) readBinaryMeta(meta IndexInput, info FieldInfo, segmentName string) (bool, error) {
	var compressed bool
	if p.version >= VersionConfigurableCompression {
		value := info.GetAttribute(ModeKey)
		if value == "" {
			return false, fmt.Errorf("missing value for %s for field: %s in segment: %s", ModeKey, info.name, segmentName)
		}
		mode := Mode(0) // Default to BestSpeed
		if value == "ModeBestCompression" {
			mode = ModeBestCompression
		}
		compressed = (mode == ModeBestCompression)
	} else {
		compressed = p.version >= VersionBinCompressed
	}
	return compressed, nil
}

func (p *lucene80DocValuesProducer) readBinary(meta IndexInput, compressed bool) (*binaryEntry, error) {
	entry := &binaryEntry{compressed: compressed}
	entry.dataOffset = meta.ReadLong()
	entry.dataLength = meta.ReadLong()
	entry.docsWithFieldOffset = meta.ReadLong()
	entry.docsWithFieldLength = meta.ReadLong()
	entry.jumpTableEntryCount = meta.ReadShort()
	entry.denseRankPower = meta.ReadByte()
	entry.numDocsWithField = meta.ReadInt()
	entry.minLength = meta.ReadInt()
	entry.maxLength = meta.ReadInt()

	if (compressed && entry.numDocsWithField > 0) || entry.minLength < entry.maxLength {
		entry.addressesOffset = meta.ReadLong()
		var numAddresses int64
		if compressed {
			entry.numCompressedChunks = meta.ReadVInt()
			entry.docsPerChunkShift = meta.ReadVInt()
			entry.maxUncompressedChunkSize = meta.ReadVInt()
			numAddresses = int64(entry.numCompressedChunks)
		} else {
			numAddresses = int64(entry.numDocsWithField + 1)
		}

		blockShift := meta.ReadVInt()
		entry.addressesMeta = packed.NewDirectMonotonicReader(meta, numAddresses, blockShift)
		entry.addressesLength = meta.ReadLong()
	}

	return entry, nil
}

func (p *lucene80DocValuesProducer) readSorted(meta IndexInput) (*sortedEntry, error) {
	entry := &sortedEntry{}
	entry.docsWithFieldOffset = meta.ReadLong()
	entry.docsWithFieldLength = meta.ReadLong()
	entry.jumpTableEntryCount = meta.ReadShort()
	entry.denseRankPower = meta.ReadByte()
	entry.numDocsWithField = meta.ReadInt()
	entry.bitsPerValue = meta.ReadByte()
	entry.ordsOffset = meta.ReadLong()
	entry.ordsLength = meta.ReadLong()

	if err := p.readTermDict(meta, entry); err != nil {
		return nil, err
	}
	return entry, nil
}

func (p *lucene80DocValuesProducer) readSortedSet(meta IndexInput) (*sortedSetEntry, error) {
	entry := &sortedSetEntry{}
	multiValued := meta.ReadByte()
	switch multiValued {
	case 0:
		single, err := p.readSorted(meta)
		if err != nil {
			return nil, err
		}
		entry.singleValueEntry = single
		return entry, nil
	case 1:
		// multi-valued
	default:
		return nil, fmt.Errorf("invalid multiValued flag: %d", multiValued)
	}

	entry.docsWithFieldOffset = meta.ReadLong()
	entry.docsWithFieldLength = meta.ReadLong()
	entry.jumpTableEntryCount = meta.ReadShort()
	entry.denseRankPower = meta.ReadByte()
	entry.bitsPerValue = meta.ReadByte()
	entry.ordsOffset = meta.ReadLong()
	entry.ordsLength = meta.ReadLong()
	entry.numDocsWithField = meta.ReadInt()
	entry.addressesOffset = meta.ReadLong()

	blockShift := meta.ReadVInt()
	entry.addressesMeta = packed.NewDirectMonotonicReader(meta, int64(entry.numDocsWithField+1), blockShift)
	entry.addressesLength = meta.ReadLong()

	if err := p.readTermDict(meta, entry); err != nil {
		return nil, err
	}
	return entry, nil
}

func (p *lucene80DocValuesProducer) readTermDict(meta IndexInput, entry TermsDictEntry) error {
	entry.termsDictSize = meta.ReadVLong()
	termsDictBlockCode := meta.ReadInt()
	if termsDictBlockCode == TermsDictBlockLZ4Code {
		entry.compressed = true
		entry.termsDictBlockShift = TermsDictBlockLZ4Shift
	} else {
		entry.termsDictBlockShift = termsDictBlockCode
	}

	blockShift := meta.ReadInt()
	addressesSize := (entry.termsDictSize + int64(1<<entry.termsDictBlockShift) - 1) >>> uint(entry.termsDictBlockShift)
	entry.termsAddressesMeta = packed.NewDirectMonotonicReader(meta, addressesSize, blockShift)
	entry.maxTermLength = meta.ReadInt()

	if entry.compressed {
		entry.maxBlockLength = meta.ReadInt()
	}

	entry.termsDataOffset = meta.ReadLong()
	entry.termsDataLength = meta.ReadLong()
	entry.termsAddressesOffset = meta.ReadLong()
	entry.termsAddressesLength = meta.ReadLong()
	entry.termsDictIndexShift = meta.ReadInt()

	indexSize := (entry.termsDictSize + int64(1<<entry.termsDictIndexShift) - 1) >>> uint(entry.termsDictIndexShift)
	entry.termsIndexAddressesMeta = packed.NewDirectMonotonicReader(meta, 1+indexSize, blockShift)
	entry.termsIndexOffset = meta.ReadLong()
	entry.termsIndexLength = meta.ReadLong()
	entry.termsIndexAddressesOffset = meta.ReadLong()
	entry.termsIndexAddressesLength = meta.ReadLong()

	return nil
}

func (p *lucene80DocValuesProducer) readSortedNumeric(meta IndexInput) (*sortedNumericEntry, error) {
	entry := &sortedNumericEntry{}
	// We need to reuse readNumeric
	pNumeric := &numericEntry{}
	pNumeric.docsWithFieldOffset = meta.ReadLong()
	pNumeric.docsWithFieldLength = meta.ReadLong()
	pNumeric.jumpTableEntryCount = meta.ReadShort()
	pNumeric.denseRankPower = meta.ReadByte()
	pNumeric.numValues = meta.ReadLong()

	tableSize := meta.ReadInt()
	if tableSize > 256 {
		return nil, fmt.Errorf("invalid table size: %d", tableSize)
	}
	if tableSize >= 0 {
		pNumeric.table = make([]int64, tableSize)
		for i := 0; i < tableSize; i++ {
			pNumeric.table[i] = meta.ReadLong()
		}
	}
	if tableSize < -1 {
		pNumeric.blockShift = -2 - tableSize
	} else {
		pNumeric.blockShift = -1
	}
	pNumeric.bitsPerValue = meta.ReadByte()
	pNumeric.minValue = meta.ReadLong()
	pNumeric.gcd = meta.ReadLong()
	pNumeric.valuesOffset = meta.ReadLong()
	pNumeric.valuesLength = meta.ReadLong()
	pNumeric.valueJumpTableOffset = meta.ReadLong()

	entry.numDocsWithField = meta.ReadInt()
	if entry.numDocsWithField != int(pNumeric.numValues) {
		entry.addressesOffset = meta.ReadLong()
		blockShift := meta.ReadVInt()
		entry.addressesMeta = packed.NewDirectMonotonicReader(meta, int64(entry.numDocsWithField+1), blockShift)
		entry.addressesLength = meta.ReadLong()
	}

	// Copy basic numeric fields to sorted numeric entry
	entry.docsWithFieldOffset = pNumeric.docsWithFieldOffset
	entry.docsWithFieldLength = pNumeric.docsWithFieldLength
	entry.jumpTableEntryCount = pNumeric.jumpTableEntryCount
	entry.denseRankPower = pNumeric.denseRankPower
	entry.numValues = pNumeric.numValues
	entry.minValue = pNumeric.minValue
	entry.gcd = pNumeric.gcd
	entry.valuesOffset = pNumeric.valuesOffset
	entry.valuesLength = pNumeric.valuesLength
	entry.valueJumpTableOffset = pNumeric.valueJumpTableOffset
	entry.bitsPerValue = pNumeric.bitsPerValue
	entry.blockShift = pNumeric.blockShift
	entry.table = pNumeric.table

	return entry, nil
}

func (p *lucene80DocValuesProducer) Close() error {
	if p.data != nil {
		return p.data.Close()
	}
	return nil
}

func (p *lucene80DocValuesProducer) GetNumeric(field FieldInfo) NumericDocValues {
	entry := p.numerics[field.number]
	if entry == nil {
		return DocValues.EmptyNumeric()
	}
	return p.getNumeric(entry)
}

func (p *lucene80DocValuesProducer) getNumeric(entry *numericEntry) NumericDocValues {
	if entry.docsWithFieldOffset == -2 {
		return DocValues.EmptyNumeric()
	} else if entry.docsWithFieldOffset == -1 {
		if entry.bitsPerValue == 0 {
			return &denseNumericDocValues{
				maxDoc: p.maxDoc,
				entry:  entry,
			}
		} else {
			slice := p.data.RandomAccessSlice(entry.valuesOffset, entry.valuesLength)
			if entry.blockShift >= 0 {
				return &denseNumericVaryingBPV{
					maxDoc: p.maxDoc,
					entry:  entry,
					slice:  slice,
				}
			} else {
				values := packed.NewDirectReader(slice, int(entry.bitsPerValue))
				if entry.table != nil {
					return &denseNumericTable{
						maxDoc: p.maxDoc,
						values: values,
						table:  entry.table,
					}
				} else {
					return &denseNumericDelta{
						maxDoc: p.maxDoc,
						values: values,
						mul:    entry.gcd,
						delta:  entry.minValue,
					}
				}
			}
		}
	} else {
		disi := NewIndexedDISI(p.data, entry.docsWithFieldOffset, entry.docsWithFieldLength, entry.jumpTableEntryCount, entry.denseRankPower, entry.numValues)
		if entry.bitsPerValue == 0 {
			return &sparseNumericDocValues{
				disi:  disi,
				entry: entry,
			}
		} else {
			slice := p.data.RandomAccessSlice(entry.valuesOffset, entry.valuesLength)
			if entry.blockShift >= 0 {
				return &sparseNumericVaryingBPV{
					disi:  disi,
					entry:  entry,
					slice:  slice,
				}
			} else {
				values := packed.NewDirectReader(slice, int(entry.bitsPerValue))
				if entry.table != nil {
					return &sparseNumericTable{
						disi:   disi,
						values: values,
						table:  entry.table,
					}
				} else {
					return &sparseNumericDelta{
						disi:   disi,
						values: values,
						mul:    entry.gcd,
						delta:  entry.minValue,
					}
				}
			}
		}
	}
}

type denseNumericDocValues struct {
	maxDoc int
	entry  *numericEntry
}

func (d *denseNumericdocValues) DocID() int { return d.doc }
func (d *denseNumericdocValues) NextDoc() int { return d.advance(d.doc + 1) }
func (d *denseNumericdocValues) Advance(target int) int {
	if target >= d.maxDoc {
		return d.doc = DocIdSetIterator.NoMoreDocs
	}
	return d.doc = target
}
func (d *denseNumericdocValues) AdvanceExact(target int) bool {
	d.doc = target
	return true
}
func (d *denseNumericdocValues) Cost() int64 { return int64(d.maxDoc) }
func (d *denseNumericdocValues) LongValue() int64 { return d.entry.minValue }

type denseNumericVaryingBPV struct {
	maxDoc int
	entry  *numericEntry
	slice  RandomAccessInput
	doc    int
}

func (d *denseNumericVaryingBPV) DocID() int { return d.doc }
func (d *denseNumericVaryingBPV) NextDoc() int { return d.advance(d.doc + 1) }
func (d *denseNumericVaryingBPV) Advance(target int) int {
	if target >= d.maxDoc {
		return d.doc = DocIdSetIterator.NoMoreDocs
	}
	return d.doc = target
}
func (d *denseNumericVaryingBPV) AdvanceExact(target int) bool {
	d.doc = target
	return true
}
func (d *denseNumericVaryingBPV) Cost() int64 { return int64(d.maxDoc) }
func (d *denseNumericVaryingBPV) LongValue() int64 {
	return d.getLongValue(d.doc)
}

func (d *denseNumericVaryingBPV) getLongValue(index int) int64 {
	// Simplified implementation of VaryingBPVReader
	// In a real port, we'd implement the full logic
	return 0
}

type denseNumericTable struct {
	maxDoc int
	values packed.DirectReader
	table  []int64
	doc    int
}

func (d *denseNumericTable) DocID() int { return d.doc }
func (d *denseNumericTable) NextDoc() int { return d.advance(d.doc + 1) }
func (d *denseNumericTable) Advance(target int) int {
	if target >= d.maxDoc {
		return d.doc = DocIdSetIterator.NoMoreDocs
	}
	return d.doc = target
}
func (d *denseNumericTable) AdvanceExact(target int) bool {
	d.doc = target
	return true
}
func (d *denseNumericTable) Cost() int64 { return int64(d.maxDoc) }
func (d *denseNumericTable) LongValue() int64 {
	return d.table[d.values.Get(int64(d.doc))]
}

type denseNumericDelta struct {
	maxDoc int
	values packed.DirectReader
	mul    int64
	delta  int64
	doc    int
}

func (d *denseNumericDelta) DocID() int { return d.doc }
func (d *denseNumericDelta) NextDoc() int { return d.advance(d.doc + 1) }
func (d *denseNumericDelta) Advance(target int) int {
	if target >= d.maxDoc {
		return d.doc = DocIdSetIterator.NoMoreDocs
	}
	return d.doc = target
}
func (d *denseNumericDelta) AdvanceExact(target int) bool {
	d.doc = target
	return true
}
func (d *denseNumericDelta) Cost() int64 { return int64(d.maxDoc) }
func (d *denseNumericDelta) LongValue() int64 {
	return d.mul*d.values.Get(int64(d.doc)) + d.delta
}

type sparseNumericDocValues struct {
	disi  *IndexedDISI
	entry *numericEntry
}

func (s *sparseNumericDocValues) DocID() int { return s.disi.DocID() }
func (s *sparseNumericDocValues) NextDoc() int { return s.disi.NextDoc() }
func (s *sparseNumericDocValues) Advance(target int) int { return s.disi.Advance(target) }
func (s *sparseNumericDocValues) AdvanceExact(target int) bool { return s.disi.AdvanceExact(target) }
func (s *sparseNumericDocValues) Cost() int64 { return s.disi.Cost() }
func (s *sparseNumericDocValues) LongValue() int64 { return s.entry.minValue }

type sparseNumericVaryingBPV struct {
	disi  *IndexedDISI
	entry *numericEntry
	slice  RandomAccessInput
}

func (s *sparseNumericVaryingBPV) DocID() int { return s.disi.DocID() }
func (s *sparseNumericVaryingBPV) NextDoc() int { return s.disi.NextDoc() }
func (s *sparseNumericVaryingBPV) Advance(int) int { return s.disi.Advance(int) }
func (s *sparseNumericVaryingBPV) AdvanceExact(int) bool { return s.disi.AdvanceExact(int) }
func (s *sparseNumericVaryingBPV) Cost() int64 { return s.disi.Cost() }
func (s *sparseNumericVaryingBPV) LongValue() int64 {
	return 0 // Simplified
}

type sparseNumericTable struct {
	disi   *IndexedDISI
	values packed.DirectReader
	table  []int64
}

func (s *sparseNumericTable) DocID() int { return s.disi.DocID() }
func (s *sparseNumericTable) NextDoc() int { return s.disi.NextDoc() }
func (s *sparseNumericTable) Advance(target int) int { return s.disi.Advance(target) }
func (s *sparseNumericTable) AdvanceExact(target int) bool { return s.disi.AdvanceExact(target) }
func (s *sparseNumericTable) Cost() int64 { return s.disi.Cost() }
func (s *sparseNumericTable) LongValue() int64 {
	return s.table[s.values.Get(int64(s.disi.Index()))]
}

type sparseNumericDelta struct {
	disi   *IndexedDISI
	values packed.DirectReader
	mul    int64
	delta  int64
}

func (s *sparseNumericDelta) DocID() int { return s.disi.DocID() }
func (s *sparseNumericDelta) NextDoc() int { return s.disi.NextDoc() }
func (s *sparseNumericDelta) Advance(target int) int { return s.disi.Advance(target) }
func (s *sparseNumericDelta) AdvanceExact(target int) bool { return s.disi.AdvanceExact(target) }
func (s *sparseNumericDelta) Cost() int64 { return s.disi.Cost() }
func (s *sparseNumericDelta) LongValue() int64 {
	return s.mul*s.values.Get(int64(s.disi.Index())) + s.delta
}

func (p *lucene80DocValuesProducer) GetBinary(field FieldInfo) BinaryDocValues {
	entry := p.binaries[field.number]
	if entry == nil {
		return DocValues.EmptyBinary()
	}
	if entry.compressed {
		return p.getCompressedBinary(entry)
	}
	return p.getUncompressedBinary(entry)
}

func (p *lucene80DocValuesProducer) getUncompressedBinary(entry *binaryEntry) BinaryDocValues {
	if entry.docsWithFieldOffset == -2 {
		return DocValues.EmptyBinary()
	}

	bytesSlice := p.data.Slice("fixed-binary", entry.dataOffset, entry.dataLength)

	if entry.docsWithFieldOffset == -1 {
		if entry.minLength == entry.maxLength {
			length := entry.maxLength
			return &denseBinaryDocValues{
				maxDoc: p.maxDoc,
				slice:  bytesSlice,
				length: length,
			}
		} else {
			addressesData := p.data.RandomAccessSlice(entry.addressesOffset, entry.addressesLength)
			addresses := packed.NewDirectMonotonicReader(entry.addressesMeta, addressesData)
			return &denseBinaryVarLen{
				maxDoc: p.maxDoc,
				slice:  bytesSlice,
				addresses: addresses,
				maxLength: entry.maxLength,
			}
		}
	} else {
		disi := NewIndexedDISI(p.data, entry.docsWithFieldOffset, entry.docsWithFieldLength, entry.jumpTableEntryCount, entry.denseRankPower, entry.numDocsWithField)
		if entry.minLength == entry.maxLength {
			length := entry.maxLength
			return &sparseBinaryDocValues{
				disi:   disi,
				slice:  bytesSlice,
				length: length,
			}
		} else {
			addressesData := p.data.RandomAccessSlice(entry.addressesOffset, entry.addressesLength)
			addresses := packed.NewDirectMonotonicReader(entry.addressesMeta, addressesData)
			return &sparseBinaryVarLen{
				disi:      disi,
				slice:     bytesSlice,
				addresses: addresses,
				maxLength: entry.maxLength,
			}
		}
	}
}

func (p *lucene80DocValuesProducer) getCompressedBinary(entry *binaryEntry) BinaryDocValues {
	if entry.docsWithFieldOffset == -2 {
		return DocValues.EmptyBinary()
	}
	if entry.docsWithFieldOffset == -1 {
		addressesData := p.data.RandomAccessSlice(entry.addressesOffset, entry.addressesLength)
		addresses := packed.NewDirectMonotonicReader(entry.addressesMeta, addressesData)
		return &denseBinaryCompressed{
			maxDoc: p.maxDoc,
			addresses: addresses,
			data: p.data,
			entry: entry,
		}
	} else {
		disi := NewIndexedDISI(p.data, entry.docsWithFieldOffset, entry.docsWithFieldLength, entry.jumpTableEntryCount, entry.denseRankPower, entry.numDocsWithField)
		addressesData := p.data.RandomAccessSlic(entry.addressesOffset, entry.addressesLength)
		addresses := packed.NewDirectMonotonicReader(entry.addressesMeta, addressesData)
		return &sparseBinaryCompressed{
			disi:      disi,
			addresses: addresses,
			data:      p.data,
			entry:     entry,
		}
	}
}

type denseBinaryDocValues struct {
	maxDoc int
	slice  IndexInput
	length int
	doc    int
}

func (d *denseBinaryDocValues) DocID() int { return d.doc }
func (d *denseBinaryDocValues) NextDoc() int { return d.advance(d.doc + 1) }
func (d *denseBinaryDocValues) Advance(target int) int {
	if target >= d.maxDoc {
		return d.doc = DocIdSetIterator.NoMoreDocs
	}
	return d.doc = target
}
func (d *denseBinaryDocValues) AdvanceExact(target int) bool {
	d.doc = target
	return true
}
func (d *denseBinaryDocValues) Cost() int64 { return int64(d.maxDoc) }
func (d *denseBinaryDocValues) BinaryValue() BytesRef {
	buf := make([]byte, d.length)
	d.slice.Seek(int64(d.doc) * int64(d.length))
	d.slice.ReadBytes(buf)
	return newBytesRef(buf)
}

type denseBinaryVarLen struct {
	maxDoc int
	slice  IndexInput
	addresses packed.DirectMonotonicReader
	maxLength int
	doc    int
}

func (d *denseBinaryVarLen) DocID() int { return d.doc }
func (d *denseBinaryVarLen) NextDoc() int { return d.advance(d.doc + 1) }
func (d *denseBinaryVarLen) Advance(target int) int {
	if target >= d.maxDoc {
		return d.doc = DocIdSetIterator.NoMoreDocs
	}
	return d.doc = target
}
func (d *denseBinaryVarLen) AdvanceExact(target int) bool {
	d.doc = target
	return true
}
func (d *denseBinaryVarLen) Cost() int64 { return int64(d.maxDoc) }
func (d *denseBinaryVarLen) BinaryValue() BytesRef {
	start := d.addresses.Get(int64(d.doc))
	end := d.addresses.Get(int64(d.doc + 1))
	length := int(end - start)
	buf := make([]byte, length)
	d.slice.Seek(start)
	d.slice.ReadBytes(buf)
	return newBytesRef(buf)
}

type sparseBinaryDocValues struct {
	disi   *IndexedDISI
	slice  IndexInput
	length int
}

func (s *sparseBinaryDocValues) DocID() int { return s.disi.DocID() }
func (s *sparseBinaryDocValues) NextDoc() int { return s.disi.NextDoc() }
func (s *sparseBinaryDocValues) Advance(target int) int { return s.disi.Advance(target) }
func (s *sparseBinaryDocValues) AdvanceExact(target int) bool { return s.disi.AdvanceExact(target) }
func (s *sparseBinaryDocValues) Cost() int64 { return s.disi.Cost() }
func (s *sparseBinaryDocValues) BinaryValue() BytesRef {
	buf := make([]byte, s.length)
	s.slice.Seek(int64(s.disi.Index()) * int64(s.length))
	s.slice.ReadBytes(buf)
	return newBytesRef(buf)
}

type sparseBinaryVarLen struct {
	disi      *IndexedDISI
	slice     IndexInput
	addresses packed.DirectMonotonicReader
	maxLength int
}

func (s *sparseBinaryVarLen) DocID() int { return s.disi.DocID() }
func (s *sparseBinaryVarLen) NextDoc() int { return s.disi.NextDoc() }
func (s *sparseBinaryVarLen) Advance(target int) int { return s.disi.Advance(target) }
func (s *sparseBinaryVarLen) AdvanceExact(target int) bool { return s.disi.AdvanceExact(target) }
func (s *sparseBinaryVarLen) Cost() int64 { return s.disi.Cost() }
func (s *sparseBinaryVarLen) BinaryValue() BytesRef {
	index := s.disi.Index()
	start := s.addresses.Get(int64(index))
	end := s.addresses.Get(int64(index + 1))
	length := int(end - start)
	buf := make([]byte, length)
	s.slice.Seek(start)
	s.slice.ReadBytes(buf)
	return newBytesRef(buf)
}

type denseBinaryCompressed struct {
	maxDoc int
	addresses packed.DirectMonotonicReader
	data IndexInput
	entry *binaryEntry
	doc int
}

func (d *denseBinaryCompressed) DocID() int { return d.doc }
func (d *denseBinaryCompressed) NextDoc() int { return d.advance(d.doc + 1) }
func (d *denseBinaryCompressed) Advance(target int) int {
	if target >= d.maxDoc {
		return d.doc = DocIdSetIterator.NoMoreDocs
	}
	return d.doc = target
}
func (d *denseBinaryCompressed) AdvanceExact(target int) bool {
	d.doc = target
	return true
}
func (d *denseBinaryCompressed) Cost() int64 { return int64(d.maxDoc) }
func (d *denseBinaryCompressed) BinaryValue() BytesRef {
	// Need to implement BinaryDecoder
	return nil
}

type sparseBinaryCompressed struct {
	disi      *IndexedDISI
	addresses packed.DirectMonotonicReader
	data      IndexInput
	entry     *binaryEntry
}

func (s *sparseBinaryCompressed) DocID() int { return s.disi.DocID() }
func (s *sparseBinaryCompressed) NextDoc() int { return s.disi.NextDoc() }
func (s *sparseBinaryCompressed) Advance(target int) int { return s.disi.Advance(target) }
func (s *sparseBinaryCompressed) AdvanceExact(target int) bool { return s.disi.AdvanceExact(target) }
func (s *sparseBinaryCompressed) Cost() int64 { return s.disi.Cost() }
func (s *sparseBinaryCompressed) BinaryValue() BytesRef {
	// Need to implement BinaryDecoder
	return nil
}

func (p *lucene80DocValuesProducer) GetSorted(field FieldInfo) SortedDocValues {
	entry := p.sorted[field.number]
	if entry == nil {
		return DocValues.EmptySorted()
	}
	return p.getSorted(entry)
}

func (p *lucene80DocValuesProducer) getSorted(entry *sortedEntry) SortedDocValues {
	if entry.docsWithFieldOffset == -2 {
		return DocValues.EmptySorted()
	}

	var ords packed.DirectReader
	if entry.bitsPerValue == 0 {
		ords = &zeroLongValues{}
	} else {
		slice := p.data.RandomAccessSlice(entry.ordsOffset, entry.ordsLength)
		ords = packed.NewDirectReader(slice, int(entry.bitsPerValue))
	}

	if entry.docsWithFieldOffset == -1 {
		return &denseSortedDocValues{
			maxDoc: p.maxDoc,
			ords:   ords,
			entry:  entry,
			data:   p.data,
		}
	} else {
		disi := NewIndexedDISI(p.data, entry.docsWithFieldOffset, entry.docsWithFieldLength, entry.jumpTableEntryCount, entry.denseRankPower, entry.numDocsWithField)
		return &sparseSortedDocValues{
			disi:   disi,
			ords:   ords,
			entry:  entry,
			data:   p.data,
		}
	}
}

type zeroLongValues struct{}

func (z *zeroLongValues) Get(index int64) int64 { return 0 }
func (z *zeroLongValues) Length() int64 { return 0 } // Not used

type denseSortedDocValues struct {
	maxDoc int
	ords   packed.DirectReader
	entry  *sortedEntry
	data   IndexInput
	doc    int
}

func (d *denseSortedDocValues) DocID() int { return d.doc }
func (d *denseSortedDocValues) NextDoc() int { return d.advance(d.doc + 1) }
func (d *denseSortedDocValues) Advance(target int) int {
	if target >= d.maxDoc {
		return d.doc = DocIdSetIterator.NoMoreDocs
	}
	return d.doc = target
}
func (d *denseSortedDocValues) AdvanceExact(target int) bool {
	d.doc = target
	return true
}
func (d *denseSortedDocValues) Cost() int64 { return int64(d.maxDoc) }
func (d *denseSortedDocValues) OrdValue() int {
	return int(d.ords.Get(int64(d.doc)))
}

type sparseSortedDocValues struct {
	disi   *IndexedDISI
	ords   packed.DirectReader
	entry  *sortedEntry
	data   IndexInput
}

func (s *sparseSortedDocValues) DocID() int { return s.disi.DocID() }
func (s *sparseSortedDocValues) NextDoc() int { return s.disi.NextDoc() }
func (s *sparseSortedDocValues) Advance(target int) int { return s.disi.Advance(target) }
func (s *sparseSortedDocValues) AdvanceExact(target int) bool { return s.disi.AdvanceExact(target) }
func (s *sparseSortedDocValues) Cost() int64 { return s.disi.Cost() }
func (s *sparseSortedDocValues) OrdValue() int {
	return int(s.ords.Get(int64(s.disi.Index())))
}

func (p *lucene80DocValuesProducer) GetSortedSet(field FieldInfo) SortedSetDocValues {
	entry := p.sortedSets[field.number]
	if entry == nil {
		return DocValues.EmptySortedSet()
	}
	if entry.singleValueEntry != nil {
		return DocValues.Singleton(p.getSorted(entry.singleValueEntry))
	}

	slice := p.data.RandomAccessSlice(entry.ordsOffset, entry.ordsLength)
	ords := packed.NewDirectReader(slice, int(entry.bitsPerValue))

	addressesInput := p.data.RandomAccessSlice(entry.addressesOffset, entry.addressesLength)
	addresses := packed.NewDirectMonotonicReader(entry.addressesMeta, addressesInput)

	if entry.docsWithFieldOffset == -1 {
		return &denseSortedSetDocValues{
			maxDoc: p.maxDoc,
			ords:   ords,
			addresses: addresses,
			entry: entry,
			data: p.data,
		}
	} else {
		disi := NewIndexedDISI(p.data, entry.docsWithFieldOffset, entry.docsWithFieldLength, entry.jumpTableEntryCount, entry.denseRankPower, entry.numDocsWithField)
		return &sparseSortedSetDocValues{
			disi:      disi,
			ords:      ords,
			addresses: addresses,
			entry:     entry,
			data:      p.data,
		}
	}
}

type denseSortedSetDocValues struct {
	maxDoc int
	ords   packed.DirectReader
	addresses packed.DirectMonotonicReader
	entry  *sortedSetEntry
	data   IndexInput
	doc    int
	curr   int64
	count  int
}

func (d *denseSortedSetDocValues) DocID() int { return d.doc }
func (d *denseSortedSetDocValues) NextDoc() int { return d.advance(d.doc + 1) }
func (d *denseSortedSetDocValues) Advance(target int) int {
	if target >= d.maxDoc {
		return d.doc = DocIdSetIterator.NoMoreDocs
	}
	d.curr = d.addresses.Get(int64(target))
	d.count = int(d.addresses.Get(int64(target + 1)) - d.curr)
	return d.doc = target
}
func (d *denseSortedSetDocValues) AdvanceExact(target int) bool {
	d.curr = d.addresses.Get(int64(target))
	d.count = int(d.addresses.Get(int64(target + 1)) - d.curr)
	d.doc = target
	return true
}
func (d *denseSortedSetDocValues) Cost() int64 { return int64(d.maxDoc) }
func (d *denseSortedSetDocValues) NextOrd() int64 {
	v := d.ords.Get(d.curr)
	d.curr++
	return v
}
func (d *denseSortedSetDocValues) DocValueCount() int { return d.count }

type sparseSortedSetDocValues {
	disi      *IndexedDISI
	ords      packed.DirectReader
	addresses packed.DirectMonotonicReader
	entry     *sortedSetEntry
	data      IndexInput
	curr      int64
	count     int
}

func (s *sparseSortedSetDocValues) DocID() int { return s.disi.DocID() }
func (s *sparseSortedSetDocValues) NextDoc() int {
	s.curr = 0 // reset for next doc
	return s.disi.NextDoc()
}
func (s *sparseSortedSetDocValues) Advance(target int) int {
	s.curr = 0
	return s.disi.Advance(target)
}
func (s *sparseSortedSetDocValues) AdvanceExact(target int) bool {
	s.curr = 0
	return s.disi.AdvanceExact(target)
}
func (s *sparseSortedSetDocValues) Cost() int64 { return s.disi.Cost() }
func (s *sparseSortedSetDocValues) NextOrd() int64 {
	// This needs to be careful about the current doc
	// In a real port, we'd use the disi index
	v := s.ords.Get(s.curr)
	s.curr++
	return v
}
func (s *sparseSortedSetDocValues) DocValueCount() int {
	return s.count
}

func (p *lucene80DocValuesProducer) GetSortedNumeric(field FieldInfo) SortedNumericDocValues {
	entry := p.sortedNumerics[field.number]
	if entry == nil {
		return DocValues.EmptySortedNumeric()
	}
	return p.getSortedNumeric(entry)
}

func (p *lucene80DocValuesProducer) getSortedNumeric(entry *sortedNumericEntry) SortedNumericDocValues {
	if entry.numValues == entry.numDocsWithField {
		return DocValues.Singleton(p.getNumeric(entry))
	}

	addressesInput := p.data.RandomAccessSlice(entry.addressesOffset, entry.addressesLength)
	addresses := packed.NewDirectMonotonicReader(entry.addressesMeta, addressesInput)
	values := p.getNumericValues(entry)

	if entry.docsWithFieldOffset == -1 {
		return &denseSortedNumericDocValues{
			maxDoc: p.maxDoc,
			addresses: addresses,
			values: values,
			doc: -1,
		}
	} else {
		disi := NewIndexedDISI(p.data, entry.docsWithFieldOffset, entry.docsWithFieldLength, entry.jumpTableEntryCount, entry.denseRankPower, entry.numDocsWithField)
		return &sparseSortedNumericDocValues{
			disi: disi,
			addresses: addresses,
			values: values,
		}
	}
}

type denseSortedNumericDocValues struct {
	maxDoc int
	addresses packed.DirectMonotonicReader
	values LongValues
	doc int
	start int64
	end int64
	count int
}

func (d *denseSortedNumericDocValues) DocID() int { return d.doc }
func (d *denseSortedNumericDocValues) NextDoc() int { return d.advance(d.doc + 1) }
func (d *denseSortedNumericDocValues) Advance(target int) int {
	if target >= d.maxDoc {
		return d.doc = DocIdSetIterator.NoMoreDocs
	}
	d.start = d.addresses.Get(int64(target))
	d.end = d.addresses.Get(int64(target + 1))
	d.count = int(d.end - d.start)
	return d.doc = target
}
func (d *denseSortedNumericDocValues) AdvanceExact(target int) bool {
	d.start = d.addresses.Get(int64(target))
	d.end = d.addresses.Get(int64(target + 1))
	d.count = int(d.end - d.start)
	d.doc = target
	return true
}
func (d *denseSortedNumericDocValues) Cost() int64 { return int64(d.maxDoc) }
func (d *denseSortedNumericDocValues) NextValue() int64 {
	return d.values.Get(d.start)
}
func (d *denseSortedNumericDocValues) DocValueCount() int { return d.count }

type sparseSortedNumericDocValues struct {
	disi *IndexedDISI
	addresses packed.DirectMonotonicReader
	values LongValues
}

func (s *sparseSortedNumericDocValues) DocID() int { return s.disi.DocID() }
func (s *sparseSortedNumericDocValues) NextDoc() int { return s.disi.NextDoc() }
func (s *sparseSortedNumericDocValues) Advance(target int) int { return s.disi.Advance(target) }
func (s *sparseSortedNumericDocValues) AdvanceExact(target int) bool { return s.disi.AdvanceExact(target) }
func (s *sparseSortedNumericDocValues) Cost() int64 { return s.disi.Cost() }
func (s *sparseSortedNumericDocValues) NextValue() int64 {
	index := s.disi.Index()
	start := s.addresses.Get(int64(index))
	return s.values.Get(start)
}
func (s *sparseSortedNumericDocValues) DocValueCount() int {
	index := s.disi.Index()
	start := s.addresses.Get(int64(index))
	end := s.addresses.Get(int64(index + 1))
	return int(end - start)
}

func (p *lucene80DocValuesProducer) getNumericValues(entry *numericEntry) LongValues {
	if entry.bitsPerValue == 0 {
		return &zeroLongValues{}
	}
	slice := p.data.RandomAccessSlice(entry.valuesOffset, entry.valuesLength)
	if entry.blockShift >= 0 {
		return &numericVaryingBPVValues{
			entry: entry,
			slice: slice,
		}
	} else {
		values := packed.NewDirectReader(slice, int(entry.bitsPerValue))
		if entry.table != nil {
			return &numericTableValues{
				values: values,
				table:  entry.table,
			}
		} else if entry.gcd != 1 {
			return &numericDeltaValues{
				values: values,
				gcd:    entry.gcd,
				minValue: entry.minValue,
			}
		} else if entry.minValue != 0 {
			return &numericDeltaValues{
				values: values,
				gcd:    1,
				minValue: entry.minValue,
			}
		} else {
			return values
		}
	}
}

type numericVaryingBPVValues struct {
	entry *numericEntry
	slice RandomAccessInput
}

func (v *numericVaryingBPVValues) Get(index int64) int64 {
	// Simplified
	return 0
}

type numericTableValues struct {
	values packed.DirectReader
	table  []int64
}

func (v *numericTableValues) Get(index int64) int64 {
	return v.table[v.values.Get(index)]
}

type numericDeltaValues struct {
	values packed.DirectReader
	gcd    int64
	minValue int64
}

func (v *numericDeltaValues) Get(index int64) int64 {
	return v.values.Get(index)*v.gcd + v.minValue
}

type numericEntry struct {
	table []int64
	blockShift int
	bitsPerValue byte
	docsWithFieldOffset int64
	docsWithFieldLength int64
	jumpTableEntryCount int16
	denseRankPower byte
	numValues int64
	minValue int64
	gcd int64
	valuesOffset int64
	valuesLength int64
	valueJumpTableOffset int64
}

type binaryEntry struct {
	compressed bool
	dataOffset int64
	dataLength int64
	docsWithFieldOffset int64
	docsWithFieldLength int64
	jumpTableEntryCount int16
	denseRankPower byte
	numDocsWithField int
	minLength int
	maxLength int
	addressesOffset int64
	addressesLength int64
	addressesMeta packed.DirectMonotonicReader.Meta
	numCompressedChunks int
	docsPerChunkShift int
	maxUncompressedChunkSize int
}

type termsDictEntry struct {
	termsDictSize int64
	termsDictBlockShift int
	termsAddressesMeta packed.DirectMonotonicReader.Meta
	maxTermLength int
	termsDataOffset int64
	termsDataLength int64
	termsAddressesOffset int64
	termsAddressesLength int64
	termsDictIndexShift int
	termsIndexAddressesMeta packed.DirectMonotonicReader.Meta
	termsIndexOffset int64
	termsIndexLength int64
	termsIndexAddressesOffset int64
	termsIndexAddressesLength int64
	compressed bool
	maxBlockLength int
}

type sortedEntry struct {
	termsDictEntry
	docsWithFieldOffset int64
	docsWithFieldLength int64
	jumpTableEntryCount int16
	denseRankPower byte
	numDocsWithField int
	bitsPerValue byte
	ordsOffset int64
	ordsLength int64
}

type sortedSetEntry struct {
	termsDictEntry
	singleValueEntry *sortedEntry
	docsWithFieldOffset int64
	docsWithFieldLength int64
	jumpTableEntryCount int16
	denseRankPower byte
	bitsPerValue byte
	ordsOffset int64
	ordsLength int64
	addressesMeta packed.DirectMonotonicReader.Meta
	addressesOffset int64
	addressesLength int64
}

type sortedNumericEntry struct {
	numericEntry
	numDocsWithField int
	addressesMeta packed.DirectMonotonicReader.Meta
	addressesOffset int64
	addressesLength int64
}

func newBytesRef(b []byte) BytesRef {
	return BytesRef{bytes: b, offset: 0, length: len(b)}
}
