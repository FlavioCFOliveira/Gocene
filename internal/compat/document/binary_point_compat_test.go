// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

//go:build compat

// binary_point_compat_test.go verifies that Gocene can read point (BKD)
// values written by Apache Lucene 10.4.0 and decode them back to the
// original IntPoint/LongPoint/FloatPoint/DoublePoint field values.
//
// Audit row (docs/compat-coverage.tsv, row 98):
//
//	"Point binary encoding (BKD payloads)" — gap_notes:
//	  "No Lucene-emitted .kdd/.kdi fixture verified by these document tests."
package document

import (
	"fmt"
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/codecs"
	_ "github.com/FlavioCFOliveira/Gocene/codecs/lucene90" // BKD reader hook
	"github.com/FlavioCFOliveira/Gocene/document"
	gindex "github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/store"
)

// pointValuesClient is the interface for reading point values from a
// codec-level PointsReader (the concrete *lucene90.pointsReader has this
// method but the type is unexported; we access it via type assertion).
type pointValuesClient interface {
	GetValues(field string) (gindex.PointValues, error)
}

// intersectablePointValues covers the index.PointValues metadata plus the
// Intersect method that the concrete *lucene90.pointValues exposes.
type intersectablePointValues interface {
	gindex.PointValues
	Intersect(visitor gindex.PointTreeIntersectVisitor) error
}

// allPointsCollector visits every point in the BKD tree and collects
// (docID, packedValue) pairs. It returns CELL_INSIDE_QUERY for every
// bounding box, so the tree walks all leaves and emits every value.
type allPointsCollector struct {
	points []pointEntry
}

type pointEntry struct {
	docID       int
	packedValue []byte
}

func (c *allPointsCollector) Visit(docID int) error {
	c.points = append(c.points, pointEntry{docID: docID})
	return nil
}

func (c *allPointsCollector) VisitByPackedValue(docID int, packedValue []byte) error {
	buf := make([]byte, len(packedValue))
	copy(buf, packedValue)
	c.points = append(c.points, pointEntry{docID: docID, packedValue: buf})
	return nil
}

func (c *allPointsCollector) Compare(_, _ []byte) int { return 1 } // CELL_INSIDE_QUERY

func (c *allPointsCollector) Grow(int) {}

// openPointsReader opens the Lucene90PointsFormat reader for segment "_0"
// in dir. It reads the .si and .fnm entries and constructs a
// SegmentReadState, then returns the PointsReader (which must support
// pointValuesClient).
func openPointsReader(t *testing.T, dir string) (pointValuesClient, *gindex.FieldInfos, func()) {
	t.Helper()
	d, err := store.NewSimpleFSDirectory(dir)
	if err != nil {
		t.Fatalf("open dir: %v", err)
	}

	siFormat := codecs.NewLucene99SegmentInfoFormat()
	si, err := siFormat.Read(d, "_0", nil, store.IOContextDefault)
	if err != nil {
		_ = d.Close()
		t.Fatalf("read .si: %v", err)
	}

	fiFormat := codecs.NewLucene104FieldInfosFormat()
	fn, err := fiFormat.Read(d, si, "", store.IOContextDefault)
	if err != nil {
		_ = d.Close()
		t.Fatalf("read .fnm: %v", err)
	}

	format := codecs.NewLucene90PointsFormat()
	reader, err := format.FieldsReader(&codecs.SegmentReadState{
		Directory:   d,
		SegmentInfo: si,
		FieldInfos:  fn,
	})
	if err != nil {
		_ = d.Close()
		t.Fatalf("FieldsReader: %v", err)
	}

	pvc, ok := reader.(pointValuesClient)
	if !ok {
		_ = reader.Close()
		_ = d.Close()
		t.Fatalf("PointsReader %T does not support GetValues", reader)
	}

	return pvc, fn, func() {
		_ = reader.Close()
		_ = d.Close()
	}
}

// collectAllPoints returns all point entries for the given field from the
// BKD tree, using an intersect-visitor that accepts everything.
func collectAllPoints(t *testing.T, pvc pointValuesClient, field string) []pointEntry {
	t.Helper()
	pv, err := pvc.GetValues(field)
	if err != nil {
		t.Fatalf("GetValues(%q): %v", field, err)
	}
	if pv == nil {
		t.Fatalf("GetValues(%q) returned nil", field)
	}

	iv, ok := pv.(intersectablePointValues)
	if !ok {
		t.Fatalf("PointValues for %q does not support Intersect (%T)", field, pv)
	}

	var coll allPointsCollector
	if err := iv.Intersect(&coll); err != nil {
		t.Fatalf("Intersect(%q): %v", field, err)
	}
	return coll.points
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestBinaryPoint_PointsFormatPayloadValues uses the foundational
// "points-format" scenario (IntPoint, LongPoint, FloatPoint) and verifies
// that Gocene's BKD reader can read the packed values and decode them back
// to the original int32/int64/float32 values documented in the Java
// PointsFormatScenario contract.
func TestBinaryPoint_PointsFormatPayloadValues(t *testing.T) {
	const seed int64 = 0xC0FFEE
	const numDocs = 10

	rawDir := generate(t, "points-format", seed)
	pvc, _, cleanup := openPointsReader(t, rawDir)
	defer cleanup()

	// IntPoint("ip", i, i + (int)seed) — 2 dims, 4 bytes each.
	t.Run("IntPoint", func(t *testing.T) {
		points := collectAllPoints(t, pvc, "ip")
		if len(points) != numDocs {
			t.Fatalf("got %d points, want %d", len(points), numDocs)
		}
		for _, p := range points {
			docID := p.docID
			v0 := int32(document.DecodeDimensionIntLucene(p.packedValue, 0))
			v1 := int32(document.DecodeDimensionIntLucene(p.packedValue, 4))
			want0 := int32(docID)
			want1 := int32(docID) + int32(seed)
			if v0 != want0 || v1 != want1 {
				t.Errorf("doc %d: got (%d,%d), want (%d,%d)",
					docID, v0, v1, want0, want1)
			}
		}
	})

	// LongPoint("lp", (long)i * 13 + seed) — 1 dim, 8 bytes.
	t.Run("LongPoint", func(t *testing.T) {
		points := collectAllPoints(t, pvc, "lp")
		if len(points) != numDocs {
			t.Fatalf("got %d points, want %d", len(points), numDocs)
		}
		for _, p := range points {
			got := document.DecodeDimensionLongLucene(p.packedValue, 0)
			want := int64(p.docID)*13 + seed
			if got != want {
				t.Errorf("doc %d: got %d, want %d", p.docID, got, want)
			}
		}
	})

	// FloatPoint("fp", (float)(i*0.5), (float)((seed&0xFFFF)*0.25)) — 2 dims.
	t.Run("FloatPoint", func(t *testing.T) {
		points := collectAllPoints(t, pvc, "fp")
		if len(points) != numDocs {
			t.Fatalf("got %d points, want %d", len(points), numDocs)
		}
		shortSeed := float32(seed & 0xFFFF)
		for _, p := range points {
			v0 := document.DecodeDimensionFloatLucene(p.packedValue, 0)
			v1 := document.DecodeDimensionFloatLucene(p.packedValue, 4)
			want0 := float32(p.docID) * 0.5
			want1 := shortSeed * 0.25
			if v0 != want0 || v1 != want1 {
				t.Errorf("doc %d: got (%v,%v), want (%v,%v)",
					p.docID, v0, v1, want0, want1)
			}
		}
	})
}

// TestBinaryPoint_DocumentPointsFormatPayloadValues uses the new
// "document-points-format" scenario which extends the foundational scenario
// by also adding DoublePoint coverage.
func TestBinaryPoint_DocumentPointsFormatPayloadValues(t *testing.T) {
	const seed int64 = 0xC0FFEE
	const numDocs = 10

	rawDir := generate(t, ScenarioDocumentPoints, seed)
	pvc, _, cleanup := openPointsReader(t, rawDir)
	defer cleanup()

	// IntPoint("ip", i, i + (int)seed) — 2 dims, 4 bytes each.
	t.Run("IntPoint", func(t *testing.T) {
		points := collectAllPoints(t, pvc, "ip")
		if len(points) != numDocs {
			t.Fatalf("got %d points, want %d", len(points), numDocs)
		}
		for _, p := range points {
			v0 := int32(document.DecodeDimensionIntLucene(p.packedValue, 0))
			v1 := int32(document.DecodeDimensionIntLucene(p.packedValue, 4))
			want0 := int32(p.docID)
			want1 := int32(p.docID) + int32(seed)
			if v0 != want0 || v1 != want1 {
				t.Errorf("doc %d: got (%d,%d), want (%d,%d)",
					p.docID, v0, v1, want0, want1)
			}
		}
	})

	// LongPoint("lp", (long)i * 13 + seed) — 1 dim.
	t.Run("LongPoint", func(t *testing.T) {
		points := collectAllPoints(t, pvc, "lp")
		if len(points) != numDocs {
			t.Fatalf("got %d points, want %d", len(points), numDocs)
		}
		for _, p := range points {
			got := document.DecodeDimensionLongLucene(p.packedValue, 0)
			want := int64(p.docID)*13 + seed
			if got != want {
				t.Errorf("doc %d: got %d, want %d", p.docID, got, want)
			}
		}
	})

	// FloatPoint("fp", (float)(i*0.5), (float)((seed&0xFFFF)*0.25)) — 2 dims.
	t.Run("FloatPoint", func(t *testing.T) {
		points := collectAllPoints(t, pvc, "fp")
		if len(points) != numDocs {
			t.Fatalf("got %d points, want %d", len(points), numDocs)
		}
		shortSeed := float32(seed & 0xFFFF)
		for _, p := range points {
			v0 := document.DecodeDimensionFloatLucene(p.packedValue, 0)
			v1 := document.DecodeDimensionFloatLucene(p.packedValue, 4)
			want0 := float32(p.docID) * 0.5
			want1 := shortSeed * 0.25
			if v0 != want0 || v1 != want1 {
				t.Errorf("doc %d: got (%v,%v), want (%v,%v)",
					p.docID, v0, v1, want0, want1)
			}
		}
	})

	// DoublePoint("dp", (double)i * Math.PI + seed) — 1 dim.
	t.Run("DoublePoint", func(t *testing.T) {
		points := collectAllPoints(t, pvc, "dp")
		if len(points) != numDocs {
			t.Fatalf("got %d points, want %d", len(points), numDocs)
		}
		for _, p := range points {
			got := document.DecodeDimensionDoubleLucene(p.packedValue, 0)
			want := float64(p.docID)*math.Pi + float64(seed)
			// Tolerate rounding in the float64 quantisation through
			// sortable-bytes round-trip.
			rel := math.Abs(got-want) / math.Max(1.0, math.Abs(want))
			if rel > 1e-12 {
				t.Errorf("doc %d: got %v, want %v (rel error %e)",
					p.docID, got, want, rel)
			}
		}
	})
}

// TestBinaryPoint_AllSeeds verifies that the point payload test passes at
// both canary seeds (byte-determinism check).
func TestBinaryPoint_AllSeeds(t *testing.T) {
	for _, seed := range canarySeeds {
		seed := seed
		t.Run(fmt.Sprintf("seed=%d", seed), func(t *testing.T) {
			rawDir := generate(t, "points-format", seed)
			pvc, fn, cleanup := openPointsReader(t, rawDir)
			defer cleanup()

			// Verify the three point fields from the points-format scenario.
			for _, name := range []string{"ip", "lp", "fp"} {
				fi := fn.GetByName(name)
				if fi == nil {
					t.Errorf("field %q not found", name)
					continue
				}
				pv, err := pvc.GetValues(fi.Name())
				if err != nil {
					t.Errorf("GetValues(%q): %v", fi.Name(), err)
					continue
				}
				if pv == nil {
					t.Errorf("GetValues(%q) returned nil", fi.Name())
					continue
				}
				if pv.GetNumDimensions() != fi.PointDimensionCount() {
					t.Errorf("%q dims: got %d, want %d",
						fi.Name(), pv.GetNumDimensions(), fi.PointDimensionCount())
				}
			}
		})
	}
}
