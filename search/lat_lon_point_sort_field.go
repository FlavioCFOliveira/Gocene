// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/geo"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// ErrLatLonPointSortFieldInvalidMissingValue mirrors the IllegalArgumentException
// raised by Lucene's LatLonPointSortField.setMissingValue when called with any
// value other than Double.POSITIVE_INFINITY. The sentinel lets callers
// differentiate this specific contract violation from other input errors.
var ErrLatLonPointSortFieldInvalidMissingValue = errors.New(
	"missing value can only be +Inf (missing values last)",
)

// LatLonPointSortField sorts hits by Haversine distance from an origin point.
// It is the Go port of the package-private
// org.apache.lucene.document.LatLonPointSortField; the type lives in the
// search package because Gocene's document/search split keeps comparators
// next to SortField (the Java original is package-private inside document/
// only because of Java's visibility model).
//
// Distance sorting is always ascending (closest first), with missing values
// treated as +Inf so documents without the field land last. The embedded
// *SortField carries the field name and Type=Custom.
//
// Concurrency: a LatLonPointSortField is read-only after construction. The
// per-query comparator instances returned by GetComparator are not safe for
// concurrent use.
type LatLonPointSortField struct {
	*SortField

	latitude  float64
	longitude float64
}

// NewLatLonPointSortField creates a LatLonPointSortField centred on
// (originLat, originLon). Both coordinates are validated via
// geo.CheckLatitude / geo.CheckLongitude; an empty field or an out-of-range
// coordinate is rejected with the same error text Lucene would raise.
//
// The constructor wires:
//
//   - Type     -> SortFieldTypeCustom
//   - Reverse  -> false (ascending: closer documents win)
//   - Missing  -> MissingValueLast (mirrors the +Inf default)
//   - MissingValue -> +Inf
func NewLatLonPointSortField(field string, originLat, originLon float64) (*LatLonPointSortField, error) {
	if field == "" {
		return nil, errors.New("field must not be empty")
	}
	if err := geo.CheckLatitude(originLat); err != nil {
		return nil, err
	}
	if err := geo.CheckLongitude(originLon); err != nil {
		return nil, err
	}
	sf := NewSortField(field, SortFieldTypeCustom)
	sf.Reverse = false
	sf.Missing = MissingValueLast
	sf.MissingValue = math.Inf(1)
	return &LatLonPointSortField{
		SortField: sf,
		latitude:  originLat,
		longitude: originLon,
	}, nil
}

// Latitude returns the origin latitude in decimal degrees.
func (sf *LatLonPointSortField) Latitude() float64 { return sf.latitude }

// Longitude returns the origin longitude in decimal degrees.
func (sf *LatLonPointSortField) Longitude() float64 { return sf.longitude }

// GetMissingValue returns the configured missing-value sentinel as a float64.
// Mirrors the typed return of the Java override; the embedded SortField stores
// the value as interface{} for the generic surface.
func (sf *LatLonPointSortField) GetMissingValue() float64 {
	if sf.SortField == nil || sf.SortField.MissingValue == nil {
		return math.Inf(1)
	}
	if v, ok := sf.SortField.MissingValue.(float64); ok {
		return v
	}
	return math.Inf(1)
}

// SetMissingValue accepts only +Inf, mirroring Lucene's contract. Any other
// value returns ErrLatLonPointSortFieldInvalidMissingValue; the embedded
// SortField is not mutated on rejection.
func (sf *LatLonPointSortField) SetMissingValue(value interface{}) error {
	v, ok := value.(float64)
	if !ok || !math.IsInf(v, 1) {
		return ErrLatLonPointSortFieldInvalidMissingValue
	}
	sf.SortField.MissingValue = v
	return nil
}

// GetComparator returns a fresh LatLonPointDistanceComparator sized for
// numHits priority-queue slots. The pruning parameter is accepted for parity
// with Lucene's signature; the comparator does not currently exploit pruning
// hints, matching the reference implementation.
func (sf *LatLonPointSortField) GetComparator(numHits int, pruning Pruning) *LatLonPointDistanceComparator {
	_ = pruning
	return NewLatLonPointDistanceComparator(sf.SortField.Field, sf.latitude, sf.longitude, numHits)
}

// Equals reports whether other is a LatLonPointSortField with the same field
// name, type, reverse flag, and origin coordinates. Coordinate equality uses
// bit-equality (math.Float64bits) to mirror Java's Double.doubleToLongBits
// semantics — two NaNs compare equal, and +0.0 differs from -0.0.
func (sf *LatLonPointSortField) Equals(other *LatLonPointSortField) bool {
	if sf == other {
		return true
	}
	if sf == nil || other == nil {
		return false
	}
	if sf.SortField == nil || other.SortField == nil {
		return false
	}
	if sf.SortField.Field != other.SortField.Field ||
		sf.SortField.Type != other.SortField.Type ||
		sf.SortField.Reverse != other.SortField.Reverse {
		return false
	}
	if math.Float64bits(sf.latitude) != math.Float64bits(other.latitude) {
		return false
	}
	if math.Float64bits(sf.longitude) != math.Float64bits(other.longitude) {
		return false
	}
	return true
}

// HashCode mirrors the Java implementation: prime-31 chaining over the parent
// SortField surface plus the two coordinate doubleToLongBits values. The
// helper makes the type usable as a Sort key in deduplication paths.
func (sf *LatLonPointSortField) HashCode() int {
	const prime = 31
	h := 0
	if sf.SortField != nil {
		for i := 0; i < len(sf.SortField.Field); i++ {
			h = prime*h + int(sf.SortField.Field[i])
		}
		h = prime*h + int(sf.SortField.Type)
		if sf.SortField.Reverse {
			h = prime*h + 1
		} else {
			h = prime * h
		}
	}
	latBits := math.Float64bits(sf.latitude)
	h = prime*h + int(int32(latBits^(latBits>>32)))
	lonBits := math.Float64bits(sf.longitude)
	h = prime*h + int(int32(lonBits^(lonBits>>32)))
	return h
}

// String renders the Lucene-equivalent toString representation. The
// missingValue suffix appears only when the configured sentinel diverges from
// the +Inf default, matching the Java reference.
func (sf *LatLonPointSortField) String() string {
	field := ""
	if sf.SortField != nil {
		field = sf.SortField.Field
	}
	missing := sf.GetMissingValue()
	if math.IsInf(missing, 1) {
		return fmt.Sprintf(`<distance:"%s" latitude=%s longitude=%s>`,
			field, formatJavaDouble(sf.latitude), formatJavaDouble(sf.longitude))
	}
	return fmt.Sprintf(`<distance:"%s" latitude=%s longitude=%s missingValue=%s>`,
		field, formatJavaDouble(sf.latitude), formatJavaDouble(sf.longitude),
		formatJavaDouble(missing))
}

// LatLonPointDistanceComparator compares documents by Haversine distance from
// an origin point. It mirrors the package-private
// org.apache.lucene.document.LatLonPointDistanceComparator and uses the
// SortedNumericDocValues stream produced by LatLonDocValuesField (upper 32
// bits = encoded latitude, lower 32 bits = encoded longitude).
//
// The comparator caches a pre-encoded bounding box derived from the current
// bottom-of-queue distance; uncompetitive candidates are rejected on the
// integer-encoded coordinates without paying the Haversine cost. After 1024
// setBottom calls the box is refreshed only every 64 calls, matching the
// adversarial-input guard in Lucene.
//
// Concurrency: not safe for concurrent use; TopFieldCollector owns one
// instance per slice.
type LatLonPointDistanceComparator struct {
	field     string
	latitude  float64
	longitude float64

	values   []float64
	bottom   float64
	topValue float64

	currentDocs index.SortedNumericDocValues

	// Pre-encoded bounding box for the bottom distance on the priority queue.
	// minLon2 carries the second half of a dateline-crossing box; when the box
	// does not cross, minLon2 is MaxInt32 to disable the comparison.
	minLon  int32
	maxLon  int32
	minLat  int32
	maxLat  int32
	minLon2 int32

	// setBottomCounter counts setBottom invocations for the adversary guard.
	setBottomCounter int

	// currentValues caches the decoded values for the current docID, so that
	// CompareBottom and the sortKey helper do not refetch.
	currentValues []int64
	valuesDocID   int
}

// NewLatLonPointDistanceComparator builds a comparator backed by numHits slots
// targeting the SortedNumericDocValues stream of field.
func NewLatLonPointDistanceComparator(field string, latitude, longitude float64, numHits int) *LatLonPointDistanceComparator {
	return &LatLonPointDistanceComparator{
		field:         field,
		latitude:      latitude,
		longitude:     longitude,
		values:        make([]float64, numHits),
		minLon:        math.MinInt32,
		maxLon:        math.MaxInt32,
		minLat:        math.MinInt32,
		maxLat:        math.MaxInt32,
		minLon2:       math.MaxInt32,
		currentValues: make([]int64, 0, 4),
		valuesDocID:   -1,
	}
}

// SetScorer is a no-op, matching the Java reference.
func (c *LatLonPointDistanceComparator) SetScorer(_ Scorable) error { return nil }

// Compare returns the natural ordering of the two slots' sort keys. The keys
// are Haversine sort keys, so a smaller value means closer to the origin.
func (c *LatLonPointDistanceComparator) Compare(slot1, slot2 int) int {
	return compareFloat64(c.values[slot1], c.values[slot2])
}

// SetBottom records the slot of the queue's weakest entry and refreshes the
// pre-encoded competitive bounding box. The bounding box is rebuilt on every
// call for the first 1024 invocations, then every 64 calls thereafter, in
// line with the Java adversary guard.
func (c *LatLonPointDistanceComparator) SetBottom(slot int) error {
	c.bottom = c.values[slot]
	if c.setBottomCounter < 1024 || (c.setBottomCounter&0x3F) == 0x3F {
		box, err := geo.FromPointDistance(c.latitude, c.longitude, util.HaversinMetersFromSortKey(c.bottom))
		if err == nil {
			c.minLat = geo.EncodeLatitude(box.MinLat())
			c.maxLat = geo.EncodeLatitude(box.MaxLat())
			if box.CrossesDateline() {
				c.minLon = math.MinInt32
				c.maxLon = geo.EncodeLongitude(box.MaxLon())
				c.minLon2 = geo.EncodeLongitude(box.MinLon())
			} else {
				c.minLon = geo.EncodeLongitude(box.MinLon())
				c.maxLon = geo.EncodeLongitude(box.MaxLon())
				c.minLon2 = math.MaxInt32
			}
		}
	}
	c.setBottomCounter++
	return nil
}

// SetTopValue stores the value used as the top reference for CompareTop. The
// value is interpreted in metres, matching Lucene.
func (c *LatLonPointDistanceComparator) SetTopValue(value float64) {
	c.topValue = value
}

// Value returns the distance in metres for the document stored in slot. The
// internal slot value is a Haversine sort key, so we convert via
// HaversinMetersFromSortKey, mirroring the Java haversin2 helper.
func (c *LatLonPointDistanceComparator) Value(slot int) float64 {
	v := c.values[slot]
	if math.IsInf(v, 1) {
		return v
	}
	return util.HaversinMetersFromSortKey(v)
}

// CompareBottom returns a positive integer when doc is more competitive than
// the current bottom, zero on tie, and a negative value when doc cannot
// improve the queue. The implementation mirrors the Java logic: bounding-box
// rejection first, then exact Haversine sort-key comparison on survivors.
func (c *LatLonPointDistanceComparator) CompareBottom(doc int) (int, error) {
	if c.currentDocs == nil {
		return compareFloat64(c.bottom, math.Inf(1)), nil
	}
	if doc > c.currentDocs.DocID() {
		if _, err := c.currentDocs.Advance(doc); err != nil {
			return 0, fmt.Errorf("lat lon sort: advance to %d: %w", doc, err)
		}
	}
	if doc < c.currentDocs.DocID() {
		return compareFloat64(c.bottom, math.Inf(1)), nil
	}
	if err := c.setValues(); err != nil {
		return 0, err
	}
	cmp := -1
	for _, encoded := range c.currentValues {
		latBits := int32(encoded >> 32)
		if latBits < c.minLat || latBits > c.maxLat {
			continue
		}
		lonBits := int32(encoded)
		if (lonBits < c.minLon || lonBits > c.maxLon) && lonBits < c.minLon2 {
			continue
		}
		docLat := geo.DecodeLatitude(latBits)
		docLon := geo.DecodeLongitude(lonBits)
		thisCmp := compareFloat64(c.bottom, util.HaversinSortKey(c.latitude, c.longitude, docLat, docLon))
		if thisCmp > cmp {
			cmp = thisCmp
		}
		if cmp > 0 {
			return cmp, nil
		}
	}
	return cmp, nil
}

// Copy stores the distance sort key of doc into slot, computed across all of
// the document's multi-valued points (the minimum wins).
func (c *LatLonPointDistanceComparator) Copy(slot, doc int) error {
	key, err := c.sortKey(doc)
	if err != nil {
		return err
	}
	c.values[slot] = key
	return nil
}

// CompareTop compares the top reference (in metres) against the distance to
// doc, returning a Java-style sign convention.
func (c *LatLonPointDistanceComparator) CompareTop(doc int) (int, error) {
	key, err := c.sortKey(doc)
	if err != nil {
		return 0, err
	}
	docMeters := key
	if !math.IsInf(key, 1) {
		docMeters = util.HaversinMetersFromSortKey(key)
	}
	return compareFloat64(c.topValue, docMeters), nil
}

// CompetitiveIterator returns nil — the comparator does not expose a
// competitive iterator optimisation, matching the Java reference.
func (c *LatLonPointDistanceComparator) CompetitiveIterator() (DocIdSetIterator, error) {
	return nil, nil
}

// SetHitsThresholdReached is a no-op for this comparator.
func (c *LatLonPointDistanceComparator) SetHitsThresholdReached() {}

// GetLeafComparator binds the comparator to ctx by resolving the
// SortedNumericDocValues stream for the configured field. The leaf reader is
// reached through a narrow type assertion (the LeafReaderContext exposes
// IndexReaderInterface, which does not declare the doc-values accessor); a
// reader without the surface falls back to an empty stream, mirroring
// Lucene's DocValues.getSortedNumeric null-defence path.
func (c *LatLonPointDistanceComparator) GetLeafComparator(ctx *index.LeafReaderContext) (*LatLonPointDistanceComparator, error) {
	if ctx == nil {
		return nil, errors.New("leaf reader context must not be nil")
	}
	type sortedNumericProvider interface {
		GetSortedNumericDocValues(field string) (index.SortedNumericDocValues, error)
	}
	c.valuesDocID = -1
	leaf := ctx.LeafReader()
	if leaf == nil {
		c.currentDocs = index.EmptySortedNumeric()
		return c, nil
	}
	provider, ok := leaf.(sortedNumericProvider)
	if !ok {
		c.currentDocs = index.EmptySortedNumeric()
		return c, nil
	}
	dv, err := provider.GetSortedNumericDocValues(c.field)
	if err != nil {
		return nil, fmt.Errorf("lat lon sort: read doc values %q: %w", c.field, err)
	}
	if dv == nil {
		c.currentDocs = index.EmptySortedNumeric()
	} else {
		c.currentDocs = dv
	}
	return c, nil
}

// setValues materialises the per-document multi-values into currentValues,
// re-fetching only when the docID changes. Mirrors the Java setValues hook
// that drains SortedNumericDocValues.nextValue into a long[] cache.
func (c *LatLonPointDistanceComparator) setValues() error {
	if c.currentDocs == nil {
		c.currentValues = c.currentValues[:0]
		return nil
	}
	docID := c.currentDocs.DocID()
	if c.valuesDocID == docID {
		return nil
	}
	c.valuesDocID = docID
	vals, err := c.currentDocs.Get(docID)
	if err != nil {
		return fmt.Errorf("lat lon sort: get values @doc=%d: %w", docID, err)
	}
	if cap(c.currentValues) < len(vals) {
		c.currentValues = make([]int64, len(vals))
	} else {
		c.currentValues = c.currentValues[:len(vals)]
	}
	copy(c.currentValues, vals)
	return nil
}

// sortKey returns the minimum Haversine sort key across the multi-valued
// points of doc; +Inf when doc has no values in the current leaf or the
// stream cannot be advanced to doc.
func (c *LatLonPointDistanceComparator) sortKey(doc int) (float64, error) {
	if c.currentDocs == nil {
		return math.Inf(1), nil
	}
	if doc > c.currentDocs.DocID() {
		if _, err := c.currentDocs.Advance(doc); err != nil {
			return 0, fmt.Errorf("lat lon sort: advance to %d: %w", doc, err)
		}
	}
	if doc != c.currentDocs.DocID() {
		return math.Inf(1), nil
	}
	if err := c.setValues(); err != nil {
		return 0, err
	}
	minValue := math.Inf(1)
	for _, encoded := range c.currentValues {
		docLat := geo.DecodeLatitude(int32(encoded >> 32))
		docLon := geo.DecodeLongitude(int32(encoded))
		if k := util.HaversinSortKey(c.latitude, c.longitude, docLat, docLon); k < minValue {
			minValue = k
		}
	}
	return minValue, nil
}

// compareFloat64 mirrors Java's Double.compare semantics: NaN is treated as
// strictly greater than any non-NaN value, and -0.0 is strictly less than
// +0.0. The helper is package-private so the SortField surface can stay
// terse.
func compareFloat64(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	aBits := math.Float64bits(a)
	bBits := math.Float64bits(b)
	if aBits == bBits {
		return 0
	}
	// At least one operand is NaN, or one is -0.0 and the other +0.0.
	aNaN := math.IsNaN(a)
	bNaN := math.IsNaN(b)
	switch {
	case aNaN && bNaN:
		return 0
	case aNaN:
		return 1
	case bNaN:
		return -1
	}
	// -0.0 < +0.0 per Double.compare.
	if a == 0 && b == 0 {
		if math.Signbit(a) && !math.Signbit(b) {
			return -1
		}
		if !math.Signbit(a) && math.Signbit(b) {
			return 1
		}
	}
	return 0
}

// formatJavaDouble renders a float64 using Java's Double.toString convention
// closely enough to match Lucene's toString output. Integral values get a
// trailing ".0"; +Inf, -Inf and NaN map to their Java textual forms.
func formatJavaDouble(v float64) string {
	if math.IsNaN(v) {
		return "NaN"
	}
	if math.IsInf(v, 1) {
		return "Infinity"
	}
	if math.IsInf(v, -1) {
		return "-Infinity"
	}
	if v == math.Trunc(v) && !math.IsInf(v, 0) && math.Abs(v) < 1e16 {
		return fmt.Sprintf("%.1f", v)
	}
	return strconvFormatFloatJava(v)
}

// strconvFormatFloatJava renders v with the shortest representation that
// round-trips, matching Go's strconv.FormatFloat 'g' verb with precision -1.
// Wrapped in a helper to keep formatJavaDouble readable.
func strconvFormatFloatJava(v float64) string {
	// %v with float64 already uses the shortest round-trip representation in
	// Go's fmt package; this mirrors strconv.FormatFloat(v, 'g', -1, 64).
	return fmt.Sprintf("%v", v)
}
