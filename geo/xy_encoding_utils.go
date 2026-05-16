// Code in this file mirrors org.apache.lucene.geo.XYEncodingUtils
// from Apache Lucene 10.4.0.

package geo

import (
	"errors"
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// XY value bounds. Lucene defines these as [-Float.MAX_VALUE,
// +Float.MAX_VALUE]; Go's math.MaxFloat32 is the same value.
const (
	// XYMinValIncl is the inclusive minimum cartesian value.
	XYMinValIncl = float64(-math.MaxFloat32)

	// XYMaxValIncl is the inclusive maximum cartesian value.
	XYMaxValIncl = float64(math.MaxFloat32)
)

// ErrInvalidXYValue is wrapped by every validation error from
// XYEncodingUtils so callers can errors.Is to detect the case.
var ErrInvalidXYValue = errors.New("geo: invalid XY value")

// XYCheckVal validates that x is a finite float32 (not NaN, not Inf)
// and within [XYMinValIncl, XYMaxValIncl]. On invalid input it
// returns an error whose message matches the Java
// IllegalArgumentException format ("invalid value X; must be
// between -<MAX> and <MAX>").
func XYCheckVal(x float32) (float32, error) {
	if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
		return 0, fmt.Errorf("%w %s; must be between %s and %s",
			ErrInvalidXYValue,
			formatJavaDouble(float64(x)),
			formatJavaDouble(XYMinValIncl),
			formatJavaDouble(XYMaxValIncl))
	}
	return x, nil
}

// XYEncode quantises a float32 value into a 32-bit sortable integer
// via NumericUtils.floatToSortableInt. Validates the input via
// XYCheckVal; panics on invalid input (callers in hot codec loops
// stay allocation-free).
func XYEncode(x float32) int32 {
	if _, err := XYCheckVal(x); err != nil {
		panic(err)
	}
	return util.FloatToSortableInt(x)
}

// XYDecode reverses XYEncode.
func XYDecode(encoded int32) float32 {
	return util.SortableIntToFloat(encoded)
}

// XYDecodeBytes decodes from a 4-byte sortable representation.
func XYDecodeBytes(src []byte, offset int) float32 {
	return XYDecode(util.SortableBytesToInt(src, offset))
}

// XYFloatArrayToDoubleArray widens a float32 slice to float64. The
// returned slice is independent of the input. Mirrors Java's static
// helper of the same name.
func XYFloatArrayToDoubleArray(f []float32) []float64 {
	d := make([]float64, len(f))
	for i, v := range f {
		d[i] = float64(v)
	}
	return d
}
