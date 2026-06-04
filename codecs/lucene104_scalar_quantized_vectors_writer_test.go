// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
	"github.com/FlavioCFOliveira/Gocene/util/packed"
	"github.com/FlavioCFOliveira/Gocene/util/quantization"
)

// scalarSimToCodecs maps an index.VectorSimilarityFunction to the codecs-side
// VectorSimilarityFunction (same ordinal order) consumed by Load.
func scalarSimToCodecs(f index.VectorSimilarityFunction) VectorSimilarityFunction {
	switch f {
	case index.VectorSimilarityFunctionEuclidean:
		return VectorSimilarityFunctionEuclidean
	case index.VectorSimilarityFunctionDotProduct:
		return VectorSimilarityFunctionDotProduct
	case index.VectorSimilarityFunctionCosine:
		return VectorSimilarityFunctionCosine
	default:
		return VectorSimilarityFunctionMaximumInnerProduct
	}
}

// scalarQuantizedRoundTripConfig is a test-side ordToDocDISIReaderConfig built
// from a parsed Lucene104ScalarQuantizedFieldEntry. It reproduces the dense /
// empty / sparse dispatch of the Java OrdToDocDISIReaderConfiguration, reusing
// the proven flat-reader-style sparse helpers (newDVIndexedDISI +
// DirectMonotonicReader). This lets the round-trip test drive
// OffHeapScalarQuantizedFloatVectorValues.Load without depending on the
// deferred search-integration reader (rmp #134).
type scalarQuantizedRoundTripConfig struct {
	entry      *Lucene104ScalarQuantizedFieldEntry
	vectorData store.IndexInput
}

func (c *scalarQuantizedRoundTripConfig) IsEmpty() bool {
	return c.entry.DocsWithFieldOffset == -2
}

func (c *scalarQuantizedRoundTripConfig) IsDense() bool {
	return c.entry.DocsWithFieldOffset == -1
}

func (c *scalarQuantizedRoundTripConfig) GetDirectMonotonicReader(dataIn store.IndexInput) (OrdToDocReader, error) {
	addrSlice, err := dvSliceRandomAccess(dataIn, c.entry.AddressesOffset, c.entry.AddressesLength)
	if err != nil {
		return nil, err
	}
	return packed.NewDirectMonotonicReader(c.entry.OrdToDocMeta, addrSlice)
}

func (c *scalarQuantizedRoundTripConfig) GetIndexedDISI(dataIn store.IndexInput) (IndexedDISIView, error) {
	return newDVIndexedDISI(
		dataIn, c.entry.DocsWithFieldOffset, c.entry.DocsWithFieldLength,
		c.entry.JumpTableEntryCount, c.entry.DenseRankPower, int64(c.entry.Size),
	)
}

// writeScalarQuantizedSegment writes the given (docID -> vector) values for a
// single FLOAT32 field through the faithful scalar-quantized writer and
// returns the directory, SegmentInfo and FieldInfos for the read-back.
func writeScalarQuantizedSegment(
	t *testing.T,
	encoding ScalarEncoding,
	field string,
	dim, maxDoc int,
	sim index.VectorSimilarityFunction,
	docIDs []int,
	vecByDoc map[int][]float32,
) (store.Directory, *index.SegmentInfo, *index.FieldInfos) {
	t.Helper()

	fis := index.NewFieldInfos()
	if err := fis.Add(vectorFieldInfoFloat(field, 0, dim, sim)); err != nil {
		t.Fatalf("fis.Add: %v", err)
	}

	dir, err := store.NewSimpleFSDirectory(t.TempDir())
	if err != nil {
		t.Fatalf("NewSimpleFSDirectory: %v", err)
	}
	si := index.NewSegmentInfo("_0", maxDoc, dir)
	if err := si.SetID(seqID()); err != nil {
		t.Fatalf("SetID: %v", err)
	}
	ws := &SegmentWriteState{Directory: dir, SegmentInfo: si, FieldInfos: fis}

	w, err := NewLucene104ScalarQuantizedVectorsWriter(ws, encoding)
	if err != nil {
		t.Fatalf("NewLucene104ScalarQuantizedVectorsWriter: %v", err)
	}
	fw, err := w.AddField(fis.GetByName(field))
	if err != nil {
		t.Fatalf("AddField: %v", err)
	}
	for _, doc := range docIDs {
		if err := fw.AddValue(doc, vecByDoc[doc]); err != nil {
			t.Fatalf("AddValue(%d): %v", doc, err)
		}
	}
	if err := fw.Finish(); err != nil {
		t.Fatalf("field Finish: %v", err)
	}
	if err := w.Flush(maxDoc, nil); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if err := w.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return dir, si, fis
}

// loadScalarQuantizedValues reads back the segment written by
// writeScalarQuantizedSegment: it validates the CodecUtil framing via the
// reader and constructs an OffHeapScalarQuantizedFloatVectorValues view over
// the quantized data via Load. It returns the values view, the parsed entry,
// and a cleanup func.
func loadScalarQuantizedValues(
	t *testing.T,
	dir store.Directory, si *index.SegmentInfo, fis *index.FieldInfos, field string,
) (*OffHeapScalarQuantizedFloatVectorValues, *Lucene104ScalarQuantizedFieldEntry, func()) {
	t.Helper()

	rs := &SegmentReadState{Directory: dir, SegmentInfo: si, FieldInfos: fis}
	r, err := NewLucene104ScalarQuantizedVectorsReader(rs, ScalarEncodingUnsignedByte)
	if err != nil {
		t.Fatalf("NewLucene104ScalarQuantizedVectorsReader: %v", err)
	}

	// (a) CodecUtil framing: the .veq index header + footer checksum must be
	// valid end-to-end (the constructor already validated both files' headers
	// and the .vemq footer).
	if err := r.CheckIntegrity(); err != nil {
		t.Fatalf("CheckIntegrity: %v", err)
	}

	entry, err := r.FieldEntry(field)
	if err != nil {
		t.Fatalf("FieldEntry: %v", err)
	}

	scorer := NewDefaultFlatVectorScorer()
	cfg := &scalarQuantizedRoundTripConfig{entry: entry, vectorData: r.VectorData()}
	values, err := Load(
		cfg,
		entry.Dimension, entry.Size, entry.Encoding,
		scalarSimToCodecs(entry.SimilarityFunction),
		scorer, entry.Centroid,
		entry.VectorDataOffset, entry.VectorDataLength,
		r.VectorData(),
	)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return values, entry, func() { _ = r.Close() }
}

// dequantizeTolerance returns an acceptable per-component absolute error for a
// given bit-width over a vector range. Lower bit-widths quantize more
// coarsely, so the tolerance scales with the step size implied by the bits.
func dequantizeTolerance(bits int, valueRange float32) float32 {
	steps := float32(int(1<<bits) - 1)
	// One full quantization step, plus generous slack for the optimized
	// interval clamping (the optimizer narrows the interval, so the effective
	// step can be smaller; we bound the worst case at ~2 steps).
	return 2 * valueRange / steps
}

// TestLucene104ScalarQuantized_DenseRoundTrip writes scalar-quantized FLOAT32
// vectors for several encodings/bit-widths through the faithful writer, then
// reads them back via OffHeapScalarQuantizedFloatVectorValues.Load, asserting
// (a) valid CodecUtil framing (CheckIntegrity), (b) each vector dequantizes
// within quantization tolerance of the original, (c) the centroid/quantiles
// meta round-trips.
func TestLucene104ScalarQuantized_DenseRoundTrip(t *testing.T) {
	const (
		field  = "vec"
		dim    = 8
		maxDoc = 6
	)
	// Even dimension (8) so PACKED_NIBBLE's discretized length equals the raw
	// dimension and the read side's dequantize buffer is sized correctly.
	vectors := [][]float32{
		{0.10, -0.20, 0.30, -0.40, 0.50, -0.60, 0.70, -0.80},
		{1.00, 0.90, 0.80, 0.70, 0.60, 0.50, 0.40, 0.30},
		{-1.00, -0.90, -0.80, -0.70, -0.60, -0.50, -0.40, -0.30},
		{0.05, 0.15, 0.25, 0.35, 0.45, 0.55, 0.65, 0.75},
		{2.00, -2.00, 1.50, -1.50, 1.00, -1.00, 0.50, -0.50},
		{0.00, 0.00, 0.00, 0.00, 0.00, 0.00, 0.00, 0.00},
	}
	vecByDoc := make(map[int][]float32, len(vectors))
	docIDs := make([]int, len(vectors))
	for i, v := range vectors {
		vecByDoc[i] = v
		docIDs[i] = i
	}

	encodings := []struct {
		name string
		enc  ScalarEncoding
		bits int
	}{
		{"UNSIGNED_BYTE_8bit", ScalarEncodingUnsignedByte, 8},
		{"SEVEN_BIT_7bit", ScalarEncodingSevenBit, 7},
		{"PACKED_NIBBLE_4bit", ScalarEncodingPackedNibble, 4},
	}

	for _, ec := range encodings {
		t.Run(ec.name, func(t *testing.T) {
			dir, si, fis := writeScalarQuantizedSegment(
				t, ec.enc, field, dim, maxDoc, index.VectorSimilarityFunctionEuclidean, docIDs, vecByDoc)
			defer dir.Close()

			values, entry, cleanup := loadScalarQuantizedValues(t, dir, si, fis, field)
			defer cleanup()

			// (c) meta round-trips: size, dimension, encoding, dense layout.
			if values.Size() != maxDoc {
				t.Fatalf("Size = %d, want %d", values.Size(), maxDoc)
			}
			if values.Dimension() != dim {
				t.Fatalf("Dimension = %d, want %d", values.Dimension(), dim)
			}
			if entry.Encoding != ec.enc {
				t.Fatalf("entry.Encoding = %s, want %s", entry.Encoding, ec.enc)
			}
			if entry.DocsWithFieldOffset != -1 {
				t.Fatalf("dense field should record docsWithFieldOffset = -1, got %d", entry.DocsWithFieldOffset)
			}
			// Centroid round-trips and matches the mean of the inputs.
			if len(entry.Centroid) != dim {
				t.Fatalf("centroid len = %d, want %d", len(entry.Centroid), dim)
			}
			wantCentroid := meanVectors(vectors)
			for i := range wantCentroid {
				if diff := absF32(entry.Centroid[i] - wantCentroid[i]); diff > 1e-5 {
					t.Errorf("centroid[%d] = %g, want %g (diff %g)", i, entry.Centroid[i], wantCentroid[i], diff)
				}
			}
			// CentroidDP must equal dot(centroid, centroid).
			var wantDP float32
			for _, c := range entry.Centroid {
				wantDP += c * c
			}
			if diff := absF32(entry.CentroidDP - wantDP); diff > 1e-4 {
				t.Errorf("centroidDP = %g, want %g (diff %g)", entry.CentroidDP, wantDP, diff)
			}

			// (b) dequantization within tolerance.
			tol := dequantizeTolerance(ec.bits, 4.0) // range ~[-2, 2]
			for doc := 0; doc < maxDoc; doc++ {
				got, err := values.VectorValue(doc)
				if err != nil {
					t.Fatalf("VectorValue(%d): %v", doc, err)
				}
				want := vectors[doc]
				for i := range want {
					if diff := absF32(got[i] - want[i]); diff > tol {
						t.Errorf("doc %d comp %d: got %g, want %g (diff %g > tol %g)",
							doc, i, got[i], want[i], diff, tol)
					}
				}
			}
		})
	}
}

// TestLucene104ScalarQuantized_SparseRoundTrip writes scalar-quantized vectors
// to only a subset of the documents in a segment (count < maxDoc -> sparse),
// then reads them back through the IndexedDISI + DirectMonotonic ord->doc
// layout via Load, asserting framing, dequantization tolerance, and the
// ord->doc mapping.
func TestLucene104ScalarQuantized_SparseRoundTrip(t *testing.T) {
	const (
		field  = "vec"
		dim    = 8
		maxDoc = 10
	)
	// docs 1, 3, 4, 7, 9 carry a vector (5 of 10 -> sparse). ords assigned in
	// docID order: ord 0->doc 1, 1->doc 3, 2->doc 4, 3->doc 7, 4->doc 9.
	docIDs := []int{1, 3, 4, 7, 9}
	vecByDoc := map[int][]float32{
		1: {0.10, -0.20, 0.30, -0.40, 0.50, -0.60, 0.70, -0.80},
		3: {1.00, 0.90, 0.80, 0.70, 0.60, 0.50, 0.40, 0.30},
		4: {-1.00, -0.90, -0.80, -0.70, -0.60, -0.50, -0.40, -0.30},
		7: {2.00, -2.00, 1.50, -1.50, 1.00, -1.00, 0.50, -0.50},
		9: {0.05, 0.15, 0.25, 0.35, 0.45, 0.55, 0.65, 0.75},
	}

	encodings := []struct {
		name string
		enc  ScalarEncoding
		bits int
	}{
		{"UNSIGNED_BYTE_8bit", ScalarEncodingUnsignedByte, 8},
		{"SEVEN_BIT_7bit", ScalarEncodingSevenBit, 7},
		{"PACKED_NIBBLE_4bit", ScalarEncodingPackedNibble, 4},
	}

	for _, ec := range encodings {
		t.Run(ec.name, func(t *testing.T) {
			dir, si, fis := writeScalarQuantizedSegment(
				t, ec.enc, field, dim, maxDoc, index.VectorSimilarityFunctionEuclidean, docIDs, vecByDoc)
			defer dir.Close()

			values, entry, cleanup := loadScalarQuantizedValues(t, dir, si, fis, field)
			defer cleanup()

			if values.Size() != len(docIDs) {
				t.Fatalf("Size = %d, want %d", values.Size(), len(docIDs))
			}
			if entry.DocsWithFieldOffset < 0 {
				t.Fatalf("sparse field should record docsWithFieldOffset >= 0, got %d", entry.DocsWithFieldOffset)
			}
			if entry.OrdToDocMeta == nil {
				t.Fatalf("sparse field should carry an ord->doc monotonic meta")
			}

			// ord -> doc mapping round-trips via the iterator (which is driven
			// by the IndexedDISI), and each vector dequantizes within tolerance.
			tol := dequantizeTolerance(ec.bits, 4.0)
			it := values.Iterator()
			ord := 0
			for {
				doc, err := it.NextDoc()
				if err != nil {
					t.Fatalf("NextDoc: %v", err)
				}
				if doc == noMoreDocsView {
					break
				}
				wantDoc := docIDs[ord]
				if doc != wantDoc {
					t.Errorf("ord %d -> doc %d, want %d", ord, doc, wantDoc)
				}
				if mapped := values.OrdToDoc(ord); mapped != wantDoc {
					t.Errorf("OrdToDoc(%d) = %d, want %d", ord, mapped, wantDoc)
				}
				got, err := values.VectorValue(ord)
				if err != nil {
					t.Fatalf("VectorValue(ord=%d): %v", ord, err)
				}
				want := vecByDoc[wantDoc]
				for i := range want {
					if diff := absF32(got[i] - want[i]); diff > tol {
						t.Errorf("doc %d comp %d: got %g, want %g (diff %g > tol %g)",
							wantDoc, i, got[i], want[i], diff, tol)
					}
				}
				ord++
			}
			if ord != len(docIDs) {
				t.Errorf("iterated %d ords, want %d", ord, len(docIDs))
			}
		})
	}
}

// TestLucene104ScalarQuantized_CosineNormalization verifies that COSINE fields
// normalize the centroid to unit length and dequantize within tolerance.
func TestLucene104ScalarQuantized_CosineNormalization(t *testing.T) {
	const (
		field  = "cvec"
		dim    = 4
		maxDoc = 4
	)
	// Pre-normalized unit vectors (the writer further normalizes for COSINE;
	// these are already unit length so normalization is a near no-op).
	vectors := [][]float32{
		unit([]float32{1, 0, 0, 0}),
		unit([]float32{0, 1, 0, 0}),
		unit([]float32{1, 1, 0, 0}),
		unit([]float32{0.5, 0.5, 0.5, 0.5}),
	}
	vecByDoc := make(map[int][]float32, len(vectors))
	docIDs := make([]int, len(vectors))
	for i, v := range vectors {
		vecByDoc[i] = v
		docIDs[i] = i
	}

	dir, si, fis := writeScalarQuantizedSegment(
		t, ScalarEncodingSevenBit, field, dim, maxDoc, index.VectorSimilarityFunctionCosine, docIDs, vecByDoc)
	defer dir.Close()

	values, entry, cleanup := loadScalarQuantizedValues(t, dir, si, fis, field)
	defer cleanup()

	// The centroid must be unit length (l2normalized) for COSINE.
	var centroidNorm2 float32
	for _, c := range entry.Centroid {
		centroidNorm2 += c * c
	}
	if diff := absF32(centroidNorm2 - 1.0); diff > 1e-4 {
		t.Errorf("COSINE centroid not unit length: norm2 = %g (diff %g)", centroidNorm2, diff)
	}

	tol := dequantizeTolerance(7, 2.0)
	for doc := 0; doc < maxDoc; doc++ {
		got, err := values.VectorValue(doc)
		if err != nil {
			t.Fatalf("VectorValue(%d): %v", doc, err)
		}
		want := unit(vectors[doc])
		for i := range want {
			if diff := absF32(got[i] - want[i]); diff > tol {
				t.Errorf("doc %d comp %d: got %g, want %g (diff %g > tol %g)",
					doc, i, got[i], want[i], diff, tol)
			}
		}
	}
}

// TestLucene104ScalarQuantized_FramingViaReader proves the CodecUtil framing is
// real: a fresh reader validates the .veq / .vemq index headers (segment id +
// suffix) and the footer checksums; corrupting a byte must be detected.
func TestLucene104ScalarQuantized_FramingViaReader(t *testing.T) {
	const (
		field  = "vec"
		dim    = 4
		maxDoc = 3
	)
	vecByDoc := map[int][]float32{
		0: {0.1, 0.2, 0.3, 0.4},
		1: {0.5, 0.6, 0.7, 0.8},
		2: {-0.1, -0.2, -0.3, -0.4},
	}
	dir, si, fis := writeScalarQuantizedSegment(
		t, ScalarEncodingUnsignedByte, field, dim, maxDoc, index.VectorSimilarityFunctionDotProduct,
		[]int{0, 1, 2}, vecByDoc)
	defer dir.Close()

	// A valid reader opens and passes integrity.
	rs := &SegmentReadState{Directory: dir, SegmentInfo: si, FieldInfos: fis}
	r, err := NewLucene104ScalarQuantizedVectorsReader(rs, ScalarEncodingUnsignedByte)
	if err != nil {
		t.Fatalf("NewLucene104ScalarQuantizedVectorsReader: %v", err)
	}
	if err := r.CheckIntegrity(); err != nil {
		t.Fatalf("CheckIntegrity (clean): %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Verify the .veq file actually carries a CodecUtil header by reading the
	// magic + codec name directly.
	dataName := index.SegmentFileName(si.Name(), "", Lucene104ScalarQuantizedVectorsFormat_VECTOR_DATA_EXTENSION)
	in, err := dir.OpenInput(dataName, store.IOContextRead)
	if err != nil {
		t.Fatalf("OpenInput .veq: %v", err)
	}
	defer in.Close()
	if _, err := CheckIndexHeader(
		in, lucene104SQDataCodecName, lucene104SQVersionStart, lucene104SQVersionCurrent, si.GetID(), "",
	); err != nil {
		t.Fatalf(".veq CheckIndexHeader: %v", err)
	}
}

// --- small test helpers ---

func meanVectors(vectors [][]float32) []float32 {
	if len(vectors) == 0 {
		return nil
	}
	dim := len(vectors[0])
	out := make([]float32, dim)
	for _, v := range vectors {
		for i := range v {
			out[i] += v[i]
		}
	}
	n := float32(len(vectors))
	for i := range out {
		out[i] /= n
	}
	return out
}

func unit(v []float32) []float32 {
	cp := make([]float32, len(v))
	copy(cp, v)
	var norm2 float64
	for _, x := range cp {
		norm2 += float64(x) * float64(x)
	}
	norm := float32(math.Sqrt(norm2))
	if norm == 0 {
		return cp
	}
	for i := range cp {
		cp[i] /= norm
	}
	return cp
}

func absF32(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

// Compile-time guard: the round-trip config satisfies the Load contract.
var _ ordToDocDISIReaderConfig = (*scalarQuantizedRoundTripConfig)(nil)

// quantization import guard (used indirectly via the writer); referenced here
// to keep the import even if helpers change.
var _ = quantization.NewOptimizedScalarQuantizer
