package spatial3d

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
	"github.com/FlavioCFOliveira/Gocene/spatial3d/geom"
	"github.com/FlavioCFOliveira/Gocene/util"
)

type mockSortedNumericDocValues struct {
	values []int64
	pos    int
}

func (m *mockSortedNumericDocValues) DocID() int { return m.pos }
func (m *mockSortedNumericDocValues) Advance(doc int) (int, error) {
	m.pos = doc
	return doc, nil
}
func (m *mockSortedNumericDocValues) NextDoc() (int, error) {
	m.pos++
	if m.pos >= len(m.values) {
		m.pos = search.NO_MORE_DOCS
	}
	return m.pos, nil
}
func (m *mockSortedNumericDocValues) AdvanceExact(doc int) (bool, error) {
	m.pos = doc
	return doc < len(m.values), nil
}
func (m *mockSortedNumericDocValues) Cost() int64 { return int64(len(m.values)) }
func (m *mockSortedNumericDocValues) DocValueCount() (int, error) {
	if m.pos >= len(m.values) {
		return 0, nil
	}
	return 1, nil
}
func (m *mockSortedNumericDocValues) NextValue() (int64, error) {
	return m.values[m.pos], nil
}
func (m *mockSortedNumericDocValues) LongValue() (int64, error) {
	return m.values[m.pos], nil
}
func (m *mockSortedNumericDocValues) Size() int { return len(m.values) }

// DocIDRunEnd carries the default body Lucene gives SortedNumericDocValues.DocIDRunEnd.
func (m *mockSortedNumericDocValues) DocIDRunEnd() (int, error) {
	return util.DefaultDocIDRunEnd(m)
}

// IntoBitSet carries the default body Lucene gives SortedNumericDocValues.IntoBitSet.
func (m *mockSortedNumericDocValues) IntoBitSet(upTo int, bitSet *util.FixedBitSet, offset int) error {
	return util.DefaultIntoBitSet(m, upTo, bitSet, offset)
}

// asDistanceShape renders Java's GeoCircle extends GeoDistanceShape: the
// circle is passed where a distance shape is expected.
func asDistanceShape(t *testing.T, shape geom.GeoCircle) geom.GeoDistanceShape {
	t.Helper()
	ds, ok := shape.(geom.GeoDistanceShape)
	if !ok {
		t.Fatalf("%T does not implement geom.GeoDistanceShape", shape)
	}
	return ds
}

func TestGeo3DDocValueEncoding(t *testing.T) {
	pm := geom.WGS84
	encoder := geom.NewDocValueEncoder(pm)

	// Test point: (0, 0, 0) in normalized space (roughly)
	// Let's use a real point
	p := geom.NewGeoPointModel(pm, 0, 0) // lat 0, lon 0
	encoded := encoder.EncodePoint(p)
	decoded := encoder.DecodePoint(encoded)

	if math.Abs(decoded.X-p.X) > 1e-7 || math.Abs(decoded.Y-p.Y) > 1e-7 || math.Abs(decoded.Z-p.Z) > 1e-7 {
		t.Errorf("roundtrip failed: got [%g,%g,%g], want [%g,%g,%g]", decoded.X, decoded.Y, decoded.Z, p.X, p.Y, p.Z)
	}
}

func TestGeo3DPointDistanceComparator(t *testing.T) {
	pm := geom.WGS84
	encoder := geom.NewDocValueEncoder(pm)
	shape, _ := geom.MakeGeoCircle(pm, 0, 0, 1000.0/pm.MeanRadius) // 1km radius

	// Points at different distances from (0,0)
	// P1: at center (dist 0)
	// P2: at 500m
	// P3: at 1500m
	p1 := geom.NewGeoPointModel(pm, 0, 0)
	p2 := geom.NewGeoPointModel(pm, 0, 0.005) // approx 500m
	p3 := geom.NewGeoPointModel(pm, 0, 0.015) // approx 1500m

	vals := []int64{
		encoder.EncodePoint(p1),
		encoder.EncodePoint(p2),
		encoder.EncodePoint(p3),
	}

	comp := NewGeo3DPointDistanceComparator("field", pm, asDistanceShape(t, shape), 3)
	comp.currentDocs = &mockSortedNumericDocValues{values: vals}

	// Test Copy
	comp.Copy(0, 0)
	comp.Copy(1, 1)
	comp.Copy(2, 2)

	if comp.values[0] >= comp.values[1] || comp.values[1] >= comp.values[2] {
		t.Errorf("distances not monotonic: %g, %g, %g", comp.values[0], comp.values[1], comp.values[2])
	}

	// Test Compare
	if comp.Compare(0, 1) != -1 {
		t.Errorf("expected slot 0 < slot 1")
	}
}

func TestGeo3DPointOutsideDistanceComparator(t *testing.T) {
	pm := geom.WGS84
	encoder := geom.NewDocValueEncoder(pm)
	shape, _ := geom.MakeGeoCircle(pm, 0, 0, 1000.0/pm.MeanRadius) // 1km radius

	// P1: inside (dist 0)
	// P2: outside (dist > 0)
	p1 := geom.NewGeoPointModel(pm, 0, 0)
	p2 := geom.NewGeoPointModel(pm, 0, 0.02) // well outside

	vals := []int64{
		encoder.EncodePoint(p1),
		encoder.EncodePoint(p2),
	}

	comp := NewGeo3DPointOutsideDistanceComparator("field", pm, asDistanceShape(t, shape), 2)
	comp.currentDocs = &mockSortedNumericDocValues{values: vals}

	comp.Copy(0, 0)
	comp.Copy(1, 1)

	if comp.values[0] != 0 {
		t.Errorf("point inside should have distance 0, got %g", comp.values[0])
	}
	if comp.values[1] <= 0 {
		t.Errorf("point outside should have distance > 0, got %g", comp.values[1])
	}
}
