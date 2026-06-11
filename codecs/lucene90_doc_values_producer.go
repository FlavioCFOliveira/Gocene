// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Portions adapted from Apache Lucene 10.4.0:
//
//   Licensed to the Apache Software Foundation (ASF) under one or more
//   contributor license agreements. See the NOTICE file distributed with
//   this work for additional information regarding copyright ownership.
//   The ASF licenses this file to You under the Apache License, Version
//   2.0 (the "License"); you may not use this file except in compliance
//   with the License. You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
//   implied. See the License for the specific language governing
//   permissions and limitations under the License.

package codecs

import (
	"errors"
	"fmt"
	"io"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util/compress"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// ---------------------------------------------------------------------------
// Metadata entry types
// ---------------------------------------------------------------------------

type dvNumericEntry struct {
	table                []int64
	blockShift           int
	bitsPerValue         byte
	docsWithFieldOffset  int64
	docsWithFieldLength  int64
	jumpTableEntryCount  int16
	denseRankPower       byte
	numValues            int64
	minValue             int64
	gcd                  int64
	valuesOffset         int64
	valuesLength         int64
	valueJumpTableOffset int64
}

type dvBinaryEntry struct {
	dataOffset          int64
	dataLength          int64
	docsWithFieldOffset int64
	docsWithFieldLength int64
	jumpTableEntryCount int16
	denseRankPower      byte
	numDocsWithField    int
	minLength           int
	maxLength           int
	addressesOffset     int64
	addressesLength     int64
	addressesMeta       *packed.DirectMonotonicMeta
}

type dvTermsDictEntry struct {
	termsDictSize             int64
	termsAddressesMeta        *packed.DirectMonotonicMeta
	maxTermLength             int
	maxBlockLength            int
	termsDataOffset           int64
	termsDataLength           int64
	termsAddressesOffset      int64
	termsAddressesLength      int64
	termsDictIndexShift       int
	termsIndexAddressesMeta   *packed.DirectMonotonicMeta
	termsIndexOffset          int64
	termsIndexLength          int64
	termsIndexAddressesOffset int64
	termsIndexAddressesLength int64
}

type dvSortedEntry struct {
	ordsEntry      dvNumericEntry
	termsDictEntry dvTermsDictEntry
}

type dvSortedSetEntry struct {
	singleValueEntry *dvSortedEntry // non-nil when single-valued
	ordsEntry        *dvSortedNumericEntry
	termsDictEntry   dvTermsDictEntry
}

type dvSortedNumericEntry struct {
	dvNumericEntry
	numDocsWithField int
	addressesMeta    *packed.DirectMonotonicMeta
	addressesOffset  int64
	addressesLength  int64
}

type dvSkipperEntry struct {
	offset   int64
	length   int64
	minValue int64
	maxValue int64
	docCount int
	maxDocID int
}

// lucene90DocValuesSkipper is a DocValuesSkipper that exposes the global
// block metadata written by the Lucene90 doc-values format. It provides
// only the level-0 view (the single block covering all documents with a
// value for the field). Per-block level decoding is not yet implemented.
type lucene90DocValuesSkipper struct {
	entry *dvSkipperEntry
	docID int
}

func (s *lucene90DocValuesSkipper) SkipTo(target int) (int, error) {
	if s.docID == dvNoMoreDocs {
		return dvNoMoreDocs, nil
	}
	if target > s.entry.maxDocID {
		s.docID = dvNoMoreDocs
		return dvNoMoreDocs, nil
	}
	s.docID = target
	return target, nil
}

func (s *lucene90DocValuesSkipper) GetDocID() int {
	return s.docID
}

// ---------------------------------------------------------------------------
// lucene90DVProducer
// ---------------------------------------------------------------------------

// lucene90DVProducer reads the Lucene 9.0 doc values binary format.
//
// This is the Go port of
// org.apache.lucene.codecs.lucene90.Lucene90DocValuesProducer (Lucene 10.4.0).
type lucene90DVProducer struct {
	numerics       map[int]*dvNumericEntry
	binaries       map[int]*dvBinaryEntry
	sorted         map[int]*dvSortedEntry
	sortedSets     map[int]*dvSortedSetEntry
	sortedNumerics map[int]*dvSortedNumericEntry
	skippers       map[int]*dvSkipperEntry
	data           store.IndexInput
	maxDoc         int
	closed         bool
}

// newLucene90DVProducer opens and reads the .dvm metadata then holds .dvd open.
func newLucene90DVProducer(state *SegmentReadState) (*lucene90DVProducer, error) {
	seg := state.SegmentInfo.Name()
	suffix := state.SegmentSuffix
	id := state.SegmentInfo.GetID()

	dvmName := seg + "." + Lucene90DocValuesMetaExtension
	if suffix != "" {
		dvmName = seg + "_" + suffix + "." + Lucene90DocValuesMetaExtension
	}

	p := &lucene90DVProducer{
		numerics:       make(map[int]*dvNumericEntry),
		binaries:       make(map[int]*dvBinaryEntry),
		sorted:         make(map[int]*dvSortedEntry),
		sortedSets:     make(map[int]*dvSortedSetEntry),
		sortedNumerics: make(map[int]*dvSortedNumericEntry),
		skippers:       make(map[int]*dvSkipperEntry),
		maxDoc:         state.SegmentInfo.DocCount(),
	}

	// Read metadata
	metaRaw, err := state.Directory.OpenInput(dvmName, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return nil, fmt.Errorf("lucene90 dv producer: open meta %q: %w", dvmName, err)
	}
	metaIn := store.NewChecksumIndexInput(metaRaw)

	var readErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				readErr = fmt.Errorf("lucene90 dv producer: panic reading meta: %v", r)
			}
		}()
		if _, err := CheckIndexHeader(metaIn, Lucene90DocValuesMetaCodec,
			Lucene90DocValuesVersionStart, Lucene90DocValuesVersionCurrent, id, suffix); err != nil {
			readErr = err
			return
		}
		readErr = p.readFields(metaIn, state.FieldInfos)
	}()

	// always verify footer
	_, footerErr := CheckFooter(metaIn)
	_ = metaRaw.Close()
	if readErr != nil {
		return nil, readErr
	}
	if footerErr != nil {
		return nil, fmt.Errorf("lucene90 dv producer: footer %q: %w", dvmName, footerErr)
	}

	// Open data file
	dvdName := seg + "." + Lucene90DocValuesDataExtension
	if suffix != "" {
		dvdName = seg + "_" + suffix + "." + Lucene90DocValuesDataExtension
	}
	dvd, err := state.Directory.OpenInput(dvdName, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return nil, fmt.Errorf("lucene90 dv producer: open data %q: %w", dvdName, err)
	}
	if _, err := CheckIndexHeader(dvd, Lucene90DocValuesDataCodec,
		Lucene90DocValuesVersionStart, Lucene90DocValuesVersionCurrent, id, suffix); err != nil {
		_ = dvd.Close()
		return nil, fmt.Errorf("lucene90 dv producer: data header %q: %w", dvdName, err)
	}
	p.data = dvd
	return p, nil
}

func (p *lucene90DVProducer) readFields(meta store.DataInput, fieldInfos *index.FieldInfos) error {
	for {
		fieldNum, err := meta.ReadInt()
		if err != nil {
			return fmt.Errorf("lucene90 dv producer: reading field number: %w", err)
		}
		if fieldNum == -1 {
			break
		}
		info := fieldInfos.GetByNumber(int(fieldNum))
		if info == nil {
			return fmt.Errorf("lucene90 dv producer: invalid field number %d", fieldNum)
		}
		dvType, err := meta.ReadByte()
		if err != nil {
			return err
		}
		if info.DocValuesSkipIndexType() != index.DocValuesSkipIndexTypeNone {
			sk, err := p.readSkipper(meta)
			if err != nil {
				return err
			}
			p.skippers[int(fieldNum)] = sk
		}
		switch dvType {
		case Lucene90DocValuesTypeNumeric:
			e, err := p.readNumericEntry(meta)
			if err != nil {
				return err
			}
			p.numerics[int(fieldNum)] = e
		case Lucene90DocValuesTypeBinary:
			e, err := p.readBinaryEntry(meta)
			if err != nil {
				return err
			}
			p.binaries[int(fieldNum)] = e
		case Lucene90DocValuesTypeSorted:
			e, err := p.readSortedEntry(meta)
			if err != nil {
				return err
			}
			p.sorted[int(fieldNum)] = e
		case Lucene90DocValuesTypeSortedSet:
			e, err := p.readSortedSetEntry(meta)
			if err != nil {
				return err
			}
			p.sortedSets[int(fieldNum)] = e
		case Lucene90DocValuesTypeSortedNumeric:
			e, err := p.readSortedNumericEntry(meta)
			if err != nil {
				return err
			}
			p.sortedNumerics[int(fieldNum)] = e
		default:
			return fmt.Errorf("lucene90 dv producer: invalid type %d for field %d", dvType, fieldNum)
		}
	}
	return nil
}

func (p *lucene90DVProducer) readSkipper(meta store.DataInput) (*dvSkipperEntry, error) {
	sk := &dvSkipperEntry{}
	var err error
	if sk.offset, err = meta.ReadLong(); err != nil {
		return nil, err
	}
	if sk.length, err = meta.ReadLong(); err != nil {
		return nil, err
	}
	if sk.maxValue, err = meta.ReadLong(); err != nil {
		return nil, err
	}
	if sk.minValue, err = meta.ReadLong(); err != nil {
		return nil, err
	}
	v, err := meta.ReadInt()
	if err != nil {
		return nil, err
	}
	sk.docCount = int(v)
	v, err = meta.ReadInt()
	if err != nil {
		return nil, err
	}
	sk.maxDocID = int(v)
	return sk, nil
}

func (p *lucene90DVProducer) readNumericEntry(meta store.DataInput) (*dvNumericEntry, error) {
	e := &dvNumericEntry{}
	if err := p.readNumericEntryInto(meta, e); err != nil {
		return nil, err
	}
	return e, nil
}

func (p *lucene90DVProducer) readNumericEntryInto(meta store.DataInput, e *dvNumericEntry) error {
	var err error
	if e.docsWithFieldOffset, err = meta.ReadLong(); err != nil {
		return err
	}
	if e.docsWithFieldLength, err = meta.ReadLong(); err != nil {
		return err
	}
	jt, err := meta.ReadShort()
	if err != nil {
		return err
	}
	e.jumpTableEntryCount = jt
	drp, err := meta.ReadByte()
	if err != nil {
		return err
	}
	e.denseRankPower = drp
	if e.numValues, err = meta.ReadLong(); err != nil {
		return err
	}
	tableSize, err := meta.ReadInt()
	if err != nil {
		return err
	}
	if tableSize > 256 {
		return fmt.Errorf("lucene90 dv producer: invalid table size %d", tableSize)
	}
	if tableSize >= 0 {
		e.table = make([]int64, tableSize)
		for i := range e.table {
			if e.table[i], err = meta.ReadLong(); err != nil {
				return err
			}
		}
	}
	if tableSize < -1 {
		e.blockShift = -2 - int(tableSize)
	} else {
		e.blockShift = -1
	}
	bpv, err := meta.ReadByte()
	if err != nil {
		return err
	}
	e.bitsPerValue = bpv
	if e.minValue, err = meta.ReadLong(); err != nil {
		return err
	}
	if e.gcd, err = meta.ReadLong(); err != nil {
		return err
	}
	if e.valuesOffset, err = meta.ReadLong(); err != nil {
		return err
	}
	if e.valuesLength, err = meta.ReadLong(); err != nil {
		return err
	}
	if e.valueJumpTableOffset, err = meta.ReadLong(); err != nil {
		return err
	}
	return nil
}

func (p *lucene90DVProducer) readBinaryEntry(meta store.DataInput) (*dvBinaryEntry, error) {
	e := &dvBinaryEntry{}
	var err error
	if e.dataOffset, err = meta.ReadLong(); err != nil {
		return nil, err
	}
	if e.dataLength, err = meta.ReadLong(); err != nil {
		return nil, err
	}
	if e.docsWithFieldOffset, err = meta.ReadLong(); err != nil {
		return nil, err
	}
	if e.docsWithFieldLength, err = meta.ReadLong(); err != nil {
		return nil, err
	}
	jt, err := meta.ReadShort()
	if err != nil {
		return nil, err
	}
	e.jumpTableEntryCount = jt
	drp, err := meta.ReadByte()
	if err != nil {
		return nil, err
	}
	e.denseRankPower = drp
	n, err := meta.ReadInt()
	if err != nil {
		return nil, err
	}
	e.numDocsWithField = int(n)
	n, err = meta.ReadInt()
	if err != nil {
		return nil, err
	}
	e.minLength = int(n)
	n, err = meta.ReadInt()
	if err != nil {
		return nil, err
	}
	e.maxLength = int(n)
	if e.minLength < e.maxLength {
		if e.addressesOffset, err = meta.ReadLong(); err != nil {
			return nil, err
		}
		blockShift, err := store.ReadVInt(meta)
		if err != nil {
			return nil, err
		}
		numAddr := int64(e.numDocsWithField) + 1
		e.addressesMeta, err = packed.LoadDirectMonotonicMeta(meta, numAddr, int(blockShift))
		if err != nil {
			return nil, err
		}
		if e.addressesLength, err = meta.ReadLong(); err != nil {
			return nil, err
		}
	}
	return e, nil
}

func (p *lucene90DVProducer) readSortedEntry(meta store.DataInput) (*dvSortedEntry, error) {
	e := &dvSortedEntry{}
	if err := p.readNumericEntryInto(meta, &e.ordsEntry); err != nil {
		return nil, err
	}
	if err := readTermsDictEntry(meta, &e.termsDictEntry); err != nil {
		return nil, err
	}
	return e, nil
}

func (p *lucene90DVProducer) readSortedSetEntry(meta store.DataInput) (*dvSortedSetEntry, error) {
	e := &dvSortedSetEntry{}
	multiValued, err := meta.ReadByte()
	if err != nil {
		return nil, err
	}
	switch multiValued {
	case 0: // single-valued
		sv, err := p.readSortedEntry(meta)
		if err != nil {
			return nil, err
		}
		e.singleValueEntry = sv
	case 1: // multi-valued
		sne := &dvSortedNumericEntry{}
		if err := p.readSortedNumericEntryInto(meta, sne); err != nil {
			return nil, err
		}
		e.ordsEntry = sne
		if err := readTermsDictEntry(meta, &e.termsDictEntry); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("lucene90 dv producer: invalid multiValued flag %d", multiValued)
	}
	return e, nil
}

func readTermsDictEntry(meta store.DataInput, e *dvTermsDictEntry) error {
	size, err := store.ReadVLong(meta)
	if err != nil {
		return err
	}
	e.termsDictSize = size

	blockShiftInt, err := meta.ReadInt()
	if err != nil {
		return err
	}
	blockShift := int(blockShiftInt)
	addrSize := (e.termsDictSize + int64(Lucene90DocValuesTermsDictBlockLZ4Size) - 1) >> uint(Lucene90DocValuesTermsDictBlockLZ4Shift)
	e.termsAddressesMeta, err = packed.LoadDirectMonotonicMeta(meta, addrSize, blockShift)
	if err != nil {
		return err
	}
	n, err := meta.ReadInt()
	if err != nil {
		return err
	}
	e.maxTermLength = int(n)
	n, err = meta.ReadInt()
	if err != nil {
		return err
	}
	e.maxBlockLength = int(n)
	if e.termsDataOffset, err = meta.ReadLong(); err != nil {
		return err
	}
	if e.termsDataLength, err = meta.ReadLong(); err != nil {
		return err
	}
	if e.termsAddressesOffset, err = meta.ReadLong(); err != nil {
		return err
	}
	if e.termsAddressesLength, err = meta.ReadLong(); err != nil {
		return err
	}

	idxShift, err := meta.ReadInt()
	if err != nil {
		return err
	}
	e.termsDictIndexShift = int(idxShift)
	indexSize := (e.termsDictSize + int64(1<<uint(e.termsDictIndexShift)) - 1) >> uint(e.termsDictIndexShift)
	e.termsIndexAddressesMeta, err = packed.LoadDirectMonotonicMeta(meta, 1+indexSize, blockShift)
	if err != nil {
		return err
	}
	if e.termsIndexOffset, err = meta.ReadLong(); err != nil {
		return err
	}
	if e.termsIndexLength, err = meta.ReadLong(); err != nil {
		return err
	}
	if e.termsIndexAddressesOffset, err = meta.ReadLong(); err != nil {
		return err
	}
	if e.termsIndexAddressesLength, err = meta.ReadLong(); err != nil {
		return err
	}
	return nil
}

func (p *lucene90DVProducer) readSortedNumericEntry(meta store.DataInput) (*dvSortedNumericEntry, error) {
	e := &dvSortedNumericEntry{}
	if err := p.readSortedNumericEntryInto(meta, e); err != nil {
		return nil, err
	}
	return e, nil
}

func (p *lucene90DVProducer) readSortedNumericEntryInto(meta store.DataInput, e *dvSortedNumericEntry) error {
	if err := p.readNumericEntryInto(meta, &e.dvNumericEntry); err != nil {
		return err
	}
	n, err := meta.ReadInt()
	if err != nil {
		return err
	}
	e.numDocsWithField = int(n)
	if int64(e.numDocsWithField) != e.numValues {
		if e.addressesOffset, err = meta.ReadLong(); err != nil {
			return err
		}
		blockShift, err := store.ReadVInt(meta)
		if err != nil {
			return err
		}
		e.addressesMeta, err = packed.LoadDirectMonotonicMeta(meta, int64(e.numDocsWithField)+1, int(blockShift))
		if err != nil {
			return err
		}
		if e.addressesLength, err = meta.ReadLong(); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// DocValuesProducer interface
// ---------------------------------------------------------------------------

func (p *lucene90DVProducer) GetNumeric(field *index.FieldInfo) (NumericDocValues, error) {
	e, ok := p.numerics[field.Number()]
	if !ok {
		return nil, nil
	}
	return p.getNumeric(e)
}

func (p *lucene90DVProducer) GetBinary(field *index.FieldInfo) (BinaryDocValues, error) {
	e, ok := p.binaries[field.Number()]
	if !ok {
		return nil, nil
	}
	return p.getBinary(e)
}

func (p *lucene90DVProducer) GetSorted(field *index.FieldInfo) (SortedDocValues, error) {
	e, ok := p.sorted[field.Number()]
	if !ok {
		return nil, nil
	}
	return p.getSorted(e)
}

func (p *lucene90DVProducer) GetSortedSet(field *index.FieldInfo) (SortedSetDocValues, error) {
	e, ok := p.sortedSets[field.Number()]
	if !ok {
		return nil, nil
	}
	return p.getSortedSet(e)
}

func (p *lucene90DVProducer) GetSortedNumeric(field *index.FieldInfo) (SortedNumericDocValues, error) {
	e, ok := p.sortedNumerics[field.Number()]
	if !ok {
		return nil, nil
	}
	return p.getSortedNumericFromEntry(e)
}

// GetSkipper returns the DocValuesSkipper companion for the field, or
// nil when this format did not write a sparse skipper for it.
//
// The returned skipper exposes the global block metadata (min/max/docCount
// across all values) but does not yet decode per-block level data for
// fine-grained skip navigation. The implementation returned is a
// lucene90DocValuesSkipper that covers a single block spanning all
// documents that have a value for the field.
//
// Required by spi.DocValuesProducer since rmp #4708 lifted the
// doc-values family onto the SPI with the Lucene-faithful method set.
func (p *lucene90DVProducer) GetSkipper(field *index.FieldInfo) (DocValuesSkipper, error) {
	entry, ok := p.skippers[field.Number()]
	if !ok {
		return nil, nil
	}
	return &lucene90DocValuesSkipper{entry: entry, docID: -1}, nil
}

func (p *lucene90DVProducer) CheckIntegrity() error { return nil }

func (p *lucene90DVProducer) Close() error {
	if p.closed {
		return nil
	}
	p.closed = true
	return p.data.Close()
}

// ---------------------------------------------------------------------------
// Numeric
// ---------------------------------------------------------------------------

func (p *lucene90DVProducer) getNumericValues(e *dvNumericEntry) (dvLongValues, error) {
	if e.bitsPerValue == 0 {
		return constLongValues(e.minValue), nil
	}
	slice, err := dvSliceRandomAccess(p.data, e.valuesOffset, e.valuesLength)
	if err != nil {
		return nil, err
	}
	if e.blockShift >= 0 {
		var jt store.RandomAccessInput
		if e.valueJumpTableOffset != -1 {
			// Number of blocks = ceil(numValues / (1<<blockShift))
			numBlocks := (e.numValues + int64(1<<uint(e.blockShift)) - 1) >> uint(e.blockShift)
			jtLen := numBlocks * 8
			var jtErr error
			jt, jtErr = dvSliceRandomAccess(p.data, e.valueJumpTableOffset, jtLen)
			if jtErr != nil {
				return nil, jtErr
			}
		}
		return &varyingBPVReader{entry: e, slice: slice, jumpTable: jt, block: -1}, nil
	}
	inner, err := packed.GetDirectReader(slice, int(e.bitsPerValue))
	if err != nil {
		return nil, err
	}
	if e.table != nil {
		tbl := e.table
		return funcLongValues(func(index int64) (int64, error) {
			v, err := inner.Get(index)
			if err != nil {
				return 0, err
			}
			return tbl[int(v)], nil
		}), nil
	} else if e.gcd != 1 {
		mul := e.gcd
		delta := e.minValue
		return funcLongValues(func(index int64) (int64, error) {
			v, err := inner.Get(index)
			if err != nil {
				return 0, err
			}
			return v*mul + delta, nil
		}), nil
	} else if e.minValue != 0 {
		delta := e.minValue
		return funcLongValues(func(index int64) (int64, error) {
			v, err := inner.Get(index)
			if err != nil {
				return 0, err
			}
			return v + delta, nil
		}), nil
	}
	return funcLongValues(func(index int64) (int64, error) { return inner.Get(index) }), nil
}

func (p *lucene90DVProducer) getNumeric(e *dvNumericEntry) (NumericDocValues, error) {
	if e.docsWithFieldOffset == -2 {
		return emptyNumericDV{}, nil
	}
	if e.docsWithFieldOffset == -1 {
		// dense
		if e.bitsPerValue == 0 {
			return newDenseConstNumericDV(p.maxDoc, e.minValue), nil
		}
		vals, err := p.getNumericValues(e)
		if err != nil {
			return nil, err
		}
		return &denseNumericDV{maxDoc: p.maxDoc, vals: vals, doc: -1}, nil
	}
	// sparse
	disi, err := newDVIndexedDISI(p.data, e.docsWithFieldOffset, e.docsWithFieldLength,
		int(e.jumpTableEntryCount), e.denseRankPower, e.numValues)
	if err != nil {
		return nil, err
	}
	if e.bitsPerValue == 0 {
		return &sparseConstNumericDV{disi: disi, val: e.minValue}, nil
	}
	vals, err := p.getNumericValues(e)
	if err != nil {
		return nil, err
	}
	return &sparseNumericDV{disi: disi, vals: vals}, nil
}

// ---------------------------------------------------------------------------
// Binary
// ---------------------------------------------------------------------------

func (p *lucene90DVProducer) getBinary(e *dvBinaryEntry) (BinaryDocValues, error) {
	if e.docsWithFieldOffset == -2 {
		return emptyBinaryDV{}, nil
	}
	bytesSlice, err := dvSliceRandomAccess(p.data, e.dataOffset, e.dataLength)
	if err != nil {
		return nil, err
	}
	if e.docsWithFieldOffset == -1 {
		// dense
		if e.minLength == e.maxLength {
			return &denseFixedBinaryDV{maxDoc: p.maxDoc, bytes: bytesSlice, length: e.minLength, doc: -1}, nil
		}
		// variable
		addrSlice, err := dvSliceRandomAccess(p.data, e.addressesOffset, e.addressesLength)
		if err != nil {
			return nil, err
		}
		addrs, err := packed.NewDirectMonotonicReader(e.addressesMeta, addrSlice)
		if err != nil {
			return nil, err
		}
		return &denseVarBinaryDV{maxDoc: p.maxDoc, bytes: bytesSlice, addrs: addrs, maxLen: e.maxLength, doc: -1}, nil
	}
	// sparse
	disi, err := newDVIndexedDISI(p.data, e.docsWithFieldOffset, e.docsWithFieldLength,
		int(e.jumpTableEntryCount), e.denseRankPower, int64(e.numDocsWithField))
	if err != nil {
		return nil, err
	}
	if e.minLength == e.maxLength {
		return &sparseFixedBinaryDV{disi: disi, bytes: bytesSlice, length: e.minLength}, nil
	}
	addrSlice, err := dvSliceRandomAccess(p.data, e.addressesOffset, e.addressesLength)
	if err != nil {
		return nil, err
	}
	addrs, err := packed.NewDirectMonotonicReader(e.addressesMeta, addrSlice)
	if err != nil {
		return nil, err
	}
	return &sparseVarBinaryDV{disi: disi, bytes: bytesSlice, addrs: addrs, maxLen: e.maxLength}, nil
}

// ---------------------------------------------------------------------------
// Sorted
// ---------------------------------------------------------------------------

func (p *lucene90DVProducer) getSorted(e *dvSortedEntry) (SortedDocValues, error) {
	ordsEntry := &e.ordsEntry
	// fast path: single direct-reader block
	if ordsEntry.blockShift < 0 && ordsEntry.bitsPerValue > 0 {
		if ordsEntry.gcd != 1 || ordsEntry.minValue != 0 || ordsEntry.table != nil {
			return nil, errors.New("lucene90 dv producer: ordinals with unexpected GCD/offset/table")
		}
		slice, err := dvSliceRandomAccess(p.data, ordsEntry.valuesOffset, ordsEntry.valuesLength)
		if err != nil {
			return nil, err
		}
		vals, err := packed.GetDirectReader(slice, int(ordsEntry.bitsPerValue))
		if err != nil {
			return nil, err
		}
		td, err := newDVTermsDict(&e.termsDictEntry, p.data)
		if err != nil {
			return nil, err
		}

		if ordsEntry.docsWithFieldOffset == -1 { // dense
			return &sortedDVDense{sortedDVBase: sortedDVBase{td: td}, maxDoc: p.maxDoc, vals: vals, doc: -1}, nil
		}
		if ordsEntry.docsWithFieldOffset >= 0 { // sparse
			disi, err := newDVIndexedDISI(p.data, ordsEntry.docsWithFieldOffset,
				ordsEntry.docsWithFieldLength, int(ordsEntry.jumpTableEntryCount),
				ordsEntry.denseRankPower, ordsEntry.numValues)
			if err != nil {
				return nil, err
			}
			return &sortedDVSparse{sortedDVBase: sortedDVBase{td: td}, disi: disi, vals: vals}, nil
		}
	}
	// general: use getNumeric path
	numericDV, err := p.getNumeric(ordsEntry)
	if err != nil {
		return nil, err
	}
	td, err := newDVTermsDict(&e.termsDictEntry, p.data)
	if err != nil {
		return nil, err
	}
	return &sortedDVGeneral{sortedDVBase: sortedDVBase{td: td}, ndv: numericDV}, nil
}

// ---------------------------------------------------------------------------
// SortedSet
// ---------------------------------------------------------------------------

func (p *lucene90DVProducer) getSortedSet(e *dvSortedSetEntry) (SortedSetDocValues, error) {
	if e.singleValueEntry != nil {
		sdv, err := p.getSorted(e.singleValueEntry)
		if err != nil {
			return nil, err
		}
		return singletonSortedSet(sdv), nil
	}
	ordsEntry := e.ordsEntry
	// fast path for dense packed-int ordinals
	if ordsEntry.blockShift < 0 && ordsEntry.bitsPerValue > 0 &&
		ordsEntry.gcd == 1 && ordsEntry.minValue == 0 && ordsEntry.table == nil {

		addrSlice, err := dvSliceRandomAccess(p.data, ordsEntry.addressesOffset, ordsEntry.addressesLength)
		if err != nil {
			return nil, err
		}
		addrs, err := packed.NewDirectMonotonicReader(ordsEntry.addressesMeta, addrSlice)
		if err != nil {
			return nil, err
		}
		slice, err := dvSliceRandomAccess(p.data, ordsEntry.valuesOffset, ordsEntry.valuesLength)
		if err != nil {
			return nil, err
		}
		vals, err := packed.GetDirectReader(slice, int(ordsEntry.bitsPerValue))
		if err != nil {
			return nil, err
		}
		td, err := newDVTermsDict(&e.termsDictEntry, p.data)
		if err != nil {
			return nil, err
		}
		if ordsEntry.docsWithFieldOffset == -1 { // dense
			return &sortedSetDVDense{sortedSetDVBase: sortedSetDVBase{td: td}, maxDoc: p.maxDoc, vals: vals, addrs: addrs, doc: -1}, nil
		}
		if ordsEntry.docsWithFieldOffset >= 0 { // sparse
			disi, err := newDVIndexedDISI(p.data, ordsEntry.docsWithFieldOffset,
				ordsEntry.docsWithFieldLength, int(ordsEntry.jumpTableEntryCount),
				ordsEntry.denseRankPower, ordsEntry.numValues)
			if err != nil {
				return nil, err
			}
			return &sortedSetDVSparse{sortedSetDVBase: sortedSetDVBase{td: td}, disi: disi, vals: vals, addrs: addrs}, nil
		}
	}
	// general path
	sndv, err := p.getSortedNumericFromEntry(ordsEntry)
	if err != nil {
		return nil, err
	}
	td, err := newDVTermsDict(&e.termsDictEntry, p.data)
	if err != nil {
		return nil, err
	}
	return &sortedSetDVGeneral{sortedSetDVBase: sortedSetDVBase{td: td}, sndv: sndv}, nil
}

// ---------------------------------------------------------------------------
// SortedNumeric
// ---------------------------------------------------------------------------

func (p *lucene90DVProducer) getSortedNumericFromEntry(e *dvSortedNumericEntry) (SortedNumericDocValues, error) {
	if int64(e.numDocsWithField) == e.numValues {
		// effectively single-value per doc: wrap numeric
		ndv, err := p.getNumeric(&e.dvNumericEntry)
		if err != nil {
			return nil, err
		}
		return singletonSortedNumeric(ndv), nil
	}
	addrSlice, err := dvSliceRandomAccess(p.data, e.addressesOffset, e.addressesLength)
	if err != nil {
		return nil, err
	}
	addrs, err := packed.NewDirectMonotonicReader(e.addressesMeta, addrSlice)
	if err != nil {
		return nil, err
	}
	vals, err := p.getNumericValues(&e.dvNumericEntry)
	if err != nil {
		return nil, err
	}
	if e.docsWithFieldOffset == -1 { // dense
		return &sortedNumericDVDense{maxDoc: p.maxDoc, addrs: addrs, vals: vals, doc: -1}, nil
	}
	disi, err := newDVIndexedDISI(p.data, e.docsWithFieldOffset, e.docsWithFieldLength,
		int(e.jumpTableEntryCount), e.denseRankPower, int64(e.numDocsWithField))
	if err != nil {
		return nil, err
	}
	return &sortedNumericDVSparse{disi: disi, addrs: addrs, vals: vals}, nil
}

// ---------------------------------------------------------------------------
// TermsDict
// ---------------------------------------------------------------------------

const lz4DecompressorPadding = 7

// dvTermsDict implements seekable term dictionary using LZ4-compressed blocks.
type dvTermsDict struct {
	entry        *dvTermsDictEntry
	blockAddrs   *packed.DirectMonotonicReader
	bytes        store.IndexInput // seekable slice over termsData
	blockMask    int64
	indexAddrs   *packed.DirectMonotonicReader
	indexBytes   store.RandomAccessInput
	term         []byte
	blockBuf     []byte
	blockInput   *store.ByteArrayDataInput // cursor into decompressed block
	ord          int64
	compBStart   int64 // start of last decompressed block in bytes stream
	compBEnd     int64 // end of last decompressed block in bytes stream
	blockTermLen int   // term length for the first term in the last decompressed block
	blockUncLen  int   // uncompressed length for the last decompressed block
}

func newDVTermsDict(e *dvTermsDictEntry, data store.IndexInput) (*dvTermsDict, error) {
	addrSlice, err := dvSliceRandomAccess(data, e.termsAddressesOffset, e.termsAddressesLength)
	if err != nil {
		return nil, err
	}
	blockAddrs, err := packed.NewDirectMonotonicReader(e.termsAddressesMeta, addrSlice)
	if err != nil {
		return nil, err
	}
	bytesSlice, err := data.Slice("terms", e.termsDataOffset, e.termsDataLength)
	if err != nil {
		return nil, err
	}
	idxAddrSlice, err := dvSliceRandomAccess(data, e.termsIndexAddressesOffset, e.termsIndexAddressesLength)
	if err != nil {
		return nil, err
	}
	idxAddrs, err := packed.NewDirectMonotonicReader(e.termsIndexAddressesMeta, idxAddrSlice)
	if err != nil {
		return nil, err
	}
	idxBytesSlice, err := dvSliceRandomAccess(data, e.termsIndexOffset, e.termsIndexLength)
	if err != nil {
		return nil, err
	}
	bufSize := e.maxBlockLength + e.maxTermLength + lz4DecompressorPadding
	td := &dvTermsDict{
		entry:      e,
		blockAddrs: blockAddrs,
		bytes:      bytesSlice,
		blockMask:  int64(Lucene90DocValuesTermsDictBlockLZ4Size) - 1,
		indexAddrs: idxAddrs,
		indexBytes: idxBytesSlice,
		term:       make([]byte, e.maxTermLength),
		blockBuf:   make([]byte, bufSize),
		blockInput: store.NewByteArrayDataInput(nil),
		ord:        -1,
		compBStart: -1,
		compBEnd:   -1,
	}
	return td, nil
}

// Next advances to the next term. Returns nil when exhausted.
func (td *dvTermsDict) Next() ([]byte, error) {
	td.ord++
	if td.ord >= td.entry.termsDictSize {
		return nil, nil
	}
	if (td.ord & td.blockMask) == 0 {
		if err := td.decompressBlock(); err != nil {
			return nil, err
		}
	} else {
		tokenByte, err := td.blockInput.ReadByte()
		if err != nil {
			return nil, err
		}
		token := int(tokenByte) & 0xFF
		prefixLen := token & 0x0F
		suffixLen := 1 + (token >> 4)
		if prefixLen == 15 {
			v, err := store.ReadVInt(td.blockInput)
			if err != nil {
				return nil, err
			}
			prefixLen += int(v)
		}
		if suffixLen == 16 {
			v, err := store.ReadVInt(td.blockInput)
			if err != nil {
				return nil, err
			}
			suffixLen += int(v)
		}
		total := prefixLen + suffixLen
		if cap(td.term) < total {
			td.term = make([]byte, total)
		}
		td.term = td.term[:total]
		if err := td.blockInput.ReadBytes(td.term[prefixLen:]); err != nil {
			return nil, err
		}
	}
	return td.term[:len(td.term)], nil
}

// SeekExact positions the dict at the given ordinal.
func (td *dvTermsDict) SeekExact(ord int64) error {
	if ord < 0 || ord >= td.entry.termsDictSize {
		return fmt.Errorf("dvTermsDict: ord %d out of bounds [0,%d)", ord, td.entry.termsDictSize)
	}
	currentBlockIndex := td.ord >> int64(Lucene90DocValuesTermsDictBlockLZ4Shift)
	blockIndex := ord >> int64(Lucene90DocValuesTermsDictBlockLZ4Shift)
	if ord < td.ord || blockIndex != currentBlockIndex {
		blockAddr, err := td.blockAddrs.Get(blockIndex)
		if err != nil {
			return fmt.Errorf("dvTermsDict: get block addr: %w", err)
		}
		if err := td.bytes.SetPosition(blockAddr); err != nil {
			return err
		}
		td.ord = (blockIndex << int64(Lucene90DocValuesTermsDictBlockLZ4Shift)) - 1
	}
	for td.ord < ord {
		if _, err := td.Next(); err != nil {
			return err
		}
	}
	return nil
}

// LookupOrd returns the bytes for ordinal ord.
func (td *dvTermsDict) LookupOrd(ord int) ([]byte, error) {
	if err := td.SeekExact(int64(ord)); err != nil {
		return nil, err
	}
	result := make([]byte, len(td.term))
	copy(result, td.term)
	return result, nil
}

func (td *dvTermsDict) getTermFromIndex(index int64) ([]byte, error) {
	start, err := td.indexAddrs.Get(index)
	if err != nil {
		return nil, fmt.Errorf("dvTermsDict: index addr: %w", err)
	}
	endAddr, err := td.indexAddrs.Get(index + 1)
	if err != nil {
		return nil, fmt.Errorf("dvTermsDict: index addr end: %w", err)
	}
	length := int(endAddr - start)
	if cap(td.term) < length {
		td.term = make([]byte, length)
	}
	td.term = td.term[:length]
	if length > 0 {
		if err := dvReadBytesAt(td.indexBytes, start, td.term); err != nil {
			return nil, err
		}
	}
	return td.term[:length], nil
}

func (td *dvTermsDict) decompressBlock() error {
	vint, err := store.ReadVInt(td.bytes)
	if err != nil {
		return err
	}
	termLen := int(vint)
	if cap(td.term) < termLen {
		td.term = make([]byte, termLen)
	}
	td.term = td.term[:termLen]
	if err := td.bytes.ReadBytes(td.term); err != nil {
		return err
	}
	offset := td.bytes.GetFilePointer()
	if offset < td.entry.termsDataLength-1 {
		if td.compBStart != offset {
			// copy first term as dict
			copy(td.blockBuf[:termLen], td.term)
			// read uncompressed length
			ulen, err := store.ReadVInt(td.bytes)
			if err != nil {
				return err
			}
			uncompressedLen := int(ulen)
			// decompress into blockBuf[termLen:]
			// ensure capacity
			need := termLen + uncompressedLen + lz4DecompressorPadding
			if need > len(td.blockBuf) {
				td.blockBuf = make([]byte, need)
				copy(td.blockBuf[:termLen], td.term)
			}
			_, err = compress.LZ4Decompress(td.bytes, uncompressedLen, td.blockBuf, termLen)
			if err != nil {
				return err
			}
			td.compBStart = offset
			td.compBEnd = td.bytes.GetFilePointer()
			td.blockTermLen = termLen
			td.blockUncLen = uncompressedLen
			// reset block input
			td.blockInput.Reset(td.blockBuf[termLen : termLen+uncompressedLen])
		} else {
			// already decompressed this block — re-seek past it in the bytes stream
			if err := td.bytes.SetPosition(td.compBEnd); err != nil {
				return err
			}
			// reset blockInput to the start of the previously decompressed data
			td.blockInput.Reset(td.blockBuf[td.blockTermLen : td.blockTermLen+td.blockUncLen])
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// VaryingBPVReader
// ---------------------------------------------------------------------------

type varyingBPVReader struct {
	entry          *dvNumericEntry
	slice          store.RandomAccessInput
	jumpTable      store.RandomAccessInput // LE int64 per block; nil when valueJumpTableOffset==-1
	block          int64
	delta          int64
	offset         int64
	blockEndOffset int64
	vals           packed.LongValues
}

func (r *varyingBPVReader) Get(index int64) (int64, error) {
	shift := uint(r.entry.blockShift)
	mask := int64((1 << shift) - 1)
	block := index >> shift
	if r.block != block {
		var bpv byte
		for {
			if r.jumpTable != nil && block != r.block+1 {
				off, err := r.jumpTable.ReadLongAt(block * 8)
				if err == nil {
					r.blockEndOffset = off - r.entry.valuesOffset
					r.block = block - 1
				}
			}
			r.offset = r.blockEndOffset
			b, err := r.slice.ReadByteAt(r.offset)
			if err != nil {
				return 0, fmt.Errorf("lucene90 dv: varyingBPVReader: read byte: %w", err)
			}
			bpv = b
			r.offset++
			d, err := r.slice.ReadLongAt(r.offset)
			if err != nil {
				return 0, fmt.Errorf("lucene90 dv: varyingBPVReader: read delta: %w", err)
			}
			r.delta = d
			r.offset += 8
			if bpv == 0 {
				r.blockEndOffset = r.offset
			} else {
				l, err := r.slice.ReadIntAt(r.offset)
				if err != nil {
					return 0, fmt.Errorf("lucene90 dv: varyingBPVReader: read length: %w", err)
				}
				r.offset += 4
				r.blockEndOffset = r.offset + int64(l)
			}
			r.block++
			if r.block == block {
				break
			}
		}
		if bpv == 0 {
			r.vals = constLongValues(0)
		} else {
			inner, err := packed.GetDirectReaderAt(r.slice, int(bpv), r.offset)
			if err != nil {
				return 0, fmt.Errorf("lucene90 dv: varyingBPVReader: get direct reader: %w", err)
			}
			r.vals = inner
		}
	}
	val, err := r.vals.Get(index & mask)
	if err != nil {
		return 0, err
	}
	return r.entry.gcd*val + r.delta, nil
}

// ---------------------------------------------------------------------------
// NumericDocValues implementations
// ---------------------------------------------------------------------------

// emptyNumericDV returns NO_MORE_DOCS immediately (docsWithFieldOffset == -2).
type emptyNumericDV struct{}

func (emptyNumericDV) DocID() int                      { return dvNoMoreDocs }
func (emptyNumericDV) NextDoc() (int, error)           { return dvNoMoreDocs, nil }
func (emptyNumericDV) Advance(target int) (int, error) { return dvNoMoreDocs, nil }
func (emptyNumericDV) LongValue() (int64, error)       { return 0, nil }
func (emptyNumericDV) Cost() int64                     { return 0 }

// denseConstNumericDV: dense (all docs have a value), bitsPerValue == 0.
type denseConstNumericDV struct {
	maxDoc int
	val    int64
	doc    int
}

func newDenseConstNumericDV(maxDoc int, val int64) *denseConstNumericDV {
	return &denseConstNumericDV{maxDoc: maxDoc, val: val, doc: -1}
}
func (d *denseConstNumericDV) DocID() int { return d.doc }
func (d *denseConstNumericDV) NextDoc() (int, error) {
	d.doc++
	if d.doc >= d.maxDoc {
		d.doc = dvNoMoreDocs
	}
	return d.doc, nil
}
func (d *denseConstNumericDV) Advance(target int) (int, error) {
	if target >= d.maxDoc {
		d.doc = dvNoMoreDocs
		return dvNoMoreDocs, nil
	}
	d.doc = target
	return d.doc, nil
}
func (d *denseConstNumericDV) LongValue() (int64, error) { return d.val, nil }
func (d *denseConstNumericDV) Cost() int64               { return int64(d.maxDoc) }

// denseNumericDV: dense, bitsPerValue > 0.
type denseNumericDV struct {
	maxDoc int
	vals   dvLongValues
	doc    int
}

func (d *denseNumericDV) DocID() int { return d.doc }
func (d *denseNumericDV) NextDoc() (int, error) {
	d.doc++
	if d.doc >= d.maxDoc {
		d.doc = dvNoMoreDocs
	}
	return d.doc, nil
}
func (d *denseNumericDV) Advance(target int) (int, error) {
	if target >= d.maxDoc {
		d.doc = dvNoMoreDocs
		return dvNoMoreDocs, nil
	}
	d.doc = target
	return d.doc, nil
}
func (d *denseNumericDV) LongValue() (int64, error) { return d.vals.Get(int64(d.doc)) }
func (d *denseNumericDV) Cost() int64               { return int64(d.maxDoc) }

// sparseConstNumericDV: sparse (DISI), bitsPerValue == 0.
type sparseConstNumericDV struct {
	disi *dvIndexedDISI
	val  int64
}

func (s *sparseConstNumericDV) DocID() int                 { return s.disi.DocID() }
func (s *sparseConstNumericDV) NextDoc() (int, error)      { return s.disi.NextDoc() }
func (s *sparseConstNumericDV) Advance(t int) (int, error) { return s.disi.Advance(t) }
func (s *sparseConstNumericDV) LongValue() (int64, error)  { return s.val, nil }
func (s *sparseConstNumericDV) Cost() int64                { return s.disi.Cost() }

// sparseNumericDV: sparse, bitsPerValue > 0.
type sparseNumericDV struct {
	disi *dvIndexedDISI
	vals dvLongValues
}

func (s *sparseNumericDV) DocID() int                 { return s.disi.DocID() }
func (s *sparseNumericDV) NextDoc() (int, error)      { return s.disi.NextDoc() }
func (s *sparseNumericDV) Advance(t int) (int, error) { return s.disi.Advance(t) }
func (s *sparseNumericDV) LongValue() (int64, error) {
	v, err := s.vals.Get(int64(s.disi.Index()))
	if err != nil {
		return 0, err
	}
	return v, nil
}
func (s *sparseNumericDV) Cost() int64 { return s.disi.Cost() }

// ---------------------------------------------------------------------------
// BinaryDocValues implementations
// ---------------------------------------------------------------------------

type emptyBinaryDV struct{}

func (emptyBinaryDV) DocID() int                      { return dvNoMoreDocs }
func (emptyBinaryDV) NextDoc() (int, error)           { return dvNoMoreDocs, nil }
func (emptyBinaryDV) Advance(target int) (int, error) { return dvNoMoreDocs, nil }
func (emptyBinaryDV) BinaryValue() ([]byte, error)    { return nil, nil }
func (emptyBinaryDV) Cost() int64                     { return 0 }

type denseFixedBinaryDV struct {
	maxDoc int
	bytes  store.RandomAccessInput
	length int
	doc    int
	buf    []byte
}

func (d *denseFixedBinaryDV) DocID() int { return d.doc }
func (d *denseFixedBinaryDV) NextDoc() (int, error) {
	d.doc++
	if d.doc >= d.maxDoc {
		d.doc = dvNoMoreDocs
	}
	return d.doc, nil
}
func (d *denseFixedBinaryDV) Advance(target int) (int, error) {
	if target >= d.maxDoc {
		d.doc = dvNoMoreDocs
		return dvNoMoreDocs, nil
	}
	d.doc = target
	return d.doc, nil
}
func (d *denseFixedBinaryDV) BinaryValue() ([]byte, error) {
	if cap(d.buf) < d.length {
		d.buf = make([]byte, d.length)
	}
	d.buf = d.buf[:d.length]
	if err := dvReadBytesAt(d.bytes, int64(d.doc)*int64(d.length), d.buf); err != nil {
		return nil, err
	}
	return d.buf, nil
}
func (d *denseFixedBinaryDV) Cost() int64 { return int64(d.maxDoc) }

type denseVarBinaryDV struct {
	maxDoc int
	bytes  store.RandomAccessInput
	addrs  *packed.DirectMonotonicReader
	maxLen int
	doc    int
	buf    []byte
}

func (d *denseVarBinaryDV) DocID() int { return d.doc }
func (d *denseVarBinaryDV) NextDoc() (int, error) {
	d.doc++
	if d.doc >= d.maxDoc {
		d.doc = dvNoMoreDocs
	}
	return d.doc, nil
}
func (d *denseVarBinaryDV) Advance(target int) (int, error) {
	if target >= d.maxDoc {
		d.doc = dvNoMoreDocs
		return dvNoMoreDocs, nil
	}
	d.doc = target
	return d.doc, nil
}
func (d *denseVarBinaryDV) BinaryValue() ([]byte, error) {
	start, err := d.addrs.Get(int64(d.doc))
	if err != nil {
		return nil, err
	}
	endAddr, err := d.addrs.Get(int64(d.doc) + 1)
	if err != nil {
		return nil, err
	}
	length := int(endAddr - start)
	if cap(d.buf) < length {
		d.buf = make([]byte, length)
	}
	d.buf = d.buf[:length]
	if length > 0 {
		if err := dvReadBytesAt(d.bytes, start, d.buf); err != nil {
			return nil, err
		}
	}
	return d.buf, nil
}
func (d *denseVarBinaryDV) Cost() int64 { return int64(d.maxDoc) }

type sparseFixedBinaryDV struct {
	disi   *dvIndexedDISI
	bytes  store.RandomAccessInput
	length int
	buf    []byte
}

func (s *sparseFixedBinaryDV) DocID() int                 { return s.disi.DocID() }
func (s *sparseFixedBinaryDV) NextDoc() (int, error)      { return s.disi.NextDoc() }
func (s *sparseFixedBinaryDV) Advance(t int) (int, error) { return s.disi.Advance(t) }
func (s *sparseFixedBinaryDV) BinaryValue() ([]byte, error) {
	if cap(s.buf) < s.length {
		s.buf = make([]byte, s.length)
	}
	s.buf = s.buf[:s.length]
	if err := dvReadBytesAt(s.bytes, int64(s.disi.Index())*int64(s.length), s.buf); err != nil {
		return nil, err
	}
	return s.buf, nil
}
func (s *sparseFixedBinaryDV) Cost() int64 { return s.disi.Cost() }

type sparseVarBinaryDV struct {
	disi   *dvIndexedDISI
	bytes  store.RandomAccessInput
	addrs  *packed.DirectMonotonicReader
	maxLen int
	buf    []byte
}

func (s *sparseVarBinaryDV) DocID() int                 { return s.disi.DocID() }
func (s *sparseVarBinaryDV) NextDoc() (int, error)      { return s.disi.NextDoc() }
func (s *sparseVarBinaryDV) Advance(t int) (int, error) { return s.disi.Advance(t) }
func (s *sparseVarBinaryDV) BinaryValue() ([]byte, error) {
	idx := int64(s.disi.Index())
	start, err := s.addrs.Get(idx)
	if err != nil {
		return nil, err
	}
	endAddr, err := s.addrs.Get(idx + 1)
	if err != nil {
		return nil, err
	}
	length := int(endAddr - start)
	if cap(s.buf) < length {
		s.buf = make([]byte, length)
	}
	s.buf = s.buf[:length]
	if length > 0 {
		if err := dvReadBytesAt(s.bytes, start, s.buf); err != nil {
			return nil, err
		}
	}
	return s.buf, nil
}
func (s *sparseVarBinaryDV) Cost() int64 { return s.disi.Cost() }

// ---------------------------------------------------------------------------
// SortedDocValues implementations
// ---------------------------------------------------------------------------

// sortedDVBase provides LookupOrd and GetValueCount via dvTermsDict.
type sortedDVBase struct {
	td *dvTermsDict
}

func (b *sortedDVBase) LookupOrd(ord int) ([]byte, error) {
	return b.td.LookupOrd(ord)
}
func (b *sortedDVBase) GetValueCount() int {
	return int(b.td.entry.termsDictSize)
}

type sortedDVDense struct {
	sortedDVBase
	maxDoc int
	vals   packed.LongValues
	doc    int
}

func (s *sortedDVDense) DocID() int { return s.doc }
func (s *sortedDVDense) NextDoc() (int, error) {
	s.doc++
	if s.doc >= s.maxDoc {
		s.doc = dvNoMoreDocs
	}
	return s.doc, nil
}
func (s *sortedDVDense) Advance(t int) (int, error) {
	if t >= s.maxDoc {
		s.doc = dvNoMoreDocs
		return dvNoMoreDocs, nil
	}
	s.doc = t
	return s.doc, nil
}
func (s *sortedDVDense) OrdValue() (int, error) {
	v, err := s.vals.Get(int64(s.doc))
	if err != nil {
		return 0, err
	}
	return int(v), nil
}
func (s *sortedDVDense) LongValue() (int64, error) {
	v, err := s.vals.Get(int64(s.doc))
	if err != nil {
		return 0, err
	}
	return v, nil
}
func (s *sortedDVDense) Cost() int64 { return int64(s.maxDoc) }

type sortedDVSparse struct {
	sortedDVBase
	disi *dvIndexedDISI
	vals packed.LongValues
}

func (s *sortedDVSparse) DocID() int                 { return s.disi.DocID() }
func (s *sortedDVSparse) NextDoc() (int, error)      { return s.disi.NextDoc() }
func (s *sortedDVSparse) Advance(t int) (int, error) { return s.disi.Advance(t) }
func (s *sortedDVSparse) OrdValue() (int, error) {
	v, err := s.vals.Get(int64(s.disi.Index()))
	if err != nil {
		return 0, err
	}
	return int(v), nil
}
func (s *sortedDVSparse) LongValue() (int64, error) {
	v, err := s.vals.Get(int64(s.disi.Index()))
	if err != nil {
		return 0, err
	}
	return v, nil
}
func (s *sortedDVSparse) Cost() int64 { return s.disi.Cost() }

// sortedDVGeneral falls back to the generic numeric reader for ords.
type sortedDVGeneral struct {
	sortedDVBase
	ndv NumericDocValues
}

func (s *sortedDVGeneral) DocID() int                 { return s.ndv.DocID() }
func (s *sortedDVGeneral) NextDoc() (int, error)      { return s.ndv.NextDoc() }
func (s *sortedDVGeneral) Advance(t int) (int, error) { return s.ndv.Advance(t) }
func (s *sortedDVGeneral) LongValue() (int64, error)  { return s.ndv.LongValue() }
func (s *sortedDVGeneral) Cost() int64                { return s.ndv.Cost() }
func (s *sortedDVGeneral) OrdValue() (int, error) {
	v, err := s.ndv.LongValue()
	return int(v), err
}

// ---------------------------------------------------------------------------
// SortedSetDocValues implementations
// ---------------------------------------------------------------------------

// sortedSetDVBase provides LookupOrd and GetValueCount.
type sortedSetDVBase struct {
	td *dvTermsDict
}

func (b *sortedSetDVBase) LookupOrd(ord int) ([]byte, error) {
	return b.td.LookupOrd(ord)
}
func (b *sortedSetDVBase) GetValueCount() int { return int(b.td.entry.termsDictSize) }

type sortedSetDVDense struct {
	sortedSetDVBase
	maxDoc int
	vals   packed.LongValues
	addrs  *packed.DirectMonotonicReader
	doc    int
	curr   int64
	count  int
}

func (s *sortedSetDVDense) DocID() int { return s.doc }
func (s *sortedSetDVDense) NextDoc() (int, error) {
	s.doc++
	if s.doc >= s.maxDoc {
		s.doc = dvNoMoreDocs
		return dvNoMoreDocs, nil
	}
	start, err := s.addrs.Get(int64(s.doc))
	if err != nil {
		return 0, fmt.Errorf("sortedSetDVDense: NextDoc: get addr: %w", err)
	}
	s.curr = start
	endAddr, err := s.addrs.Get(int64(s.doc) + 1)
	if err != nil {
		return 0, fmt.Errorf("sortedSetDVDense: NextDoc: get end addr: %w", err)
	}
	s.count = int(endAddr - s.curr)
	return s.doc, nil
}
func (s *sortedSetDVDense) Advance(t int) (int, error) {
	if t >= s.maxDoc {
		s.doc = dvNoMoreDocs
		return dvNoMoreDocs, nil
	}
	s.doc = t
	start, err := s.addrs.Get(int64(s.doc))
	if err != nil {
		return 0, fmt.Errorf("sortedSetDVDense: Advance: get addr: %w", err)
	}
	s.curr = start
	endAddr, err := s.addrs.Get(int64(s.doc) + 1)
	if err != nil {
		return 0, fmt.Errorf("sortedSetDVDense: Advance: get end addr: %w", err)
	}
	s.count = int(endAddr - s.curr)
	return s.doc, nil
}
func (s *sortedSetDVDense) NextOrd() (int, error) {
	start, err := s.addrs.Get(int64(s.doc))
	if err != nil {
		return -1, err
	}
	if s.curr >= start+int64(s.count) {
		return -1, nil
	}
	v, err := s.vals.Get(s.curr)
	if err != nil {
		return -1, err
	}
	s.curr++
	return int(v), nil
}
func (s *sortedSetDVDense) DocValueCount() int { return s.count }
func (s *sortedSetDVDense) Cost() int64        { return int64(s.maxDoc) }

type sortedSetDVSparse struct {
	sortedSetDVBase
	disi  *dvIndexedDISI
	vals  packed.LongValues
	addrs *packed.DirectMonotonicReader
	set   bool
	curr  int64
	count int
}

func (s *sortedSetDVSparse) DocID() int                 { return s.disi.DocID() }
func (s *sortedSetDVSparse) NextDoc() (int, error)      { s.set = false; return s.disi.NextDoc() }
func (s *sortedSetDVSparse) Advance(t int) (int, error) { s.set = false; return s.disi.Advance(t) }
func (s *sortedSetDVSparse) setIfNeeded() error {
	if !s.set {
		idx := int64(s.disi.Index())
		start, err := s.addrs.Get(idx)
		if err != nil {
			return fmt.Errorf("sortedSetDVSparse: setIfNeeded: get addr: %w", err)
		}
		s.curr = start
		endAddr, err := s.addrs.Get(idx + 1)
		if err != nil {
			return fmt.Errorf("sortedSetDVSparse: setIfNeeded: get end addr: %w", err)
		}
		s.count = int(endAddr - s.curr)
		s.set = true
	}
	return nil
}
func (s *sortedSetDVSparse) NextOrd() (int, error) {
	if err := s.setIfNeeded(); err != nil {
		return -1, err
	}
	idx := int64(s.disi.Index())
	end, err := s.addrs.Get(idx + 1)
	if err != nil {
		return -1, err
	}
	if s.curr >= end {
		return -1, nil
	}
	v, err := s.vals.Get(s.curr)
	if err != nil {
		return -1, err
	}
	s.curr++
	return int(v), nil
}
func (s *sortedSetDVSparse) DocValueCount() int {
	if err := s.setIfNeeded(); err != nil {
		// This path is unreachable in practice; panic retains the
		// Lucene RuntimeException semantics for I/O errors in
		// non-error-returning contexts matching the util.Bits pattern.
		panic(fmt.Sprintf("sortedSetDVSparse: DocValueCount: %v", err))
	}
	return s.count
}
func (s *sortedSetDVSparse) Cost() int64        { return s.disi.Cost() }

// sortedSetDVGeneral wraps a SortedNumericDocValues for multi-valued case.
type sortedSetDVGeneral struct {
	sortedSetDVBase
	sndv     SortedNumericDocValues
	consumed int // ords consumed for the current doc
	count    int // total ords for the current doc (0 until NextDoc called)
}

func (s *sortedSetDVGeneral) DocID() int { return s.sndv.DocID() }
func (s *sortedSetDVGeneral) NextDoc() (int, error) {
	doc, err := s.sndv.NextDoc()
	if err != nil || doc == dvNoMoreDocs {
		return doc, err
	}
	c, err := s.sndv.DocValueCount()
	if err != nil {
		return 0, err
	}
	s.count = c
	s.consumed = 0
	return doc, nil
}
func (s *sortedSetDVGeneral) Advance(t int) (int, error) {
	doc, err := s.sndv.Advance(t)
	if err != nil || doc == dvNoMoreDocs {
		return doc, err
	}
	c, err := s.sndv.DocValueCount()
	if err != nil {
		return 0, err
	}
	s.count = c
	s.consumed = 0
	return doc, nil
}
func (s *sortedSetDVGeneral) NextOrd() (int, error) {
	if s.consumed >= s.count {
		return -1, nil
	}
	v, err := s.sndv.NextValue()
	if err != nil {
		return -1, err
	}
	s.consumed++
	return int(v), nil
}
func (s *sortedSetDVGeneral) DocValueCount() int { return s.count }
func (s *sortedSetDVGeneral) Cost() int64        { return s.sndv.Cost() }

// singletonSortedSet wraps a SortedDocValues as a SortedSetDocValues.
func singletonSortedSet(sdv SortedDocValues) SortedSetDocValues {
	return &singletonSS{sdv: sdv}
}

type singletonSS struct {
	sdv         SortedDocValues
	ordConsumed bool
}

func (s *singletonSS) DocID() int                 { return s.sdv.DocID() }
func (s *singletonSS) NextDoc() (int, error)      { s.ordConsumed = false; return s.sdv.NextDoc() }
func (s *singletonSS) Advance(t int) (int, error) { s.ordConsumed = false; return s.sdv.Advance(t) }
func (s *singletonSS) NextOrd() (int, error) {
	if s.ordConsumed {
		return -1, nil
	}
	s.ordConsumed = true
	return s.sdv.OrdValue()
}
func (s *singletonSS) LookupOrd(ord int) ([]byte, error) { return s.sdv.LookupOrd(ord) }
func (s *singletonSS) GetValueCount() int                { return s.sdv.GetValueCount() }
func (s *singletonSS) DocValueCount() int                { return 1 }
func (s *singletonSS) Cost() int64                       { return s.sdv.Cost() }

// ---------------------------------------------------------------------------
// SortedNumericDocValues implementations
// ---------------------------------------------------------------------------

// singletonSortedNumeric wraps a NumericDocValues as single-value SortedNumericDocValues.
func singletonSortedNumeric(ndv NumericDocValues) SortedNumericDocValues {
	return &singletonSN{ndv: ndv}
}

type singletonSN struct {
	ndv         NumericDocValues
	valConsumed bool
}

func (s *singletonSN) DocID() int                  { return s.ndv.DocID() }
func (s *singletonSN) NextDoc() (int, error)       { s.valConsumed = false; return s.ndv.NextDoc() }
func (s *singletonSN) Advance(t int) (int, error)  { s.valConsumed = false; return s.ndv.Advance(t) }
func (s *singletonSN) LongValue() (int64, error)   { return s.ndv.LongValue() }
func (s *singletonSN) NextValue() (int64, error)   { return s.ndv.LongValue() }
func (s *singletonSN) DocValueCount() (int, error) { return 1, nil }
func (s *singletonSN) Cost() int64                 { return s.ndv.Cost() }

type sortedNumericDVDense struct {
	maxDoc int
	addrs  *packed.DirectMonotonicReader
	vals   dvLongValues
	doc    int
	start  int64
	end    int64
	count  int
}

func (d *sortedNumericDVDense) DocID() int { return d.doc }
func (d *sortedNumericDVDense) NextDoc() (int, error) {
	d.doc++
	if d.doc >= d.maxDoc {
		d.doc = dvNoMoreDocs
		return dvNoMoreDocs, nil
	}
	start, err := d.addrs.Get(int64(d.doc))
	if err != nil {
		return 0, fmt.Errorf("sortedNumericDVDense: NextDoc: get addr: %w", err)
	}
	d.start = start
	endVal, err := d.addrs.Get(int64(d.doc) + 1)
	if err != nil {
		return 0, fmt.Errorf("sortedNumericDVDense: NextDoc: get end addr: %w", err)
	}
	d.end = endVal
	d.count = int(d.end - d.start)
	return d.doc, nil
}
func (d *sortedNumericDVDense) Advance(t int) (int, error) {
	if t >= d.maxDoc {
		d.doc = dvNoMoreDocs
		return dvNoMoreDocs, nil
	}
	d.doc = t
	start, err := d.addrs.Get(int64(d.doc))
	if err != nil {
		return 0, fmt.Errorf("sortedNumericDVDense: Advance: get addr: %w", err)
	}
	d.start = start
	endVal, err := d.addrs.Get(int64(d.doc) + 1)
	if err != nil {
		return 0, fmt.Errorf("sortedNumericDVDense: Advance: get end addr: %w", err)
	}
	d.end = endVal
	d.count = int(d.end - d.start)
	return d.doc, nil
}
func (d *sortedNumericDVDense) NextValue() (int64, error) {
	v, err := d.vals.Get(d.start)
	if err != nil {
		return 0, fmt.Errorf("sortedNumericDVDense: NextValue: %w", err)
	}
	d.start++
	return v, nil
}
func (d *sortedNumericDVDense) LongValue() (int64, error)   { return d.NextValue() }
func (d *sortedNumericDVDense) DocValueCount() (int, error) { return d.count, nil }
func (d *sortedNumericDVDense) Cost() int64                 { return int64(d.maxDoc) }

type sortedNumericDVSparse struct {
	disi  *dvIndexedDISI
	addrs *packed.DirectMonotonicReader
	vals  dvLongValues
	set   bool
	start int64
	count int
}

func (s *sortedNumericDVSparse) DocID() int                 { return s.disi.DocID() }
func (s *sortedNumericDVSparse) NextDoc() (int, error)      { s.set = false; return s.disi.NextDoc() }
func (s *sortedNumericDVSparse) Advance(t int) (int, error) { s.set = false; return s.disi.Advance(t) }
func (s *sortedNumericDVSparse) setIfNeeded() error {
	if !s.set {
		idx := int64(s.disi.Index())
		start, err := s.addrs.Get(idx)
		if err != nil {
			return fmt.Errorf("sortedNumericDVSparse: setIfNeeded: get addr: %w", err)
		}
		s.start = start
		endAddr, err := s.addrs.Get(idx + 1)
		if err != nil {
			return fmt.Errorf("sortedNumericDVSparse: setIfNeeded: get end addr: %w", err)
		}
		s.count = int(endAddr - s.start)
		s.set = true
	}
	return nil
}
func (s *sortedNumericDVSparse) NextValue() (int64, error) {
	if err := s.setIfNeeded(); err != nil {
		return 0, err
	}
	v, err := s.vals.Get(s.start)
	if err != nil {
		return 0, err
	}
	s.start++
	return v, nil
}
func (s *sortedNumericDVSparse) LongValue() (int64, error)   { return s.NextValue() }
func (s *sortedNumericDVSparse) DocValueCount() (int, error) {
	if err := s.setIfNeeded(); err != nil {
		return 0, err
	}
	return s.count, nil
}
func (s *sortedNumericDVSparse) Cost() int64                 { return s.disi.Cost() }

// ---------------------------------------------------------------------------
// Utility interfaces and helpers
// ---------------------------------------------------------------------------

// dvLongValues is a random-access long value array (subset of packed.LongValues).
type dvLongValues interface {
	Get(index int64) (int64, error)
}

// constLongValues returns a fixed value for any index.
func constLongValues(val int64) dvLongValues { return constLV(val) }

type constLV int64

func (c constLV) Get(_ int64) (int64, error) { return int64(c), nil }

// funcLongValues wraps a function as dvLongValues.
type funcLongValues func(index int64) (int64, error)

func (f funcLongValues) Get(index int64) (int64, error) { return f(index) }

// dvReadBytesAt reads length bytes at the given absolute position from a
// RandomAccessInput, using ReadByteAt since RandomAccessInput has no ReadBytesAt.
func dvReadBytesAt(in store.RandomAccessInput, pos int64, buf []byte) error {
	for i := range buf {
		b, err := in.ReadByteAt(pos + int64(i))
		if err != nil {
			return err
		}
		buf[i] = b
	}
	return nil
}

// dvSliceRandomAccess loads a byte region from data into a RandomAccessInput.
// It tries a type-assert first (NIO-backed inputs), then loads into memory.
func dvSliceRandomAccess(data store.IndexInput, offset, length int64) (store.RandomAccessInput, error) {
	if length == 0 {
		return store.NewByteArrayRandomAccessInput(nil), nil
	}
	sub, err := data.Slice("dv-slice", offset, length)
	if err != nil {
		return nil, err
	}
	// type-assert first
	if ra, ok := sub.(store.RandomAccessInput); ok {
		return ra, nil
	}
	// fall back: read into memory
	buf := make([]byte, length)
	if err := sub.ReadBytes(buf); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("dvSliceRandomAccess: read %d bytes at %d: %w", length, offset, err)
	}
	return store.NewByteArrayRandomAccessInput(buf), nil
}
