// Code in this file mirrors org.apache.lucene.geo.GeoEncodingUtils
// from Apache Lucene 10.4.0. The encode/decode helpers are critical
// for byte-for-byte compatibility with Java-produced geo indices and
// are reproduced bit-for-bit (32-bit signed quantisation, floor on
// encode, multiplicative decode).

package geo

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// GeoEncodingBits is the number of bits used to quantise a latitude
// or longitude coordinate. Always 32 in Apache Lucene 10.4.0.
const GeoEncodingBits = 32

// Scale factors. latScale = 2^32 / 180; lonScale = 2^32 / 360. The
// matching decode factors are their reciprocals. These four values
// are bit-for-bit identical to Lucene's LAT_SCALE / LAT_DECODE /
// LON_SCALE / LON_DECODE because the IEEE-754 arithmetic is the
// same.
var (
	latScale  = float64(uint64(1)<<GeoEncodingBits) / 180.0
	latDecode = 1.0 / latScale
	lonScale  = float64(uint64(1)<<GeoEncodingBits) / 360.0
	lonDecode = 1.0 / lonScale
)

// MinLonEncoded / MaxLonEncoded are the encoded forms of the
// longitude bounds, mirroring Java's MIN_LON_ENCODED /
// MAX_LON_ENCODED constants (resolved at package init from
// EncodeLongitude(MinLonIncl) / EncodeLongitude(MaxLonIncl)).
var (
	MinLonEncoded = EncodeLongitude(MinLonIncl)
	MaxLonEncoded = EncodeLongitude(MaxLonIncl)
)

// EncodeLatitude quantises a latitude into a 32-bit signed integer
// (rounding toward -90). The input is validated against the
// inclusive latitude bounds; on out-of-range input the function
// panics with the same Java-style error CheckLatitude returns.
//
// The Java reference uses a `throws IllegalArgumentException` style
// signature here, but mirroring that as a returned error would break
// every caller (these helpers are called from constructors and hot
// query loops). The Go port panics on invalid input, matching the
// effective semantics in Lucene where the exception is unrecoverable
// in practice.
func EncodeLatitude(latitude float64) int32 {
	if err := CheckLatitude(latitude); err != nil {
		panic(err)
	}
	// The maximum +90 cannot be represented without overflow; nudge
	// it down by one ULP, matching Math.nextDown.
	if latitude == 90.0 {
		latitude = math.Nextafter(latitude, math.Inf(-1))
	}
	return int32(math.Floor(latitude / latDecode))
}

// EncodeLatitudeCeil quantises a latitude into a 32-bit signed
// integer (rounding toward +90). Mirrors encodeLatitudeCeil.
func EncodeLatitudeCeil(latitude float64) int32 {
	if err := CheckLatitude(latitude); err != nil {
		panic(err)
	}
	if latitude == 90.0 {
		latitude = math.Nextafter(latitude, math.Inf(-1))
	}
	return int32(math.Ceil(latitude / latDecode))
}

// EncodeLongitude quantises a longitude into a 32-bit signed integer
// (rounding toward -180). Mirrors encodeLongitude.
func EncodeLongitude(longitude float64) int32 {
	if err := CheckLongitude(longitude); err != nil {
		panic(err)
	}
	if longitude == 180.0 {
		longitude = math.Nextafter(longitude, math.Inf(-1))
	}
	return int32(math.Floor(longitude / lonDecode))
}

// EncodeLongitudeCeil quantises a longitude into a 32-bit signed
// integer (rounding toward +180). Mirrors encodeLongitudeCeil.
func EncodeLongitudeCeil(longitude float64) int32 {
	if err := CheckLongitude(longitude); err != nil {
		panic(err)
	}
	if longitude == 180.0 {
		longitude = math.Nextafter(longitude, math.Inf(-1))
	}
	return int32(math.Ceil(longitude / lonDecode))
}

// DecodeLatitude reverses EncodeLatitude / EncodeLatitudeCeil. The
// returned value is in [-90, 90), matching Java's assertion that the
// decoded value is strictly less than MaxLatIncl.
func DecodeLatitude(encoded int32) float64 {
	return float64(encoded) * latDecode
}

// DecodeLatitudeBytes decodes a latitude from a 4-byte sortable
// big-endian representation, mirroring NumericUtils.sortableBytesToInt
// followed by DecodeLatitude.
func DecodeLatitudeBytes(src []byte, offset int) float64 {
	return DecodeLatitude(util.SortableBytesToInt(src, offset))
}

// DecodeLongitude reverses EncodeLongitude / EncodeLongitudeCeil.
// The returned value is in [-180, 180), matching Java.
func DecodeLongitude(encoded int32) float64 {
	return float64(encoded) * lonDecode
}

// DecodeLongitudeBytes decodes a longitude from a 4-byte sortable
// big-endian representation.
func DecodeLongitudeBytes(src []byte, offset int) float64 {
	return DecodeLongitude(util.SortableBytesToInt(src, offset))
}

// ----- DistancePredicate / Component2DPredicate -----

// gridARITY is the maximum side length of the sub-box grid. The Java
// constant is Grid.ARITY = 64.
const gridARITY = 64

// minInt32U is the unsigned representation of math.MinInt32. Java's
// reference uses `lat - Integer.MIN_VALUE` to shift the signed
// 32-bit space into the unsigned space starting at 0; Go does not
// allow the literal -2147483648 to be cast directly to uint32
// because constant evaluation happens before the cast. The named
// constant below sidesteps that by carrying the value as a uint32
// literal (0x8000_0000).
const minInt32U = uint32(0x8000_0000)

// grid is the per-shape sub-box decomposition shared by both
// DistancePredicate and Component2DPredicate.
type grid struct {
	latShift, lonShift     int
	latBase, lonBase       int
	maxLatDelta, maxLonDelta int
	relations              []byte // ordinal-per-cell, length = maxLatDelta * maxLonDelta
}

// DistancePredicate is a fast point-in-disk test that operates on
// the encoded representation of points. It is the Go port of
// GeoEncodingUtils.DistancePredicate.
type DistancePredicate struct {
	grid
	lat         float64
	lon         float64
	distanceKey float64
}

// CreateDistancePredicate builds the sub-box grid for a disk query
// of the given centre and radius. Mirrors the Java factory.
func CreateDistancePredicate(lat, lon, radiusMeters float64) DistancePredicate {
	bbox, err := FromPointDistance(lat, lon, radiusMeters)
	if err != nil {
		panic(err)
	}
	axis := AxisLat(lat, radiusMeters)
	key := DistanceQuerySortKey(radiusMeters)

	boxToRelation := func(b Rectangle) Relation {
		return Relate(b.MinLat(), b.MaxLat(), b.MinLon(), b.MaxLon(),
			lat, lon, key, axis)
	}
	g := createSubBoxes(bbox.MinLat(), bbox.MaxLat(), bbox.MinLon(), bbox.MaxLon(), boxToRelation)
	return DistancePredicate{grid: g, lat: lat, lon: lon, distanceKey: key}
}

// Test reports whether the encoded (lat, lon) point lies within the
// disk. Uses pre-computed sub-box relations to avoid a haversine
// computation in the common case where the sub-box is fully inside
// or outside the disk.
func (p *DistancePredicate) Test(lat, lon int32) bool {
	if p.maxLatDelta == 0 {
		return false
	}
	lat2 := int((uint32(lat) - minInt32U) >> uint(p.latShift))
	if lat2 < p.latBase || lat2-p.latBase >= p.maxLatDelta {
		return false
	}
	lon2 := int((uint32(lon) - minInt32U) >> uint(p.lonShift))
	if lon2 < p.lonBase {
		lon2 += 1 << uint(32-p.lonShift)
	}
	if lon2-p.lonBase >= p.maxLonDelta {
		return false
	}
	relation := Relation(p.relations[(lat2-p.latBase)*p.maxLonDelta+(lon2-p.lonBase)])
	if relation == CellCrossesQuery {
		return util.HaversinSortKey(DecodeLatitude(lat), DecodeLongitude(lon), p.lat, p.lon) <= p.distanceKey
	}
	return relation == CellInsideQuery
}

// Component2DPredicate is a fast point-in-shape test for an arbitrary
// Component2D, operating on the encoded representation of points.
// Mirrors the Java GeoEncodingUtils.Component2DPredicate.
type Component2DPredicate struct {
	grid
	tree Component2D
}

// CreateComponentPredicate builds the sub-box grid for a query
// shaped by an arbitrary Component2D.
func CreateComponentPredicate(tree Component2D) Component2DPredicate {
	boxToRelation := func(b Rectangle) Relation {
		return tree.Relate(b.MinLon(), b.MaxLon(), b.MinLat(), b.MaxLat())
	}
	g := createSubBoxes(tree.MinY(), tree.MaxY(), tree.MinX(), tree.MaxX(), boxToRelation)
	return Component2DPredicate{grid: g, tree: tree}
}

// Test reports whether the encoded (lat, lon) point lies within the
// component2D shape.
func (p *Component2DPredicate) Test(lat, lon int32) bool {
	if p.maxLatDelta == 0 {
		return false
	}
	lat2 := int((uint32(lat) - minInt32U) >> uint(p.latShift))
	if lat2 < p.latBase || lat2-p.latBase >= p.maxLatDelta {
		return false
	}
	lon2 := int((uint32(lon) - minInt32U) >> uint(p.lonShift))
	if lon2 < p.lonBase {
		lon2 += 1 << uint(32-p.lonShift)
	}
	if lon2-p.lonBase >= p.maxLonDelta {
		return false
	}
	relation := Relation(p.relations[(lat2-p.latBase)*p.maxLonDelta+(lon2-p.lonBase)])
	if relation == CellCrossesQuery {
		return p.tree.Contains(DecodeLongitude(lon), DecodeLatitude(lat))
	}
	return relation == CellInsideQuery
}

// createSubBoxes builds the per-shape grid of sub-boxes and their
// pre-computed relations to the query. Mirrors the Java private
// helper of the same name.
func createSubBoxes(shapeMinLat, shapeMaxLat, shapeMinLon, shapeMaxLon float64,
	boxToRelation func(Rectangle) Relation) grid {

	minLat := EncodeLatitudeCeil(shapeMinLat)
	maxLat := EncodeLatitude(shapeMaxLat)
	minLon := EncodeLongitudeCeil(shapeMinLon)
	maxLon := EncodeLongitude(shapeMaxLon)

	if maxLat < minLat || (shapeMaxLon >= shapeMinLon && maxLon < minLon) {
		// No quantised point can satisfy the query. Return a sentinel
		// grid whose Test method always returns false (maxLatDelta=0
		// makes the early-return on the Test path trip).
		return grid{}
	}

	// Lat axis.
	minLat2 := int64(minLat) - int64(math.MinInt32)
	maxLat2 := int64(maxLat) - int64(math.MinInt32)
	latShift := computeShift(minLat2, maxLat2)
	latBase := int(uint64(minLat2) >> uint(latShift))
	maxLatDelta := int(uint64(maxLat2)>>uint(latShift)) - latBase + 1

	// Lon axis.
	minLon2 := int64(minLon) - int64(math.MinInt32)
	maxLon2 := int64(maxLon) - int64(math.MinInt32)
	if shapeMaxLon < shapeMinLon {
		// Dateline crossing — extend the unsigned range past 2^32.
		maxLon2 += 1 << 32
	}
	lonShift := computeShift(minLon2, maxLon2)
	lonBase := int(uint64(minLon2) >> uint(lonShift))
	maxLonDelta := int(uint64(maxLon2)>>uint(lonShift)) - lonBase + 1

	relations := make([]byte, maxLatDelta*maxLonDelta)
	for i := 0; i < maxLatDelta; i++ {
		for j := 0; j < maxLonDelta; j++ {
			boxMinLat := int32(((int64(latBase)+int64(i))<<uint(latShift)) + int64(math.MinInt32))
			boxMinLon := int32(((int64(lonBase)+int64(j))<<uint(lonShift)) + int64(math.MinInt32))
			boxMaxLat := int32(int64(boxMinLat) + int64((1<<uint(latShift))-1))
			boxMaxLon := int32(int64(boxMinLon) + int64((1<<uint(lonShift))-1))

			rect, err := NewRectangle(
				DecodeLatitude(boxMinLat), DecodeLatitude(boxMaxLat),
				DecodeLongitude(boxMinLon), DecodeLongitude(boxMaxLon))
			if err != nil {
				panic(fmt.Errorf("geo: createSubBoxes built invalid box: %w", err))
			}
			relations[i*maxLonDelta+j] = byte(boxToRelation(rect))
		}
	}
	return grid{
		latShift: latShift, lonShift: lonShift,
		latBase: latBase, lonBase: lonBase,
		maxLatDelta: maxLatDelta, maxLonDelta: maxLonDelta,
		relations: relations,
	}
}

// computeShift returns the smallest shift such that
// `(b>>>shift) - (a>>>shift) < gridARITY`. The minimum shift is 1 so
// that the sign bit is cleared, allowing the same unsigned
// comparison logic regardless of the input's sign.
func computeShift(a, b int64) int {
	for shift := 1; ; shift++ {
		delta := (uint64(b) >> uint(shift)) - (uint64(a) >> uint(shift))
		if delta < gridARITY {
			return shift
		}
	}
}
