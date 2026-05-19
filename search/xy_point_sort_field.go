// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

import (
	"errors"
	"fmt"
	"math"
)

// ErrXYPointSortFieldInvalidMissingValue mirrors the IllegalArgumentException
// raised by Lucene's XYPointSortField.setMissingValue when called with any
// value other than Double.POSITIVE_INFINITY. The sentinel lets callers
// differentiate this specific contract violation from other input errors.
var ErrXYPointSortFieldInvalidMissingValue = errors.New(
	"missing value can only be +Inf (missing values last)",
)

// XYPointSortField sorts hits by Euclidean distance from a cartesian origin.
// It is the Go port of the package-private
// org.apache.lucene.document.XYPointSortField; the type lives in the search
// package because Gocene's document/search split keeps comparators next to
// SortField (the Java original is package-private inside document/ only
// because of Java's visibility model).
//
// Distance sorting is always ascending (closest first), with missing values
// treated as +Inf so documents without the field land last. The embedded
// *SortField carries the field name and Type=Custom.
//
// Coordinates are float32 to match Lucene's XYPoint surface. Validation
// rejects only NaN/Inf — the cartesian plane is unbounded, so any finite
// float32 is a legal origin.
//
// Concurrency: an XYPointSortField is read-only after construction. The
// per-query comparator instances returned by GetComparator are not safe for
// concurrent use.
type XYPointSortField struct {
	*SortField

	x float32
	y float32
}

// NewXYPointSortField creates an XYPointSortField centred on (originX, originY).
// Both coordinates must be finite (no NaN, no ±Inf); an empty field or a
// non-finite coordinate is rejected with the same error text Lucene would
// raise from XYEncodingUtils.checkVal.
//
// The constructor wires:
//
//   - Type     -> SortFieldTypeCustom
//   - Reverse  -> false (ascending: closer documents win)
//   - Missing  -> MissingValueLast (mirrors the +Inf default)
//   - MissingValue -> +Inf
func NewXYPointSortField(field string, originX, originY float32) (*XYPointSortField, error) {
	if field == "" {
		return nil, errors.New("field must not be empty")
	}
	if err := checkXYValue(originX); err != nil {
		return nil, err
	}
	if err := checkXYValue(originY); err != nil {
		return nil, err
	}
	sf := NewSortField(field, SortFieldTypeCustom)
	sf.Reverse = false
	sf.Missing = MissingValueLast
	sf.MissingValue = math.Inf(1)
	return &XYPointSortField{
		SortField: sf,
		x:         originX,
		y:         originY,
	}, nil
}

// X returns the origin x coordinate.
func (sf *XYPointSortField) X() float32 { return sf.x }

// Y returns the origin y coordinate.
func (sf *XYPointSortField) Y() float32 { return sf.y }

// GetMissingValue returns the configured missing-value sentinel as a float64.
// Mirrors the typed return of the Java override; the embedded SortField stores
// the value as interface{} for the generic surface.
func (sf *XYPointSortField) GetMissingValue() float64 {
	if sf.SortField == nil || sf.SortField.MissingValue == nil {
		return math.Inf(1)
	}
	if v, ok := sf.SortField.MissingValue.(float64); ok {
		return v
	}
	return math.Inf(1)
}

// SetMissingValue accepts only +Inf, mirroring Lucene's contract. Any other
// value returns ErrXYPointSortFieldInvalidMissingValue; the embedded
// SortField is not mutated on rejection.
func (sf *XYPointSortField) SetMissingValue(value interface{}) error {
	v, ok := value.(float64)
	if !ok || !math.IsInf(v, 1) {
		return ErrXYPointSortFieldInvalidMissingValue
	}
	sf.SortField.MissingValue = v
	return nil
}

// GetComparator returns a fresh XYPointDistanceComparator sized for numHits
// priority-queue slots. The pruning parameter is accepted for parity with
// Lucene's signature; the comparator does not currently exploit pruning
// hints, matching the reference implementation.
func (sf *XYPointSortField) GetComparator(numHits int, pruning Pruning) *XYPointDistanceComparator {
	_ = pruning
	return NewXYPointDistanceComparator(sf.SortField.Field, sf.x, sf.y, numHits)
}

// Equals reports whether other is an XYPointSortField with the same field
// name, type, reverse flag, and origin coordinates. Coordinate equality uses
// bit-equality (math.Float32bits) to mirror Java's Float.floatToIntBits
// semantics — two NaNs compare equal, and +0.0 differs from -0.0.
func (sf *XYPointSortField) Equals(other *XYPointSortField) bool {
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
	if math.Float32bits(sf.x) != math.Float32bits(other.x) {
		return false
	}
	if math.Float32bits(sf.y) != math.Float32bits(other.y) {
		return false
	}
	return true
}

// HashCode mirrors the Java implementation: prime-31 chaining over the parent
// SortField surface plus the two coordinate floatToIntBits values. The helper
// makes the type usable as a Sort key in deduplication paths.
func (sf *XYPointSortField) HashCode() int {
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
	// Java: temp = Float.floatToIntBits(x); result = prime*result + (int)(temp ^ (temp >>> 32))
	// Float bits fit in 32, so the XOR with the high half is a no-op; the int
	// cast keeps the Java-equivalent low 32 bits.
	xBits := uint64(math.Float32bits(sf.x))
	h = prime*h + int(int32(xBits^(xBits>>32)))
	yBits := uint64(math.Float32bits(sf.y))
	h = prime*h + int(int32(yBits^(yBits>>32)))
	return h
}

// String renders the Lucene-equivalent toString representation. The
// missingValue suffix appears only when the configured sentinel diverges from
// the +Inf default, matching the Java reference.
func (sf *XYPointSortField) String() string {
	field := ""
	if sf.SortField != nil {
		field = sf.SortField.Field
	}
	missing := sf.GetMissingValue()
	if math.IsInf(missing, 1) {
		return fmt.Sprintf(`<distance:"%s" x=%s y=%s>`,
			field, formatJavaFloat(sf.x), formatJavaFloat(sf.y))
	}
	return fmt.Sprintf(`<distance:"%s" x=%s y=%s missingValue=%s>`,
		field, formatJavaFloat(sf.x), formatJavaFloat(sf.y),
		formatJavaDouble(missing))
}

// checkXYValue mirrors XYEncodingUtils.checkVal: a value must be finite (not
// NaN, not ±Inf). The cartesian plane has no fixed bounds beyond the float32
// representable range, so finiteness is the only contract.
func checkXYValue(v float32) error {
	if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
		return fmt.Errorf("invalid value %v; must be between %v and %v",
			v, -math.MaxFloat32, math.MaxFloat32)
	}
	return nil
}

// formatJavaFloat renders a float32 using Java's Float.toString convention
// closely enough to match Lucene's toString output. Integral values get a
// trailing ".0"; +Inf, -Inf and NaN map to their Java textual forms. The
// helper is float32-typed so callers do not silently widen and pick up
// double-precision tail digits.
func formatJavaFloat(v float32) string {
	f := float64(v)
	if math.IsNaN(f) {
		return "NaN"
	}
	if math.IsInf(f, 1) {
		return "Infinity"
	}
	if math.IsInf(f, -1) {
		return "-Infinity"
	}
	if f == math.Trunc(f) && math.Abs(f) < 1e16 {
		return fmt.Sprintf("%.1f", f)
	}
	// Round-trip the float32 with its native precision rather than the wider
	// float64 default, so toString output mirrors Java's Float.toString.
	return fmt.Sprintf("%g", f)
}
