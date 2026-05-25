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
	"math/bits"
	"sort"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/compress"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
)

// lucene90DVConsumer writes the Lucene 9.0 doc values binary format (.dvd + .dvm).
//
// This is the real implementation of Lucene90DocValuesConsumer; it replaces the
// Phase-1 shell that was in lucene90_doc_values_format.go.
//
// This is the Go port of
// org.apache.lucene.codecs.lucene90.Lucene90DocValuesConsumer (Lucene 10.4.0).
type lucene90DVConsumer struct {
	data              *store.ChecksumIndexOutput // .dvd
	meta              *store.ChecksumIndexOutput // .dvm
	maxDoc            int
	skipIndexInterval int
	termsDictBuf      []byte // reusable LZ4 staging buffer
	closed            bool
}

// newLucene90DVConsumer creates a new consumer that opens the .dvd and .dvm
// files and stamps their IndexHeaders.
func newLucene90DVConsumer(state *SegmentWriteState, skipIndexIntervalSize int) (*lucene90DVConsumer, error) {
	seg := state.SegmentInfo.Name()
	suffix := state.SegmentSuffix
	id := state.SegmentInfo.GetID()

	// open data file
	dvdName := seg + "." + Lucene90DocValuesDataExtension
	if suffix != "" {
		dvdName = seg + "_" + suffix + "." + Lucene90DocValuesDataExtension
	}
	dvdRaw, err := state.Directory.CreateOutput(dvdName, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		return nil, fmt.Errorf("lucene90 dv consumer: create %q: %w", dvdName, err)
	}
	dvd := store.NewChecksumIndexOutput(dvdRaw)
	if err := WriteIndexHeader(dvd, Lucene90DocValuesDataCodec, Lucene90DocValuesVersionCurrent, id, suffix); err != nil {
		_ = dvd.Close()
		return nil, fmt.Errorf("lucene90 dv consumer: header %q: %w", dvdName, err)
	}

	// open meta file
	dvmName := seg + "." + Lucene90DocValuesMetaExtension
	if suffix != "" {
		dvmName = seg + "_" + suffix + "." + Lucene90DocValuesMetaExtension
	}
	dvmRaw, err := state.Directory.CreateOutput(dvmName, store.IOContext{Context: store.ContextWrite})
	if err != nil {
		_ = dvd.Close()
		return nil, fmt.Errorf("lucene90 dv consumer: create %q: %w", dvmName, err)
	}
	dvm := store.NewChecksumIndexOutput(dvmRaw)
	if err := WriteIndexHeader(dvm, Lucene90DocValuesMetaCodec, Lucene90DocValuesVersionCurrent, id, suffix); err != nil {
		_ = dvd.Close()
		_ = dvm.Close()
		return nil, fmt.Errorf("lucene90 dv consumer: header %q: %w", dvmName, err)
	}

	return &lucene90DVConsumer{
		data:              dvd,
		meta:              dvm,
		maxDoc:            state.SegmentInfo.DocCount(),
		skipIndexInterval: skipIndexIntervalSize,
		termsDictBuf:      make([]byte, 1<<14),
	}, nil
}

// Close writes the EOF sentinel and CodecUtil footers to both files.
func (c *lucene90DVConsumer) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true
	// EOF sentinel: meta.writeInt(-1) — big-endian via ChecksumIndexOutput
	if err := c.meta.WriteInt(-1); err != nil {
		return err
	}
	if err := WriteFooter(c.meta); err != nil {
		return err
	}
	if err := c.meta.Close(); err != nil {
		return err
	}
	if err := WriteFooter(c.data); err != nil {
		return err
	}
	return c.data.Close()
}

// ---------------------------------------------------------------------------
// AddNumericField
// ---------------------------------------------------------------------------

// AddNumericField writes a numeric DV field. The provided iterator must
// implement dvSortedNumericValues (the richer read interface) so that the
// field can be iterated multiple times for the statistics pass, the skip-index
// pass, and the value-encoding pass.
//
// Callers from tests pass a dvSortedNumericValues directly; the
// DocValuesConsumer interface wrapper in lucene90_doc_values_format.go adapts
// from the write-time iterator.
func (c *lucene90DVConsumer) AddNumericField(field *index.FieldInfo, values dvSortedNumericValues) error {
	if c.closed {
		return errors.New("lucene90 dv consumer: closed")
	}
	if err := c.meta.WriteInt(int32(field.Number())); err != nil {
		return err
	}
	if err := c.meta.WriteByte(Lucene90DocValuesTypeNumeric); err != nil {
		return err
	}
	// For numeric fields, wrap the dvSortedNumericValues with a passthrough
	// dvSortedNumericValues producer so we can call writeValues.
	if field.DocValuesSkipIndexType() != index.DocValuesSkipIndexTypeNone {
		if err := c.writeSkipIndex(field, values); err != nil {
			return err
		}
		// reset
		if err := values.Reset(); err != nil {
			return err
		}
	}
	_, err := c.writeValues(field, values, false)
	return err
}

// ---------------------------------------------------------------------------
// AddBinaryField
// ---------------------------------------------------------------------------

// AddBinaryField writes a binary DV field.
func (c *lucene90DVConsumer) AddBinaryField(field *index.FieldInfo, values dvBinaryValues) error {
	if c.closed {
		return errors.New("lucene90 dv consumer: closed")
	}
	if err := c.meta.WriteInt(int32(field.Number())); err != nil {
		return err
	}
	if err := c.meta.WriteByte(Lucene90DocValuesTypeBinary); err != nil {
		return err
	}
	return c.addBinaryField(field, values)
}

func (c *lucene90DVConsumer) addBinaryField(field *index.FieldInfo, values dvBinaryValues) error {
	start := c.data.GetFilePointer()
	if err := c.meta.WriteLong(start); err != nil { // dataOffset
		return err
	}

	numDocsWithField := 0
	minLength := math.MaxInt32
	maxLength := 0

	// first pass: write raw bytes, count stats
	if err := values.Reset(); err != nil {
		return err
	}
	for {
		doc, err := values.NextDoc()
		if err != nil {
			return err
		}
		if doc == dvNoMoreDocs {
			break
		}
		numDocsWithField++
		v, err := values.BinaryValue()
		if err != nil {
			return err
		}
		if err := c.data.WriteBytes(v); err != nil {
			return err
		}
		l := len(v)
		if l < minLength {
			minLength = l
		}
		if l > maxLength {
			maxLength = l
		}
	}

	if err := c.meta.WriteLong(c.data.GetFilePointer() - start); err != nil { // dataLength
		return err
	}

	// docs-with-field DISI
	if numDocsWithField == 0 {
		if err := c.meta.WriteLong(-2); err != nil {
			return err
		}
		if err := c.meta.WriteLong(0); err != nil {
			return err
		}
		if err := c.meta.WriteShort(-1); err != nil {
			return err
		}
		if err := c.meta.WriteByte(0xFF); err != nil {
			return err
		}
	} else if numDocsWithField == c.maxDoc {
		if err := c.meta.WriteLong(-1); err != nil {
			return err
		}
		if err := c.meta.WriteLong(0); err != nil {
			return err
		}
		if err := c.meta.WriteShort(-1); err != nil {
			return err
		}
		if err := c.meta.WriteByte(0xFF); err != nil {
			return err
		}
	} else {
		offset := c.data.GetFilePointer()
		if err := c.meta.WriteLong(offset); err != nil {
			return err
		}
		if err := values.Reset(); err != nil {
			return err
		}
		jt, err := writeDVBitSet(binaryDVToDocIdSetIterator(values), c.data)
		if err != nil {
			return err
		}
		if err := c.meta.WriteLong(c.data.GetFilePointer() - offset); err != nil {
			return err
		}
		if err := c.meta.WriteShort(jt); err != nil {
			return err
		}
		if err := c.meta.WriteByte(dvDefaultDenseRankPower); err != nil {
			return err
		}
	}

	if minLength == math.MaxInt32 {
		minLength = 0
	}
	if err := c.meta.WriteInt(int32(numDocsWithField)); err != nil {
		return err
	}
	if err := c.meta.WriteInt(int32(minLength)); err != nil {
		return err
	}
	if err := c.meta.WriteInt(int32(maxLength)); err != nil {
		return err
	}

	// variable-length: write addresses monotonic
	if maxLength > minLength {
		addrStart := c.data.GetFilePointer()
		if err := c.meta.WriteLong(addrStart); err != nil {
			return err
		}
		if err := store.WriteVInt(c.meta, int32(Lucene90DocValuesDirectMonotonicBlockShift)); err != nil {
			return err
		}
		addrWriter, err := packed.NewDirectMonotonicWriter(
			dvChecksumDataOutputAt{c.meta},
			dvChecksumDataOutputAt{c.data},
			int64(numDocsWithField+1),
			Lucene90DocValuesDirectMonotonicBlockShift,
		)
		if err != nil {
			return err
		}
		var addr int64
		if err := addrWriter.Add(addr); err != nil {
			return err
		}
		if err := values.Reset(); err != nil {
			return err
		}
		for {
			doc, err := values.NextDoc()
			if err != nil {
				return err
			}
			if doc == dvNoMoreDocs {
				break
			}
			v, err := values.BinaryValue()
			if err != nil {
				return err
			}
			addr += int64(len(v))
			if err := addrWriter.Add(addr); err != nil {
				return err
			}
		}
		if err := addrWriter.Finish(); err != nil {
			return err
		}
		if err := c.meta.WriteLong(c.data.GetFilePointer() - addrStart); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// AddSortedField
// ---------------------------------------------------------------------------

// AddSortedField writes a SORTED DV field. The values parameter must implement
// dvSortedValues (exposing LookupOrd/GetValueCount) so that the terms
// dictionary can be written.
func (c *lucene90DVConsumer) AddSortedField(field *index.FieldInfo, values dvSortedValues) error {
	if c.closed {
		return errors.New("lucene90 dv consumer: closed")
	}
	if err := c.meta.WriteInt(int32(field.Number())); err != nil {
		return err
	}
	if err := c.meta.WriteByte(Lucene90DocValuesTypeSorted); err != nil {
		return err
	}
	return c.doAddSortedField(field, values, false)
}

func (c *lucene90DVConsumer) doAddSortedField(field *index.FieldInfo, values dvSortedValues, writeTypeByte bool) error {
	if field.DocValuesSkipIndexType() != index.DocValuesSkipIndexTypeNone {
		if err := c.writeSkipIndexFromSorted(field, values); err != nil {
			return err
		}
		if err := values.Reset(); err != nil {
			return err
		}
	}
	if writeTypeByte {
		if err := c.meta.WriteByte(0); err != nil { // singleValued
			return err
		}
	}
	// write ordinals as numeric
	ords := &sortedOrdsAsSortedNumeric{sorted: values}
	if _, err := c.writeValues(field, ords, true); err != nil {
		return err
	}
	if err := values.Reset(); err != nil {
		return err
	}
	// write terms dict via dvSortedSetValues adapter
	return c.addTermsDict(sortedToSortedSet(values))
}

// ---------------------------------------------------------------------------
// AddSortedSetField
// ---------------------------------------------------------------------------

// AddSortedSetField writes a SORTED_SET DV field.
func (c *lucene90DVConsumer) AddSortedSetField(field *index.FieldInfo, values dvSortedSetValues) error {
	if c.closed {
		return errors.New("lucene90 dv consumer: closed")
	}
	if err := c.meta.WriteInt(int32(field.Number())); err != nil {
		return err
	}
	if err := c.meta.WriteByte(Lucene90DocValuesTypeSortedSet); err != nil {
		return err
	}
	// detect single-valued
	single, err := isSortedSetSingleValued(values)
	if err != nil {
		return err
	}
	if err := values.Reset(); err != nil {
		return err
	}
	if single {
		// delegate to sorted path with singleValued byte
		sv := &sortedSetAsSorted{ss: values}
		return c.doAddSortedField(field, sv, true)
	}
	// multi-valued
	if err := c.doAddSortedNumericField(field, sortedSetOrdsAsSortedNumeric(values), true); err != nil {
		return err
	}
	if err := values.Reset(); err != nil {
		return err
	}
	return c.addTermsDict(values)
}

// ---------------------------------------------------------------------------
// AddSortedNumericField
// ---------------------------------------------------------------------------

// AddSortedNumericField writes a SORTED_NUMERIC DV field.
func (c *lucene90DVConsumer) AddSortedNumericField(field *index.FieldInfo, values dvSortedNumericValues) error {
	if c.closed {
		return errors.New("lucene90 dv consumer: closed")
	}
	if err := c.meta.WriteInt(int32(field.Number())); err != nil {
		return err
	}
	if err := c.meta.WriteByte(Lucene90DocValuesTypeSortedNumeric); err != nil {
		return err
	}
	return c.doAddSortedNumericField(field, values, false)
}

func (c *lucene90DVConsumer) doAddSortedNumericField(field *index.FieldInfo, values dvSortedNumericValues, ords bool) error {
	if field.DocValuesSkipIndexType() != index.DocValuesSkipIndexTypeNone {
		if err := c.writeSkipIndex(field, values); err != nil {
			return err
		}
		if err := values.Reset(); err != nil {
			return err
		}
	}
	if ords {
		if err := c.meta.WriteByte(1); err != nil { // multiValued
			return err
		}
	}
	stats, err := c.writeValues(field, values, ords)
	if err != nil {
		return err
	}
	numDocsWithField := int(stats[0])
	numValues := stats[1]

	if err := c.meta.WriteInt(int32(numDocsWithField)); err != nil {
		return err
	}
	// write per-doc addresses when multi-value
	if numValues > int64(numDocsWithField) {
		addrStart := c.data.GetFilePointer()
		if err := c.meta.WriteLong(addrStart); err != nil {
			return err
		}
		if err := store.WriteVInt(c.meta, int32(Lucene90DocValuesDirectMonotonicBlockShift)); err != nil {
			return err
		}
		addrWriter, err := packed.NewDirectMonotonicWriter(
			dvChecksumDataOutputAt{c.meta},
			dvChecksumDataOutputAt{c.data},
			int64(numDocsWithField+1),
			Lucene90DocValuesDirectMonotonicBlockShift,
		)
		if err != nil {
			return err
		}
		var addr int64
		if err := addrWriter.Add(addr); err != nil {
			return err
		}
		if err := values.Reset(); err != nil {
			return err
		}
		for {
			doc, err := values.NextDoc()
			if err != nil {
				return err
			}
			if doc == dvNoMoreDocs {
				break
			}
			cnt, err := values.DocValueCount()
			if err != nil {
				return err
			}
			addr += int64(cnt)
			if err := addrWriter.Add(addr); err != nil {
				return err
			}
			// consume remaining values
			for i := 0; i < cnt; i++ {
				if _, err := values.NextValue(); err != nil {
					return err
				}
			}
		}
		if err := addrWriter.Finish(); err != nil {
			return err
		}
		if err := c.meta.WriteLong(c.data.GetFilePointer() - addrStart); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// writeValues — core numeric encoder
// ---------------------------------------------------------------------------

// writeValues is the core of numeric encoding. It mirrors
// Lucene90DocValuesConsumer.writeValues. Returns [numDocsWithField, numValues].
func (c *lucene90DVConsumer) writeValues(field *index.FieldInfo, values dvSortedNumericValues, ords bool) ([2]int64, error) {
	zero := [2]int64{}

	// --- first pass: gather statistics ---
	type minMaxTracker struct {
		min, max, numValues, spaceInBits int64
	}
	var (
		mmGlobal   minMaxTracker
		mmBlock    minMaxTracker
		gcd        int64
		firstValue int64
		gotFirst   bool
		uniqueVals map[int64]struct{}
		skipUnique bool // uniqueVals exceeded 256 entries
	)
	mmGlobal.min = math.MaxInt64
	mmGlobal.max = math.MinInt64
	mmBlock.min = math.MaxInt64
	mmBlock.max = math.MinInt64

	if !ords {
		uniqueVals = make(map[int64]struct{}, 256)
	} else {
		// ordinals are never tracked for uniqueness; skip that path entirely
		skipUnique = true
	}

	numDocsWithField := int64(0)
	if err := values.Reset(); err != nil {
		return zero, err
	}
	for {
		doc, err := values.NextDoc()
		if err != nil {
			return zero, err
		}
		if doc == dvNoMoreDocs {
			break
		}
		cnt, err := values.DocValueCount()
		if err != nil {
			return zero, err
		}
		for i := 0; i < cnt; i++ {
			v, err := values.NextValue()
			if err != nil {
				return zero, err
			}
			if !gotFirst {
				firstValue = v
				gotFirst = true
			}
			if gcd != 1 {
				if v < math.MinInt64/2 || v > math.MaxInt64/2 {
					gcd = 1
				} else {
					gcd = util.MathGCD(gcd, v-firstValue)
				}
			}
			if v < mmBlock.min {
				mmBlock.min = v
			}
			if v > mmBlock.max {
				mmBlock.max = v
			}
			mmBlock.numValues++
			if mmBlock.numValues == Lucene90DocValuesNumericBlockSize {
				// flush block tracker
				if mmBlock.max > mmBlock.min {
					bpv := int64(packed.DirectWriterUnsignedBitsRequired(uint64(mmBlock.max - mmBlock.min)))
					mmBlock.spaceInBits += bpv * mmBlock.numValues
				}
				if mmBlock.min < mmGlobal.min {
					mmGlobal.min = mmBlock.min
				}
				if mmBlock.max > mmGlobal.max {
					mmGlobal.max = mmBlock.max
				}
				mmGlobal.numValues += mmBlock.numValues
				mmBlock = minMaxTracker{min: math.MaxInt64, max: math.MinInt64}
			}
			if !skipUnique {
				uniqueVals[v] = struct{}{}
				if len(uniqueVals) > 256 {
					skipUnique = true
					uniqueVals = nil
				}
			}
		}
		numDocsWithField++
	}
	// flush residual block
	if mmBlock.numValues > 0 {
		if mmBlock.max > mmBlock.min {
			bpv := int64(packed.DirectWriterUnsignedBitsRequired(uint64(mmBlock.max - mmBlock.min)))
			mmBlock.spaceInBits += bpv * mmBlock.numValues
		}
		if mmBlock.min < mmGlobal.min {
			mmGlobal.min = mmBlock.min
		}
		if mmBlock.max > mmGlobal.max {
			mmGlobal.max = mmBlock.max
		}
		mmGlobal.numValues += mmBlock.numValues
	}
	numValues := mmGlobal.numValues
	minVal := mmGlobal.min
	maxVal := mmGlobal.max

	// globalSpaceInBits — recompute as one block (matching Java minMax.finish())
	var globalSpaceInBits int64
	if maxVal > minVal {
		globalSpaceInBits = int64(packed.DirectWriterUnsignedBitsRequired(uint64(maxVal-minVal))) * numValues
	}

	// --- write DISI ---
	if numDocsWithField == 0 {
		if err := c.meta.WriteLong(-2); err != nil {
			return zero, err
		}
		if err := c.meta.WriteLong(0); err != nil {
			return zero, err
		}
		if err := c.meta.WriteShort(-1); err != nil {
			return zero, err
		}
		if err := c.meta.WriteByte(0xFF); err != nil {
			return zero, err
		}
	} else if numDocsWithField == int64(c.maxDoc) {
		if err := c.meta.WriteLong(-1); err != nil {
			return zero, err
		}
		if err := c.meta.WriteLong(0); err != nil {
			return zero, err
		}
		if err := c.meta.WriteShort(-1); err != nil {
			return zero, err
		}
		if err := c.meta.WriteByte(0xFF); err != nil {
			return zero, err
		}
	} else {
		offset := c.data.GetFilePointer()
		if err := c.meta.WriteLong(offset); err != nil {
			return zero, err
		}
		if err := values.Reset(); err != nil {
			return zero, err
		}
		jt, err := writeDVBitSet(sortedNumericToDocIdSet(values), c.data)
		if err != nil {
			return zero, err
		}
		if err := c.meta.WriteLong(c.data.GetFilePointer() - offset); err != nil {
			return zero, err
		}
		if err := c.meta.WriteShort(jt); err != nil {
			return zero, err
		}
		if err := c.meta.WriteByte(dvDefaultDenseRankPower); err != nil {
			return zero, err
		}
	}

	if err := c.meta.WriteLong(numValues); err != nil {
		return zero, err
	}

	// --- choose encoding ---
	numBitsPerValue := 0
	doBlocks := false
	var encode map[int64]int // table encode map
	var tableVals []int64

	if minVal >= maxVal {
		// const-compressed (numBitsPerValue = 0)
		if err := c.meta.WriteInt(-1); err != nil { // tablesize = -1
			return zero, err
		}
	} else {
		// try table compression
		bitsFromRange := packed.DirectWriterUnsignedBitsRequired(uint64((maxVal - minVal) / gcdOrOne(gcd)))
		if !skipUnique && len(uniqueVals) > 1 &&
			packed.DirectWriterUnsignedBitsRequired(uint64(len(uniqueVals)-1)) < bitsFromRange {
			// table compression
			tableVals = make([]int64, 0, len(uniqueVals))
			for v := range uniqueVals {
				tableVals = append(tableVals, v)
			}
			sort.Slice(tableVals, func(i, j int) bool { return tableVals[i] < tableVals[j] })
			numBitsPerValue = packed.DirectWriterUnsignedBitsRequired(uint64(len(tableVals) - 1))
			if err := c.meta.WriteInt(int32(len(tableVals))); err != nil {
				return zero, err
			}
			for _, tv := range tableVals {
				if err := c.meta.WriteLong(tv); err != nil {
					return zero, err
				}
			}
			encode = make(map[int64]int, len(tableVals))
			for i, tv := range tableVals {
				encode[tv] = i
			}
			minVal = 0
			gcd = 1
		} else {
			// check blocks vs single
			blockSpaceInBits := mmBlock.spaceInBits // was accumulated per-block during pass
			_ = blockSpaceInBits
			// Recalculate block space properly: rerun a block-level min/max
			blockSpaceInBits2, err := c.computeBlockSpaceInBits(values)
			if err != nil {
				return zero, err
			}
			doBlocks = globalSpaceInBits > 0 && float64(blockSpaceInBits2)/float64(globalSpaceInBits) <= 0.9
			if doBlocks {
				numBitsPerValue = 0xFF
				if err := c.meta.WriteInt(int32(-2 - Lucene90DocValuesNumericBlockShift)); err != nil {
					return zero, err
				}
			} else {
				gcdActual := gcdOrOne(gcd)
				numBitsPerValue = packed.DirectWriterUnsignedBitsRequired(uint64((maxVal - minVal) / gcdActual))
				// if GCD==1 and min>0 and bits(max)==bits(max-min), set min=0
				if gcdActual == 1 && minVal > 0 &&
					packed.DirectWriterUnsignedBitsRequired(uint64(maxVal)) == numBitsPerValue {
					minVal = 0
				}
				if err := c.meta.WriteInt(-1); err != nil {
					return zero, err
				}
			}
		}
	}

	if err := c.meta.WriteByte(byte(numBitsPerValue)); err != nil {
		return zero, err
	}
	if err := c.meta.WriteLong(minVal); err != nil {
		return zero, err
	}
	if err := c.meta.WriteLong(gcdOrOne(gcd)); err != nil {
		return zero, err
	}
	startOffset := c.data.GetFilePointer()
	if err := c.meta.WriteLong(startOffset); err != nil { // valueOffset
		return zero, err
	}
	jumpTableOffset := int64(-1)

	if err := values.Reset(); err != nil {
		return zero, err
	}
	if doBlocks {
		jumpTableOffset, err := c.writeValuesMultipleBlocks(values, gcdOrOne(gcd))
		if err != nil {
			return zero, err
		}
		if err := c.meta.WriteLong(c.data.GetFilePointer() - startOffset); err != nil { // valuesLength
			return zero, err
		}
		if err := c.meta.WriteLong(jumpTableOffset); err != nil {
			return zero, err
		}
	} else {
		if numBitsPerValue != 0 {
			if err := c.writeValuesSingleBlock(values, numValues, numBitsPerValue, minVal, gcdOrOne(gcd), encode); err != nil {
				return zero, err
			}
		}
		if err := c.meta.WriteLong(c.data.GetFilePointer() - startOffset); err != nil { // valuesLength
			return zero, err
		}
		if err := c.meta.WriteLong(jumpTableOffset); err != nil {
			return zero, err
		}
	}

	return [2]int64{numDocsWithField, numValues}, nil
}

// computeBlockSpaceInBits does a second pass computing per-block min/max
// to determine whether per-block BPV saves >=10% vs single-block BPV.
func (c *lucene90DVConsumer) computeBlockSpaceInBits(values dvSortedNumericValues) (int64, error) {
	if err := values.Reset(); err != nil {
		return 0, err
	}
	var total int64
	blockCount := int64(0)
	blockMin := int64(math.MaxInt64)
	blockMax := int64(math.MinInt64)
	for {
		doc, err := values.NextDoc()
		if err != nil {
			return 0, err
		}
		if doc == dvNoMoreDocs {
			break
		}
		cnt, err := values.DocValueCount()
		if err != nil {
			return 0, err
		}
		for i := 0; i < cnt; i++ {
			v, err := values.NextValue()
			if err != nil {
				return 0, err
			}
			if v < blockMin {
				blockMin = v
			}
			if v > blockMax {
				blockMax = v
			}
			blockCount++
			if blockCount == Lucene90DocValuesNumericBlockSize {
				if blockMax > blockMin {
					bpv := int64(packed.DirectWriterUnsignedBitsRequired(uint64(blockMax - blockMin)))
					total += bpv * blockCount
				}
				blockMin = math.MaxInt64
				blockMax = math.MinInt64
				blockCount = 0
			}
		}
	}
	if blockCount > 0 && blockMax > blockMin {
		bpv := int64(packed.DirectWriterUnsignedBitsRequired(uint64(blockMax - blockMin)))
		total += bpv * blockCount
	}
	return total, nil
}

func (c *lucene90DVConsumer) writeValuesSingleBlock(
	values dvSortedNumericValues,
	numValues int64,
	numBitsPerValue int,
	min, gcd int64,
	encode map[int64]int,
) error {
	w, err := packed.GetDirectWriter(c.data, numValues, numBitsPerValue)
	if err != nil {
		return err
	}
	for {
		doc, err := values.NextDoc()
		if err != nil {
			return err
		}
		if doc == dvNoMoreDocs {
			break
		}
		cnt, err := values.DocValueCount()
		if err != nil {
			return err
		}
		for i := 0; i < cnt; i++ {
			v, err := values.NextValue()
			if err != nil {
				return err
			}
			var encoded int64
			if encode != nil {
				encoded = int64(encode[v])
			} else {
				encoded = (v - min) / gcd
			}
			if err := w.Add(encoded); err != nil {
				return err
			}
		}
	}
	return w.Finish()
}

// writeValuesMultipleBlocks writes variable-BPV per block. Returns the offset
// of the jump table.
func (c *lucene90DVConsumer) writeValuesMultipleBlocks(values dvSortedNumericValues, gcd int64) (int64, error) {
	var offsets []int64
	buf := make([]int64, 0, Lucene90DocValuesNumericBlockSize)
	encBuf := store.NewByteBuffersDataOutput()

	flush := func() error {
		offsets = append(offsets, c.data.GetFilePointer())
		return c.writeBlock(buf, gcd, encBuf)
	}

	for {
		doc, err := values.NextDoc()
		if err != nil {
			return 0, err
		}
		if doc == dvNoMoreDocs {
			break
		}
		cnt, err := values.DocValueCount()
		if err != nil {
			return 0, err
		}
		for i := 0; i < cnt; i++ {
			v, err := values.NextValue()
			if err != nil {
				return 0, err
			}
			buf = append(buf, v)
			if len(buf) == Lucene90DocValuesNumericBlockSize {
				if err := flush(); err != nil {
					return 0, err
				}
				buf = buf[:0]
			}
		}
	}
	if len(buf) > 0 {
		if err := flush(); err != nil {
			return 0, err
		}
	}

	// write jump table (LE: varyingBPVReader reads via RandomAccessInput.ReadLongAt which is LE)
	jumpTableOffset := c.data.GetFilePointer()
	for _, off := range offsets {
		if err := dvWriteLongLE(c.data, off); err != nil {
			return 0, err
		}
	}
	if err := dvWriteLongLE(c.data, jumpTableOffset); err != nil {
		return 0, err
	}
	return jumpTableOffset, nil
}

// writeBlock encodes one block with uniform BPV.
func (c *lucene90DVConsumer) writeBlock(vals []int64, gcd int64, encBuf *store.ByteBuffersDataOutput) error {
	minVal := vals[0]
	maxVal := vals[0]
	for _, v := range vals[1:] {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}
	if minVal == maxVal {
		if err := c.data.WriteByte(0); err != nil {
			return err
		}
		// delta written LE: varyingBPVReader reads via RandomAccessInput.ReadLongAt (LE)
		return dvWriteLongLE(c.data, minVal)
	}
	bpv := packed.DirectWriterUnsignedBitsRequired(uint64((maxVal - minVal) / gcd))
	encBuf.Reset()
	w, err := packed.GetDirectWriter(bbdoIndexOutputAt{encBuf}, int64(len(vals)), bpv)
	if err != nil {
		return err
	}
	for _, v := range vals {
		if err := w.Add((v - minVal) / gcd); err != nil {
			return err
		}
	}
	if err := w.Finish(); err != nil {
		return err
	}
	if err := c.data.WriteByte(byte(bpv)); err != nil {
		return err
	}
	// delta and size written LE: varyingBPVReader reads via RandomAccessInput (LE)
	if err := dvWriteLongLE(c.data, minVal); err != nil {
		return err
	}
	sz := encBuf.Size()
	if err := dvWriteIntLE(c.data, int32(sz)); err != nil {
		return err
	}
	return encBuf.CopyTo(c.data)
}

// ---------------------------------------------------------------------------
// addTermsDict + writeTermsIndex
// ---------------------------------------------------------------------------

// addTermsDict writes the LZ4-prefix-compressed terms dictionary for a
// SortedSet or Sorted field.
func (c *lucene90DVConsumer) addTermsDict(values dvSortedSetValues) error {
	size := int64(values.GetValueCount())
	if err := store.WriteVLong(c.meta, size); err != nil {
		return err
	}

	blockMask := int64(Lucene90DocValuesTermsDictBlockLZ4Mask)
	shift := uint(Lucene90DocValuesTermsDictBlockLZ4Shift)

	if err := c.meta.WriteInt(int32(Lucene90DocValuesDirectMonotonicBlockShift)); err != nil {
		return err
	}

	// address buffer accumulates block offsets for DirectMonotonicWriter
	addrBuf := store.NewByteBuffersDataOutput()
	numBlocks := (size + blockMask) >> shift
	addrWriter, err := packed.NewDirectMonotonicWriter(
		dvChecksumDataOutputAt{c.meta},
		bbdoIndexOutputAt{addrBuf},
		numBlocks,
		Lucene90DocValuesDirectMonotonicBlockShift,
	)
	if err != nil {
		return err
	}

	var prevTerm []byte
	ord := int64(0)
	start := c.data.GetFilePointer()
	maxLength := 0
	maxBlockLength := 0
	ht := compress.NewFastCompressionHashTable()
	// termsDictBuf doubles as staging buffer; wrap it directly so
	// compressAndGetTermsDictBlockLength reads from the same backing slice.
	buf := store.NewByteArrayDataOutputAt(c.termsDictBuf, 0)
	dictLen := 0

	if err := values.Reset(); err != nil {
		return err
	}
	for {
		term, err := values.NextOrdTerm()
		if err != nil {
			return err
		}
		if term == nil {
			break
		}
		if (ord & blockMask) == 0 {
			if ord != 0 {
				// flush previous block
				uncompLen, err := c.compressAndGetTermsDictBlockLength(buf, dictLen, ht)
				if err != nil {
					return err
				}
				if uncompLen > maxBlockLength {
					maxBlockLength = uncompLen
				}
				buf.Reset()
			}
			if err := addrWriter.Add(c.data.GetFilePointer() - start); err != nil {
				return err
			}
			// write first term verbatim to data
			if err := store.WriteVInt(c.data, int32(len(term))); err != nil {
				return err
			}
			if err := c.data.WriteBytes(term); err != nil {
				return err
			}
			// also buffer as dict for next block; re-wrap buf at pos 0 after reset
			buf = c.growTermsDictBuf(buf, len(term))
			if err := buf.WriteBytes(term); err != nil {
				return err
			}
			dictLen = len(term)
		} else {
			prefixLen := commonPrefixLen(prevTerm, term)
			suffixLen := len(term) - prefixLen
			buf = c.growTermsDictBuf(buf, suffixLen+11)
			pf := prefixLen
			if pf > 15 {
				pf = 15
			}
			sf := suffixLen - 1
			if sf > 15 {
				sf = 15
			}
			if err := buf.WriteByte(byte(pf | (sf << 4))); err != nil {
				return err
			}
			if prefixLen >= 15 {
				if err := store.WriteVInt(buf, int32(prefixLen-15)); err != nil {
					return err
				}
			}
			if suffixLen >= 16 {
				if err := store.WriteVInt(buf, int32(suffixLen-16)); err != nil {
					return err
				}
			}
			if err := buf.WriteBytes(term[prefixLen:]); err != nil {
				return err
			}
		}
		if len(term) > maxLength {
			maxLength = len(term)
		}
		prevTerm = term
		ord++
	}
	// flush last block
	if buf.GetPosition() > dictLen {
		uncompLen, err := c.compressAndGetTermsDictBlockLength(buf, dictLen, ht)
		if err != nil {
			return err
		}
		if uncompLen > maxBlockLength {
			maxBlockLength = uncompLen
		}
	}

	if err := addrWriter.Finish(); err != nil {
		return err
	}
	if err := c.meta.WriteInt(int32(maxLength)); err != nil {
		return err
	}
	if err := c.meta.WriteInt(int32(maxBlockLength)); err != nil {
		return err
	}
	if err := c.meta.WriteLong(start); err != nil {
		return err
	}
	if err := c.meta.WriteLong(c.data.GetFilePointer() - start); err != nil {
		return err
	}
	// flush address buffer to data
	addrDataStart := c.data.GetFilePointer()
	if err := addrBuf.CopyTo(c.data); err != nil {
		return err
	}
	if err := c.meta.WriteLong(addrDataStart); err != nil {
		return err
	}
	if err := c.meta.WriteLong(c.data.GetFilePointer() - addrDataStart); err != nil {
		return err
	}

	// write reverse terms index
	if err := values.Reset(); err != nil {
		return err
	}
	return c.writeTermsIndex(values)
}

// growTermsDictBuf ensures c.termsDictBuf has enough room and returns a
// ByteArrayDataOutput re-wrapped at the current write position. This mirrors
// Java's maybeGrowBuffer: new ByteArrayDataOutput(termsDictBuffer, pos, len-pos).
func (c *lucene90DVConsumer) growTermsDictBuf(buf *store.ByteArrayDataOutput, needed int) *store.ByteArrayDataOutput {
	pos := buf.GetPosition()
	if pos+needed < len(c.termsDictBuf)-1 {
		// no growth needed — re-wrap at current pos to get a fresh view
		return store.NewByteArrayDataOutputAt(c.termsDictBuf, pos)
	}
	newLen := len(c.termsDictBuf) + needed
	grown := make([]byte, newLen)
	copy(grown, c.termsDictBuf)
	c.termsDictBuf = grown
	return store.NewByteArrayDataOutputAt(c.termsDictBuf, pos)
}

func (c *lucene90DVConsumer) compressAndGetTermsDictBlockLength(
	buf *store.ByteArrayDataOutput,
	dictLen int,
	ht *compress.FastCompressionHashTable,
) (int, error) {
	pos := buf.GetPosition()
	uncompLen := pos - dictLen
	if err := store.WriteVInt(c.data, int32(uncompLen)); err != nil {
		return 0, err
	}
	if err := compress.LZ4CompressWithDictionary(c.termsDictBuf, 0, dictLen, uncompLen, c.data, ht); err != nil {
		return 0, err
	}
	return uncompLen, nil
}

func (c *lucene90DVConsumer) writeTermsIndex(values dvSortedSetValues) error {
	size := int64(values.GetValueCount())
	if err := c.meta.WriteInt(int32(Lucene90DocValuesTermsDictReverseIndexShift)); err != nil {
		return err
	}
	start := c.data.GetFilePointer()

	numBlocks := 1 + ((size + int64(Lucene90DocValuesTermsDictReverseIndexMask)) >> uint(Lucene90DocValuesTermsDictReverseIndexShift))
	addrBuf := store.NewByteBuffersDataOutput()
	addrWriter, err := packed.NewDirectMonotonicWriter(
		dvChecksumDataOutputAt{c.meta},
		bbdoIndexOutputAt{addrBuf},
		numBlocks,
		Lucene90DocValuesDirectMonotonicBlockShift,
	)
	if err != nil {
		return err
	}

	var prevTerm []byte
	offset := int64(0)
	ord := int64(0)

	if err := values.Reset(); err != nil {
		return err
	}
	for {
		term, err := values.NextOrdTerm()
		if err != nil {
			return err
		}
		if term == nil {
			break
		}
		mask := int64(Lucene90DocValuesTermsDictReverseIndexMask)
		if (ord & mask) == 0 {
			if err := addrWriter.Add(offset); err != nil {
				return err
			}
			var sortKeyLen int
			if ord == 0 {
				sortKeyLen = 0
			} else {
				sortKeyLen = sortKeyLength(prevTerm, term)
			}
			offset += int64(sortKeyLen)
			if sortKeyLen > 0 {
				if err := c.data.WriteBytes(term[:sortKeyLen]); err != nil {
					return err
				}
			}
		} else if (ord & mask) == mask {
			prevTerm = term
		}
		ord++
	}
	if err := addrWriter.Add(offset); err != nil {
		return err
	}
	if err := addrWriter.Finish(); err != nil {
		return err
	}
	if err := c.meta.WriteLong(start); err != nil {
		return err
	}
	if err := c.meta.WriteLong(c.data.GetFilePointer() - start); err != nil {
		return err
	}
	addrDataStart := c.data.GetFilePointer()
	if err := addrBuf.CopyTo(c.data); err != nil {
		return err
	}
	if err := c.meta.WriteLong(addrDataStart); err != nil {
		return err
	}
	return c.meta.WriteLong(c.data.GetFilePointer() - addrDataStart)
}

// ---------------------------------------------------------------------------
// Skip-index
// ---------------------------------------------------------------------------

type skipAccum struct {
	minDocID, maxDocID int
	docCount           int
	minValue, maxValue int64
}

func (a *skipAccum) isDone(intervalSize int, valueCount int, nextValue int64, nextDoc int) bool {
	if a.docCount < intervalSize {
		return false
	}
	return valueCount > 1 ||
		a.minValue != a.maxValue ||
		a.minValue != nextValue ||
		a.docCount != nextDoc-a.minDocID
}

func (a *skipAccum) accumulate(v int64) {
	if v < a.minValue {
		a.minValue = v
	}
	if v > a.maxValue {
		a.maxValue = v
	}
}

func (a *skipAccum) nextDoc(docID int) {
	a.maxDocID = docID
	a.docCount++
}

func mergeSkipAccums(list []*skipAccum, index, length int) *skipAccum {
	acc := &skipAccum{
		minDocID: list[index].minDocID,
		minValue: math.MaxInt64,
		maxValue: math.MinInt64,
	}
	for i := 0; i < length; i++ {
		src := list[index+i]
		acc.maxDocID = src.maxDocID
		if src.minValue < acc.minValue {
			acc.minValue = src.minValue
		}
		if src.maxValue > acc.maxValue {
			acc.maxValue = src.maxValue
		}
		acc.docCount += src.docCount
	}
	return acc
}

func (c *lucene90DVConsumer) writeSkipIndex(field *index.FieldInfo, values dvSortedNumericValues) error {
	start := c.data.GetFilePointer()

	globalMaxValue := int64(math.MinInt64)
	globalMinValue := int64(math.MaxInt64)
	globalDocCount := 0
	maxDocID := -1

	var accums []*skipAccum
	var current *skipAccum
	maxAccums := 1 << uint(Lucene90DocValuesSkipIndexLevelShift*(Lucene90DocValuesSkipIndexMaxLevel-1))

	if err := values.Reset(); err != nil {
		return err
	}
	for {
		doc, err := values.NextDoc()
		if err != nil {
			return err
		}
		if doc == dvNoMoreDocs {
			break
		}
		cnt, err := values.DocValueCount()
		if err != nil {
			return err
		}
		firstVal, err := values.NextValue()
		if err != nil {
			return err
		}
		if current != nil && current.isDone(c.skipIndexInterval, cnt, firstVal, doc) {
			globalMaxValue = maxOf(globalMaxValue, current.maxValue)
			globalMinValue = minOf(globalMinValue, current.minValue)
			globalDocCount += current.docCount
			maxDocID = current.maxDocID
			current = nil
			if len(accums) == maxAccums {
				if err := c.writeLevels(accums); err != nil {
					return err
				}
				accums = accums[:0]
			}
		}
		if current == nil {
			current = &skipAccum{minDocID: doc, minValue: math.MaxInt64, maxValue: math.MinInt64}
			accums = append(accums, current)
		}
		current.nextDoc(doc)
		current.accumulate(firstVal)
		for i := 1; i < cnt; i++ {
			v, err := values.NextValue()
			if err != nil {
				return err
			}
			current.accumulate(v)
		}
	}
	if len(accums) > 0 {
		globalMaxValue = maxOf(globalMaxValue, current.maxValue)
		globalMinValue = minOf(globalMinValue, current.minValue)
		globalDocCount += current.docCount
		maxDocID = current.maxDocID
		if err := c.writeLevels(accums); err != nil {
			return err
		}
	}

	if err := c.meta.WriteLong(start); err != nil {
		return err
	}
	if err := c.meta.WriteLong(c.data.GetFilePointer() - start); err != nil {
		return err
	}
	if err := c.meta.WriteLong(globalMaxValue); err != nil {
		return err
	}
	if err := c.meta.WriteLong(globalMinValue); err != nil {
		return err
	}
	if err := c.meta.WriteInt(int32(globalDocCount)); err != nil {
		return err
	}
	return c.meta.WriteInt(int32(maxDocID))
}

func (c *lucene90DVConsumer) writeSkipIndexFromSorted(field *index.FieldInfo, values dvSortedValues) error {
	// wrap sorted as sorted-numeric and delegate
	sn := &sortedOrdsAsSortedNumeric{sorted: values}
	return c.writeSkipIndex(field, sn)
}

func (c *lucene90DVConsumer) writeLevels(accums []*skipAccum) error {
	// build pyramid levels
	levels := make([][]*skipAccum, Lucene90DocValuesSkipIndexMaxLevel)
	levels[0] = accums
	for i := 0; i < Lucene90DocValuesSkipIndexMaxLevel-1; i++ {
		levels[i+1] = buildLevel(levels[i])
	}
	total := len(accums)
	for index := 0; index < total; index++ {
		lvl := getLevels(index, total)
		if err := c.data.WriteByte(byte(lvl)); err != nil {
			return err
		}
		for l := lvl - 1; l >= 0; l-- {
			acc := levels[l][index>>(uint(Lucene90DocValuesSkipIndexLevelShift*l))]
			if err := c.data.WriteInt(int32(acc.maxDocID)); err != nil {
				return err
			}
			if err := c.data.WriteInt(int32(acc.minDocID)); err != nil {
				return err
			}
			if err := c.data.WriteLong(acc.maxValue); err != nil {
				return err
			}
			if err := c.data.WriteLong(acc.minValue); err != nil {
				return err
			}
			if err := c.data.WriteInt(int32(acc.docCount)); err != nil {
				return err
			}
		}
	}
	return nil
}

func buildLevel(in []*skipAccum) []*skipAccum {
	sz := 1 << uint(Lucene90DocValuesSkipIndexLevelShift)
	var out []*skipAccum
	for i := 0; i+sz-1 < len(in); i += sz {
		out = append(out, mergeSkipAccums(in, i, sz))
	}
	return out
}

func getLevels(index, size int) int {
	if bits.TrailingZeros(uint(index)) >= Lucene90DocValuesSkipIndexLevelShift {
		left := size - index
		for level := Lucene90DocValuesSkipIndexMaxLevel - 1; level > 0; level-- {
			num := 1 << uint(Lucene90DocValuesSkipIndexLevelShift*level)
			if left >= num && index%num == 0 {
				return level + 1
			}
		}
	}
	return 1
}

// ---------------------------------------------------------------------------
// Resettable DV interfaces used internally by lucene90DVConsumer
// ---------------------------------------------------------------------------

// dvSortedNumericValues is the internal read interface for sorted-numeric DV
// with Reset() support so the consumer can iterate multiple times.
type dvSortedNumericValues interface {
	// Reset positions the iterator before doc -1 for re-iteration.
	Reset() error
	// NextDoc returns the next document that has a value, or dvNoMoreDocs.
	NextDoc() (int, error)
	// DocValueCount returns the number of values for the current document.
	DocValueCount() (int, error)
	// NextValue returns the next value for the current document.
	NextValue() (int64, error)
}

// dvBinaryValues is the internal read interface for binary DV with Reset().
type dvBinaryValues interface {
	Reset() error
	NextDoc() (int, error)
	BinaryValue() ([]byte, error)
}

// dvSortedValues is the internal read interface for sorted DV with Reset().
type dvSortedValues interface {
	Reset() error
	NextDoc() (int, error)
	OrdValue() (int, error)
	LookupOrd(ord int) ([]byte, error)
	GetValueCount() int
}

// dvSortedSetValues is the internal read interface for sorted-set DV with Reset().
type dvSortedSetValues interface {
	Reset() error
	NextDoc() (int, error)
	NextOrd() (int, error)
	// NextOrdTerm returns the next term in ordinal order (for terms-dict writing),
	// or nil when exhausted.
	NextOrdTerm() ([]byte, error)
	LookupOrd(ord int) ([]byte, error)
	GetValueCount() int
	DocValueCount() (int, error)
}

// ---------------------------------------------------------------------------
// Adapters
// ---------------------------------------------------------------------------

// sortedOrdsAsSortedNumeric wraps dvSortedValues as dvSortedNumericValues.
type sortedOrdsAsSortedNumeric struct {
	sorted dvSortedValues
	doc    int
	ord    int64
	done   bool
}

func (a *sortedOrdsAsSortedNumeric) Reset() error {
	a.done = false
	return a.sorted.Reset()
}
func (a *sortedOrdsAsSortedNumeric) NextDoc() (int, error) {
	if a.done {
		return dvNoMoreDocs, nil
	}
	doc, err := a.sorted.NextDoc()
	if err != nil {
		return 0, err
	}
	a.doc = doc
	if doc == dvNoMoreDocs {
		a.done = true
	} else {
		ord, err := a.sorted.OrdValue()
		if err != nil {
			return 0, err
		}
		a.ord = int64(ord)
	}
	return doc, nil
}
func (a *sortedOrdsAsSortedNumeric) DocValueCount() (int, error) { return 1, nil }
func (a *sortedOrdsAsSortedNumeric) NextValue() (int64, error)   { return a.ord, nil }

// sortedSetOrdsAsSortedNumeric adapts a dvSortedSetValues so each ord is
// returned as a separate int64 value.
func sortedSetOrdsAsSortedNumeric(ss dvSortedSetValues) dvSortedNumericValues {
	return &ssOrdsAsSN{ss: ss}
}

type ssOrdsAsSN struct {
	ss   dvSortedSetValues
	cnt  int
	ords []int64
	idx  int
}

func (a *ssOrdsAsSN) Reset() error { return a.ss.Reset() }
func (a *ssOrdsAsSN) NextDoc() (int, error) {
	doc, err := a.ss.NextDoc()
	if err != nil || doc == dvNoMoreDocs {
		return doc, err
	}
	cnt, err := a.ss.DocValueCount()
	if err != nil {
		return 0, err
	}
	a.ords = a.ords[:0]
	if cap(a.ords) < cnt {
		a.ords = make([]int64, 0, cnt)
	}
	for i := 0; i < cnt; i++ {
		ord, err := a.ss.NextOrd()
		if err != nil {
			return 0, err
		}
		a.ords = append(a.ords, int64(ord))
	}
	a.cnt = cnt
	a.idx = 0
	return doc, nil
}
func (a *ssOrdsAsSN) DocValueCount() (int, error) { return a.cnt, nil }
func (a *ssOrdsAsSN) NextValue() (int64, error) {
	v := a.ords[a.idx]
	a.idx++
	return v, nil
}

// sortedToSortedSet wraps a dvSortedValues as a single-value dvSortedSetValues.
func sortedToSortedSet(sd dvSortedValues) dvSortedSetValues {
	return &sortedAsSS{sd: sd}
}

type sortedAsSS struct {
	sd        dvSortedValues
	termOrd   int64 // for NextOrdTerm iteration
	termsDone bool
}

func (a *sortedAsSS) Reset() error {
	a.termOrd = 0
	a.termsDone = false
	return a.sd.Reset()
}
func (a *sortedAsSS) NextDoc() (int, error)             { return a.sd.NextDoc() }
func (a *sortedAsSS) NextOrd() (int, error)             { return a.sd.OrdValue() }
func (a *sortedAsSS) LookupOrd(ord int) ([]byte, error) { return a.sd.LookupOrd(ord) }
func (a *sortedAsSS) GetValueCount() int                { return a.sd.GetValueCount() }
func (a *sortedAsSS) DocValueCount() (int, error)       { return 1, nil }
func (a *sortedAsSS) NextOrdTerm() ([]byte, error) {
	if a.termsDone || a.termOrd >= int64(a.sd.GetValueCount()) {
		a.termsDone = true
		return nil, nil
	}
	term, err := a.sd.LookupOrd(int(a.termOrd))
	a.termOrd++
	return term, err
}

// sortedSetAsSorted presents a dvSortedSetValues (single-value path) as
// dvSortedValues.
type sortedSetAsSorted struct {
	ss        dvSortedSetValues
	termOrd   int64
	termsDone bool
}

func (a *sortedSetAsSorted) Reset() error {
	a.termOrd = 0
	a.termsDone = false
	return a.ss.Reset()
}
func (a *sortedSetAsSorted) NextDoc() (int, error) { return a.ss.NextDoc() }
func (a *sortedSetAsSorted) OrdValue() (int, error) {
	ord, err := a.ss.NextOrd()
	return ord, err
}
func (a *sortedSetAsSorted) LookupOrd(ord int) ([]byte, error) { return a.ss.LookupOrd(ord) }
func (a *sortedSetAsSorted) GetValueCount() int                { return a.ss.GetValueCount() }

// ---------------------------------------------------------------------------
// dvDocIDIterator adapters for writeDVBitSet
// ---------------------------------------------------------------------------

type binaryDVIter struct {
	values dvBinaryValues
	doc    int
}

func binaryDVToDocIdSetIterator(values dvBinaryValues) dvDocIDIterator {
	return &binaryDVIter{values: values, doc: -1}
}
func (it *binaryDVIter) DocID() int { return it.doc }
func (it *binaryDVIter) NextDoc() (int, error) {
	doc, err := it.values.NextDoc()
	it.doc = doc
	return doc, err
}

type snDVDocIdSet struct {
	values dvSortedNumericValues
	doc    int
}

func sortedNumericToDocIdSet(values dvSortedNumericValues) dvDocIDIterator {
	return &snDVDocIdSet{values: values, doc: -1}
}
func (it *snDVDocIdSet) DocID() int { return it.doc }
func (it *snDVDocIdSet) NextDoc() (int, error) {
	doc, err := it.values.NextDoc()
	if err != nil {
		return 0, err
	}
	it.doc = doc
	if doc != dvNoMoreDocs {
		// consume all values for this doc so the iterator is consistent
		cnt, err := it.values.DocValueCount()
		if err != nil {
			return 0, err
		}
		for i := 0; i < cnt; i++ {
			if _, err := it.values.NextValue(); err != nil {
				return 0, err
			}
		}
	}
	return doc, nil
}

// ---------------------------------------------------------------------------
// DataOutputAt adapters for DirectMonotonicWriter
// ---------------------------------------------------------------------------

// dvChecksumDataOutputAt adapts *store.ChecksumIndexOutput to DataOutputAt.
type dvChecksumDataOutputAt struct {
	out *store.ChecksumIndexOutput
}

func (d dvChecksumDataOutputAt) WriteByte(b byte) error            { return d.out.WriteByte(b) }
func (d dvChecksumDataOutputAt) WriteBytes(b []byte) error         { return d.out.WriteBytes(b) }
func (d dvChecksumDataOutputAt) WriteBytesN(b []byte, n int) error { return d.out.WriteBytesN(b, n) }
func (d dvChecksumDataOutputAt) WriteShort(v int16) error          { return d.out.WriteShort(v) }
func (d dvChecksumDataOutputAt) WriteInt(v int32) error            { return d.out.WriteInt(v) }
func (d dvChecksumDataOutputAt) WriteLong(v int64) error           { return d.out.WriteLong(v) }
func (d dvChecksumDataOutputAt) WriteString(s string) error        { return d.out.WriteString(s) }
func (d dvChecksumDataOutputAt) GetFilePointer() int64             { return d.out.GetFilePointer() }

// bbdoIndexOutputAt adapts *store.ByteBuffersDataOutput to DataOutputAt
// (used for in-memory address buffers in addTermsDict).
type bbdoIndexOutputAt struct {
	out *store.ByteBuffersDataOutput
}

func (b bbdoIndexOutputAt) WriteByte(v byte) error            { return b.out.WriteByte(v) }
func (b bbdoIndexOutputAt) WriteBytes(v []byte) error         { return b.out.WriteBytes(v) }
func (b bbdoIndexOutputAt) WriteBytesN(v []byte, n int) error { return b.out.WriteBytesN(v, n) }
func (b bbdoIndexOutputAt) WriteShort(v int16) error          { return b.out.WriteShort(v) }
func (b bbdoIndexOutputAt) WriteInt(v int32) error            { return b.out.WriteInt(v) }
func (b bbdoIndexOutputAt) WriteLong(v int64) error           { return b.out.WriteLong(v) }
func (b bbdoIndexOutputAt) WriteString(s string) error        { return b.out.WriteString(s) }
func (b bbdoIndexOutputAt) GetFilePointer() int64             { return b.out.Size() }

// ---------------------------------------------------------------------------
// Utility functions
// ---------------------------------------------------------------------------

func isSortedSetSingleValued(values dvSortedSetValues) (bool, error) {
	if err := values.Reset(); err != nil {
		return false, err
	}
	for {
		doc, err := values.NextDoc()
		if err != nil {
			return false, err
		}
		if doc == dvNoMoreDocs {
			break
		}
		cnt, err := values.DocValueCount()
		if err != nil {
			return false, err
		}
		if cnt > 1 {
			return false, nil
		}
	}
	return true, nil
}

func commonPrefixLen(a, b []byte) int {
	limit := len(a)
	if len(b) < limit {
		limit = len(b)
	}
	for i := 0; i < limit; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return limit
}

// sortKeyLength returns the number of leading bytes of b that distinguish it
// from a when a < b lexicographically. This is Lucene's StringHelper.sortKeyLength.
func sortKeyLength(a, b []byte) int {
	cp := commonPrefixLen(a, b)
	if cp+1 >= len(b) {
		return len(b)
	}
	return cp + 1
}

func gcdOrOne(g int64) int64 {
	if g == 0 {
		return 1
	}
	if g < 0 {
		return -g
	}
	return g
}

func maxOf(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func minOf(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
