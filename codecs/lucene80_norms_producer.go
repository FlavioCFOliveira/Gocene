// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"fmt"
	"io"
)

type lucene80NormsProducer struct {
	norms          map[int]*normsEntry
	maxDoc         int
	data           IndexInput
	merging        bool
	disiInputs     map[int]IndexInput
	disiJumpTables map[int]RandomAccessInput
	dataInputs     map[int]RandomAccessInput
}

type normsEntry struct {
	denseRankPower      byte
	bytesPerNorm        byte
	docsWithFieldOffset int64
	docsWithFieldLength int64
	jumpTableEntryCount int16
	numDocsWithField    int
	normsOffset         int64
}

func NewLucene80NormsProducer(
	state SegmentReadState,
	dataCodec, dataExtension, metaCodec, metaExtension string) (NormsProducer, error) {

	maxDoc := state.segmentInfo.maxDoc()
	metaName := IndexFileNames.segmentFileName(state.segmentInfo.name, state.segmentSuffix, metaExtension)

	var version int
	var err error

	meta, err := state.directory.OpenChecksumInput(metaName, state.context)
	if err != nil {
		return nil, err
	}
	defer meta.Close()

	version, err = CodecUtil.CheckIndexHeader(
		meta,
		metaCodec,
		NormsVersionStart,
		NormsVersionCurrent,
		state.segmentInfo.getId(),
		state.segmentSuffix)
	if err != nil {
		return nil, err
	}

	producer := &lucene80NormsProducer{
		norms:          make(map[int]*normsEntry),
		maxDoc:         maxDoc,
		disiInputs:     make(map[int]IndexInput),
		disiJumpTables: make(map[int]RandomAccessInput),
		dataInputs:     make(map[int]RandomAccessInput),
	}

	if err := producer.readFields(meta, state.fieldInfos); err != nil {
		return nil, err
	}

	dataName := IndexFileNames.segmentFileName(state.segmentInfo.name, state.segmentSuffix, dataExtension)
	data, err := state.directory.OpenInput(dataName, state.context)
	if err != nil {
		return nil, err
	}
	producer.data = data

	version2, err := CodecUtil.CheckIndexHeader(
		data,
		dataCodec,
		NormsVersionStart,
		NormsVersionCurrent,
		state.segmentInfo.getId(),
		state.segmentSuffix)
	if err != nil {
		data.Close()
		return nil, err
	}

	if version != version2 {
		data.Close()
		return nil, fmt.Errorf("format versions mismatch: meta=%d, data=%d", version, version2)
	}

	if err := CodecUtil.RetrieveChecksum(data); err != nil {
		data.Close()
		return nil, err
	}

	return producer, nil
}

func (p *lucene80NormsProducer) readFields(meta IndexInput, infos FieldInfos) error {
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
		if !info.HasNorms() {
			return fmt.Errorf("invalid field: %s", info.name)
		}

		entry := &normsEntry{}
		entry.docsWithFieldOffset = meta.ReadLong()
		entry.docsWithFieldLength = meta.ReadLong()
		entry.jumpTableEntryCount = meta.ReadShort()
		entry.denseRankPower = meta.ReadByte()
		entry.numDocsWithField = meta.ReadInt()
		entry.bytesPerNorm = meta.ReadByte()

		switch entry.bytesPerNorm {
		case 0, 1, 2, 4, 8:
			// ok
		default:
			return fmt.Errorf("invalid bytesPerValue: %d, field: %s", entry.bytesPerNorm, info.name)
		}

		entry.normsOffset = meta.ReadLong()
		p.norms[info.number] = entry
	}
	return nil
}

func (p *lucene80NormsProducer) GetNorms(field FieldInfo) NumericDocValues {
	entry := p.norms[field.number]
	if entry == nil {
		return DocValues.EmptyNumeric()
	}

	if entry.docsWithFieldOffset == -2 {
		return DocValues.EmptyNumeric()
	} else if entry.docsWithFieldOffset == -1 {
		// dense
		if entry.bytesPerNorm == 0 {
			return &denseNormsIterator{
				maxDoc: p.maxDoc,
				doc:    -1,
				val:    entry.normsOffset,
			}
		}

		slice := p.getDataInput(field, entry)
		switch entry.bytesPerNorm {
		case 1:
			return &denseNormsIteratorByte{
				maxDoc: p.maxDoc,
				doc:    -1,
				slice:  slice,
			}
		case 2:
			return &denseNormsIteratorShort{
				maxDoc: p.maxDoc,
				doc:    -1,
				slice:  slice,
			}
		case 4:
			return &denseNormsIteratorInt{
				maxDoc: p.maxDoc,
				doc:    -1,
				slice:  slice,
			}
		case 8:
			return &denseNormsIteratorLong{
				maxDoc: p.maxDoc,
				doc:    -1,
				slice:  slice,
			}
		}
	} else {
		// sparse
		disiInput := p.getDisiInput(field, entry)
		disiJumpTable := p.getDisiJumpTable(field, entry)
		disi := NewIndexedDISI(disiInput, disiJumpTable, entry.jumpTableEntryCount, entry.denseRankPower, entry.numDocsWithField)

		if entry.bytesPerNorm == 0 {
			return &sparseNormsIterator{
				disi: disi,
				val:  entry.normsOffset,
			}
		}

		slice := p.getDataInput(field, entry)
		switch entry.bytesPerNorm {
		case 1:
			return &sparseNormsIteratorByte{
				disi:  disi,
				slice: slice,
			}
		case 2:
			return &sparseNormsIteratorShort{
				disi:  disi,
				slice: slice,
			}
		case 4:
			return &sparseNormsIteratorInt{
				disi:  disi,
				slice: slice,
			}
		case 8:
			return &sparseNormsIteratorLong{
				disi:  disi,
				slice: slice,
			}
		}
	}
	return DocValues.EmptyNumeric()
}

func (p *lucene80NormsProducer) getDataInput(field FieldInfo, entry *normsEntry) RandomAccessInput {
	if p.merging {
		if slice, ok := p.dataInputs[field.number]; ok {
			return slice
		}
	}

	slice := p.data.RandomAccessSlice(
		entry.normsOffset,
		int64(entry.numDocsWithField)*int64(entry.bytesPerNorm))

	if p.merging {
		p.dataInputs[field.number] = slice
	}
	return slice
}

func (p *lucene80NormsProducer) getDisiInput(field FieldInfo, entry *normsEntry) IndexInput {
	if !p.merging {
		return IndexedDISI.CreateBlockSlice(
			p.data,
			"docs",
			entry.docsWithFieldOffset,
			entry.docsWithFieldLength,
			entry.jumpTableEntryCount)
	}

	if in, ok := p.disiInputs[field.number]; ok {
		return in
	}

	in := IndexedDISI.CreateBlockSlice(
		p.data,
		"docs",
		entry.docsWithFieldOffset,
		entry.docsWithFieldLength,
		entry.jumpTableEntryCount)
	p.disiInputs[field.number] = in
	return in
}

func (p *lucene80NormsProducer) getDisiJumpTable(field FieldInfo, entry *normsEntry) RandomAccessInput {
	if p.merging {
		if jt, ok := p.disiJumpTables[field.number]; ok {
			return jt
		}
	}

	jt := IndexedDISI.CreateJumpTable(
		p.data,
		entry.docsWithFieldOffset,
		entry.docsWithFieldLength,
		entry.jumpTableEntryCount)

	if p.merging {
		p.disiJumpTables[field.number] = jt
	}
	return jt
}

func (p *lucene80NormsProducer) Close() error {
	if p.data != nil {
		return p.data.Close()
	}
	return nil
}

func (p *lucene80NormsProducer) CheckIntegrity() error {
	if p.data != nil {
		return CodecUtil.ChecksumEntireFile(p.data)
	}
	return nil
}

type denseNormsIterator struct {
	maxDoc int
	doc    int
	val    int64
}

func (d *denseNormsIterator) DocID() int   { return d.doc }
func (d *denseNormsIterator) NextDoc() int { return d.advance(d.doc + 1) }
func (d *denseNormsIterator) Advance(target int) int {
	if target >= d.maxDoc {
		d.doc = dvNoMoreDocs
		return d.doc
	}
	d.doc = target
	return d.doc
}
func (d *denseNormsIterator) AdvanceExact(target int) bool {
	d.doc = target
	return true
}
func (d *denseNormsIterator) Cost() int64      { return int64(d.maxDoc) }
func (d *denseNormsIterator) LongValue() int64 { return d.val }

type denseNormsIteratorByte struct {
	maxDoc int
	doc    int
	slice  RandomAccessInput
}

func (d *denseNormsIteratorByte) DocID() int   { return d.doc }
func (d *denseNormsIteratorByte) NextDoc() int { return d.advance(d.doc + 1) }
func (d *denseNormsIteratorByte) Advance(target int) int {
	if target >= d.maxDoc {
		d.doc = dvNoMoreDocs
		return d.doc
	}
	d.doc = target
	return d.doc
}
func (d *denseNormsIteratorByte) AdvanceExact(target int) bool {
	d.doc = target
	return true
}
func (d *denseNormsIteratorByte) Cost() int64 { return int64(d.maxDoc) }
func (d *denseNormsIteratorByte) LongValue() int64 {
	return int64(d.slice.ReadByte(d.doc))
}

type denseNormsIteratorShort struct {
	maxDoc int
	doc    int
	slice  RandomAccessInput
}

func (d *denseNormsIteratorShort) DocID() int   { return d.doc }
func (d *denseNormsIteratorShort) NextDoc() int { return d.advance(d.doc + 1) }
func (d *denseNormsIteratorShort) Advance(target int) int {
	if target >= d.maxDoc {
		d.doc = dvNoMoreDocs
		return d.doc
	}
	d.doc = target
	return d.doc
}
func (d *denseNormsIteratorShort) AdvanceExact(target int) bool {
	d.doc = target
	return true
}
func (d *denseNormsIteratorShort) Cost() int64 { return int64(d.maxDoc) }
func (d *denseNormsIteratorShort) LongValue() int64 {
	return int64(d.slice.ReadShort(int64(d.doc) << 1))
}

type denseNormsIteratorInt struct {
	maxDoc int
	doc    int
	slice  RandomAccessInput
}

func (d *denseNormsIteratorInt) DocID() int   { return d.doc }
func (d *denseNormsIteratorInt) NextDoc() int { return d.advance(d.doc + 1) }
func (d *denseNormsIteratorInt) Advance(target int) int {
	if target >= d.maxDoc {
		d.doc = dvNoMoreDocs
		return d.doc
	}
	d.doc = target
	return d.doc
}
func (d *denseNormsIteratorInt) AdvanceExact(target int) bool {
	d.doc = target
	return true
}
func (d *denseNormsIteratorInt) Cost() int64 { return int64(d.maxDoc) }
func (d *denseNormsIteratorInt) LongValue() int64 {
	return int64(d.slice.ReadInt(int64(d.doc) << 2))
}

type denseNormsIteratorLong struct {
	maxDoc int
	doc    int
	slice  RandomAccessInput
}

func (d *denseNormsIteratorLong) DocID() int   { return d.doc }
func (d *denseNormsIteratorLong) NextDoc() int { return d.advance(d.doc + 1) }
func (d *denseNormsIteratorLong) Advance(target int) int {
	if target >= d.maxDoc {
		d.doc = dvNoMoreDocs
		return d.doc
	}
	d.doc = target
	return d.doc
}
func (d *denseNormsIteratorLong) AdvanceExact(target int) bool {
	d.doc = target
	return true
}
func (d *denseNormsIteratorLong) Cost() int64 { return int64(d.maxDoc) }
func (d *denseNormsIteratorLong) LongValue() int64 {
	return d.slice.ReadLong(int64(d.doc) << 3)
}

type sparseNormsIterator struct {
	disi *IndexedDISI
	val  int64
}

func (s *sparseNormsIterator) DocID() int                   { return s.disi.DocID() }
func (s *sparseNormsIterator) NextDoc() int                 { return s.disi.NextDoc() }
func (s *sparseNormsIterator) Advance(target int) int       { return s.disi.Advance(target) }
func (s *sparseNormsIterator) AdvanceExact(target int) bool { return s.disi.AdvanceExact(target) }
func (s *sparseNormsIterator) Cost() int64                  { return s.disi.Cost() }
func (s *sparseNormsIterator) LongValue() int64             { return s.val }

type sparseNormsIteratorByte struct {
	disi  *IndexedDISI
	slice RandomAccessInput
}

func (s *sparseNormsIteratorByte) DocID() int                   { return s.disi.DocID() }
func (s *sparseNormsIteratorByte) NextDoc() int                 { return s.disi.NextDoc() }
func (s *sparseNormsIteratorByte) Advance(target int) int       { return s.disi.Advance(target) }
func (s *sparseNormsIteratorByte) AdvanceExact(target int) bool { return s.disi.AdvanceExact(target) }
func (s *sparseNormsIteratorByte) Cost() int64                  { return s.disi.Cost() }
func (s *sparseNormsIteratorByte) LongValue() int64 {
	return int64(s.slice.ReadByte(s.disi.Index()))
}

type sparseNormsIteratorShort struct {
	disi  *IndexedDISI
	slice RandomAccessInput
}

func (s *sparseNormsIteratorShort) DocID() int                   { return s.disi.DocID() }
func (s *sparseNormsIteratorShort) NextDoc() int                 { return s.disi.NextDoc() }
func (s *sparseNormsIteratorShort) Advance(target int) int       { return s.disi.Advance(target) }
func (s *sparseNormsIteratorShort) AdvanceExact(target int) bool { return s.disi.AdvanceExact(target) }
func (s *sparseNormsIteratorShort) Cost() int64                  { return s.disi.Cost() }
func (s *sparseNormsIteratorShort) LongValue() int64 {
	return int64(s.slice.ReadShort(int64(s.disi.Index()) << 1))
}

type sparseNormsIteratorInt struct {
	disi  *IndexedDISI
	slice RandomAccessInput
}

func (s *sparseNormsIteratorInt) DocID() int                   { return s.disi.DocID() }
func (s *sparseNormsIteratorInt) NextDoc() int                 { return s.disi.NextDoc() }
func (s *sparseNormsIteratorInt) Advance(target int) int       { return s.disi.Advance(target) }
func (s *sparseNormsIteratorInt) AdvanceExact(target int) bool { return s.disi.AdvanceExact(target) }
func (s *sparseNormsIteratorInt) Cost() int64                  { return s.disi.Cost() }
func (s *sparseNormsIteratorInt) LongValue() int64 {
	return int64(s.slice.ReadInt(int64(s.disi.Index()) << 2))
}

type sparseNormsIteratorLong struct {
	disi  *IndexedDISI
	slice RandomAccessInput
}

func (s *sparseNormsIteratorLong) DocID() int                   { return s.disi.DocID() }
func (s *sparseNormsIteratorLong) NextDoc() int                 { return s.disi.NextDoc() }
func (s *sparseNormsIteratorLong) Advance(target int) int       { return s.disi.Advance(target) }
func (s *sparseNormsIteratorLong) AdvanceExact(target int) bool { return s.disi.AdvanceExact(target) }
func (s *sparseNormsIteratorLong) Cost() int64                  { return s.disi.Cost() }
func (s *sparseNormsIteratorLong) LongValue() int64 {
	return s.slice.ReadLong(int64(s.disi.Index()) << 3)
}
