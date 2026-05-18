// Package spatial4j implements
// org.apache.lucene.spatial.spatial4j: the bridge between spatial4j shapes
// and Lucene's geo3d code path.
package spatial4j

// Geo3dShape is the marker satisfied by every shape backed by the geo3d
// engine. Mirrors org.apache.lucene.spatial.spatial4j.Geo3dShape.
type Geo3dShape interface {
	IsGeo3d() bool
}

type baseGeo3dShape struct{}

func (baseGeo3dShape) IsGeo3d() bool { return true }

// Geo3dPointShape is the point variant.
type Geo3dPointShape struct {
	baseGeo3dShape
	Lat float64
	Lon float64
}

// NewGeo3dPointShape builds a point.
func NewGeo3dPointShape(lat, lon float64) *Geo3dPointShape {
	return &Geo3dPointShape{Lat: lat, Lon: lon}
}

// Geo3dCircleShape is the circle variant.
type Geo3dCircleShape struct {
	baseGeo3dShape
	Lat    float64
	Lon    float64
	Radius float64
}

// NewGeo3dCircleShape builds a circle (radius in metres).
func NewGeo3dCircleShape(lat, lon, radius float64) *Geo3dCircleShape {
	return &Geo3dCircleShape{Lat: lat, Lon: lon, Radius: radius}
}

// Geo3dRectangleShape is the rectangle variant.
type Geo3dRectangleShape struct {
	baseGeo3dShape
	MinLat float64
	MinLon float64
	MaxLat float64
	MaxLon float64
}

// NewGeo3dRectangleShape builds a rectangle.
func NewGeo3dRectangleShape(minLat, minLon, maxLat, maxLon float64) *Geo3dRectangleShape {
	return &Geo3dRectangleShape{MinLat: minLat, MinLon: minLon, MaxLat: maxLat, MaxLon: maxLon}
}

// Geo3dDistanceCalculator returns geodesic distance between two points.
type Geo3dDistanceCalculator struct{}

// Distance returns an approximate haversine distance (metres).
func (Geo3dDistanceCalculator) Distance(lat1, lon1, lat2, lon2 float64) float64 {
	const earth = 6371000.0
	dLat := (lat2 - lat1) * 0.017453292519943295
	dLon := (lon2 - lon1) * 0.017453292519943295
	a := sin2(dLat/2) + cosDeg(lat1)*cosDeg(lat2)*sin2(dLon/2)
	return 2 * earth * asin(sqrt(a))
}

// Geo3dShapeFactory builds Geo3d shapes from latitudes / longitudes.
type Geo3dShapeFactory struct{}

// NewPoint forwards to NewGeo3dPointShape.
func (Geo3dShapeFactory) NewPoint(lat, lon float64) Geo3dShape {
	return NewGeo3dPointShape(lat, lon)
}

// NewCircle forwards to NewGeo3dCircleShape.
func (Geo3dShapeFactory) NewCircle(lat, lon, radius float64) Geo3dShape {
	return NewGeo3dCircleShape(lat, lon, radius)
}

// NewRectangle forwards to NewGeo3dRectangleShape.
func (Geo3dShapeFactory) NewRectangle(minLat, minLon, maxLat, maxLon float64) Geo3dShape {
	return NewGeo3dRectangleShape(minLat, minLon, maxLat, maxLon)
}

// Geo3dSpatialContextFactory bundles the configuration for the spatial4j
// SpatialContext when using the geo3d engine. Mirrors
// org.apache.lucene.spatial.spatial4j.Geo3dSpatialContextFactory.
type Geo3dSpatialContextFactory struct {
	Geo bool
}

// NewGeo3dSpatialContextFactory builds the factory with Geo=true.
func NewGeo3dSpatialContextFactory() *Geo3dSpatialContextFactory {
	return &Geo3dSpatialContextFactory{Geo: true}
}

// Geo3dBinaryCodec encodes / decodes a Geo3d shape to bytes. Mirrors
// org.apache.lucene.spatial.spatial4j.Geo3dBinaryCodec.
type Geo3dBinaryCodec struct{}

// Encode writes a shape to a debug-friendly byte form. The Go port writes
// the type label followed by the parameters as 8-byte big-endian floats.
func (Geo3dBinaryCodec) Encode(shape Geo3dShape) []byte {
	switch s := shape.(type) {
	case *Geo3dPointShape:
		return packFloats('P', s.Lat, s.Lon)
	case *Geo3dCircleShape:
		return packFloats('C', s.Lat, s.Lon, s.Radius)
	case *Geo3dRectangleShape:
		return packFloats('R', s.MinLat, s.MinLon, s.MaxLat, s.MaxLon)
	}
	return nil
}
