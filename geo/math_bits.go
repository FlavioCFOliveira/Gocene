// Helpers that mirror small slices of java.lang.Math and
// java.lang.Double on top of Go's math package, kept in one place so
// the surrounding port code stays import-light.

package geo

import "math"

// mathFloat64bits is the Java equivalent of
// Double.doubleToRawLongBits(double) cast to uint64.
func mathFloat64bits(v float64) uint64 { return math.Float64bits(v) }
