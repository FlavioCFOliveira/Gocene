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

package bkd

import (
	"errors"
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// histogramSize is the per-byte bucket count used by the offline
// radix partitioner. Mirrors Lucene's HISTOGRAM_SIZE constant.
const histogramSize = 256

// maxSizeOfflineBuffer is the upper bound (in bytes) on the on-heap
// buffer used when streaming points through the offline reader.
// Mirrors Lucene's MAX_SIZE_OFFLINE_BUFFER (8 KiB).
const maxSizeOfflineBuffer = 1024 * 8

// BKDRadixSelector partitions BKD-tree points either in memory or
// through a histogram-based offline pipeline. Port of
// org.apache.lucene.util.bkd.BKDRadixSelector (Lucene 10.4.0).
//
// The selector is constructed once and reused across many partition
// operations on the same configuration. It is not safe for concurrent
// use.
type BKDRadixSelector struct {
	// config bundles the dimension and per-doc layout parameters.
	config BKDConfig
	// maxPointsSortInHeap is the inclusive upper bound on the number
	// of points partitioned via in-memory radix selection before
	// switching to the offline pipeline.
	maxPointsSortInHeap int
	// tempDir is the directory used to allocate temporary writers
	// when the selector needs to materialise intermediate slices on
	// disk.
	tempDir store.Directory
	// tempFileNamePrefix is forwarded to temporary writer creation
	// so on-disk file names follow Lucene's <prefix>_bkd_<desc>_<n>
	// convention.
	tempFileNamePrefix string

	// bytesSorted is the total number of bytes considered by the
	// radix selector for a single point: the indexed-dim bytes for
	// the split dimension, then the data-dim bytes, then 4 bytes
	// for the docID. Matches Java's bytesSorted field.
	bytesSorted int
	// histogram is the reusable per-byte bucket count.
	histogram [histogramSize]int64
	// partitionBucket stores the leading bucket indices that define
	// the partition point as the histogram-based recursion deepens.
	partitionBucket []int
	// offlineBuffer is the reusable read buffer used by
	// OfflinePointReader; sized as a multiple of bytesPerDoc.
	offlineBuffer []byte
	// scratch holds the first point's projected key during the
	// common-prefix scan. Length: bytesSorted.
	scratch []byte
}

// NewBKDRadixSelector constructs a selector for the given
// configuration. maxPointsSortInHeap caps the in-memory selection
// pipeline; tempDir is where temporary point writers are created;
// tempFileNamePrefix is propagated to OfflinePointWriter's naming.
//
// Returns an error when the supplied tempDir is nil.
func NewBKDRadixSelector(
	config BKDConfig,
	maxPointsSortInHeap int,
	tempDir store.Directory,
	tempFileNamePrefix string,
) (*BKDRadixSelector, error) {
	if tempDir == nil {
		return nil, errors.New("bkd: tempDir cannot be nil")
	}
	if maxPointsSortInHeap < 0 {
		return nil, fmt.Errorf("bkd: maxPointsSortInHeap must be non-negative; got %d", maxPointsSortInHeap)
	}
	dataDimsBytes := (config.NumDims() - config.NumIndexDims()) * config.BytesPerDim()
	bytesSorted := config.BytesPerDim() + dataDimsBytes + integerBytes
	// Guard against degenerate sizing: when bytesPerDoc exceeds the
	// 8 KiB cap, Java's int division yields zero, leaving an empty
	// buffer. We mirror that behaviour but allocate at least one
	// point so OfflinePointReader's buffer-length check never trips.
	numberOfPointsOffline := maxSizeOfflineBuffer / config.BytesPerDoc()
	if numberOfPointsOffline < 1 {
		numberOfPointsOffline = 1
	}
	return &BKDRadixSelector{
		config:              config,
		maxPointsSortInHeap: maxPointsSortInHeap,
		tempDir:             tempDir,
		tempFileNamePrefix:  tempFileNamePrefix,
		bytesSorted:         bytesSorted,
		partitionBucket:     make([]int, bytesSorted),
		offlineBuffer:       make([]byte, numberOfPointsOffline*config.BytesPerDoc()),
		scratch:             make([]byte, bytesSorted),
	}, nil
}

// PathSlice is a sliced reference to a contiguous range of points in
// a PointWriter. Port of the package-private Java record
// BKDRadixSelector.PathSlice.
type PathSlice struct {
	// Writer is the backing writer holding the points.
	Writer PointWriter
	// Start is the index of the first point in the slice.
	Start int64
	// Count is the number of points in the slice.
	Count int64
}

// NewPathSlice returns a PathSlice initialised with the supplied
// fields. Provided for parity with Java's compact-constructor record.
func NewPathSlice(writer PointWriter, start, count int64) PathSlice {
	return PathSlice{Writer: writer, Start: start, Count: count}
}

// Select partitions the points in `points` (covering the half-open
// range [from, to)) around `partitionPoint` on dimension `dim`.
//
// On return, partitionSlices[0] holds (partitionPoint - from) points
// whose value on dim is lower than or equal to the values in
// partitionSlices[1], which holds (to - partitionPoint) points.
//
// dimCommonPrefix is a known lower bound on the common prefix length
// along `dim`; it lets the selector skip bytes already known to be
// shared. The returned slice carries the value of `dim` at the
// partition point (bytesPerDim bytes).
//
// When `points.Writer` is an OfflinePointWriter, the writer is
// destroyed in the process to recover disk space.
func (s *BKDRadixSelector) Select(
	points PathSlice,
	partitionSlices []PathSlice,
	from, to int64,
	partitionPoint int64,
	dim int,
	dimCommonPrefix int,
) ([]byte, error) {
	if err := s.checkArgs(from, to, partitionPoint); err != nil {
		return nil, err
	}
	if len(partitionSlices) < 2 {
		return nil, fmt.Errorf("bkd: partitionSlices must have length >= 2 (got %d)", len(partitionSlices))
	}

	// In-heap fast path: the entire slice lives in a HeapPointWriter,
	// so we partition without involving disk.
	if heap, ok := points.Writer.(*HeapPointWriter); ok {
		fromInt, err := toIntExact(from, "from")
		if err != nil {
			return nil, err
		}
		toInt, err := toIntExact(to, "to")
		if err != nil {
			return nil, err
		}
		partitionInt, err := toIntExact(partitionPoint, "partitionPoint")
		if err != nil {
			return nil, err
		}
		partition := s.heapRadixSelect(heap, dim, fromInt, toInt, partitionInt, dimCommonPrefix)
		partitionSlices[0] = PathSlice{Writer: points.Writer, Start: from, Count: partitionPoint - from}
		partitionSlices[1] = PathSlice{Writer: points.Writer, Start: partitionPoint, Count: to - partitionPoint}
		return partition, nil
	}

	offline, ok := points.Writer.(*OfflinePointWriter)
	if !ok {
		return nil, fmt.Errorf("bkd: unsupported PointWriter type %T", points.Writer)
	}

	left, err := s.getPointWriter(partitionPoint-from, fmt.Sprintf("left%d", dim))
	if err != nil {
		return nil, err
	}
	right, err := s.getPointWriter(to-partitionPoint, fmt.Sprintf("right%d", dim))
	if err != nil {
		_ = left.Close()
		_ = left.Destroy()
		return nil, err
	}

	// Java uses try-with-resources to close `left` and `right` after
	// buildHistogramAndPartition completes; the writers themselves are
	// referenced from the returned PathSlices, so their underlying
	// storage outlives the close. We mirror that by closing both
	// before returning success, regardless of how deep the recursion
	// went.
	partition, perr := s.buildHistogramAndPartition(
		offline, left, right,
		from, to, partitionPoint, 0, dimCommonPrefix, dim,
	)
	closeLeftErr := left.Close()
	closeRightErr := right.Close()

	if perr != nil {
		_ = left.Destroy()
		_ = right.Destroy()
		return nil, perr
	}
	if closeLeftErr != nil {
		_ = right.Destroy()
		return nil, closeLeftErr
	}
	if closeRightErr != nil {
		_ = left.Destroy()
		return nil, closeRightErr
	}

	partitionSlices[0] = PathSlice{Writer: left, Start: 0, Count: partitionPoint - from}
	partitionSlices[1] = PathSlice{Writer: right, Start: 0, Count: to - partitionPoint}
	return partition, nil
}

// checkArgs validates the half-open range invariant for the partition
// point. Java throws IllegalArgumentException; we map both branches to
// typed Go errors so callers can inspect them with errors.Is.
func (s *BKDRadixSelector) checkArgs(from, to, partitionPoint int64) error {
	if partitionPoint < from {
		return fmt.Errorf("bkd: partitionPoint must be >= from (got partitionPoint=%d from=%d)", partitionPoint, from)
	}
	if partitionPoint >= to {
		return fmt.Errorf("bkd: partitionPoint must be < to (got partitionPoint=%d to=%d)", partitionPoint, to)
	}
	return nil
}

// findCommonPrefixAndHistogram scans the offline points in the
// half-open range [from, to), tracking the longest common prefix
// across all points (in the radix-sort byte stream: split-dim bytes,
// then data-dim bytes, then docID), and building the histogram for
// the byte at the first divergence position. Mirrors the Java method
// of the same name. Returns the resolved commonPrefixPosition (which
// may equal bytesSorted if every point is identical).
func (s *BKDRadixSelector) findCommonPrefixAndHistogram(
	points *OfflinePointWriter,
	from, to int64,
	dim int,
	dimCommonPrefix int,
) (int, error) {
	commonPrefixPosition := s.bytesSorted
	offset := dim * s.config.BytesPerDim()

	reader, err := s.openOfflineReader(points, from, to-from)
	if err != nil {
		return 0, err
	}
	defer func() { _ = reader.Close() }()

	hasNext, err := reader.Next()
	if err != nil {
		return 0, err
	}
	if !hasNext {
		return 0, fmt.Errorf("bkd: offline reader empty for range [%d,%d)", from, to)
	}

	pointValue := reader.PointValue()
	combo := pointValue.PackedValueDocIDBytes()
	// First bytesPerDim bytes of the scratch are the split-dim bytes.
	copy(
		s.scratch[:s.config.BytesPerDim()],
		combo.Bytes[combo.Offset+offset:combo.Offset+offset+s.config.BytesPerDim()],
	)
	// Remaining bytes are the data-dim bytes followed by the 4-byte
	// docID. They live contiguously at offset packedIndexBytesLength
	// in the packed-value-doc-id stream.
	tailLen := (s.config.NumDims()-s.config.NumIndexDims())*s.config.BytesPerDim() + integerBytes
	pStart := combo.Offset + s.config.PackedIndexBytesLength()
	copy(
		s.scratch[s.config.BytesPerDim():s.config.BytesPerDim()+tailLen],
		combo.Bytes[pStart:pStart+tailLen],
	)

	for i := from + 1; i < to; i++ {
		hasNext, err = reader.Next()
		if err != nil {
			return 0, err
		}
		if !hasNext {
			return 0, fmt.Errorf("bkd: offline reader exhausted before expected end (i=%d to=%d)", i, to)
		}
		pointValue = reader.PointValue()

		if commonPrefixPosition == dimCommonPrefix {
			// No more checking needed; just finish the histogram for
			// the byte at commonPrefixPosition.
			s.histogram[s.getBucket(offset, commonPrefixPosition, pointValue)]++
			for j := i + 1; j < to; j++ {
				hasNext, err = reader.Next()
				if err != nil {
					return 0, err
				}
				if !hasNext {
					return 0, fmt.Errorf("bkd: offline reader exhausted before expected end (j=%d to=%d)", j, to)
				}
				pointValue = reader.PointValue()
				s.histogram[s.getBucket(offset, commonPrefixPosition, pointValue)]++
			}
			break
		}

		// Java: int startIndex = min(dimCommonPrefix, bytesPerDim);
		startIndex := dimCommonPrefix
		if startIndex > s.config.BytesPerDim() {
			startIndex = s.config.BytesPerDim()
		}
		// Java: int endIndex = min(commonPrefixPosition, bytesPerDim);
		endIndex := commonPrefixPosition
		if endIndex > s.config.BytesPerDim() {
			endIndex = s.config.BytesPerDim()
		}
		combo = pointValue.PackedValueDocIDBytes()
		j := mismatchRange(
			s.scratch[startIndex:endIndex],
			combo.Bytes[combo.Offset+offset+startIndex:combo.Offset+offset+endIndex],
		)
		if j == -1 {
			if commonPrefixPosition > s.config.BytesPerDim() {
				// Tie-break on data-dim bytes + docID.
				startTieBreak := s.config.PackedIndexBytesLength()
				endTieBreak := startTieBreak + commonPrefixPosition - s.config.BytesPerDim()
				k := mismatchRange(
					s.scratch[s.config.BytesPerDim():commonPrefixPosition],
					combo.Bytes[combo.Offset+startTieBreak:combo.Offset+endTieBreak],
				)
				if k != -1 {
					commonPrefixPosition = s.config.BytesPerDim() + k
					for h := range s.histogram {
						s.histogram[h] = 0
					}
					s.histogram[int(s.scratch[commonPrefixPosition])&0xff] = i - from
				}
			}
		} else {
			commonPrefixPosition = dimCommonPrefix + j
			for h := range s.histogram {
				s.histogram[h] = 0
			}
			s.histogram[int(s.scratch[commonPrefixPosition])&0xff] = i - from
		}
		if commonPrefixPosition != s.bytesSorted {
			s.histogram[s.getBucket(offset, commonPrefixPosition, pointValue)]++
		}
	}

	// Materialise the prefix bytes into the bucket trail.
	for i := 0; i < commonPrefixPosition; i++ {
		s.partitionBucket[i] = int(s.scratch[i]) & 0xff
	}
	return commonPrefixPosition, nil
}

// getBucket projects the byte at the given commonPrefixPosition (in
// the radix byte stream) for the supplied point. Below bytesPerDim
// the byte comes from the split dimension; at or above bytesPerDim it
// comes from the contiguous data-dim + docID block.
func (s *BKDRadixSelector) getBucket(offset int, commonPrefixPosition int, pointValue PointValue) int {
	if commonPrefixPosition < s.config.BytesPerDim() {
		packed := pointValue.PackedValue()
		return int(packed.Bytes[packed.Offset+offset+commonPrefixPosition]) & 0xff
	}
	combo := pointValue.PackedValueDocIDBytes()
	pos := combo.Offset + s.config.PackedIndexBytesLength() + commonPrefixPosition - s.config.BytesPerDim()
	return int(combo.Bytes[pos]) & 0xff
}

// buildHistogramAndPartition is the offline workhorse: it builds the
// per-byte histogram over [from, to), determines the bucket containing
// the partition point, materialises the left/right slices for the
// non-tied buckets, and recurses into the tied bucket (the "delta")
// until the partition is resolved. Mirrors the Java method of the
// same name.
func (s *BKDRadixSelector) buildHistogramAndPartition(
	points *OfflinePointWriter,
	left, right PointWriter,
	from, to, partitionPoint int64,
	iteration int,
	baseCommonPrefix int,
	dim int,
) ([]byte, error) {
	// Reset the histogram before the scan; findCommonPrefixAndHistogram
	// fills it incrementally and assumes a clean baseline.
	for i := range s.histogram {
		s.histogram[i] = 0
	}
	commonPrefix, err := s.findCommonPrefixAndHistogram(points, from, to, dim, baseCommonPrefix)
	if err != nil {
		return nil, err
	}

	// If all points are byte-for-byte equal across bytesSorted, the
	// partition reduces to splitting the docs by counts only.
	if commonPrefix == s.bytesSorted {
		if err := s.offlinePartition(points, left, right, nil, from, to, dim, commonPrefix-1, partitionPoint); err != nil {
			return nil, err
		}
		return s.partitionPointFromCommonPrefix(), nil
	}

	var leftCount, rightCount int64
	// Walk the histogram from the lowest bucket, accumulating until
	// the partition point lands inside the current bucket.
	for i := 0; i < histogramSize; i++ {
		size := s.histogram[i]
		if leftCount+size > partitionPoint-from {
			s.partitionBucket[commonPrefix] = i
			break
		}
		leftCount += size
	}
	for i := s.partitionBucket[commonPrefix] + 1; i < histogramSize; i++ {
		rightCount += s.histogram[i]
	}

	delta := s.histogram[s.partitionBucket[commonPrefix]]
	if leftCount+rightCount+delta != to-from {
		return nil, fmt.Errorf("bkd: invariant violation: leftCount(%d)+rightCount(%d)+delta(%d) != range(%d)",
			leftCount, rightCount, delta, to-from)
	}

	// All but the last byte are now resolved: the remaining tie at
	// this byte can be broken purely by counts.
	if commonPrefix == s.bytesSorted-1 {
		tieBreakCount := partitionPoint - from - leftCount
		if err := s.offlinePartition(points, left, right, nil, from, to, dim, commonPrefix, tieBreakCount); err != nil {
			return nil, err
		}
		return s.partitionPointFromCommonPrefix(), nil
	}

	deltaPoints, err := s.getDeltaPointWriter(left, right, delta, iteration)
	if err != nil {
		return nil, err
	}
	// We must close (and possibly destroy) the delta writer before
	// returning. Java handles this via try-with-resources around the
	// offlinePartition call only — the writer itself stays around to
	// be read back on the recursive call. We mirror that lifecycle
	// explicitly here.
	if err := s.offlinePartition(points, left, right, deltaPoints, from, to, dim, commonPrefix, 0); err != nil {
		_ = deltaPoints.Close()
		_ = deltaPoints.Destroy()
		return nil, err
	}
	if err := deltaPoints.Close(); err != nil {
		_ = deltaPoints.Destroy()
		return nil, err
	}

	newPartitionPoint := partitionPoint - from - leftCount

	// Lifecycle: when the delta is offline, the recursive call's
	// offlinePartition deletes its backing file. When the delta is on
	// heap, Destroy is a no-op. In neither case do we destroy the
	// delta writer here. Failure paths still scrub the file because
	// the recursive call may have aborted before reaching its own
	// destroy.
	switch dp := deltaPoints.(type) {
	case *HeapPointWriter:
		newPartitionInt, err := toIntExact(newPartitionPoint, "newPartitionPoint")
		if err != nil {
			return nil, err
		}
		countInt, err := toIntExact(deltaPoints.Count(), "deltaPoints.Count()")
		if err != nil {
			return nil, err
		}
		return s.heapPartition(dp, left, right, dim, 0, countInt, newPartitionInt, commonPrefix+1)
	case *OfflinePointWriter:
		partition, err := s.buildHistogramAndPartition(
			dp, left, right,
			0, deltaPoints.Count(), newPartitionPoint,
			iteration+1, commonPrefix+1, dim,
		)
		if err != nil {
			// The recursive call may have torn down its own source
			// already; an extra Destroy here is best-effort cleanup.
			_ = deltaPoints.Destroy()
			return nil, err
		}
		return partition, nil
	default:
		_ = deltaPoints.Destroy()
		return nil, fmt.Errorf("bkd: unexpected delta PointWriter type %T", deltaPoints)
	}
}

// offlinePartition streams every point in [from, to) once, routing it
// to `left`, `right`, or `deltaPoints` according to its byte at
// `bytePosition`. When `bytePosition` is the final byte (bytesSorted -
// 1), `numDocsTiebreak` left-side slots are filled first before the
// remainder spills to the right (the deltaPoints argument is unused at
// that point and must be nil).
func (s *BKDRadixSelector) offlinePartition(
	points *OfflinePointWriter,
	left, right PointWriter,
	deltaPoints PointWriter,
	from, to int64,
	dim int,
	bytePosition int,
	numDocsTiebreak int64,
) error {
	if !(bytePosition == s.bytesSorted-1 || deltaPoints != nil) {
		return fmt.Errorf("bkd: offlinePartition needs deltaPoints unless bytePosition is last (bytePosition=%d bytesSorted=%d)",
			bytePosition, s.bytesSorted)
	}
	offset := dim * s.config.BytesPerDim()
	var tieBreakCounter int64

	reader, err := s.openOfflineReader(points, from, to-from)
	if err != nil {
		return err
	}

	for {
		hasNext, err := reader.Next()
		if err != nil {
			_ = reader.Close()
			return err
		}
		if !hasNext {
			break
		}
		pv := reader.PointValue()
		bucket := s.getBucket(offset, bytePosition, pv)
		switch {
		case bucket < s.partitionBucket[bytePosition]:
			if err := left.AppendPointValue(pv); err != nil {
				_ = reader.Close()
				return err
			}
		case bucket > s.partitionBucket[bytePosition]:
			if err := right.AppendPointValue(pv); err != nil {
				_ = reader.Close()
				return err
			}
		default:
			if bytePosition == s.bytesSorted-1 {
				if tieBreakCounter < numDocsTiebreak {
					if err := left.AppendPointValue(pv); err != nil {
						_ = reader.Close()
						return err
					}
					tieBreakCounter++
				} else {
					if err := right.AppendPointValue(pv); err != nil {
						_ = reader.Close()
						return err
					}
				}
			} else {
				if err := deltaPoints.AppendPointValue(pv); err != nil {
					_ = reader.Close()
					return err
				}
			}
		}
	}
	if err := reader.Close(); err != nil {
		return err
	}
	// Delete the source file: the points have been routed to other
	// writers so the on-disk bytes are no longer needed.
	return points.Destroy()
}

// partitionPointFromCommonPrefix copies the leading bytesPerDim
// bytes of the resolved partition bucket trail into a fresh slice.
// Used when the partition is fully determined by the prefix.
func (s *BKDRadixSelector) partitionPointFromCommonPrefix() []byte {
	out := make([]byte, s.config.BytesPerDim())
	for i := 0; i < s.config.BytesPerDim(); i++ {
		out[i] = byte(s.partitionBucket[i])
	}
	return out
}

// heapPartition runs the in-memory radix selector on `points` and
// then streams each point into either `left` or `right` according to
// its index relative to `partitionPoint`. Mirrors Java's heapPartition.
func (s *BKDRadixSelector) heapPartition(
	points *HeapPointWriter,
	left, right PointWriter,
	dim, from, to int,
	partitionPoint int,
	commonPrefix int,
) ([]byte, error) {
	partition := s.heapRadixSelect(points, dim, from, to, partitionPoint, commonPrefix)
	for i := from; i < to; i++ {
		value := points.GetPackedValueSlice(i)
		if i < partitionPoint {
			if err := left.AppendPointValue(value); err != nil {
				return nil, err
			}
		} else {
			if err := right.AppendPointValue(value); err != nil {
				return nil, err
			}
		}
	}
	return partition, nil
}

// heapRadixSelect selects the partitionPoint-th element from the
// in-memory `points` writer along `dim`, treating the first
// `commonPrefixLength` bytes of the dimension as already shared.
// Returns the value of dim at the partition point (bytesPerDim bytes).
//
// Parity note on the fallback selector: Lucene's RadixSelector exposes
// a protected getFallbackSelector hook, and the Java BKDRadixSelector
// overrides it with an IntroSelector that compares by split-dim
// suffix, then data-dim bytes, then docID. Our util.RadixSelector does
// not currently expose such a hook; it falls back to a built-in
// comparison-based selector that consults the same RadixSelectorInterface
// byte stream byte-by-byte. Because our byte stream is laid out as
// <dim suffix> | <data dims> | <docID>, lexicographic comparison
// produces the same total ordering as Java's fallback — the
// IntroSelector override is an optimisation, not a behaviour
// difference. The same rationale applies in
// mutable_point_tree_reader_utils.go.
func (s *BKDRadixSelector) heapRadixSelect(
	points *HeapPointWriter,
	dim, from, to int,
	partitionPoint int,
	commonPrefixLength int,
) []byte {
	dimOffset := dim*s.config.BytesPerDim() + commonPrefixLength
	dimCmpBytes := s.config.BytesPerDim() - commonPrefixLength
	dataOffset := s.config.PackedIndexBytesLength() - dimCmpBytes

	impl := &heapRadixImpl{
		points:      points,
		dimOffset:   dimOffset,
		dimCmpBytes: dimCmpBytes,
		dataOffset:  dataOffset,
	}

	rs := util.NewRadixSelector(impl, s.bytesSorted-commonPrefixLength)
	rs.Select(from, to, partitionPoint)

	out := make([]byte, s.config.BytesPerDim())
	pv := points.GetPackedValueSlice(partitionPoint)
	packed := pv.PackedValue()
	copy(out, packed.Bytes[packed.Offset+dim*s.config.BytesPerDim():packed.Offset+dim*s.config.BytesPerDim()+s.config.BytesPerDim()])
	return out
}

// HeapRadixSort sorts the heap writer in-place by the specified
// dimension, treating `commonPrefixLength` leading bytes of that
// dimension as already shared. Port of Java's heapRadixSort, used to
// finalise leaf ordering. Not used by Select itself; exposed as the
// public companion entry point of the selector.
//
// Lucene marks this method package-private; we expose it because Go
// packages are coarser-grained than Java's and the BKD writer (which
// will land in a follow-up sprint) needs the entry point.
func (s *BKDRadixSelector) HeapRadixSort(
	points *HeapPointWriter,
	from, to int,
	dim int,
	commonPrefixLength int,
) {
	dimOffset := dim*s.config.BytesPerDim() + commonPrefixLength
	dimCmpBytes := s.config.BytesPerDim() - commonPrefixLength
	dataOffset := s.config.PackedIndexBytesLength() - dimCmpBytes

	impl := &heapRadixSortImpl{
		points:      points,
		dimOffset:   dimOffset,
		dimCmpBytes: dimCmpBytes,
		dataOffset:  dataOffset,
	}

	sorter := util.NewMSBRadixSorter(impl, s.bytesSorted-commonPrefixLength)
	sorter.Sort(from, to)
}

// getDeltaPointWriter chooses between a heap-backed and an offline
// writer for the "delta" slice (the tied bucket at the current
// commonPrefix). The choice mirrors Java's getDeltaPointWriter
// exactly: stay on heap when the delta fits within the budget left
// over after accounting for any heap allocation already taken by
// left/right; otherwise spill to disk.
func (s *BKDRadixSelector) getDeltaPointWriter(
	left, right PointWriter,
	delta int64,
	iteration int,
) (PointWriter, error) {
	if delta <= int64(s.getMaxPointsSortInHeap(left, right)) {
		size, err := toIntExact(delta, "delta")
		if err != nil {
			return nil, err
		}
		return NewHeapPointWriter(s.config, size), nil
	}
	return NewOfflinePointWriter(
		s.config, s.tempDir, s.tempFileNamePrefix,
		fmt.Sprintf("delta%d", iteration), delta,
	)
}

// getMaxPointsSortInHeap returns the remaining heap budget for a new
// HeapPointWriter, after subtracting the capacity of any heap writers
// already in flight. Java's assertion is mirrored as a runtime check
// in tests (the worker invariants make a negative result unreachable
// in production).
func (s *BKDRadixSelector) getMaxPointsSortInHeap(left, right PointWriter) int {
	pointsUsed := 0
	if heap, ok := left.(*HeapPointWriter); ok {
		pointsUsed += heap.Size()
	}
	if heap, ok := right.(*HeapPointWriter); ok {
		pointsUsed += heap.Size()
	}
	return s.maxPointsSortInHeap - pointsUsed
}

// getPointWriter allocates a fresh writer sized for `count` points.
// Heap writers are preferred when they fit within half of the heap
// budget — the worst-case recursion holds two heap writers
// simultaneously, so the budget is split evenly.
func (s *BKDRadixSelector) getPointWriter(count int64, desc string) (PointWriter, error) {
	if count <= int64(s.maxPointsSortInHeap/2) {
		size, err := toIntExact(count, "count")
		if err != nil {
			return nil, err
		}
		return NewHeapPointWriter(s.config, size), nil
	}
	return NewOfflinePointWriter(s.config, s.tempDir, s.tempFileNamePrefix, desc, count)
}

// openOfflineReader opens an OfflinePointReader from the writer's
// underlying file, reusing the selector's buffer. The writer must
// have been closed before this call (the OfflinePointReader expects
// the codec footer to be present).
func (s *BKDRadixSelector) openOfflineReader(
	points *OfflinePointWriter,
	start, length int64,
) (*OfflinePointReader, error) {
	return points.getReaderWithBuffer(start, length, s.offlineBuffer)
}

// mismatchRange returns the index (into the head slice) of the first
// position where `head` and `tail` differ, or -1 when the supplied
// ranges are equal. Both slices must have the same length; callers
// already slice them at matching ranges. Mirrors the semantics of
// Java's Arrays.mismatch(a, aStart, aEnd, b, bStart, bEnd) when both
// sub-ranges are the same length.
func mismatchRange(a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	if len(a) == len(b) {
		return -1
	}
	return n
}

// toIntExact narrows an int64 to int when it fits; otherwise returns
// a typed error mirroring Java's Math.toIntExact behaviour. Used at
// every int64 -> int boundary inside the selector.
func toIntExact(v int64, name string) (int, error) {
	if v > math.MaxInt32 || v < math.MinInt32 {
		return 0, fmt.Errorf("bkd: %s overflows int32 (got %d)", name, v)
	}
	return int(v), nil
}

// heapRadixImpl is the RadixSelectorInterface used by the in-memory
// pipeline. It projects the dim suffix followed by data-dim and docID
// bytes through ByteAt; Swap delegates straight to the underlying
// HeapPointWriter.
type heapRadixImpl struct {
	points      *HeapPointWriter
	dimOffset   int
	dimCmpBytes int
	dataOffset  int
}

// Swap delegates to the underlying heap writer.
func (h *heapRadixImpl) Swap(i, j int) { h.points.swap(i, j) }

// ByteAt maps the k-th radix byte (counted from commonPrefixLength)
// onto either the trailing dim bytes or the data-dim + docID block.
// Mirrors the anonymous class in Java's heapRadixSelect.
func (h *heapRadixImpl) ByteAt(i, k int) int {
	if k < h.dimCmpBytes {
		return h.points.byteAt(i, h.dimOffset+k)
	}
	return h.points.byteAt(i, h.dataOffset+k)
}

// heapRadixSortImpl is the MSBRadixSorterImpl variant used by
// HeapRadixSort. Structure mirrors heapRadixImpl; only the embedded
// algorithm differs (sort vs select).
type heapRadixSortImpl struct {
	points      *HeapPointWriter
	dimOffset   int
	dimCmpBytes int
	dataOffset  int
}

// Swap delegates to the underlying heap writer.
func (h *heapRadixSortImpl) Swap(i, j int) { h.points.swap(i, j) }

// ByteAt mirrors heapRadixImpl.ByteAt.
func (h *heapRadixSortImpl) ByteAt(i, k int) int {
	if k < h.dimCmpBytes {
		return h.points.byteAt(i, h.dimOffset+k)
	}
	return h.points.byteAt(i, h.dataOffset+k)
}
