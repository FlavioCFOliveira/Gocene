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
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// Lucene 9.0 norms format constants. Per Lucene 10.4.0 source, the format
// is two files:
//
//   - .nvd  (DataExtension):     IndexHeader || per-field { IndexedDISI ||
//     NumDocsWithField * BytesPerValue bytes } || Footer.
//   - .nvm  (MetadataExtension): IndexHeader || per-field { FieldNumber,
//     DocsWithFieldOffset, DocsWithFieldLength, JumpTableEntryCount,
//     DenseRankPower, NumDocsWithField, BytesPerNorm, NormsOffset } ||
//     -1 sentinel FieldNumber || Footer.
//
// All multi-byte integers are little-endian on the Lucene 10.x wire.
const (
	// Lucene90NormsDataCodec is the codec name stamped into the .nvd
	// IndexHeader.
	Lucene90NormsDataCodec = "Lucene90NormsData"
	// Lucene90NormsDataExtension is the file extension for the per-segment
	// norms data file.
	Lucene90NormsDataExtension = "nvd"
	// Lucene90NormsMetadataCodec is the codec name stamped into the .nvm
	// IndexHeader.
	Lucene90NormsMetadataCodec = "Lucene90NormsMetadata"
	// Lucene90NormsMetadataExtension is the file extension for the
	// per-segment norms metadata file.
	Lucene90NormsMetadataExtension = "nvm"
	// Lucene90NormsVersionStart is the inclusive minimum supported version.
	Lucene90NormsVersionStart int32 = 0
	// Lucene90NormsVersionCurrent is the current format version.
	Lucene90NormsVersionCurrent int32 = Lucene90NormsVersionStart
)

// Lucene90NormsFormat is the Lucene 9.0 norms format.
//
// This is the Go port of
// org.apache.lucene.codecs.lucene90.Lucene90NormsFormat (Lucene 10.4.0).
// The consumer (Lucene90NormsConsumer) and producer (Lucene90NormsProducer)
// are byte-faithful with the reference: the .nvd/.nvm framing, the per-field
// metadata layout, the IndexedDISI docs-with-field jump table, and the
// fixed-width per-document norm encoding all match Lucene 10.4.0 exactly.
type Lucene90NormsFormat struct {
	*BaseNormsFormat
}

// NewLucene90NormsFormat creates a new Lucene90NormsFormat.
func NewLucene90NormsFormat() *Lucene90NormsFormat {
	return &Lucene90NormsFormat{
		BaseNormsFormat: NewBaseNormsFormat("Lucene90NormsFormat"),
	}
}

// NormsConsumer returns a consumer for writing norms.
func (f *Lucene90NormsFormat) NormsConsumer(state *SegmentWriteState) (NormsConsumer, error) {
	return NewLucene90NormsConsumer(state)
}

// NormsProducer returns a producer for reading norms.
func (f *Lucene90NormsFormat) NormsProducer(state *SegmentReadState) (NormsProducer, error) {
	return NewLucene90NormsProducer(state)
}

// -----------------------------------------------------------------------------
// Consumer
// -----------------------------------------------------------------------------

// Lucene90NormsConsumer writes norms in Lucene 9.0 format.
//
// Go port of org.apache.lucene.codecs.lucene90.Lucene90NormsConsumer.
// The .nvd data file and .nvm metadata file are opened (with their
// IndexHeaders) at construction; AddNormsField appends one per-field record
// to each, interleaving exactly as the Java reference does; Close writes the
// -1 EOF marker into .nvm and the CodecUtil footer into both files.
type Lucene90NormsConsumer struct {
	state  *SegmentWriteState
	data   *store.ChecksumIndexOutput
	meta   *store.ChecksumIndexOutput
	maxDoc int
	closed bool
}

// NewLucene90NormsConsumer creates a new Lucene90NormsConsumer, opening the
// .nvd and .nvm outputs and writing their IndexHeaders. On any failure the
// partially-opened outputs are closed before returning.
func NewLucene90NormsConsumer(state *SegmentWriteState) (*Lucene90NormsConsumer, error) {
	c := &Lucene90NormsConsumer{
		state:  state,
		maxDoc: state.SegmentInfo.DocCount(),
	}

	id := state.SegmentInfo.GetID()

	dataName := normsSegmentFileName(state, Lucene90NormsDataExtension)
	data, err := openNormsOutput(state, dataName, Lucene90NormsDataCodec, id)
	if err != nil {
		return nil, err
	}

	metaName := normsSegmentFileName(state, Lucene90NormsMetadataExtension)
	meta, err := openNormsOutput(state, metaName, Lucene90NormsMetadataCodec, id)
	if err != nil {
		_ = data.Close()
		return nil, err
	}

	c.data = data
	c.meta = meta
	return c, nil
}

// openNormsOutput creates an output, wraps it in a ChecksumIndexOutput, and
// writes the CodecUtil IndexHeader.
func openNormsOutput(state *SegmentWriteState, name, codec string, id []byte) (*store.ChecksumIndexOutput, error) {
	raw, err := state.Directory.CreateOutput(name, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		return nil, fmt.Errorf("lucene90 norms: create %q: %w", name, err)
	}
	out := store.NewChecksumIndexOutput(raw)
	if err := WriteIndexHeader(out, codec, Lucene90NormsVersionCurrent, id, state.SegmentSuffix); err != nil {
		_ = out.Close()
		return nil, fmt.Errorf("lucene90 norms: header %q: %w", name, err)
	}
	return out, nil
}

// normsSegmentFileName returns the per-segment file name for the given norms
// extension, honouring the optional SegmentSuffix.
func normsSegmentFileName(state *SegmentWriteState, extension string) string {
	if state.SegmentSuffix == "" {
		return state.SegmentInfo.Name() + "." + extension
	}
	return state.SegmentInfo.Name() + "_" + state.SegmentSuffix + "." + extension
}

// AddNormsField writes one norms field to the .nvd/.nvm pair.
//
// Mirrors Lucene90NormsConsumer.addNormsField (Lucene 10.4.0, lines 90-138).
// The Java reference re-iterates the NumericDocValues three times (once for
// min/max, once for the IndexedDISI bitset, once for the values) by calling
// normsProducer.getNorms(field) each time. Gocene's NormsIterator is a
// single-pass cursor, so we materialise the (docID, value) pairs once and
// drive all three phases from the buffered slices. The on-disk bytes are
// identical regardless of how many times the source is scanned.
func (c *Lucene90NormsConsumer) AddNormsField(field *index.FieldInfo, values NormsIterator) error {
	if c.closed {
		return errors.New("lucene90 norms: consumer closed")
	}

	// Phase 0: materialise the value-bearing documents and compute min/max.
	// Lucene initialises min=Long.MAX_VALUE, max=Long.MIN_VALUE so that the
	// empty-field case (numDocsWithValue==0) yields min>max => bytesPerValue
	// 0, written as a constant min=Long.MAX_VALUE that the reader never
	// consults (docsWithFieldOffset==-2). We mirror that exactly.
	var docs []int
	var vals []int64
	var min int64 = math.MaxInt64
	var max int64 = math.MinInt64
	for values.Next() {
		doc := values.DocID()
		v := values.LongValue()
		docs = append(docs, doc)
		vals = append(vals, v)
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	numDocsWithValue := len(docs)
	if numDocsWithValue > c.maxDoc {
		return fmt.Errorf("lucene90 norms: numDocsWithValue=%d exceeds maxDoc=%d", numDocsWithValue, c.maxDoc)
	}

	// meta.writeInt(field.number) — Lucene90NormsConsumer line 104.
	if err := c.meta.WriteInt(int32(field.Number())); err != nil {
		return err
	}

	// docsWithField handling — Lucene90NormsConsumer lines 106-125.
	switch {
	case numDocsWithValue == 0:
		if err := c.meta.WriteLong(-2); err != nil { // docsWithFieldOffset
			return err
		}
		if err := c.meta.WriteLong(0); err != nil { // docsWithFieldLength
			return err
		}
		if err := c.meta.WriteShort(-1); err != nil { // jumpTableEntryCount
			return err
		}
		if err := c.meta.WriteByte(0xFF); err != nil { // denseRankPower == (byte) -1
			return err
		}
	case numDocsWithValue == c.maxDoc:
		if err := c.meta.WriteLong(-1); err != nil { // docsWithFieldOffset
			return err
		}
		if err := c.meta.WriteLong(0); err != nil { // docsWithFieldLength
			return err
		}
		if err := c.meta.WriteShort(-1); err != nil { // jumpTableEntryCount
			return err
		}
		if err := c.meta.WriteByte(0xFF); err != nil { // denseRankPower == (byte) -1
			return err
		}
	default:
		offset := c.data.GetFilePointer()
		if err := c.meta.WriteLong(offset); err != nil { // docsWithFieldOffset
			return err
		}
		jumpTableEntryCount, err := writeDVBitSet(newNormsDocsIterator(docs, c.maxDoc), c.data)
		if err != nil {
			return err
		}
		if err := c.meta.WriteLong(c.data.GetFilePointer() - offset); err != nil { // docsWithFieldLength
			return err
		}
		if err := c.meta.WriteShort(jumpTableEntryCount); err != nil {
			return err
		}
		if err := c.meta.WriteByte(dvDefaultDenseRankPower); err != nil {
			return err
		}
	}

	// meta.writeInt(numDocsWithValue) — line 127.
	if err := c.meta.WriteInt(int32(numDocsWithValue)); err != nil {
		return err
	}

	// numBytesPerValue and the values block — lines 128-137.
	numBytesPerValue := normsNumBytesPerValue(min, max)
	if err := c.meta.WriteByte(byte(numBytesPerValue)); err != nil {
		return err
	}
	if numBytesPerValue == 0 {
		// Constant field: the min value is stored in the metadata in place of
		// the normsOffset; no bytes are written to the data file.
		return c.meta.WriteLong(min)
	}
	// meta.writeLong(data.getFilePointer()) — normsOffset (line 134).
	if err := c.meta.WriteLong(c.data.GetFilePointer()); err != nil {
		return err
	}
	return writeNormsValues(vals, numBytesPerValue, c.data)
}

// normsNumBytesPerValue picks the per-value width from the value range,
// exactly matching Lucene90NormsConsumer.numBytesPerValue (lines 140-152):
// 0 for a constant field, else the narrowest of 1/2/4/8 bytes that holds the
// signed range [min, max].
func normsNumBytesPerValue(min, max int64) int {
	switch {
	case min >= max:
		return 0
	case min >= math.MinInt8 && max <= math.MaxInt8:
		return 1
	case min >= math.MinInt16 && max <= math.MaxInt16:
		return 2
	case min >= math.MinInt32 && max <= math.MaxInt32:
		return 4
	default:
		return 8
	}
}

// writeNormsValues writes each per-document norm at the chosen fixed width,
// big-endian-free (Gocene's IndexOutput is little-endian on the wire, like
// Lucene 10.x DataOutput). Mirrors Lucene90NormsConsumer.writeValues
// (lines 154-175). Note that Lucene writes the raw value, NOT a delta from
// min; the min is only used to size the field, never subtracted.
func writeNormsValues(vals []int64, numBytesPerValue int, out store.IndexOutput) error {
	switch numBytesPerValue {
	case 1:
		for _, v := range vals {
			if err := out.WriteByte(byte(v)); err != nil {
				return err
			}
		}
	case 2:
		for _, v := range vals {
			if err := out.WriteShort(int16(v)); err != nil {
				return err
			}
		}
	case 4:
		for _, v := range vals {
			if err := out.WriteInt(int32(v)); err != nil {
				return err
			}
		}
	case 8:
		for _, v := range vals {
			if err := out.WriteLong(v); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("lucene90 norms: invalid numBytesPerValue=%d", numBytesPerValue)
	}
	return nil
}

// Close writes the -1 EOF marker into .nvm and the CodecUtil footer into both
// files, then closes them. Mirrors Lucene90NormsConsumer.close (lines 68-88).
func (c *Lucene90NormsConsumer) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true

	var firstErr error
	record := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	if c.meta != nil {
		record(c.meta.WriteInt(-1)) // EOF marker — line 73.
		record(WriteFooter(c.meta))
		record(c.meta.Close())
	}
	if c.data != nil {
		record(WriteFooter(c.data))
		record(c.data.Close())
	}
	c.meta = nil
	c.data = nil
	return firstErr
}

// normsDocsIterator adapts the buffered, ascending docID slice into the
// dvDocIDIterator surface consumed by writeDVBitSet (which only ever calls
// DocID and NextDoc). The norms source visits documents in ascending docID
// order, so the slice is already sorted.
type normsDocsIterator struct {
	docs []int
	pos  int
	doc  int
}

func newNormsDocsIterator(docs []int, _ int) *normsDocsIterator {
	return &normsDocsIterator{docs: docs, pos: 0, doc: -1}
}

func (it *normsDocsIterator) DocID() int { return it.doc }

func (it *normsDocsIterator) NextDoc() (int, error) {
	if it.pos >= len(it.docs) {
		it.doc = math.MaxInt32
		return it.doc, nil
	}
	it.doc = it.docs[it.pos]
	it.pos++
	return it.doc, nil
}

// -----------------------------------------------------------------------------
// Producer
// -----------------------------------------------------------------------------

// normsEntry is the per-field metadata record decoded from the .nvm file.
// Mirrors Lucene90NormsProducer.NormsEntry (Lucene 10.4.0, lines 132-140).
type normsEntry struct {
	denseRankPower      byte
	bytesPerNorm        byte
	docsWithFieldOffset int64
	docsWithFieldLength int64
	jumpTableEntryCount int16
	numDocsWithField    int32
	normsOffset         int64
}

// Lucene90NormsProducer reads norms in Lucene 9.0 format.
//
// Go port of org.apache.lucene.codecs.lucene90.Lucene90NormsProducer. The
// .nvm metadata entries are decoded in the constructor; the .nvd data file is
// kept open for the lifetime of the producer and consulted lazily by
// GetNorms.
type Lucene90NormsProducer struct {
	state  *SegmentReadState
	maxDoc int
	norms  map[int]*normsEntry
	data   store.IndexInput
	closed bool
}

// NewLucene90NormsProducer creates a new Lucene90NormsProducer, decoding the
// .nvm entries and validating both headers/footers. Returns an error if the
// framing is corrupt or the metadata references an unknown field.
func NewLucene90NormsProducer(state *SegmentReadState) (*Lucene90NormsProducer, error) {
	p := &Lucene90NormsProducer{
		state:  state,
		maxDoc: state.SegmentInfo.DocCount(),
		norms:  make(map[int]*normsEntry),
	}
	id := state.SegmentInfo.GetID()

	metaName := normsReadFileName(state, Lucene90NormsMetadataExtension)
	dataName := normsReadFileName(state, Lucene90NormsDataExtension)

	// Read the metadata file under a checksum input so the footer can be
	// verified. Mirrors the openChecksumInput / checkFooter dance.
	metaIn, err := state.Directory.OpenInput(metaName, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return nil, fmt.Errorf("lucene90 norms: open %q: %w", metaName, err)
	}
	csMeta := store.NewChecksumIndexInput(metaIn)
	metaVersion, err := CheckIndexHeader(csMeta, Lucene90NormsMetadataCodec, Lucene90NormsVersionStart, Lucene90NormsVersionCurrent, id, state.SegmentSuffix)
	if err != nil {
		_ = metaIn.Close()
		return nil, fmt.Errorf("lucene90 norms: meta header %q: %w", metaName, err)
	}
	if err := p.readFields(csMeta); err != nil {
		_ = metaIn.Close()
		return nil, err
	}
	if _, err := CheckFooter(csMeta); err != nil {
		_ = metaIn.Close()
		return nil, fmt.Errorf("lucene90 norms: meta footer %q: %w", metaName, err)
	}
	if err := metaIn.Close(); err != nil {
		return nil, err
	}

	// Open the data file, validate its header (version must match the meta
	// file) and the structural footer.
	dataIn, err := state.Directory.OpenInput(dataName, store.IOContext{Context: store.ContextRead})
	if err != nil {
		return nil, fmt.Errorf("lucene90 norms: open %q: %w", dataName, err)
	}
	dataVersion, err := CheckIndexHeader(dataIn, Lucene90NormsDataCodec, Lucene90NormsVersionStart, Lucene90NormsVersionCurrent, id, state.SegmentSuffix)
	if err != nil {
		_ = dataIn.Close()
		return nil, fmt.Errorf("lucene90 norms: data header %q: %w", dataName, err)
	}
	if metaVersion != dataVersion {
		_ = dataIn.Close()
		return nil, fmt.Errorf("lucene90 norms: format versions mismatch: meta=%d data=%d", metaVersion, dataVersion)
	}
	if _, err := RetrieveChecksum(dataIn); err != nil {
		_ = dataIn.Close()
		return nil, fmt.Errorf("lucene90 norms: data footer %q: %w", dataName, err)
	}

	p.data = dataIn
	return p, nil
}

// normsReadFileName returns the per-segment file name for the given norms
// extension on the read side.
func normsReadFileName(state *SegmentReadState, extension string) string {
	if state.SegmentSuffix == "" {
		return state.SegmentInfo.Name() + "." + extension
	}
	return state.SegmentInfo.Name() + "_" + state.SegmentSuffix + "." + extension
}

// readFields decodes the per-field metadata entries until the -1 EOF marker.
// Mirrors Lucene90NormsProducer.readFields (lines 215-244).
func (p *Lucene90NormsProducer) readFields(in store.IndexInput) error {
	for {
		fieldNumber, err := in.ReadInt()
		if err != nil {
			return fmt.Errorf("lucene90 norms: read field number: %w", err)
		}
		if fieldNumber == -1 {
			return nil
		}
		info := p.state.FieldInfos.GetByNumber(int(fieldNumber))
		if info == nil {
			return fmt.Errorf("lucene90 norms: invalid field number: %d", fieldNumber)
		}
		if !info.HasNorms() {
			return fmt.Errorf("lucene90 norms: invalid field (no norms): %s", info.Name())
		}
		entry := &normsEntry{}
		if entry.docsWithFieldOffset, err = in.ReadLong(); err != nil {
			return err
		}
		if entry.docsWithFieldLength, err = in.ReadLong(); err != nil {
			return err
		}
		if entry.jumpTableEntryCount, err = in.ReadShort(); err != nil {
			return err
		}
		var rankByte byte
		if rankByte, err = in.ReadByte(); err != nil {
			return err
		}
		entry.denseRankPower = rankByte
		if entry.numDocsWithField, err = in.ReadInt(); err != nil {
			return err
		}
		if entry.bytesPerNorm, err = in.ReadByte(); err != nil {
			return err
		}
		switch entry.bytesPerNorm {
		case 0, 1, 2, 4, 8:
		default:
			return fmt.Errorf("lucene90 norms: invalid bytesPerValue: %d, field: %s", entry.bytesPerNorm, info.Name())
		}
		if entry.normsOffset, err = in.ReadLong(); err != nil {
			return err
		}
		p.norms[info.Number()] = entry
	}
}

// GetNorms returns a NumericDocValues over the norms of the given field.
// Mirrors Lucene90NormsProducer.getNorms (lines 377-498): empty fields yield
// an empty iterator, dense fields a DenseNormsIterator, and sparse fields a
// SparseNormsIterator backed by an IndexedDISI. The bytesPerNorm==0 case is a
// constant field whose value is stored in normsOffset.
func (p *Lucene90NormsProducer) GetNorms(field *index.FieldInfo) (NumericDocValues, error) {
	if p.closed {
		return nil, errors.New("lucene90 norms: producer closed")
	}
	entry, ok := p.norms[field.Number()]
	if !ok {
		return nil, nil
	}

	switch entry.docsWithFieldOffset {
	case -2:
		// empty
		return &emptyNormsIterator{}, nil
	case -1:
		// dense
		if entry.bytesPerNorm == 0 {
			return newDenseNormsIterator(p.maxDoc, &constNormsValues{value: entry.normsOffset}), nil
		}
		slice, err := normsDataSlice(p.data, entry)
		if err != nil {
			return nil, err
		}
		return newDenseNormsIterator(p.maxDoc, &widthNormsValues{slice: slice, bytesPerNorm: entry.bytesPerNorm}), nil
	default:
		// sparse
		disi, err := newDVIndexedDISI(p.data, entry.docsWithFieldOffset, entry.docsWithFieldLength,
			int(entry.jumpTableEntryCount), entry.denseRankPower, int64(entry.numDocsWithField))
		if err != nil {
			return nil, err
		}
		if entry.bytesPerNorm == 0 {
			return &sparseNormsIterator{disi: disi, values: &constNormsValues{value: entry.normsOffset}}, nil
		}
		slice, err := normsDataSlice(p.data, entry)
		if err != nil {
			return nil, err
		}
		return &sparseNormsIterator{disi: disi, values: &widthNormsValues{slice: slice, bytesPerNorm: entry.bytesPerNorm}}, nil
	}
}

// normsDataSlice returns a RandomAccessInput over the per-document norm bytes,
// covering numDocsWithField * bytesPerNorm bytes at normsOffset. Mirrors
// Lucene90NormsProducer.getDataInput (lines 246-265).
func normsDataSlice(data store.IndexInput, entry *normsEntry) (store.RandomAccessInput, error) {
	length := int64(entry.numDocsWithField) * int64(entry.bytesPerNorm)
	if length == 0 {
		return store.NewByteArrayRandomAccessInput(nil), nil
	}
	sub, err := data.Slice("norms-data", entry.normsOffset, length)
	if err != nil {
		return nil, err
	}
	if ra, ok := sub.(store.RandomAccessInput); ok {
		return ra, nil
	}
	buf := make([]byte, length)
	if err := sub.ReadBytes(buf); err != nil {
		return nil, fmt.Errorf("lucene90 norms: read %d data bytes at %d: %w", length, entry.normsOffset, err)
	}
	return store.NewByteArrayRandomAccessInput(buf), nil
}

// CheckIntegrity verifies the .nvd checksum over the entire file. Mirrors
// Lucene90NormsProducer.checkIntegrity (line 506).
func (p *Lucene90NormsProducer) CheckIntegrity() error {
	if p.closed || p.data == nil {
		return nil
	}
	_, err := ChecksumEntireFile(p.data)
	return err
}

// Close releases the .nvd input.
func (p *Lucene90NormsProducer) Close() error {
	if p.closed {
		return nil
	}
	p.closed = true
	if p.data != nil {
		err := p.data.Close()
		p.data = nil
		return err
	}
	return nil
}

// -----------------------------------------------------------------------------
// Producer value iterators
// -----------------------------------------------------------------------------

// normsValueReader resolves a norm value given the current iterator position.
// The two concrete implementations are a constant value (bytesPerNorm==0) and
// a fixed-width slice read indexed by the document ordinal.
type normsValueReader interface {
	// valueAt returns the norm for the document whose dense ordinal (index)
	// is ord. For dense iterators ord==docID; for sparse iterators ord is the
	// IndexedDISI index().
	valueAt(ord int) (int64, error)
}

// constNormsValues returns the same constant value for every document
// (bytesPerNorm==0). Mirrors the anonymous DenseNormsIterator/
// SparseNormsIterator subclasses returning entry.normsOffset.
type constNormsValues struct {
	value int64
}

func (c *constNormsValues) valueAt(_ int) (int64, error) { return c.value, nil }

// widthNormsValues reads the norm at the chosen fixed width from the data
// slice. The byte offset is ord<<log2(bytesPerNorm), matching
// Lucene90NormsProducer's slice.readByte(ord) / readShort(ord<<1) /
// readInt(ord<<2) / readLong(ord<<3).
type widthNormsValues struct {
	slice        store.RandomAccessInput
	bytesPerNorm byte
}

func (w *widthNormsValues) valueAt(ord int) (int64, error) {
	switch w.bytesPerNorm {
	case 1:
		b, err := w.slice.ReadByteAt(int64(ord))
		if err != nil {
			return 0, err
		}
		return int64(int8(b)), nil
	case 2:
		v, err := w.slice.ReadShortAt(int64(ord) << 1)
		if err != nil {
			return 0, err
		}
		return int64(v), nil
	case 4:
		v, err := w.slice.ReadIntAt(int64(ord) << 2)
		if err != nil {
			return 0, err
		}
		return int64(v), nil
	case 8:
		v, err := w.slice.ReadLongAt(int64(ord) << 3)
		if err != nil {
			return 0, err
		}
		return v, nil
	default:
		return 0, fmt.Errorf("lucene90 norms: invalid bytesPerNorm=%d", w.bytesPerNorm)
	}
}

// emptyNormsIterator is the NumericDocValues returned for empty norms fields.
type emptyNormsIterator struct{}

func (e *emptyNormsIterator) DocID() int                     { return math.MaxInt32 }
func (e *emptyNormsIterator) NextDoc() (int, error)          { return math.MaxInt32, nil }
func (e *emptyNormsIterator) Advance(int) (int, error)       { return math.MaxInt32, nil }
func (e *emptyNormsIterator) AdvanceExact(int) (bool, error) { return false, nil }
func (e *emptyNormsIterator) LongValue() (int64, error) {
	return 0, errors.New("lucene90 norms: no value")
}
func (e *emptyNormsIterator) Cost() int64 { return 0 }

// denseNormsIterator iterates over every document in [0, maxDoc); every
// document has a value. Mirrors Lucene90NormsProducer.DenseNormsIterator
// (lines 142-179).
type denseNormsIterator struct {
	maxDoc int
	doc    int
	values normsValueReader
}

func newDenseNormsIterator(maxDoc int, values normsValueReader) *denseNormsIterator {
	return &denseNormsIterator{maxDoc: maxDoc, doc: -1, values: values}
}

func (d *denseNormsIterator) DocID() int { return d.doc }

func (d *denseNormsIterator) NextDoc() (int, error) { return d.Advance(d.doc + 1) }

func (d *denseNormsIterator) Advance(target int) (int, error) {
	if target >= d.maxDoc {
		d.doc = math.MaxInt32
		return d.doc, nil
	}
	d.doc = target
	return d.doc, nil
}

func (d *denseNormsIterator) AdvanceExact(target int) (bool, error) {
	d.doc = target
	return true, nil
}

func (d *denseNormsIterator) LongValue() (int64, error) { return d.values.valueAt(d.doc) }

func (d *denseNormsIterator) Cost() int64 { return int64(d.maxDoc) }

// sparseNormsIterator iterates over only the value-bearing documents, driven
// by an IndexedDISI. Mirrors Lucene90NormsProducer.SparseNormsIterator
// (lines 181-213); the value index is disi.Index().
type sparseNormsIterator struct {
	disi   *dvIndexedDISI
	values normsValueReader
}

func (s *sparseNormsIterator) DocID() int { return s.disi.DocID() }

func (s *sparseNormsIterator) NextDoc() (int, error) { return s.disi.NextDoc() }

func (s *sparseNormsIterator) Advance(target int) (int, error) { return s.disi.Advance(target) }

func (s *sparseNormsIterator) AdvanceExact(target int) (bool, error) {
	return s.disi.AdvanceExact(target)
}

func (s *sparseNormsIterator) LongValue() (int64, error) { return s.values.valueAt(s.disi.Index()) }

func (s *sparseNormsIterator) Cost() int64 { return s.disi.Cost() }
