// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Portions adapted from Apache Lucene 10.4.0
// (org.apache.lucene.codecs.lucene90.IndexedDISI):
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

// This file contains a package-local copy of the IndexedDISI iterator reader
// (dvIndexedDISI). It exists because codecs/lucene90 imports codecs (via
// stored_fields_format) which would create an import cycle if codecs imported
// codecs/lucene90 for the reader.
//
// Key deviation from codecs/lucene90.IndexedDISI:
//   - All multi-byte reads from the block stream use raw LE byte reads via
//     dvReadShortLE/dvReadLongLE instead of ReadShort()/ReadLong(), because the
//     writer (writeDVBitSet in lucene90_doc_values_bitset.go) emits LE bytes via
//     WriteByte, while SimpleFSIndexInput.ReadShort/ReadLong are big-endian.
//   - The jump table (stored in a RandomAccessInput) uses ReadIntAt which is
//     already LE in ByteArrayRandomAccessInput, matching dvFlushJumps output.
//   - AdvanceExact and DocIDRunEnd are included for completeness but not used
//     by the doc-values producer.

package codecs

import (
	"errors"
	"fmt"
	"io"
	"math/bits"

	"github.com/FlavioCFOliveira/Gocene/store"
)

// dvIndexedDISI is the package-local doc-values DISI reader.
// It mirrors codecs/lucene90.IndexedDISI but reads all in-block multi-byte
// values with little-endian byte order to match writeDVBitSet output.
type dvIndexedDISI struct {
	slice               store.IndexInput
	jumpTable           store.RandomAccessInput
	jumpTableEntryCount int
	denseRankPower      byte
	denseRankTable      []byte
	cost                int64

	doc            int
	block          int
	blockEnd       int64
	denseBitmapOff int64
	nextBlockIndex int
	method         dvDISIMethod
	index          int

	// SPARSE state
	exists              bool
	nextExistDocInBlock int

	// DENSE state
	word          uint64
	wordIndex     int
	numberOfOnes  int
	denseOrigoIdx int

	// ALL state
	gap int
}

type dvDISIMethod int

const (
	dvMethodSparse dvDISIMethod = iota
	dvMethodDense
	dvMethodAll
)

// newDVIndexedDISI constructs a dvIndexedDISI from an offset+length region
// of data, with an optional jump table at the tail.
func newDVIndexedDISI(data store.IndexInput, offset, length int64, jumpTableEntryCount int, denseRankPower byte, cost int64) (*dvIndexedDISI, error) {
	jumpTableBytes := int64(0)
	if jumpTableEntryCount > 0 {
		jumpTableBytes = int64(jumpTableEntryCount) * 4 * 2
	}
	blockSlice, err := data.Slice("dv-disi-docs", offset, length-jumpTableBytes)
	if err != nil {
		return nil, err
	}

	var jt store.RandomAccessInput
	if jumpTableEntryCount > 0 {
		saved := data.GetFilePointer()
		if err := data.SetPosition(offset + length - jumpTableBytes); err != nil {
			return nil, err
		}
		buf := make([]byte, jumpTableBytes)
		if err := data.ReadBytes(buf); err != nil {
			return nil, err
		}
		_ = data.SetPosition(saved)
		jt = store.NewByteArrayRandomAccessInput(buf)
	}

	if denseRankPower != 0xFF {
		if denseRankPower < 7 || denseRankPower > 15 {
			return nil, fmt.Errorf("lucene90 dv disi: invalid denseRankPower=%d", int8(denseRankPower))
		}
	}

	var rankTable []byte
	if denseRankPower != 0xFF {
		rankIndexShift := int(denseRankPower) - 7
		rankTable = make([]byte, dvDenseBlockLongs>>rankIndexShift)
	}

	return &dvIndexedDISI{
		slice:               blockSlice,
		jumpTable:           jt,
		jumpTableEntryCount: jumpTableEntryCount,
		denseRankPower:      denseRankPower,
		denseRankTable:      rankTable,
		cost:                cost,
		doc:                 -1,
		block:               -1,
		nextBlockIndex:      -1,
		index:               -1,
		nextExistDocInBlock: -1,
		wordIndex:           -1,
	}, nil
}

func (d *dvIndexedDISI) DocID() int  { return d.doc }
func (d *dvIndexedDISI) Index() int  { return d.index }
func (d *dvIndexedDISI) Cost() int64 { return d.cost }

func (d *dvIndexedDISI) NextDoc() (int, error) { return d.Advance(d.doc + 1) }

func (d *dvIndexedDISI) Advance(target int) (int, error) {
	targetBlock := target & 0xFFFF0000
	if d.block < targetBlock {
		if err := d.advanceBlock(targetBlock); err != nil {
			return 0, err
		}
	}
	if d.block == targetBlock {
		ok, err := d.advanceWithinBlock(target)
		if err != nil {
			return 0, err
		}
		if ok {
			return d.doc, nil
		}
		if err := d.readBlockHeader(); err != nil {
			return 0, err
		}
	}
	ok, err := d.advanceWithinBlock(d.block)
	if err != nil {
		return 0, err
	}
	if !ok {
		d.doc = dvNoMoreDocs
		return dvNoMoreDocs, nil
	}
	return d.doc, nil
}

// AdvanceExact positions the iterator at target if it exists.
func (d *dvIndexedDISI) AdvanceExact(target int) (bool, error) {
	targetBlock := target & 0xFFFF0000
	if d.block < targetBlock {
		if err := d.advanceBlock(targetBlock); err != nil {
			return false, err
		}
	}
	found := false
	if d.block == targetBlock {
		ok, err := d.advanceExactWithinBlock(target)
		if err != nil {
			return false, err
		}
		found = ok
	}
	d.doc = target
	return found, nil
}

func (d *dvIndexedDISI) advanceBlock(targetBlock int) error {
	blockIndex := targetBlock >> 16
	if d.jumpTable != nil && blockIndex >= (d.block>>16)+2 {
		inRange := blockIndex
		if inRange >= d.jumpTableEntryCount {
			inRange = d.jumpTableEntryCount - 1
		}
		// jump table entries are 2 ints each (index, offset), written LE by dvFlushJumps
		idx, err := d.jumpTable.ReadIntAt(int64(inRange) * 4 * 2)
		if err != nil {
			return err
		}
		off, err := d.jumpTable.ReadIntAt(int64(inRange)*4*2 + 4)
		if err != nil {
			return err
		}
		d.nextBlockIndex = int(idx) - 1
		if err := d.slice.SetPosition(int64(off)); err != nil {
			return err
		}
		return d.readBlockHeader()
	}
	for {
		if err := d.slice.SetPosition(d.blockEnd); err != nil {
			return err
		}
		if err := d.readBlockHeader(); err != nil {
			return err
		}
		if d.block >= targetBlock {
			return nil
		}
	}
}

// readBlockHeader reads the block header written by dvFlushBlock.
// The writer emits: LE short block, LE short (cardinality-1), then block body.
func (d *dvIndexedDISI) readBlockHeader() error {
	blockShort, err := dvReadShortLE(d.slice)
	if err != nil {
		return err
	}
	d.block = int(uint16(blockShort)) << 16
	if d.block < 0 {
		return fmt.Errorf("lucene90 dv disi: corrupt: negative block=%d", d.block)
	}
	cardMinus1, err := dvReadShortLE(d.slice)
	if err != nil {
		return err
	}
	numValues := 1 + int(uint16(cardMinus1))
	d.index = d.nextBlockIndex
	d.nextBlockIndex = d.index + numValues

	switch {
	case numValues <= dvMaxArrayLength:
		d.method = dvMethodSparse
		d.blockEnd = d.slice.GetFilePointer() + int64(numValues)*2
		d.nextExistDocInBlock = -1
	case numValues == dvBlockSize:
		d.method = dvMethodAll
		d.blockEnd = d.slice.GetFilePointer()
		d.gap = d.block - d.index - 1
	default:
		d.method = dvMethodDense
		d.denseBitmapOff = d.slice.GetFilePointer()
		if d.denseRankTable != nil {
			d.denseBitmapOff += int64(len(d.denseRankTable))
		}
		d.blockEnd = d.denseBitmapOff + (1 << 13) // 1024 longs × 8 bytes
		if d.denseRankPower != 0xFF {
			if err := d.slice.ReadBytes(d.denseRankTable); err != nil {
				return err
			}
		}
		d.wordIndex = -1
		d.numberOfOnes = d.index + 1
		d.denseOrigoIdx = d.numberOfOnes
	}
	return nil
}

func (d *dvIndexedDISI) advanceWithinBlock(target int) (bool, error) {
	switch d.method {
	case dvMethodAll:
		d.doc = target
		d.index = target - d.gap
		return true, nil

	case dvMethodSparse:
		targetInBlock := target & 0xFFFF
		for d.index < d.nextBlockIndex {
			docShort, err := dvReadShortLE(d.slice)
			if err != nil {
				if errors.Is(err, io.EOF) {
					return false, nil
				}
				return false, err
			}
			doc := int(uint16(docShort))
			d.index++
			if doc >= targetInBlock {
				d.doc = d.block | doc
				d.exists = true
				d.nextExistDocInBlock = doc
				return true, nil
			}
		}
		return false, nil

	case dvMethodDense:
		targetInBlock := target & 0xFFFF
		targetWordIndex := targetInBlock >> 6

		if d.denseRankPower != 0xFF && targetWordIndex-d.wordIndex >= (1<<(int(d.denseRankPower)-6)) {
			if err := d.denseRankSkip(targetInBlock); err != nil {
				return false, err
			}
		}

		for i := d.wordIndex + 1; i <= targetWordIndex; i++ {
			w, err := dvReadLongLE(d.slice)
			if err != nil {
				return false, err
			}
			d.word = uint64(w)
			d.numberOfOnes += bits.OnesCount64(d.word)
		}
		d.wordIndex = targetWordIndex

		leftBits := d.word >> uint(target&63)
		if leftBits != 0 {
			d.doc = target + bits.TrailingZeros64(leftBits)
			d.index = d.numberOfOnes - bits.OnesCount64(leftBits)
			return true, nil
		}

		for {
			d.wordIndex++
			if d.wordIndex >= dvDenseBlockLongs {
				return false, nil
			}
			w, err := dvReadLongLE(d.slice)
			if err != nil {
				return false, err
			}
			d.word = uint64(w)
			if d.word != 0 {
				d.index = d.numberOfOnes
				d.numberOfOnes += bits.OnesCount64(d.word)
				d.doc = d.block | (d.wordIndex << 6) | bits.TrailingZeros64(d.word)
				return true, nil
			}
		}
	}
	return false, nil
}

func (d *dvIndexedDISI) advanceExactWithinBlock(target int) (bool, error) {
	switch d.method {
	case dvMethodAll:
		d.index = target - d.gap
		return true, nil

	case dvMethodSparse:
		targetInBlock := target & 0xFFFF
		if d.nextExistDocInBlock > targetInBlock {
			return false, nil
		}
		if target == d.doc {
			return d.exists, nil
		}
		for d.index < d.nextBlockIndex {
			docShort, err := dvReadShortLE(d.slice)
			if err != nil {
				return false, err
			}
			doc := int(uint16(docShort))
			d.index++
			if doc >= targetInBlock {
				d.nextExistDocInBlock = doc
				if doc != targetInBlock {
					d.index--
					if err := d.slice.SetPosition(d.slice.GetFilePointer() - 2); err != nil {
						return false, err
					}
					break
				}
				d.exists = true
				return true, nil
			}
		}
		d.exists = false
		return false, nil

	case dvMethodDense:
		targetInBlock := target & 0xFFFF
		targetWordIndex := targetInBlock >> 6

		if d.denseRankPower != 0xFF && targetWordIndex-d.wordIndex >= (1<<(int(d.denseRankPower)-6)) {
			if err := d.denseRankSkip(targetInBlock); err != nil {
				return false, err
			}
		}

		for i := d.wordIndex + 1; i <= targetWordIndex; i++ {
			w, err := dvReadLongLE(d.slice)
			if err != nil {
				return false, err
			}
			d.word = uint64(w)
			d.numberOfOnes += bits.OnesCount64(d.word)
		}
		d.wordIndex = targetWordIndex

		leftBits := d.word >> uint(target&63)
		d.index = d.numberOfOnes - bits.OnesCount64(leftBits)
		return (leftBits & 1) != 0, nil
	}
	return false, nil
}

func (d *dvIndexedDISI) denseRankSkip(targetInBlock int) error {
	rankIndex := targetInBlock >> int(d.denseRankPower)
	rank := int(d.denseRankTable[rankIndex<<1]&0xFF)<<8 |
		int(d.denseRankTable[(rankIndex<<1)+1]&0xFF)

	rankAlignedWordIndex := rankIndex << int(d.denseRankPower) >> 6
	if err := d.slice.SetPosition(d.denseBitmapOff + int64(rankAlignedWordIndex)*8); err != nil {
		return err
	}
	w, err := dvReadLongLE(d.slice)
	if err != nil {
		return err
	}
	rankWord := uint64(w)

	d.wordIndex = rankAlignedWordIndex
	d.word = rankWord
	d.numberOfOnes = d.denseOrigoIdx + rank + bits.OnesCount64(rankWord)
	return nil
}

// dvReadShortLE reads 2 bytes LE from an IndexInput.
func dvReadShortLE(in store.IndexInput) (int16, error) {
	lo, err := in.ReadByte()
	if err != nil {
		return 0, err
	}
	hi, err := in.ReadByte()
	if err != nil {
		return 0, err
	}
	return int16(uint16(lo) | uint16(hi)<<8), nil
}

// dvReadLongLE reads 8 bytes LE from an IndexInput.
func dvReadLongLE(in store.IndexInput) (int64, error) {
	buf := make([]byte, 8)
	if err := in.ReadBytes(buf); err != nil {
		return 0, err
	}
	v := uint64(buf[0]) | uint64(buf[1])<<8 | uint64(buf[2])<<16 | uint64(buf[3])<<24 |
		uint64(buf[4])<<32 | uint64(buf[5])<<40 | uint64(buf[6])<<48 | uint64(buf[7])<<56
	return int64(v), nil
}
