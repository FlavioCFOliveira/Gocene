package geom

import "testing"

func TestGeoPoint_New(t *testing.T) {
	p := NewGeoPoint(1.0, 0.0, 0.0)
	if p == nil {
		t.Fatal("NewGeoPoint returned nil")
	}
}

func TestGeoPoint_PlanetModel(t *testing.T) {
	pm := NewPlanetModel(1.0, 1.0)
	p := NewGeoPointLatLon(pm, 0.0, 0.0)
	if p == nil {
		t.Fatal("NewGeoPointLatLon returned nil")
	}
	lat := p.GetLatitude()
	lon := p.GetLongitude()
	if lat != 0.0 || lon != 0.0 {
		t.Fatalf("lat=%v lon=%v, want 0,0", lat, lon)
	}
}

func TestGeoPoint_IsIdentical(t *testing.T) {
	a := NewGeoPoint(1.0, 0.0, 0.0)
	b := NewGeoPoint(1.0, 0.0, 0.0)
	c := NewGeoPoint(0.0, 1.0, 0.0)
	if !a.IsIdentical(b) {
		t.Error("identical points should compare equal")
	}
	if a.IsIdentical(c) {
		t.Error("different points should not compare equal")
	}
}

func TestPlanetModel_WGS84(t *testing.T) {
	pm := NewPlanetModel(1.0, 1.0)
	if pm == nil {
		t.Fatal("NewPlanetModel returned nil")
	}
}
