// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import (
	"errors"
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// PointValuesWriter buffers up pending byte[][] value(s) per doc, then
// flushes when the segment flushes. It is the Go port of
// org.apache.lucene.index.PointValuesWriter from Apache Lucene 10.4.0.
//
// Lifecycle:
//
//  1. AddPackedValue is called once per (docID, packedValue) pair in
//     monotonically non-decreasing docID order (a doc may contribute more
//     than one point).
//  2. Flush hands a [PointTreeBuffer]-backed adapter to a
//     [BufferedPointsCodecWriter] for codec-level encoding.
//
// PointValuesWriter is not safe for concurrent use; callers are
// expected to serialise access via DocumentsWriterPerThread, mirroring
// Lucene's per-thread buffering contract.
//
// Lucene divergences:
//
//   - Lucene's PointsWriter.writeField receives an anonymous PointsReader
//     that overrides getValues(fieldName). Gocene's codecs.PointsWriter /
//     codecs.PointsReader pair cannot be referenced from the index
//     package without creating an import cycle (codecs imports index).
//     This file therefore declares the minimal codec-facing interfaces
//     ([BufferedPointsCodecWriter], [BufferedPointsCodecReader],
//     [BufferedPointValues], [PointTreeBuffer], [BufferedPointVisitor]
//     and [BufferedPointRelation]) inside the index package. They are
//     structurally compatible with the codec-side counterparts so any
//     concrete codec writer that satisfies codecs.PointsWriter also
//     satisfies BufferedPointsCodecWriter once the codec adapter wraps
//     it. A future Sprint can collapse the duplication once the codec
//     interface surface is reorganised; today it is the cheapest way to
//     keep parity with the Lucene reference without breaking the
//     package boundary.
//   - Lucene's PointValues exposes a wider surface than the in-RAM
//     buffer can answer. As in the Java reference, every accessor other
//     than the codec iteration path returns the "unsupported" sentinel
//     ([ErrPointValuesUnsupported]) or a zero value.
type PointValuesWriter struct {
	fieldInfo         *FieldInfo
	bytes             *util.PagedBytes
	bytesOut          *util.PagedBytesDataOutput
	iwBytesUsed       util.CounterAPI
	docIDs            []int
	numPoints         int
	numDocs           int
	lastDocID         int
	packedBytesLength int
}

// ErrPointValuesUnsupported is returned by accessors that the in-RAM
// PointValuesWriter buffer cannot satisfy. Mirrors the
// UnsupportedOperationException thrown by Lucene's anonymous in-RAM
// PointValues implementation.
var ErrPointValuesUnsupported = errors.New("point values: operation not supported on in-RAM buffer")

// BufferedPointRelation mirrors codecs.Relation by integer value so the
// in-RAM visitor callback contract can be expressed without importing
// the codecs package. Values match codecs.RelationCell* one-for-one.
type BufferedPointRelation int

const (
	// BufferedPointCellOutsideQuery matches codecs.RelationCellOutsideQuery.
	BufferedPointCellOutsideQuery BufferedPointRelation = iota
	// BufferedPointCellInsideQuery matches codecs.RelationCellInsideQuery.
	BufferedPointCellInsideQuery
	// BufferedPointCellCrossesQuery matches codecs.RelationCellCrossesQuery.
	BufferedPointCellCrossesQuery
)

// BufferedPointVisitor is the index-local counterpart of
// codecs.IntersectVisitor. Methods are wired in the order Lucene
// invokes them on the in-RAM flush path.
type BufferedPointVisitor interface {
	Visit(docID int) error
	VisitByPackedValue(docID int, packedValue []byte) error
	Compare(minPackedValue, maxPackedValue []byte) BufferedPointRelation
	Grow(count int)
}

// PointTreeBuffer is the index-local counterpart of
// codecs.MutablePointTree. Implementations may alias their underlying
// storage when filling BytesRef receivers.
type PointTreeBuffer interface {
	Swap(i, j int)
	GetValue(i int, dst *util.BytesRef)
	GetByteAt(i, k int) byte
	GetDocID(i int) int
	Save(i, j int)
	Restore(i, j int)
}

// BufferedPointValues is the index-local counterpart of
// codecs.PointValues. Only [BufferedPointValues.Intersect] and
// [BufferedPointValues.EstimatePointCount] are meaningful for in-RAM
// buffered points; the remaining accessors are deliberately
// "unsupported", matching the Lucene reference.
type BufferedPointValues interface {
	Intersect(visitor BufferedPointVisitor) error
	EstimatePointCount(visitor BufferedPointVisitor) int64
	GetMinPackedValue() []byte
	GetMaxPackedValue() []byte
	GetNumDimensions() int
	GetBytesPerDimension() int
	GetDocCount() int
	// Tree returns the underlying mutable tree so codec layers that
	// understand the in-RAM contract can drive BKD partitioning
	// directly. This entry-point mirrors Lucene's PointTree but stays
	// inside the index package.
	Tree() PointTreeBuffer
}

// BufferedPointsCodecReader is the index-local counterpart of
// codecs.PointsReader for the flush path. The adapter returned by
// [PointValuesWriter.Flush] implements this interface so the codec
// layer can resolve the buffered values by field name.
type BufferedPointsCodecReader interface {
	GetValues(fieldName string) (BufferedPointValues, error)
	CheckIntegrity() error
	Close() error
}

// BufferedPointsCodecWriter is the index-local counterpart of
// codecs.PointsWriter for the flush path. Concrete codec writers can
// satisfy it by accepting the in-RAM adapter; the codec adapter layer
// is responsible for re-wrapping the values into whatever
// codecs.PointValues shape downstream readers expect.
type BufferedPointsCodecWriter interface {
	WriteField(fieldInfo *FieldInfo, reader BufferedPointsCodecReader) error
}

// intBytes is the Go counterpart of java.lang.Integer.BYTES used by the
// Lucene reference for RAM accounting. Pinned at 4 bytes because the
// docIDs buffer is logically int32-wide on the wire even though it is
// stored as Go int in memory.
const intBytes = 4

// NewPointsWriter mirrors the constructor of Lucene's
// org.apache.lucene.index.PointValuesWriter. bytesUsed receives the live
// RAM accounting deltas; fieldInfo describes the dimensions and bytes
// per dimension of the points that will be added.
func NewPointsWriter(bytesUsed util.CounterAPI, fieldInfo *FieldInfo) (*PointValuesWriter, error) {
	if bytesUsed == nil {
		return nil, errors.New("point values writer: bytesUsed must not be nil")
	}
	if fieldInfo == nil {
		return nil, errors.New("point values writer: fieldInfo must not be nil")
	}
	bytes, err := util.NewPagedBytes(12)
	if err != nil {
		return nil, fmt.Errorf("point values writer: alloc paged bytes: %w", err)
	}
	w := &PointValuesWriter{
		fieldInfo:         fieldInfo,
		bytes:             bytes,
		bytesOut:          bytes.GetDataOutput(),
		iwBytesUsed:       bytesUsed,
		docIDs:            make([]int, 16),
		lastDocID:         -1,
		packedBytesLength: fieldInfo.PointDimensionCount() * fieldInfo.PointNumBytes(),
	}
	// 16 ints worth of accounting, matching the Java constructor's
	// iwBytesUsed.addAndGet(16 * Integer.BYTES).
	bytesUsed.AddAndGet(int64(16) * int64(intBytes))
	return w, nil
}

// AddPackedValue records (docID, value). value must have length equal to
// PointDimensionCount * PointNumBytes (matching the field's packed
// layout). Callers must add points in non-decreasing docID order.
//
// TODO (Lucene parity): if exactly the same value is added to exactly
// the same doc, should we dedup?
func (w *PointValuesWriter) AddPackedValue(docID int, value *util.BytesRef) error {
	if value == nil {
		return fmt.Errorf("field=%s: point value must not be null", w.fieldInfo.Name())
	}
	if value.Length != w.packedBytesLength {
		return fmt.Errorf(
			"field=%s: this field's value has length=%d but should be %d",
			w.fieldInfo.Name(), value.Length, w.packedBytesLength,
		)
	}

	if len(w.docIDs) == w.numPoints {
		oldLen := len(w.docIDs)
		newLen := util.Oversize(w.numPoints+1, intBytes)
		grown := make([]int, newLen)
		copy(grown, w.docIDs)
		w.docIDs = grown
		w.iwBytesUsed.AddAndGet(int64(len(w.docIDs)-oldLen) * int64(intBytes))
	}

	before := w.bytes.RamBytesUsed()
	if err := w.bytesOut.WriteBytes(value.Bytes[value.Offset : value.Offset+value.Length]); err != nil {
		return fmt.Errorf("field=%s: write packed value: %w", w.fieldInfo.Name(), err)
	}
	w.iwBytesUsed.AddAndGet(w.bytes.RamBytesUsed() - before)

	w.docIDs[w.numPoints] = docID
	if docID != w.lastDocID {
		w.numDocs++
		w.lastDocID = docID
	}
	w.numPoints++
	return nil
}

// NumDocs returns the number of buffered documents. Mirrors
// PointValuesWriter#getNumDocs.
func (w *PointValuesWriter) NumDocs() int {
	return w.numDocs
}

// NumPoints reports the number of buffered (doc, value) pairs. Exposed
// for codec-level diagnostics and tests; the Java reference keeps the
// equivalent field package-private.
func (w *PointValuesWriter) NumPoints() int {
	return w.numPoints
}

// PackedBytesLength returns the byte length of a single packed value as
// derived from the field's dimensions. Exposed for codec/test code.
func (w *PointValuesWriter) PackedBytesLength() int {
	return w.packedBytesLength
}

// Flush freezes the buffered values and hands them to writer via a
// [BufferedPointsCodecReader] adapter. When sortMap is non-nil the
// points are wrapped in a [MutableSortingPointValues] view that
// translates docIDs from old- to new-order before they reach the codec.
func (w *PointValuesWriter) Flush(_ *SegmentWriteState, sortMap SorterDocMap, writer BufferedPointsCodecWriter) error {
	if writer == nil {
		return errors.New("point values writer: codec writer must not be nil")
	}
	reader, err := w.bytes.Freeze(false)
	if err != nil {
		return fmt.Errorf("point values writer: freeze paged bytes: %w", err)
	}

	points := newBufferedMutablePointTree(reader, w.docIDs, w.numPoints, w.packedBytesLength)
	var values PointTreeBuffer = points
	if sortMap != nil {
		values = &MutableSortingPointValues{in: points, docMap: sortMap}
	}

	adapter := &bufferedPointsReader{
		fieldName: w.fieldInfo.Name(),
		values: &bufferedPointValues{
			tree:              values,
			packedBytesLength: w.packedBytesLength,
		},
	}
	return writer.WriteField(w.fieldInfo, adapter)
}

// bufferedMutablePointTree is the Go counterpart of the anonymous
// MutablePointTree subclass that PointValuesWriter#flush constructs in
// the Lucene reference. It indirects every access through an ords[]
// permutation so the StableMSBRadixSorter inside the BKD writer can
// reorder slots without touching the underlying byte storage.
type bufferedMutablePointTree struct {
	bytesReader       *util.Reader
	docIDs            []int
	numPoints         int
	packedBytesLength int
	ords              []int
	temp              []int
}

func newBufferedMutablePointTree(bytesReader *util.Reader, docIDs []int, numPoints, packedBytesLength int) *bufferedMutablePointTree {
	ords := make([]int, numPoints)
	for i := 0; i < numPoints; i++ {
		ords[i] = i
	}
	return &bufferedMutablePointTree{
		bytesReader:       bytesReader,
		docIDs:            docIDs,
		numPoints:         numPoints,
		packedBytesLength: packedBytesLength,
		ords:              ords,
	}
}

// Size returns the number of buffered points (Java: long size()).
func (b *bufferedMutablePointTree) Size() int64 {
	return int64(b.numPoints)
}

// Swap exchanges slots i and j in the permutation, matching
// MutablePointTree#swap(int, int).
func (b *bufferedMutablePointTree) Swap(i, j int) {
	b.ords[i], b.ords[j] = b.ords[j], b.ords[i]
}

// GetDocID returns the docID at logical slot i (post-permutation).
func (b *bufferedMutablePointTree) GetDocID(i int) int {
	return b.docIDs[b.ords[i]]
}

// GetValue fills dst with the packed value at logical slot i. The slice
// may alias the underlying paged bytes storage when the value lies
// within a single page.
func (b *bufferedMutablePointTree) GetValue(i int, dst *util.BytesRef) {
	offset := int64(b.packedBytesLength) * int64(b.ords[i])
	// FillSlice is the only error path; on a frozen reader with a
	// well-formed offset it cannot fail, so the error is intentionally
	// dropped here. Note: ignoring the error keeps the Lucene contract,
	// which has no checked exception on this code path.
	_ = b.bytesReader.FillSlice(dst, offset, b.packedBytesLength)
}

// GetByteAt returns the k-th byte of the packed value at logical slot i.
func (b *bufferedMutablePointTree) GetByteAt(i, k int) byte {
	offset := int64(b.packedBytesLength)*int64(b.ords[i]) + int64(k)
	return b.bytesReader.GetByte(offset)
}

// Save mirrors MutablePointTree#save: copies the ord at slot i into the
// j-th scratch position. The scratch buffer is lazily allocated to
// match Lucene's "only when needed" behaviour.
func (b *bufferedMutablePointTree) Save(i, j int) {
	if b.temp == nil {
		b.temp = make([]int, len(b.ords))
	}
	b.temp[j] = b.ords[i]
}

// Restore mirrors MutablePointTree#restore: copies scratch[i:j) back
// into the live ords permutation. No-op when Save has never been
// invoked.
func (b *bufferedMutablePointTree) Restore(i, j int) {
	if b.temp == nil {
		return
	}
	copy(b.ords[i:j], b.temp[i:j])
}

// MutableSortingPointValues wraps a [PointTreeBuffer] so that every
// returned docID is remapped through a [SorterDocMap]. Mirrors
// PointValuesWriter.MutableSortingPointValues in Apache Lucene 10.4.0.
type MutableSortingPointValues struct {
	in     PointTreeBuffer
	docMap SorterDocMap
}

// NewMutableSortingPointValues wraps in with the supplied docMap.
func NewMutableSortingPointValues(in PointTreeBuffer, docMap SorterDocMap) *MutableSortingPointValues {
	return &MutableSortingPointValues{in: in, docMap: docMap}
}

// GetValue delegates straight to the wrapped tree.
func (m *MutableSortingPointValues) GetValue(i int, dst *util.BytesRef) {
	m.in.GetValue(i, dst)
}

// GetByteAt delegates straight to the wrapped tree.
func (m *MutableSortingPointValues) GetByteAt(i, k int) byte {
	return m.in.GetByteAt(i, k)
}

// GetDocID returns the *new* docID corresponding to the wrapped tree's
// slot i.
func (m *MutableSortingPointValues) GetDocID(i int) int {
	return m.docMap.OldToNew(m.in.GetDocID(i))
}

// Swap delegates straight to the wrapped tree.
func (m *MutableSortingPointValues) Swap(i, j int) {
	m.in.Swap(i, j)
}

// Save delegates straight to the wrapped tree.
func (m *MutableSortingPointValues) Save(i, j int) {
	m.in.Save(i, j)
}

// Restore delegates straight to the wrapped tree.
func (m *MutableSortingPointValues) Restore(i, j int) {
	m.in.Restore(i, j)
}

// bufferedPointsReader is the codec-side adapter that hands the
// buffered points to a [BufferedPointsCodecWriter].
type bufferedPointsReader struct {
	fieldName string
	values    *bufferedPointValues
}

// GetValues returns the buffered point values when fieldName matches.
func (r *bufferedPointsReader) GetValues(fieldName string) (BufferedPointValues, error) {
	if fieldName != r.fieldName {
		return nil, fmt.Errorf("fieldName must be %q, got %q", r.fieldName, fieldName)
	}
	return r.values, nil
}

// CheckIntegrity is never expected to be called on the buffered reader;
// the BKD writer only consumes the values returned by GetValues. We
// surface [ErrPointValuesUnsupported] to mirror the Lucene reference,
// which throws UnsupportedOperationException here.
func (r *bufferedPointsReader) CheckIntegrity() error {
	return ErrPointValuesUnsupported
}

// Close is a no-op: the buffer is owned by the PointValuesWriter and
// reclaimed when the writer is discarded.
func (r *bufferedPointsReader) Close() error {
	return nil
}

// MutablePointTreeSource is implemented by buffered point readers that can
// expose their in-memory [PointTreeBuffer] directly. Codecs use this to drive
// BKDWriter.WriteField (the heap path) instead of Add/Finish (the offline
// spill path), matching Apache Lucene 10.4.0's buffered-points behaviour.
type MutablePointTreeSource interface {
	// MutablePointTree returns the in-memory tree and its point count.
	MutablePointTree() (PointTreeBuffer, int)
}

// MutablePointTree exposes the in-memory buffered points as a [PointTreeBuffer]
// so that codecs can drive BKDWriter.WriteField directly in RAM.
func (r *bufferedPointsReader) MutablePointTree() (PointTreeBuffer, int) {
	return r.values.Tree(), sizeOfTree(r.values.Tree())
}

// bufferedPointValues is the codec-facing [BufferedPointValues] view of
// the buffered points. Only the iteration path (Intersect /
// EstimatePointCount) is meaningful; the statistics accessors are
// "unsupported" in line with Lucene's anonymous PointValues subclass.
type bufferedPointValues struct {
	tree              PointTreeBuffer
	packedBytesLength int
}

// Tree exposes the underlying [PointTreeBuffer] so codec layers that
// understand the in-RAM contract can drive the BKD partitioning
// directly. This is a Gocene-specific affordance: Lucene relies on
// PointValues#getPointTree() which Gocene's codec interface does not
// yet declare. Documented as a deviation in the file header.
func (p *bufferedPointValues) Tree() PointTreeBuffer {
	return p.tree
}

// Intersect walks every buffered point and feeds (docID, packedValue)
// to visitor. The packed value buffer is reused across calls, matching
// the no-alloc contract Lucene's BKD writer expects.
func (p *bufferedPointValues) Intersect(visitor BufferedPointVisitor) error {
	if visitor == nil {
		return errors.New("buffered point values: visitor must not be nil")
	}
	scratch := util.NewBytesRefEmpty()
	packed := make([]byte, p.packedBytesLength)
	size := sizeOfTree(p.tree)
	for i := 0; i < size; i++ {
		p.tree.GetValue(i, scratch)
		if scratch.Length != p.packedBytesLength {
			return fmt.Errorf("buffered point values: scratch length %d != packed %d", scratch.Length, p.packedBytesLength)
		}
		copy(packed, scratch.Bytes[scratch.Offset:scratch.Offset+p.packedBytesLength])
		if err := visitor.VisitByPackedValue(p.tree.GetDocID(i), packed); err != nil {
			return err
		}
	}
	return nil
}

// EstimatePointCount returns the exact buffered count; estimation is
// trivial in RAM. Mirrors the "unsupported on the anonymous accessor,
// but trivially derivable from the tree" behaviour of the reference.
func (p *bufferedPointValues) EstimatePointCount(_ BufferedPointVisitor) int64 {
	return int64(sizeOfTree(p.tree))
}

// sizeOfTree extracts the buffered point count from either the
// bufferedMutablePointTree or its sorted wrapper without leaking those
// concrete types into the BufferedPointValues interface.
func sizeOfTree(t PointTreeBuffer) int {
	switch v := t.(type) {
	case *bufferedMutablePointTree:
		return v.numPoints
	case *MutableSortingPointValues:
		return sizeOfTree(v.in)
	default:
		return 0
	}
}

// GetMinPackedValue mirrors the unsupported accessor on Lucene's
// anonymous PointValues.
func (p *bufferedPointValues) GetMinPackedValue() []byte { return nil }

// GetMaxPackedValue mirrors the unsupported accessor on Lucene's
// anonymous PointValues.
func (p *bufferedPointValues) GetMaxPackedValue() []byte { return nil }

// GetNumDimensions mirrors the unsupported accessor on Lucene's
// anonymous PointValues.
func (p *bufferedPointValues) GetNumDimensions() int { return 0 }

// GetBytesPerDimension mirrors the unsupported accessor on Lucene's
// anonymous PointValues.
func (p *bufferedPointValues) GetBytesPerDimension() int { return 0 }

// GetDocCount mirrors the unsupported accessor on Lucene's anonymous
// PointValues.
func (p *bufferedPointValues) GetDocCount() int { return 0 }
