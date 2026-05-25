// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Portions adapted from Apache Lucene 10.4.0 (org.apache.lucene.codecs.lucene90.IndexedDISI):
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

// This file contains a package-local copy of the IndexedDISI bit-set writer
// (writeBitSet + helpers). It exists because codecs/lucene90 imports codecs
// (via stored_fields_format) which would create an import cycle if codecs
// imported codecs/lucene90 for the writer. The logic is identical to
// codecs/lucene90.WriteBitSet; only the package and symbol names differ.

package codecs

import (
	"math"
	"math/bits"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

const (
	dvBlockSize       = 65536
	dvDenseBlockLongs = dvBlockSize / 64 // 1024
	dvMaxArrayLength  = (1 << 12) - 1   // 4095 — SPARSE/DENSE threshold
	// dvNoMoreDocs mirrors dvNoMoreDocs = math.MaxInt32.
	dvNoMoreDocs = math.MaxInt32
	// dvDefaultDenseRankPower matches IndexedDISI.DEFAULT_DENSE_RANK_POWER.
	dvDefaultDenseRankPower byte = 9
)

// dvDocIDIterator is the minimal interface writeDVBitSet needs from its input.
// It mirrors the relevant subset of search.DocIdSetIterator.
type dvDocIDIterator interface {
	DocID() int
	NextDoc() (int, error)
}

// writeDVBitSet writes the docIDs from it into out using the Lucene 9.0
// IndexedDISI block format with DEFAULT_DENSE_RANK_POWER. Returns the
// number of jump-table entries appended at the end (must be stored in the
// field metadata so the reader can reconstruct the jump table).
//
// This mirrors codecs/lucene90.WriteBitSet byte-for-byte.
func writeDVBitSet(it dvDocIDIterator, out store.IndexOutput) (int16, error) {
	origo := out.GetFilePointer()
	totalCardinality := 0
	blockCardinality := 0
	buffer, err := util.NewFixedBitSet(dvBlockSize)
	if err != nil {
		return 0, err
	}
	jumps := make([]int, 2*2) // [index, offset] pairs; grown as needed
	lastBlock := 0

	doc, err := it.NextDoc()
	if err != nil {
		return 0, err
	}
	for doc != dvNoMoreDocs {
		blockHi := doc >> 16
		for doc != dvNoMoreDocs && (doc>>16) == blockHi {
			buffer.Set(doc & 0xFFFF)
			doc, err = it.NextDoc()
			if err != nil {
				return 0, err
			}
		}
		blockCardinality = buffer.Cardinality()
		jumps = dvAddBlockJumps(jumps, out.GetFilePointer()-origo, totalCardinality, lastBlock, blockHi+1)
		lastBlock = blockHi + 1
		if err := dvFlushBlock(blockHi, buffer, blockCardinality, dvDefaultDenseRankPower, out); err != nil {
			return 0, err
		}
		buffer.ClearAll()
		totalCardinality += blockCardinality
	}
	// Sentinel block (NO_MORE_DOCS)
	jumps = dvAddBlockJumps(jumps, out.GetFilePointer()-origo, totalCardinality, lastBlock, lastBlock+1)
	buffer.Set(dvNoMoreDocs & 0xFFFF)
	if err := dvFlushBlock(dvNoMoreDocs>>16, buffer, 1, dvDefaultDenseRankPower, out); err != nil {
		return 0, err
	}
	return dvFlushJumps(jumps, lastBlock+1, out)
}

func dvFlushBlock(block int, buffer *util.FixedBitSet, cardinality int, denseRankPower byte, out store.IndexOutput) error {
	if err := dvWriteShortLE(out, int16(block)); err != nil {
		return err
	}
	if err := dvWriteShortLE(out, int16(cardinality-1)); err != nil {
		return err
	}
	if cardinality > dvMaxArrayLength {
		if cardinality != dvBlockSize { // DENSE
			if denseRankPower != 0xFF {
				rank := dvCreateRank(buffer, denseRankPower)
				if err := out.WriteBytes(rank); err != nil {
					return err
				}
			}
			for _, w := range buffer.Bits() {
				if err := dvWriteLongLE(out, int64(w)); err != nil {
					return err
				}
			}
		}
		// ALL: no body
		return nil
	}
	// SPARSE
	doc := buffer.NextSetBit(0)
	for doc != dvNoMoreDocs && doc >= 0 && doc < dvBlockSize {
		if err := dvWriteShortLE(out, int16(doc)); err != nil {
			return err
		}
		if doc == dvBlockSize-1 {
			break
		}
		doc = buffer.NextSetBit(doc + 1)
	}
	return nil
}

func dvCreateRank(buffer *util.FixedBitSet, denseRankPower byte) []byte {
	longsPerRank := 1 << (denseRankPower - 6)
	rankMark := longsPerRank - 1
	rankIndexShift := int(denseRankPower) - 7
	rank := make([]byte, dvDenseBlockLongs>>rankIndexShift)
	wordsArr := buffer.Bits()
	bitCount := 0
	for word := 0; word < dvDenseBlockLongs; word++ {
		if (word & rankMark) == 0 {
			rank[word>>rankIndexShift] = byte(bitCount >> 8)
			rank[(word>>rankIndexShift)+1] = byte(bitCount & 0xFF)
		}
		bitCount += bits.OnesCount64(wordsArr[word])
	}
	return rank
}

func dvAddBlockJumps(jumps []int, offset int64, index, startBlock, endBlock int) []int {
	need := (endBlock + 1) * 2
	for len(jumps) < need {
		jumps = append(jumps, 0, 0)
	}
	for b := startBlock; b < endBlock; b++ {
		jumps[b*2] = index
		jumps[b*2+1] = int(offset)
	}
	return jumps
}

func dvFlushJumps(jumps []int, blockCount int, out store.IndexOutput) (int16, error) {
	if blockCount == 2 {
		blockCount = 0
	}
	for i := 0; i < blockCount; i++ {
		if err := dvWriteIntLE(out, int32(jumps[i*2])); err != nil {
			return 0, err
		}
		if err := dvWriteIntLE(out, int32(jumps[i*2+1])); err != nil {
			return 0, err
		}
	}
	return int16(blockCount), nil
}

func dvWriteShortLE(out store.IndexOutput, v int16) error {
	uv := uint16(v)
	if err := out.WriteByte(byte(uv)); err != nil {
		return err
	}
	return out.WriteByte(byte(uv >> 8))
}

func dvWriteIntLE(out store.IndexOutput, v int32) error {
	uv := uint32(v)
	for i := 0; i < 4; i++ {
		if err := out.WriteByte(byte(uv >> (8 * uint(i)))); err != nil {
			return err
		}
	}
	return nil
}

func dvWriteLongLE(out store.IndexOutput, v int64) error {
	uv := uint64(v)
	for i := 0; i < 8; i++ {
		if err := out.WriteByte(byte(uv >> (8 * uint(i)))); err != nil {
			return err
		}
	}
	return nil
}
