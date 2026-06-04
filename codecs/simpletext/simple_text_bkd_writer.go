// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package simpletext

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util"
	"github.com/FlavioCFOliveira/Gocene/util/bkd"
)

// SimpleTextMutablePointTree is the interface required by
// SimpleTextBKDWriter.WriteField. It combines the bkd.MutablePointTree
// sorting surface with Size and VisitDocValues so the writer can:
//
//   - query the total number of points (Size),
//   - sort points in-memory (Swap/GetValue/GetByteAt/GetDocID/Save/Restore),
//   - iterate packed values with a visitor (VisitDocValues).
//
// Mirrors org.apache.lucene.codecs.MutablePointTree (Lucene 10.4.0) extended
// with PointValues.PointTree.visitDocValues.
type SimpleTextMutablePointTree interface {
	bkd.MutablePointTree
	// Size returns the number of points in this tree.
	Size() int64
	// VisitDocValues invokes visitor.VisitByPackedValue for every point.
	VisitDocValues(visitor bkd.IntersectVisitor) error
}

// ---------------------------------------------------------------------------
// Constants and version numbers.
// ---------------------------------------------------------------------------

const (
	// CodecNameBKD is the codec name stamped in the BKD index header.
	CodecNameBKD = "BKD"
	// VersionStart is the first codec version.
	VersionStart = 0
	// VersionCompressedDocIDs was added in v1.
	VersionCompressedDocIDs = 1
	// VersionCompressedValues was added in v2.
	VersionCompressedValues = 2
	// VersionImplicitSplitDim1D was added in v3.
	VersionImplicitSplitDim1D = 3
	// VersionCurrent is the current codec version.
	VersionCurrent = VersionImplicitSplitDim1D
	// DefaultMaxMBSortInHeap is the default maximum RAM in MB before spilling to disk.
	DefaultMaxMBSortInHeap = float64(16.0)
)

// ---------------------------------------------------------------------------
// SimpleTextBKDWriter
// ---------------------------------------------------------------------------

// SimpleTextBKDWriter writes a BKD tree encoded as plain text. It is a
// simplified fork of BKDWriter tailored for the SimpleText codec.
//
// Port of org.apache.lucene.codecs.simpletext.SimpleTextBKDWriter
// (Lucene 10.4.0).
type SimpleTextBKDWriter struct {
	config           bkd.BKDConfig
	scratch          *util.BytesRefBuilder
	tempDir          *store.TrackingDirectoryWrapper
	tempFilePrefix   string
	maxMBSortInHeap  float64
	scratchDiff      []byte
	scratch1         []byte
	scratch2         []byte
	scratchBytesRef1 util.BytesRef
	scratchBytesRef2 util.BytesRef
	commonPfxLens    []int

	docsSeen *util.FixedBitSet

	pointWriter *bkd.HeapPointWriter
	finished    bool
	tempInput   store.IndexOutput

	maxPointsSortInHeap int

	minPackedValue []byte
	maxPackedValue []byte

	pointCount      int64
	totalPointCount int64
	maxDoc          int
}

// NewSimpleTextBKDWriter constructs a writer for a field with the given BKD
// configuration.
//
// Port of SimpleTextBKDWriter(int, Directory, String, BKDConfig, double, long).
func NewSimpleTextBKDWriter(
	maxDoc int,
	tempDir store.Directory,
	tempFilePrefix string,
	config bkd.BKDConfig,
	maxMBSortInHeap float64,
	totalPointCount int64,
) (*SimpleTextBKDWriter, error) {
	if err := VerifyParams(maxMBSortInHeap, totalPointCount); err != nil {
		return nil, err
	}
	docsSeenBS, err := util.NewFixedBitSet(maxDoc)
	if err != nil {
		return nil, fmt.Errorf("SimpleTextBKDWriter: NewFixedBitSet: %w", err)
	}
	maxPointsSortInHeap := int((maxMBSortInHeap * 1024 * 1024) /
		float64(config.BytesPerDoc()*config.NumDims()))
	if maxPointsSortInHeap < config.MaxPointsInLeafNode() {
		return nil, fmt.Errorf(
			"maxMBSortInHeap=%.2f only allows maxPointsSortInHeap=%d, "+
				"but this is less than config.MaxPointsInLeafNode()=%d; "+
				"either increase maxMBSortInHeap or decrease maxPointsInLeafNode",
			maxMBSortInHeap, maxPointsSortInHeap, config.MaxPointsInLeafNode(),
		)
	}
	return &SimpleTextBKDWriter{
		config:              config,
		scratch:             util.NewBytesRefBuilder(),
		tempDir:             store.NewTrackingDirectoryWrapper(tempDir),
		tempFilePrefix:      tempFilePrefix,
		maxMBSortInHeap:     maxMBSortInHeap,
		scratchDiff:         make([]byte, config.BytesPerDim()),
		scratch1:            make([]byte, config.PackedBytesLength()),
		scratch2:            make([]byte, config.PackedBytesLength()),
		commonPfxLens:       make([]int, config.NumDims()),
		docsSeen:            docsSeenBS,
		maxPointsSortInHeap: maxPointsSortInHeap,
		minPackedValue:      make([]byte, config.PackedIndexBytesLength()),
		maxPackedValue:      make([]byte, config.PackedIndexBytesLength()),
		totalPointCount:     totalPointCount,
		maxDoc:              maxDoc,
	}, nil
}

// VerifyParams validates constructor parameters.
func VerifyParams(maxMBSortInHeap float64, totalPointCount int64) error {
	if maxMBSortInHeap < 0.0 {
		return fmt.Errorf("maxMBSortInHeap must be >= 0.0 (got: %.2f)", maxMBSortInHeap)
	}
	if totalPointCount < 0 {
		return fmt.Errorf("totalPointCount must be >= 0 (got: %d)", totalPointCount)
	}
	return nil
}

// Add buffers one point for later indexing.
//
// Port of SimpleTextBKDWriter.add(byte[], int).
func (w *SimpleTextBKDWriter) Add(packedValue []byte, docID int) error {
	if len(packedValue) != w.config.PackedBytesLength() {
		return fmt.Errorf(
			"SimpleTextBKDWriter.Add: packedValue length=%d, want %d",
			len(packedValue), w.config.PackedBytesLength(),
		)
	}
	if w.pointCount >= w.totalPointCount {
		return fmt.Errorf(
			"SimpleTextBKDWriter: totalPointCount=%d exceeded at %d points",
			w.totalPointCount, w.pointCount+1,
		)
	}
	if w.pointWriter == nil {
		// Mirror SimpleTextBKDWriter.add: on the first point allocate the
		// in-heap writer sized to the caller-supplied totalPointCount upper
		// bound. HeapPointWriter is fixed-size (it never grows), so the
		// previous hard-coded 16 truncated every field with more than 16
		// points. Java additionally spills to an OfflinePointWriter when
		// totalPointCount exceeds maxPointsSortInHeap; that disk-spill path is
		// not yet ported here — see the totalPointCount guard below.
		if w.totalPointCount > int64(w.maxPointsSortInHeap) {
			return fmt.Errorf(
				"SimpleTextBKDWriter.Add: totalPointCount=%d exceeds maxPointsSortInHeap=%d; "+
					"the OfflinePointWriter disk-spill path is not yet ported (token: simpletext-bkd-offline-spill)",
				w.totalPointCount, w.maxPointsSortInHeap,
			)
		}
		w.pointWriter = bkd.NewHeapPointWriter(w.config, int(w.totalPointCount))
	}
	if err := w.pointWriter.Append(packedValue, docID); err != nil {
		return err
	}
	w.docsSeen.Set(docID)

	// track per-dim min/max
	for dim := 0; dim < w.config.NumIndexDims(); dim++ {
		offset := dim * w.config.BytesPerDim()
		end := offset + w.config.BytesPerDim()
		if w.pointCount == 0 {
			copy(w.minPackedValue[offset:end], packedValue[offset:end])
			copy(w.maxPackedValue[offset:end], packedValue[offset:end])
		} else {
			if bytes.Compare(packedValue[offset:end], w.minPackedValue[offset:end]) < 0 {
				copy(w.minPackedValue[offset:end], packedValue[offset:end])
			}
			if bytes.Compare(packedValue[offset:end], w.maxPackedValue[offset:end]) > 0 {
				copy(w.maxPackedValue[offset:end], packedValue[offset:end])
			}
		}
	}
	w.pointCount++
	return nil
}

// GetPointCount returns the number of points added so far.
func (w *SimpleTextBKDWriter) GetPointCount() int64 { return w.pointCount }

// WriteField writes points from a MutablePointTree directly (used during
// merge / flush from an in-memory buffer).
//
// Port of SimpleTextBKDWriter.writeField(IndexOutput, String, MutablePointTree).
func (w *SimpleTextBKDWriter) WriteField(
	out store.IndexOutput,
	fieldName string,
	reader SimpleTextMutablePointTree,
) (int64, error) {
	if w.config.NumIndexDims() == 1 {
		return w.writeField1Dim(out, reader)
	}
	return w.writeFieldNDims(out, reader)
}

// writeFieldNDims handles the multi-dimensional case.
func (w *SimpleTextBKDWriter) writeFieldNDims(
	out store.IndexOutput,
	values SimpleTextMutablePointTree,
) (int64, error) {
	if w.pointCount != 0 {
		return 0, fmt.Errorf("SimpleTextBKDWriter: cannot mix Add and WriteField")
	}
	if w.finished {
		return 0, fmt.Errorf("SimpleTextBKDWriter: already finished")
	}
	w.finished = true

	w.pointCount = values.Size()
	countPerLeaf := w.pointCount
	innerNodeCount := int64(1)

	for countPerLeaf > int64(w.config.MaxPointsInLeafNode()) {
		countPerLeaf = (countPerLeaf + 1) / 2
		innerNodeCount *= 2
	}
	numLeaves := int(innerNodeCount)
	if err := w.checkMaxLeafNodeCount(numLeaves); err != nil {
		return 0, err
	}

	splitPackedValues := make([]byte, numLeaves*(w.config.BytesPerDim()+1))
	leafBlockFPs := make([]int64, numLeaves)

	// Seed the per-dim extremes so the running comparison below converges:
	// min starts at all-0xff (any value is <=) and max at all-0x00 (any value
	// is >=). Mirrors Arrays.fill(minPackedValue, 0xff) / fill(maxPackedValue,
	// 0) in SimpleTextBKDWriter.writeFieldNDims. Without this the zero-filled
	// minPackedValue would never be replaced (nothing compares < all-zeros
	// unsigned), corrupting the MIN_VALUE line.
	for i := range w.minPackedValue {
		w.minPackedValue[i] = 0xff
	}
	for i := range w.maxPackedValue {
		w.maxPackedValue[i] = 0
	}

	// compute min/max
	for i := 0; i < int(w.pointCount); i++ {
		values.GetValue(i, &w.scratchBytesRef1)
		for dim := 0; dim < w.config.NumIndexDims(); dim++ {
			offset := dim * w.config.BytesPerDim()
			end := offset + w.config.BytesPerDim()
			src := w.scratchBytesRef1.Bytes[w.scratchBytesRef1.Offset+offset : w.scratchBytesRef1.Offset+end]
			if bytes.Compare(src, w.minPackedValue[offset:end]) < 0 {
				copy(w.minPackedValue[offset:end], src)
			}
			if bytes.Compare(src, w.maxPackedValue[offset:end]) > 0 {
				copy(w.maxPackedValue[offset:end], src)
			}
		}
		w.docsSeen.Set(values.GetDocID(i))
	}

	if err := w.buildFromMutable(
		1, numLeaves, values, 0, int(w.pointCount), out,
		w.minPackedValue, w.maxPackedValue, splitPackedValues, leafBlockFPs,
		make([]int, w.config.MaxPointsInLeafNode()),
	); err != nil {
		return 0, err
	}

	indexFP := out.GetFilePointer()
	if err := w.writeIndex(out, leafBlockFPs, splitPackedValues, int(countPerLeaf)); err != nil {
		return 0, err
	}
	return indexFP, nil
}

// writeField1Dim handles the optimised single-dimension case.
func (w *SimpleTextBKDWriter) writeField1Dim(
	out store.IndexOutput,
	reader SimpleTextMutablePointTree,
) (int64, error) {
	bkd.SortMutablePointTree(w.config, w.maxDoc, reader, 0, int(reader.Size()))
	ow := newOneDimensionWriter(w, out)
	if err := reader.VisitDocValues(&oneDimVisitor{ow: ow}); err != nil {
		return 0, err
	}
	return ow.finish()
}

// ---------------------------------------------------------------------------
// Finish — flushes buffered points (add path).
// ---------------------------------------------------------------------------

// Finish writes the BKD tree to out and returns the index file pointer.
//
// Port of SimpleTextBKDWriter.finish(IndexOutput).
func (w *SimpleTextBKDWriter) Finish(out store.IndexOutput) (int64, error) {
	if w.pointCount == 0 {
		return 0, fmt.Errorf("SimpleTextBKDWriter.Finish: must index at least one point")
	}
	if w.finished {
		return 0, fmt.Errorf("SimpleTextBKDWriter.Finish: already finished")
	}
	w.finished = true

	if w.pointWriter != nil {
		if err := w.pointWriter.Close(); err != nil {
			return 0, err
		}
	}
	points := bkd.PathSlice{Writer: w.pointWriter, Start: 0, Count: w.pointCount}
	w.tempInput = nil
	w.pointWriter = nil

	countPerLeaf := w.pointCount
	innerNodeCount := int64(1)
	for countPerLeaf > int64(w.config.MaxPointsInLeafNode()) {
		countPerLeaf = (countPerLeaf + 1) / 2
		innerNodeCount *= 2
	}
	numLeaves := int(innerNodeCount)
	if err := w.checkMaxLeafNodeCount(numLeaves); err != nil {
		return 0, err
	}

	splitPackedValues := make([]byte, numLeaves*(1+w.config.BytesPerDim()))
	leafBlockFPs := make([]int64, numLeaves)

	radixSelector, err := bkd.NewBKDRadixSelector(w.config, w.maxPointsSortInHeap, w.tempDir, w.tempFilePrefix)
	if err != nil {
		return 0, err
	}

	var success bool
	defer func() {
		if !success {
			for name := range w.tempDir.GetCreatedFiles() {
				_ = w.tempDir.DeleteFile(name)
			}
		}
	}()

	if err := w.buildFromPathSlice(
		1, numLeaves, points, out, radixSelector,
		append([]byte(nil), w.minPackedValue...),
		append([]byte(nil), w.maxPackedValue...),
		splitPackedValues, leafBlockFPs,
		make([]int, w.config.MaxPointsInLeafNode()),
	); err != nil {
		return 0, err
	}
	success = true

	indexFP := out.GetFilePointer()
	if err := w.writeIndex(out, leafBlockFPs, splitPackedValues, int(countPerLeaf)); err != nil {
		return 0, err
	}
	return indexFP, nil
}

// ---------------------------------------------------------------------------
// Build — recursive tree construction from MutablePointTree.
// ---------------------------------------------------------------------------

func (w *SimpleTextBKDWriter) buildFromMutable(
	nodeID, leafNodeOffset int,
	reader SimpleTextMutablePointTree,
	from, to int,
	out store.IndexOutput,
	minPackedValue, maxPackedValue []byte,
	splitPackedValues []byte,
	leafBlockFPs []int64,
	spareDocIDs []int,
) error {
	if nodeID >= leafNodeOffset {
		// Leaf node
		count := to - from
		for i := range w.commonPfxLens {
			w.commonPfxLens[i] = w.config.BytesPerDim()
		}
		reader.GetValue(from, &w.scratchBytesRef1)
		for i := from + 1; i < to; i++ {
			reader.GetValue(i, &w.scratchBytesRef2)
			for dim := 0; dim < w.config.NumDims(); dim++ {
				offset := dim * w.config.BytesPerDim()
				for j := 0; j < w.commonPfxLens[dim]; j++ {
					if w.scratchBytesRef1.Bytes[w.scratchBytesRef1.Offset+offset+j] !=
						w.scratchBytesRef2.Bytes[w.scratchBytesRef2.Offset+offset+j] {
						w.commonPfxLens[dim] = j
						break
					}
				}
			}
		}

		sortedDim := 0
		sortedDimCardinality := -1
		usedBytes := make([]*util.FixedBitSet, w.config.NumDims())
		for dim := 0; dim < w.config.NumDims(); dim++ {
			if w.commonPfxLens[dim] < w.config.BytesPerDim() {
				bs, err := util.NewFixedBitSet(256)
				if err != nil {
					return err
				}
				usedBytes[dim] = bs
			}
		}
		for dim := 0; dim < w.config.NumDims(); dim++ {
			prefix := w.commonPfxLens[dim]
			if prefix < w.config.BytesPerDim() {
				dimOffset := dim * w.config.BytesPerDim()
				for i := from; i < to; i++ {
					reader.GetValue(i, &w.scratchBytesRef1)
					bucket := int(w.scratchBytesRef1.Bytes[w.scratchBytesRef1.Offset+dimOffset+prefix] & 0xff)
					usedBytes[dim].Set(bucket)
				}
				cardinality := usedBytes[dim].Cardinality()
				if sortedDimCardinality < 0 || cardinality < sortedDimCardinality {
					sortedDim = dim
					sortedDimCardinality = cardinality
				}
			}
		}

		bkd.SortMutablePointTreeByDim(w.config, sortedDim, w.commonPfxLens, reader, from, to, &w.scratchBytesRef1, &w.scratchBytesRef2)

		leafBlockFPs[nodeID-leafNodeOffset] = out.GetFilePointer()

		for i := 0; i < count; i++ {
			spareDocIDs[i] = reader.GetDocID(from + i)
		}
		if err := w.writeLeafBlockDocs(out, spareDocIDs, 0, count); err != nil {
			return err
		}

		pv := func(i int) []byte {
			reader.GetValue(from+i, &w.scratchBytesRef1)
			return w.scratchBytesRef1.Bytes[w.scratchBytesRef1.Offset : w.scratchBytesRef1.Offset+w.config.PackedBytesLength()]
		}
		return w.writeLeafBlockPackedValues(out, w.commonPfxLens, count, sortedDim, pv)
	}

	// Inner node: partition
	splitDim := 0
	if w.config.NumIndexDims() > 1 {
		splitDim = w.splitDim(minPackedValue, maxPackedValue)
	}

	rightCount := int64(to-from) / 2
	leftCount := int64(to-from) - rightCount

	bpd := w.config.BytesPerDim()

	// sort along splitDim (commonPfxLens is updated during leaf processing)
	bkd.SortMutablePointTreeByDim(w.config, splitDim, w.commonPfxLens, reader, from, to, &w.scratchBytesRef1, &w.scratchBytesRef2)

	splitOff := from + int(leftCount)
	reader.GetValue(splitOff, &w.scratchBytesRef1)
	splitValue := w.scratchBytesRef1.Bytes[w.scratchBytesRef1.Offset+splitDim*bpd : w.scratchBytesRef1.Offset+splitDim*bpd+bpd]

	address := nodeID * (1 + bpd)
	splitPackedValues[address] = byte(splitDim)
	copy(splitPackedValues[address+1:], splitValue)

	maxSplit := append([]byte(nil), maxPackedValue...)
	copy(maxSplit[splitDim*bpd:], splitValue)
	minSplit := append([]byte(nil), minPackedValue...)
	copy(minSplit[splitDim*bpd:], splitValue)

	if err := w.buildFromMutable(2*nodeID, leafNodeOffset, reader, from, splitOff,
		out, minPackedValue, maxSplit, splitPackedValues, leafBlockFPs, spareDocIDs); err != nil {
		return err
	}
	return w.buildFromMutable(2*nodeID+1, leafNodeOffset, reader, splitOff, to,
		out, minSplit, maxPackedValue, splitPackedValues, leafBlockFPs, spareDocIDs)
}

// ---------------------------------------------------------------------------
// Build — recursive tree construction from PathSlice (add path).
// ---------------------------------------------------------------------------

func (w *SimpleTextBKDWriter) buildFromPathSlice(
	nodeID, leafNodeOffset int,
	points bkd.PathSlice,
	out store.IndexOutput,
	radixSelector *bkd.BKDRadixSelector,
	minPackedValue, maxPackedValue []byte,
	splitPackedValues []byte,
	leafBlockFPs []int64,
	spareDocIDs []int,
) error {
	if nodeID >= leafNodeOffset {
		// Leaf
		var heapSrc *bkd.HeapPointWriter
		if hp, ok := points.Writer.(*bkd.HeapPointWriter); ok {
			heapSrc = hp
		} else {
			var err error
			heapSrc, err = w.switchToHeap(points.Writer)
			if err != nil {
				return err
			}
		}

		from := int(points.Start)
		to := int(points.Start + points.Count)

		// computeCommonPrefixLength / the cardinality scan / radix sort all
		// operate on the shared heap's [from, to) sub-range, because the
		// BKDRadixSelector heap fast path returns slices over the same writer
		// (see BKDRadixSelector.Select). Iterating 0..Size() would read points
		// from sibling leaves and panic past the written count.
		w.computeCommonPrefixLength(heapSrc, w.scratch1, from, to)

		sortedDim := 0
		sortedDimCardinality := -1
		usedBytes := make([]*util.FixedBitSet, w.config.NumDims())
		for dim := 0; dim < w.config.NumDims(); dim++ {
			if w.commonPfxLens[dim] < w.config.BytesPerDim() {
				bs, err := util.NewFixedBitSet(256)
				if err != nil {
					return err
				}
				usedBytes[dim] = bs
			}
		}
		for dim := 0; dim < w.config.NumDims(); dim++ {
			prefix := w.commonPfxLens[dim]
			if prefix < w.config.BytesPerDim() {
				offset := dim * w.config.BytesPerDim()
				for i := from; i < to; i++ {
					pval := heapSrc.GetPackedValueSlice(i)
					pvBytes := pval.PackedValue()
					bucket := int(pvBytes.Bytes[pvBytes.Offset+offset+prefix] & 0xff)
					usedBytes[dim].Set(bucket)
				}
				cardinality := usedBytes[dim].Cardinality()
				if sortedDimCardinality < 0 || cardinality < sortedDimCardinality {
					sortedDim = dim
					sortedDimCardinality = cardinality
				}
			}
		}

		radixSelector.HeapRadixSort(heapSrc, from, to, sortedDim, w.commonPfxLens[sortedDim])
		leafBlockFPs[nodeID-leafNodeOffset] = out.GetFilePointer()

		count := to - from
		for i := 0; i < count; i++ {
			spareDocIDs[i] = heapSrc.GetPackedValueSlice(from + i).DocID()
		}
		if err := w.writeLeafBlockDocs(out, spareDocIDs, 0, count); err != nil {
			return err
		}

		pv := func(i int) []byte {
			v := heapSrc.GetPackedValueSlice(from + i)
			pvb := v.PackedValue()
			return pvb.Bytes[pvb.Offset : pvb.Offset+w.config.PackedBytesLength()]
		}
		return w.writeLeafBlockPackedValues(out, w.commonPfxLens, count, sortedDim, pv)
	}

	// Inner node
	splitDim := 0
	if w.config.NumIndexDims() > 1 {
		splitDim = w.splitDim(minPackedValue, maxPackedValue)
	}

	bpd := w.config.BytesPerDim()
	commonPfxLen := 0
	for j := 0; j < bpd; j++ {
		if minPackedValue[splitDim*bpd+j] != maxPackedValue[splitDim*bpd+j] {
			commonPfxLen = j
			break
		}
		if j == bpd-1 {
			commonPfxLen = bpd
		}
	}

	rightCount := points.Count / 2
	leftCount := points.Count - rightCount
	partitionPoint := points.Start + leftCount

	pathSlices := make([]bkd.PathSlice, 2)
	splitValue, err := radixSelector.Select(points, pathSlices,
		points.Start, points.Start+points.Count, partitionPoint, splitDim, commonPfxLen)
	if err != nil {
		return err
	}

	address := nodeID * (1 + bpd)
	splitPackedValues[address] = byte(splitDim)
	copy(splitPackedValues[address+1:], splitValue)

	minSplit := append([]byte(nil), minPackedValue...)
	copy(minSplit[splitDim*bpd:], splitValue)
	maxSplit := append([]byte(nil), maxPackedValue...)
	copy(maxSplit[splitDim*bpd:], splitValue)

	if err := w.buildFromPathSlice(2*nodeID, leafNodeOffset, pathSlices[0], out, radixSelector,
		minPackedValue, maxSplit, splitPackedValues, leafBlockFPs, spareDocIDs); err != nil {
		return err
	}
	return w.buildFromPathSlice(2*nodeID+1, leafNodeOffset, pathSlices[1], out, radixSelector,
		minSplit, maxPackedValue, splitPackedValues, leafBlockFPs, spareDocIDs)
}

// ---------------------------------------------------------------------------
// Index / leaf writing.
// ---------------------------------------------------------------------------

func (w *SimpleTextBKDWriter) writeIndex(
	out store.IndexOutput,
	leafBlockFPs []int64,
	splitPackedValues []byte,
	maxPtsInLeaf int,
) error {
	bpd := w.config.BytesPerDim()
	type line struct {
		label []byte
		val   string
	}
	lines := []line{
		{PwNumDataDims, strconv.Itoa(w.config.NumDims())},
		{PwNumIndexDims, strconv.Itoa(w.config.NumIndexDims())},
		{PwBytesPerDim, strconv.Itoa(bpd)},
		{PwMaxLeafPts, strconv.Itoa(maxPtsInLeaf)},
		{PwIndexCount, strconv.Itoa(len(leafBlockFPs))},
		{PwMinValue, bytesRefString(w.minPackedValue)},
		{PwMaxValue, bytesRefString(w.maxPackedValue)},
		{PwPointCount, strconv.FormatInt(w.pointCount, 10)},
		{PwDocCount, strconv.Itoa(w.docsSeen.Cardinality())},
	}
	for _, l := range lines {
		if err := stWrite(out, l.label, w.scratch); err != nil {
			return err
		}
		if err := stWriteStr(out, l.val, w.scratch); err != nil {
			return err
		}
		if err := stWriteNewline(out); err != nil {
			return err
		}
	}
	for _, fp := range leafBlockFPs {
		if err := stWrite(out, PwBlockFP, w.scratch); err != nil {
			return err
		}
		if err := stWriteStr(out, strconv.FormatInt(fp, 10), w.scratch); err != nil {
			return err
		}
		if err := stWriteNewline(out); err != nil {
			return err
		}
	}
	count := len(splitPackedValues) / (1 + bpd)
	if err := stWrite(out, PwSplitCount, w.scratch); err != nil {
		return err
	}
	if err := stWriteStr(out, strconv.Itoa(count), w.scratch); err != nil {
		return err
	}
	if err := stWriteNewline(out); err != nil {
		return err
	}
	for i := 0; i < count; i++ {
		base := i * (1 + bpd)
		if err := stWrite(out, PwSplitDim, w.scratch); err != nil {
			return err
		}
		if err := stWriteStr(out, strconv.Itoa(int(splitPackedValues[base]&0xff)), w.scratch); err != nil {
			return err
		}
		if err := stWriteNewline(out); err != nil {
			return err
		}
		if err := stWrite(out, PwSplitValue, w.scratch); err != nil {
			return err
		}
		if err := stWriteStr(out, bytesRefString(splitPackedValues[base+1:base+1+bpd]), w.scratch); err != nil {
			return err
		}
		if err := stWriteNewline(out); err != nil {
			return err
		}
	}
	return nil
}

func (w *SimpleTextBKDWriter) writeLeafBlockDocs(out store.DataOutput, docIDs []int, start, count int) error {
	if err := stWrite(out, PwBlockCount, w.scratch); err != nil {
		return err
	}
	if err := stWriteStr(out, strconv.Itoa(count), w.scratch); err != nil {
		return err
	}
	if err := stWriteNewline(out); err != nil {
		return err
	}
	for i := 0; i < count; i++ {
		if err := stWrite(out, PwBlockDocID, w.scratch); err != nil {
			return err
		}
		if err := stWriteStr(out, strconv.Itoa(docIDs[start+i]), w.scratch); err != nil {
			return err
		}
		if err := stWriteNewline(out); err != nil {
			return err
		}
	}
	return nil
}

func (w *SimpleTextBKDWriter) writeLeafBlockPackedValues(
	out store.DataOutput,
	_ []int, // commonPrefixLengths — SimpleText does not prefix-code
	count, _ int, // sortedDim — ignored
	packedValues func(i int) []byte,
) error {
	for i := 0; i < count; i++ {
		if err := stWrite(out, PwBlockValue, w.scratch); err != nil {
			return err
		}
		if err := stWriteStr(out, bytesRefString(packedValues(i)), w.scratch); err != nil {
			return err
		}
		if err := stWriteNewline(out); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helper methods.
// ---------------------------------------------------------------------------

func (w *SimpleTextBKDWriter) splitDim(minPackedValue, maxPackedValue []byte) int {
	splitDim := -1
	for dim := 0; dim < w.config.NumIndexDims(); dim++ {
		bpd := w.config.BytesPerDim()
		if err := util.Subtract(bpd, dim, maxPackedValue, minPackedValue, w.scratchDiff); err != nil {
			continue
		}
		if splitDim == -1 || bytes.Compare(w.scratchDiff, w.scratch1) > 0 {
			copy(w.scratch1, w.scratchDiff)
			splitDim = dim
		}
	}
	return splitDim
}

func (w *SimpleTextBKDWriter) switchToHeap(src bkd.PointWriter) (*bkd.HeapPointWriter, error) {
	count := int(src.Count())
	reader, err := src.GetReader(0, int64(count))
	if err != nil {
		return nil, err
	}
	defer func() { _ = reader.Close() }()
	heap := bkd.NewHeapPointWriter(w.config, count)
	for i := 0; i < count; i++ {
		ok, err := reader.Next()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		if err := heap.AppendPointValue(reader.PointValue()); err != nil {
			return nil, err
		}
	}
	if err := src.Close(); err != nil {
		return nil, err
	}
	return heap, nil
}

// computeCommonPrefixLength scans the heap range [from, to) and emits the
// per-dim common prefix lengths into w.commonPfxLens; the first point's bytes
// are copied into commonPrefix. It must be bounded by from/to because the
// BKDRadixSelector heap fast path returns PathSlices that share the parent
// HeapPointWriter (each leaf covers a sub-range, not the whole writer); a
// 0..Size() scan would read points belonging to sibling leaves and overrun the
// written count. Mirrors BKDWriter.computeCommonPrefixLength (Lucene 10.4.0).
func (w *SimpleTextBKDWriter) computeCommonPrefixLength(heap *bkd.HeapPointWriter, commonPrefix []byte, from, to int) {
	bpd := w.config.BytesPerDim()
	for i := range w.commonPfxLens {
		w.commonPfxLens[i] = bpd
	}
	first := heap.GetPackedValueSlice(from)
	fpv := first.PackedValue()
	for dim := 0; dim < w.config.NumDims(); dim++ {
		copy(commonPrefix[dim*bpd:], fpv.Bytes[fpv.Offset+dim*bpd:fpv.Offset+(dim+1)*bpd])
	}
	for i := from + 1; i < to; i++ {
		pv := heap.GetPackedValueSlice(i).PackedValue()
		for dim := 0; dim < w.config.NumDims(); dim++ {
			if w.commonPfxLens[dim] == 0 {
				continue
			}
			offset := dim * bpd
			j := 0
			for j < w.commonPfxLens[dim] {
				if commonPrefix[offset+j] != pv.Bytes[pv.Offset+offset+j] {
					w.commonPfxLens[dim] = j
					break
				}
				j++
			}
		}
	}
}

func (w *SimpleTextBKDWriter) checkMaxLeafNodeCount(numLeaves int) error {
	if int64(1+w.config.BytesPerDim())*int64(numLeaves) > int64(util.MaxArrayLength) {
		return fmt.Errorf(
			"too many leaf nodes (%d); increase maxPointsInLeafNode (currently %d) and reindex",
			numLeaves, w.config.MaxPointsInLeafNode(),
		)
	}
	return nil
}

// Close releases any temporary resources.
//
// Port of SimpleTextBKDWriter.close().
func (w *SimpleTextBKDWriter) Close() error {
	if w.tempInput != nil {
		name := w.tempInput.GetName()
		if err := w.tempInput.Close(); err != nil {
			_ = w.tempDir.DeleteFile(name)
			return err
		}
		if err := w.tempDir.DeleteFile(name); err != nil {
			return err
		}
		w.tempInput = nil
	}
	return nil
}

// ---------------------------------------------------------------------------
// OneDimensionBKDWriter (inner writer for the 1D optimised path).
// ---------------------------------------------------------------------------

type oneDimensionWriter struct {
	w                    *SimpleTextBKDWriter
	out                  store.IndexOutput
	leafBlockFPs         []int64
	leafBlockStartValues [][]byte
	leafValues           []byte
	leafDocs             []int
	valueCount           int64
	leafCount            int
	lastPackedValue      []byte
	lastDocID            int
}

func newOneDimensionWriter(w *SimpleTextBKDWriter, out store.IndexOutput) *oneDimensionWriter {
	return &oneDimensionWriter{
		w:               w,
		out:             out,
		leafValues:      make([]byte, w.config.MaxPointsInLeafNode()*w.config.PackedBytesLength()),
		leafDocs:        make([]int, w.config.MaxPointsInLeafNode()),
		lastPackedValue: make([]byte, w.config.PackedBytesLength()),
	}
}

func (ow *oneDimensionWriter) add(packedValue []byte, docID int) error {
	copy(ow.leafValues[ow.leafCount*ow.w.config.PackedBytesLength():], packedValue)
	ow.leafDocs[ow.leafCount] = docID
	ow.w.docsSeen.Set(docID)
	ow.leafCount++

	if ow.leafCount == ow.w.config.MaxPointsInLeafNode() {
		if err := ow.writeLeafBlock(); err != nil {
			return err
		}
		ow.leafCount = 0
	}
	return nil
}

func (ow *oneDimensionWriter) finish() (int64, error) {
	if ow.leafCount > 0 {
		if err := ow.writeLeafBlock(); err != nil {
			return 0, err
		}
		ow.leafCount = 0
	}
	if ow.valueCount == 0 {
		return -1, nil
	}
	ow.w.pointCount = ow.valueCount

	indexFP := ow.out.GetFilePointer()
	numInnerNodes := len(ow.leafBlockStartValues)
	index := make([]byte, (1+numInnerNodes)*(1+ow.w.config.BytesPerDim()))
	rotateToTree(1, 0, numInnerNodes, index, ow.leafBlockStartValues, ow.w.config.BytesPerDim())
	arr := make([]int64, len(ow.leafBlockFPs))
	copy(arr, ow.leafBlockFPs)
	if err := ow.w.writeIndex(ow.out, arr, index, ow.w.config.MaxPointsInLeafNode()); err != nil {
		return 0, err
	}
	return indexFP, nil
}

func (ow *oneDimensionWriter) writeLeafBlock() error {
	if ow.valueCount == 0 {
		copy(ow.w.minPackedValue, ow.leafValues[:ow.w.config.PackedIndexBytesLength()])
	}
	copy(ow.w.maxPackedValue,
		ow.leafValues[(ow.leafCount-1)*ow.w.config.PackedBytesLength():(ow.leafCount-1)*ow.w.config.PackedBytesLength()+ow.w.config.PackedIndexBytesLength()])
	ow.valueCount += int64(ow.leafCount)

	if len(ow.leafBlockFPs) > 0 {
		sv := make([]byte, ow.w.config.PackedBytesLength())
		copy(sv, ow.leafValues[:ow.w.config.PackedBytesLength()])
		ow.leafBlockStartValues = append(ow.leafBlockStartValues, sv)
	}
	ow.leafBlockFPs = append(ow.leafBlockFPs, ow.out.GetFilePointer())
	if err := ow.w.checkMaxLeafNodeCount(len(ow.leafBlockFPs)); err != nil {
		return err
	}

	// compute common prefix lengths
	bpd := ow.w.config.BytesPerDim()
	for i := range ow.w.commonPfxLens {
		ow.w.commonPfxLens[i] = bpd
	}
	for dim := 0; dim < ow.w.config.NumDims(); dim++ {
		off1 := dim * bpd
		off2 := (ow.leafCount-1)*ow.w.config.PackedBytesLength() + off1
		for j := 0; j < ow.w.commonPfxLens[dim]; j++ {
			if ow.leafValues[off1+j] != ow.leafValues[off2+j] {
				ow.w.commonPfxLens[dim] = j
				break
			}
		}
	}

	if err := ow.w.writeLeafBlockDocs(ow.out, ow.leafDocs, 0, ow.leafCount); err != nil {
		return err
	}
	pbl := ow.w.config.PackedBytesLength()
	pv := func(i int) []byte { return ow.leafValues[i*pbl : (i+1)*pbl] }
	return ow.w.writeLeafBlockPackedValues(ow.out, ow.w.commonPfxLens, ow.leafCount, 0, pv)
}

// oneDimVisitor wraps oneDimensionWriter as a bkd.IntersectVisitor.
type oneDimVisitor struct{ ow *oneDimensionWriter }

func (v *oneDimVisitor) Visit(_ int) error                             { return fmt.Errorf("unexpected Visit without packedValue") }
func (v *oneDimVisitor) VisitByPackedValue(docID int, pv []byte) error { return v.ow.add(pv, docID) }
func (v *oneDimVisitor) Compare(_, _ []byte) codecs.Relation           { return codecs.RelationCellCrossesQuery }
func (v *oneDimVisitor) Grow(_ int)                                    {}

// rotateToTree recursively fills the BKD index array from the sorted leaf
// split values, mirroring Java's rotateToTree.
func rotateToTree(nodeID, offset, count int, index []byte, leafBlockStartValues [][]byte, bpd int) {
	stride := 1 + bpd
	if count == 1 {
		copy(index[nodeID*stride+1:], leafBlockStartValues[offset][:bpd])
	} else if count > 1 {
		countAtLevel := 1
		totalCount := 0
		for {
			countLeft := count - totalCount
			if countLeft <= countAtLevel {
				lastLeftCount := countAtLevel / 2
				if lastLeftCount > countLeft {
					lastLeftCount = countLeft
				}
				leftHalf := (totalCount-1)/2 + lastLeftCount
				rootOffset := offset + leftHalf
				copy(index[nodeID*stride+1:], leafBlockStartValues[rootOffset][:bpd])
				rotateToTree(2*nodeID, offset, leftHalf, index, leafBlockStartValues, bpd)
				rotateToTree(2*nodeID+1, rootOffset+1, count-leftHalf-1, index, leafBlockStartValues, bpd)
				return
			}
			totalCount += countAtLevel
			countAtLevel *= 2
		}
	}
}

// bytesRefString mirrors Java's BytesRef.toString() which emits each byte as
// its lower-case hexadecimal value without zero-padding.
func bytesRefString(b []byte) string {
	if len(b) == 0 {
		return "[]"
	}
	buf := make([]byte, 0, 2+len(b)*3)
	buf = append(buf, '[')
	for i, v := range b {
		if i > 0 {
			buf = append(buf, ' ')
		}
		buf = strconv.AppendUint(buf, uint64(v), 16)
	}
	buf = append(buf, ']')
	return string(buf)
}
