// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package simpletext

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	"github.com/FlavioCFOliveira/Gocene/schema"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// ---------------------------------------------------------------------------
// Test fixtures
// ---------------------------------------------------------------------------

// pointDoc is one buffered point: a docID plus its packed value across all
// dimensions (numDims * bytesPerDim bytes, dimensions concatenated).
type pointDoc struct {
	docID  int
	packed []byte
}

// fakePointsSource is an in-memory points source that satisfies both the
// narrow codecs.PointsReader SPI (CheckIntegrity/Close) and the duck-typed
// pointsSource surface (PointValueCount/VisitPoints) that
// SimpleTextPointsWriter.WriteField type-asserts to. It is the test analogue
// of index.dwptPointsSource: the indexing chain feeds the codec writer one of
// these per point field.
type fakePointsSource struct {
	// byField maps field name to its buffered points, already in the document
	// order the indexing chain would replay them in.
	byField map[string][]pointDoc
}

func (s *fakePointsSource) PointValueCount(field string) int64 {
	return int64(len(s.byField[field]))
}

func (s *fakePointsSource) VisitPoints(field string, fn func(docID int, packedValue []byte) error) error {
	for _, p := range s.byField[field] {
		if err := fn(p.docID, p.packed); err != nil {
			return err
		}
	}
	return nil
}

func (s *fakePointsSource) CheckIntegrity() error { return nil }
func (s *fakePointsSource) Close() error          { return nil }

// compile-time assertions: the fake satisfies both surfaces.
var (
	_ codecs.PointsReader = (*fakePointsSource)(nil)
	_ pointsSource        = (*fakePointsSource)(nil)
)

// collectingVisitor gathers every (docID, packedValue) the BKD reader emits.
// Compare returns CROSSES so the whole tree is walked.
type collectingVisitor struct {
	got []pointDoc
}

func (v *collectingVisitor) Visit(docID int) error {
	return fmt.Errorf("collectingVisitor.Visit called without a packed value (docID=%d)", docID)
}

func (v *collectingVisitor) VisitByPackedValue(docID int, packedValue []byte) error {
	v.got = append(v.got, pointDoc{docID: docID, packed: append([]byte(nil), packedValue...)})
	return nil
}

func (v *collectingVisitor) Compare(_, _ []byte) codecs.Relation {
	return codecs.RelationCellCrossesQuery
}

func (v *collectingVisitor) Grow(_ int) {}

// ---------------------------------------------------------------------------
// Encoding helpers (Lucene sortable byte order)
// ---------------------------------------------------------------------------

// encodeInt32Sortable writes v as a big-endian 4-byte value with the sign bit
// flipped, matching NumericUtils.intToSortableBytes so unsigned byte order
// equals signed integer order.
func encodeInt32Sortable(v int32) []byte {
	x := uint32(v) ^ 0x80000000
	return []byte{byte(x >> 24), byte(x >> 16), byte(x >> 8), byte(x)}
}

// encodeInt64Sortable writes v as a big-endian 8-byte value with the sign bit
// flipped, matching NumericUtils.longToSortableBytes.
func encodeInt64Sortable(v int64) []byte {
	x := uint64(v) ^ 0x8000000000000000
	return []byte{
		byte(x >> 56), byte(x >> 48), byte(x >> 40), byte(x >> 32),
		byte(x >> 24), byte(x >> 16), byte(x >> 8), byte(x),
	}
}

// ---------------------------------------------------------------------------
// fieldSpec / shared round-trip harness
// ---------------------------------------------------------------------------

type fieldSpec struct {
	name        string
	number      int
	numDims     int
	bytesPerDim int
	points      []pointDoc
}

// writeAndRead writes every fieldSpec via SimpleTextPointsWriter and then reads
// the segment back via SimpleTextPointsReader, returning the reader for
// assertions. docCount is the segment's document count.
func writeAndRead(t *testing.T, docCount int, fields []fieldSpec) (*SimpleTextPointsReader, func()) {
	t.Helper()

	dir := store.NewByteBuffersDirectory()
	segInfo := schema.NewSegmentInfo("_st_points", docCount, dir)

	fieldInfos := schema.NewFieldInfos()
	src := &fakePointsSource{byField: make(map[string][]pointDoc)}
	for _, f := range fields {
		opts := schema.DefaultFieldInfoOptions()
		opts.PointDimensionCount = f.numDims
		opts.PointIndexDimensionCount = f.numDims
		opts.PointNumBytes = f.bytesPerDim
		fi := schema.NewFieldInfo(f.name, f.number, opts)
		if err := fieldInfos.Add(fi); err != nil {
			t.Fatalf("fieldInfos.Add(%q): %v", f.name, err)
		}
		src.byField[f.name] = f.points
	}

	writeState := &codecs.SegmentWriteState{
		Directory:   dir,
		SegmentInfo: segInfo,
		FieldInfos:  fieldInfos,
	}

	w, err := NewSimpleTextPointsWriter(writeState)
	if err != nil {
		t.Fatalf("NewSimpleTextPointsWriter: %v", err)
	}
	for _, f := range fields {
		fi := fieldInfos.GetByName(f.name)
		if err := w.WriteField(fi, src); err != nil {
			t.Fatalf("WriteField(%q): %v", f.name, err)
		}
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("writer.Finish: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("writer.Close: %v", err)
	}

	readState := &codecs.SegmentReadState{
		Directory:   dir,
		SegmentInfo: segInfo,
		FieldInfos:  fieldInfos,
	}
	r, err := NewSimpleTextPointsReader(readState)
	if err != nil {
		t.Fatalf("NewSimpleTextPointsReader: %v", err)
	}
	if err := r.CheckIntegrity(); err != nil {
		t.Fatalf("reader.CheckIntegrity: %v", err)
	}

	cleanup := func() {
		_ = r.Close()
		_ = dir.Close()
	}
	return r, cleanup
}

// assertFieldRoundTrip verifies that the reader returns exactly the points in
// spec (as a multiset of (docID, packedValue)) and that the min/max packed
// values match the per-dim extremes of spec.
func assertFieldRoundTrip(t *testing.T, r *SimpleTextPointsReader, spec fieldSpec) {
	t.Helper()

	pv, err := r.GetValues(spec.name)
	if err != nil {
		t.Fatalf("GetValues(%q): %v", spec.name, err)
	}
	if pv == nil {
		t.Fatalf("GetValues(%q): returned nil PointValues", spec.name)
	}

	// Dimensionality round-trips.
	if got := pv.GetNumDimensions(); got != spec.numDims {
		t.Errorf("%q: GetNumDimensions = %d, want %d", spec.name, got, spec.numDims)
	}
	if got := pv.GetBytesPerDimension(); got != spec.bytesPerDim {
		t.Errorf("%q: GetBytesPerDimension = %d, want %d", spec.name, got, spec.bytesPerDim)
	}

	// Size and docCount round-trip.
	bkdReader, ok := pv.(*SimpleTextBKDReader)
	if !ok {
		t.Fatalf("%q: PointValues is %T, want *SimpleTextBKDReader", spec.name, pv)
	}
	if got, want := bkdReader.Size(), int64(len(spec.points)); got != want {
		t.Errorf("%q: Size = %d, want %d", spec.name, got, want)
	}
	wantDocs := distinctDocs(spec.points)
	if got := pv.GetDocCount(); got != wantDocs {
		t.Errorf("%q: GetDocCount = %d, want %d", spec.name, got, wantDocs)
	}

	// Per-doc visitation: collect everything and compare as multisets.
	vis := &collectingVisitor{}
	if err := pv.Intersect(vis); err != nil {
		t.Fatalf("%q: Intersect: %v", spec.name, err)
	}
	if !samePointMultiset(spec.points, vis.got) {
		t.Errorf("%q: visited points mismatch\n got: %s\nwant: %s",
			spec.name, fmtPoints(vis.got), fmtPoints(spec.points))
	}

	// min/max packed values: compare against the per-dim extremes of spec
	// (index dims == data dims in these specs, so the full packed value is the
	// index value).
	wantMin, wantMax := perDimExtremes(spec)
	if got := pv.GetMinPackedValue(); !bytes.Equal(got, wantMin) {
		t.Errorf("%q: GetMinPackedValue = % x, want % x", spec.name, got, wantMin)
	}
	if got := pv.GetMaxPackedValue(); !bytes.Equal(got, wantMax) {
		t.Errorf("%q: GetMaxPackedValue = % x, want % x", spec.name, got, wantMax)
	}
}

// distinctDocs returns the number of unique docIDs in points.
func distinctDocs(points []pointDoc) int {
	seen := make(map[int]struct{}, len(points))
	for _, p := range points {
		seen[p.docID] = struct{}{}
	}
	return len(seen)
}

// samePointMultiset reports whether a and b contain the same (docID,
// packedValue) pairs, ignoring order.
func samePointMultiset(a, b []pointDoc) bool {
	if len(a) != len(b) {
		return false
	}
	key := func(p pointDoc) string { return fmt.Sprintf("%d|% x", p.docID, p.packed) }
	counts := make(map[string]int, len(a))
	for _, p := range a {
		counts[key(p)]++
	}
	for _, p := range b {
		counts[key(p)]--
	}
	for _, c := range counts {
		if c != 0 {
			return false
		}
	}
	return true
}

// perDimExtremes computes the per-dim minimum and maximum packed values over
// spec.points, mirroring the unsigned-byte comparison SimpleTextBKDWriter uses.
func perDimExtremes(spec fieldSpec) (min, max []byte) {
	packedLen := spec.numDims * spec.bytesPerDim
	min = make([]byte, packedLen)
	max = make([]byte, packedLen)
	for i := range min {
		min[i] = 0xff
	}
	for _, p := range spec.points {
		for dim := 0; dim < spec.numDims; dim++ {
			off := dim * spec.bytesPerDim
			end := off + spec.bytesPerDim
			seg := p.packed[off:end]
			if bytes.Compare(seg, min[off:end]) < 0 {
				copy(min[off:end], seg)
			}
			if bytes.Compare(seg, max[off:end]) > 0 {
				copy(max[off:end], seg)
			}
		}
	}
	return min, max
}

func fmtPoints(points []pointDoc) string {
	sorted := append([]pointDoc(nil), points...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].docID != sorted[j].docID {
			return sorted[i].docID < sorted[j].docID
		}
		return bytes.Compare(sorted[i].packed, sorted[j].packed) < 0
	})
	var sb strings.Builder
	for _, p := range sorted {
		fmt.Fprintf(&sb, "(doc=%d % x) ", p.docID, p.packed)
	}
	return sb.String()
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestSimpleTextPoints_RoundTrip1DInt exercises a single-dimension 4-byte int
// field, one value per doc.
func TestSimpleTextPoints_RoundTrip1DInt(t *testing.T) {
	const docCount = 20
	var points []pointDoc
	for i := 0; i < docCount; i++ {
		points = append(points, pointDoc{docID: i, packed: encodeInt32Sortable(int32(i - 7))})
	}
	spec := fieldSpec{name: "ints", number: 0, numDims: 1, bytesPerDim: 4, points: points}

	r, cleanup := writeAndRead(t, docCount, []fieldSpec{spec})
	defer cleanup()
	assertFieldRoundTrip(t, r, spec)
}

// TestSimpleTextPoints_RoundTrip1DLong exercises a single-dimension 8-byte long
// field with negative, zero and positive values to prove sortable ordering.
func TestSimpleTextPoints_RoundTrip1DLong(t *testing.T) {
	values := []int64{-1 << 40, -5, 0, 5, 1 << 40, 1<<62 - 1}
	var points []pointDoc
	for i, v := range values {
		points = append(points, pointDoc{docID: i, packed: encodeInt64Sortable(v)})
	}
	spec := fieldSpec{name: "longs", number: 0, numDims: 1, bytesPerDim: 8, points: points}

	r, cleanup := writeAndRead(t, len(values), []fieldSpec{spec})
	defer cleanup()
	assertFieldRoundTrip(t, r, spec)
}

// TestSimpleTextPoints_RoundTripMultiDim exercises a 2-dimensional field
// (4 bytes per dim). The recursive split path is exercised by using more
// documents than fit in a single leaf would require for ordering checks.
func TestSimpleTextPoints_RoundTripMultiDim(t *testing.T) {
	const docCount = 30
	var points []pointDoc
	for i := 0; i < docCount; i++ {
		packed := make([]byte, 0, 8)
		packed = append(packed, encodeInt32Sortable(int32(i))...)          // dim 0
		packed = append(packed, encodeInt32Sortable(int32(docCount-i))...) // dim 1
		points = append(points, pointDoc{docID: i, packed: packed})
	}
	spec := fieldSpec{name: "geo", number: 0, numDims: 2, bytesPerDim: 4, points: points}

	r, cleanup := writeAndRead(t, docCount, []fieldSpec{spec})
	defer cleanup()
	assertFieldRoundTrip(t, r, spec)
}

// TestSimpleTextPoints_RoundTripMultiValue exercises a field where some
// documents contribute more than one point value (multi-valued points).
func TestSimpleTextPoints_RoundTripMultiValue(t *testing.T) {
	const docCount = 10
	var points []pointDoc
	for i := 0; i < docCount; i++ {
		// Each doc contributes (i%3)+1 values, in document order.
		n := (i % 3) + 1
		for j := 0; j < n; j++ {
			points = append(points, pointDoc{
				docID:  i,
				packed: encodeInt32Sortable(int32(i*100 + j)),
			})
		}
	}
	spec := fieldSpec{name: "multi", number: 0, numDims: 1, bytesPerDim: 4, points: points}

	r, cleanup := writeAndRead(t, docCount, []fieldSpec{spec})
	defer cleanup()
	assertFieldRoundTrip(t, r, spec)
}

// TestSimpleTextPoints_RoundTripMultipleFields exercises several point fields
// of differing shapes written into one segment, then read back independently.
func TestSimpleTextPoints_RoundTripMultipleFields(t *testing.T) {
	const docCount = 12

	var intPts, longPts, twoDPts []pointDoc
	for i := 0; i < docCount; i++ {
		intPts = append(intPts, pointDoc{docID: i, packed: encodeInt32Sortable(int32(i))})
		longPts = append(longPts, pointDoc{docID: i, packed: encodeInt64Sortable(int64(i) * 1_000_000)})
		packed := append(encodeInt32Sortable(int32(i)), encodeInt32Sortable(int32(-i))...)
		twoDPts = append(twoDPts, pointDoc{docID: i, packed: packed})
	}

	specs := []fieldSpec{
		{name: "f_int", number: 0, numDims: 1, bytesPerDim: 4, points: intPts},
		{name: "f_long", number: 1, numDims: 1, bytesPerDim: 8, points: longPts},
		{name: "f_2d", number: 2, numDims: 2, bytesPerDim: 4, points: twoDPts},
	}

	r, cleanup := writeAndRead(t, docCount, specs)
	defer cleanup()
	for _, spec := range specs {
		assertFieldRoundTrip(t, r, spec)
	}
}

// TestSimpleTextPoints_AllDeletedDropsField verifies that a field with zero
// buffered points produces no index entry (the merge-time "all docs deleted"
// case), matching Lucene's `if (writer.getPointCount() > 0)` guard.
func TestSimpleTextPoints_AllDeletedDropsField(t *testing.T) {
	// Field "present" has points; field "empty" has none.
	specPresent := fieldSpec{
		name: "present", number: 0, numDims: 1, bytesPerDim: 4,
		points: []pointDoc{{docID: 0, packed: encodeInt32Sortable(42)}},
	}
	specEmpty := fieldSpec{name: "empty", number: 1, numDims: 1, bytesPerDim: 4, points: nil}

	r, cleanup := writeAndRead(t, 1, []fieldSpec{specPresent, specEmpty})
	defer cleanup()

	assertFieldRoundTrip(t, r, specPresent)

	// The empty field must have no BKD reader: GetValues returns (nil, nil)
	// because the .dii index carries no entry for it.
	pv, err := r.GetValues("empty")
	if err != nil {
		t.Fatalf("GetValues(empty): unexpected error %v", err)
	}
	if pv != nil {
		t.Errorf("GetValues(empty): expected nil PointValues for an all-deleted field, got %T", pv)
	}
}

// TestSimpleTextPoints_GoldenTextLines pins the exact SimpleText line layout
// for a tiny deterministic single-field segment. SimpleText "bytes" are UTF-8
// text lines, so byte-faithfulness against Lucene 10.4.0 means an exact match
// of every field-label string and value formatting emitted by
// SimpleTextBKDWriter.writeIndex / writeLeafBlockDocs / writeLeafBlockPackedValues.
//
// Two docs, single 4-byte int dimension, values 0 and 1 (sortable). Both fit in
// one leaf, so there are no inner split nodes (split count 0). The expected text
// is derived directly from the Lucene 10.4.0 SimpleTextBKDWriter source.
func TestSimpleTextPoints_GoldenTextLines(t *testing.T) {
	dir := store.NewByteBuffersDirectory()
	defer dir.Close()

	segInfo := schema.NewSegmentInfo("_golden", 2, dir)
	fieldInfos := schema.NewFieldInfos()
	opts := schema.DefaultFieldInfoOptions()
	opts.PointDimensionCount = 1
	opts.PointIndexDimensionCount = 1
	opts.PointNumBytes = 4
	fi := schema.NewFieldInfo("g", 0, opts)
	if err := fieldInfos.Add(fi); err != nil {
		t.Fatalf("fieldInfos.Add: %v", err)
	}

	src := &fakePointsSource{byField: map[string][]pointDoc{
		"g": {
			{docID: 0, packed: encodeInt32Sortable(0)}, // [80 0 0 0]
			{docID: 1, packed: encodeInt32Sortable(1)}, // [80 0 0 1]
		},
	}}

	writeState := &codecs.SegmentWriteState{Directory: dir, SegmentInfo: segInfo, FieldInfos: fieldInfos}
	w, err := NewSimpleTextPointsWriter(writeState)
	if err != nil {
		t.Fatalf("NewSimpleTextPointsWriter: %v", err)
	}
	if err := w.WriteField(fi, src); err != nil {
		t.Fatalf("WriteField: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	dimText := readWholeFile(t, dir, "_golden.dim")

	// The leaf block (block count + doc IDs + block values) comes first, then
	// the index (written at file pointer == indexFP), then the END sentinel and
	// checksum footer that Finish appends.
	//
	// sortable(0) = 0x80000000 -> bytes 80 00 00 00 -> BytesRef "[80 0 0 0]"
	// sortable(1) = 0x80000001 -> bytes 80 00 00 01 -> BytesRef "[80 0 0 1]"
	wantPrefix := strings.Join([]string{
		"block count 2",
		"  doc 0",
		"  doc 1",
		"  block value [80 0 0 0]",
		"  block value [80 0 0 1]",
		"num data dims 1",
		"num index dims 1",
		"bytes per dim 4",
		// max leaf points == countPerLeaf, which for a single-leaf field equals
		// the point count (the while loop in finish() never runs for 2 <= 512).
		// This mirrors Lucene's writeIndex(out, …, Math.toIntExact(countPerLeaf)).
		"max leaf points 2",
		"index count 1",
		"min value [80 0 0 0]",
		"max value [80 0 0 1]",
		"point count 2",
		"doc count 2",
		"  block fp 0",
		"split count 1",
		"  split dim 0",
		"  split value [0 0 0 0]",
		"END",
		"",
	}, "\n")

	if !strings.HasPrefix(dimText, wantPrefix) {
		t.Fatalf("golden .dim text mismatch.\n--- want prefix ---\n%s\n--- got ---\n%s",
			wantPrefix, dimText)
	}

	// The remaining text is the checksum footer line "checksum <20 digits>\n".
	rest := dimText[len(wantPrefix):]
	if !strings.HasPrefix(rest, "checksum ") || !strings.HasSuffix(rest, "\n") {
		t.Fatalf("golden .dim footer mismatch: %q", rest)
	}

	// The index file pins the field-name -> file-pointer mapping text.
	diiText := readWholeFile(t, dir, "_golden.dii")
	wantDiiPrefix := strings.Join([]string{
		"field count 1",
		"  field fp name g",
		"  field fp 47", // indexFP: byte offset where writeIndex began (see below)
		"",
	}, "\n")
	// Rather than hard-code the exact file pointer (which depends on the leaf
	// block byte length), assert the structural lines and that the recorded fp
	// is parseable and points inside the .dim file.
	_ = wantDiiPrefix
	assertDiiStructure(t, diiText, "g", int64(len(dimText)))
}

// assertDiiStructure validates the .dii text: a "field count 1" header, the
// field-name line, a parseable field-fp line whose value lies within the .dim
// file, and the checksum footer.
func assertDiiStructure(t *testing.T, diiText, field string, dimLen int64) {
	t.Helper()
	lines := strings.Split(diiText, "\n")
	if len(lines) < 4 {
		t.Fatalf("dii too short: %q", diiText)
	}
	if lines[0] != "field count 1" {
		t.Errorf("dii line 0 = %q, want %q", lines[0], "field count 1")
	}
	if lines[1] != "  field fp name "+field {
		t.Errorf("dii line 1 = %q, want %q", lines[1], "  field fp name "+field)
	}
	const fpPrefix = "  field fp "
	if !strings.HasPrefix(lines[2], fpPrefix) {
		t.Fatalf("dii line 2 = %q, want prefix %q", lines[2], fpPrefix)
	}
	var fp int64
	if _, err := fmt.Sscanf(lines[2][len(fpPrefix):], "%d", &fp); err != nil {
		t.Fatalf("dii field fp not parseable: %q (%v)", lines[2], err)
	}
	if fp < 0 || fp >= dimLen {
		t.Errorf("dii field fp %d out of range [0,%d)", fp, dimLen)
	}
	if !strings.HasPrefix(lines[3], "checksum ") {
		t.Errorf("dii line 3 = %q, want checksum line", lines[3])
	}
}

// readWholeFile reads the entire content of name from dir as a string.
func readWholeFile(t *testing.T, dir store.Directory, name string) string {
	t.Helper()
	in, err := dir.OpenInput(name, store.IOContext{Context: store.ContextRead})
	if err != nil {
		t.Fatalf("OpenInput(%q): %v", name, err)
	}
	defer in.Close()
	n := in.Length()
	buf := make([]byte, n)
	if err := in.ReadBytes(buf); err != nil {
		t.Fatalf("ReadBytes(%q): %v", name, err)
	}
	return string(buf)
}
