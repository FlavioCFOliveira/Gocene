// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Licensed to the Apache Software Foundation (ASF) under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// The ASF licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0

// Ported from Apache Lucene 10.5.0:
//
//	lucene/backward-codecs/src/java/org/apache/lucene/backward_codecs/packed/LegacyDirectMonotonicReader.java

package packed

import (
	"errors"
	"math"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// legacyDirectMonotonicReaderBaseRamBytesUsed renders
// {@code RamUsageEstimator.shallowSizeOfInstance(
// LegacyDirectMonotonicReader.class)}.
const legacyDirectMonotonicReaderBaseRamBytesUsed = 48

// legacyDirectMonotonicMetaBaseRamBytesUsed renders
// {@code RamUsageEstimator.shallowSizeOfInstance(Meta.class)}.
const legacyDirectMonotonicMetaBaseRamBytesUsed = 48

// LegacyDirectMonotonicReaderLoadMeta loads metadata from the given
// IndexInput.
//
// Mirrors {@code public static Meta loadMeta(IndexInput metaIn, long
// numValues, int blockShift)}:
//
//	Meta meta = new Meta(numValues, blockShift);
//	for (int i = 0; i < meta.numBlocks; ++i) {
//	  meta.mins[i] = metaIn.readLong();
//	  meta.avgs[i] = Float.intBitsToFloat(metaIn.readInt());
//	  meta.offsets[i] = metaIn.readLong();
//	  meta.bpvs[i] = metaIn.readByte();
//	}
//	return meta;
func LegacyDirectMonotonicReaderLoadMeta(metaIn store.DataInput, numValues int64, blockShift int) (*LegacyDirectMonotonicMeta, error) {
	meta := NewLegacyDirectMonotonicMeta(numValues, blockShift)
	for i := 0; i < meta.NumBlocks; i++ {
		minValue, err := metaIn.ReadLong()
		if err != nil {
			return nil, err
		}
		meta.Mins[i] = minValue
		avgBits, err := metaIn.ReadInt()
		if err != nil {
			return nil, err
		}
		meta.Avgs[i] = math.Float32frombits(uint32(avgBits))
		offset, err := metaIn.ReadLong()
		if err != nil {
			return nil, err
		}
		meta.Offsets[i] = offset
		bpv, err := metaIn.ReadByte()
		if err != nil {
			return nil, err
		}
		meta.BPVs[i] = bpv
	}
	return meta, nil
}

// RamBytesUsed reproduces Meta.ramBytesUsed().
func (m *LegacyDirectMonotonicMeta) RamBytesUsed() int64 {
	return legacyDirectMonotonicMetaBaseRamBytesUsed +
		int64(len(m.Mins))*8 +
		int64(len(m.Avgs))*4 +
		int64(len(m.BPVs)) +
		int64(len(m.Offsets))*8
}

// LegacyDirectMonotonicReader retrieves an instance previously written by
// LegacyDirectMonotonicWriter.
//
// Mirrors {@code public final class LegacyDirectMonotonicReader extends
// LongValues implements Accountable} (Lucene 10.5.0).
type LegacyDirectMonotonicReader struct {
	blockShift  int
	readers     []util.LongValues
	mins        []int64
	avgs        []float32
	bpvs        []byte
	nonZeroBpvs int
}

// LegacyDirectMonotonicReaderGetInstance retrieves an instance from the
// specified slice.
//
// Mirrors {@code public static LegacyDirectMonotonicReader getInstance(Meta
// meta, RandomAccessInput data)} together with the private constructor it
// calls, whose argument-length checks raise IllegalArgumentException.
func LegacyDirectMonotonicReaderGetInstance(
	meta *LegacyDirectMonotonicMeta, data store.RandomAccessInput,
) (*LegacyDirectMonotonicReader, error) {
	readers := make([]util.LongValues, meta.NumBlocks)
	for i := 0; i < len(meta.Mins); i++ {
		if meta.BPVs[i] == 0 {
			readers[i] = util.ZeroLongValues
		} else {
			reader, err := LegacyDirectReaderGetInstanceAt(data, int(meta.BPVs[i]), meta.Offsets[i])
			if err != nil {
				return nil, err
			}
			readers[i] = reader
		}
	}
	if len(readers) != len(meta.Mins) ||
		len(readers) != len(meta.Avgs) ||
		len(readers) != len(meta.BPVs) {
		return nil, errors.New("backward_codecs/packed: LegacyDirectMonotonicReader: inconsistent meta array lengths")
	}
	nonZeroBpvs := 0
	for _, b := range meta.BPVs {
		if b != 0 {
			nonZeroBpvs++
		}
	}
	return &LegacyDirectMonotonicReader{
		blockShift:  meta.BlockShift,
		readers:     readers,
		mins:        meta.Mins,
		avgs:        meta.Avgs,
		bpvs:        meta.BPVs,
		nonZeroBpvs: nonZeroBpvs,
	}, nil
}

// Get reproduces
//
//	final int block = (int) (index >>> blockShift);
//	final long blockIndex = index & ((1 << blockShift) - 1);
//	final long delta = readers[block].get(blockIndex);
//	return mins[block] + (long) (avgs[block] * blockIndex) + delta;
func (r *LegacyDirectMonotonicReader) Get(index int64) (int64, error) {
	block := int(uint64(index) >> uint(r.blockShift))
	blockIndex := index & ((1 << uint(r.blockShift)) - 1)
	delta, err := r.readers[block].Get(blockIndex)
	if err != nil {
		return 0, err
	}
	return r.mins[block] + int64(r.avgs[block]*float32(blockIndex)) + delta, nil
}

// getBounds reproduces the private
// {@code long[] getBounds(long index)}: lower/upper bounds for the value at a
// given index without hitting the direct reader.
func (r *LegacyDirectMonotonicReader) getBounds(index int64) (int64, int64) {
	block := int(uint64(index) >> uint(r.blockShift))
	blockIndex := index & ((1 << uint(r.blockShift)) - 1)
	lowerBound := r.mins[block] + int64(r.avgs[block]*float32(blockIndex))
	upperBound := lowerBound + (int64(1) << uint(r.bpvs[block])) - 1
	if r.bpvs[block] == 64 || upperBound < lowerBound { // overflow
		return math.MinInt64, math.MaxInt64
	}
	return lowerBound, upperBound
}

// BinarySearch returns the index of a key if it exists, or its insertion
// point otherwise, like java.util.Arrays#binarySearch(long[], int, int, long).
//
// Mirrors {@code public long binarySearch(long fromIndex, long toIndex, long key)}.
func (r *LegacyDirectMonotonicReader) BinarySearch(fromIndex, toIndex, key int64) (int64, error) {
	if fromIndex < 0 || fromIndex > toIndex {
		return 0, errors.New("backward_codecs/packed: LegacyDirectMonotonicReader.BinarySearch: fromIndex/toIndex out of range")
	}
	lo := fromIndex
	hi := toIndex - 1

	for lo <= hi {
		mid := int64(uint64(lo+hi) >> 1)
		// Try to run as many iterations of the binary search as possible
		// without hitting the direct readers, since they might hit a page
		// fault.
		lower, upper := r.getBounds(mid)
		switch {
		case upper < key:
			lo = mid + 1
		case lower > key:
			hi = mid - 1
		default:
			midVal, err := r.Get(mid)
			if err != nil {
				return 0, err
			}
			switch {
			case midVal < key:
				lo = mid + 1
			case midVal > key:
				hi = mid - 1
			default:
				return mid, nil
			}
		}
	}

	return -1 - lo, nil
}

// RamBytesUsed reproduces
//
//	// Don't include meta, which should be accounted separately
//	return BASE_RAM_BYTES_USED
//	    + RamUsageEstimator.shallowSizeOf(readers)
//	    // Assume empty objects for the readers
//	    + nonZeroBpvs * RamUsageEstimator.alignObjectSize(NUM_BYTES_ARRAY_HEADER);
func (r *LegacyDirectMonotonicReader) RamBytesUsed() int64 {
	return legacyDirectMonotonicReaderBaseRamBytesUsed +
		util.ShallowSizeOf(r.readers) +
		int64(r.nonZeroBpvs)*util.NumBytesObjectRef
}

var (
	_ util.LongValues  = (*LegacyDirectMonotonicReader)(nil)
	_ util.Accountable = (*LegacyDirectMonotonicReader)(nil)
	_ util.Accountable = (*LegacyDirectMonotonicMeta)(nil)
)
