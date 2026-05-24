package spatial4j

import (
	"encoding/binary"
	"math"
)

func sin2(x float64) float64   { s := math.Sin(x); return s * s }
func cosDeg(x float64) float64 { return math.Cos(x * 0.017453292519943295) }
func asin(x float64) float64   { return math.Asin(x) }
func sqrt(x float64) float64   { return math.Sqrt(x) }

func packFloats(tag byte, vs ...float64) []byte {
	out := make([]byte, 1+8*len(vs))
	out[0] = tag
	for i, v := range vs {
		binary.BigEndian.PutUint64(out[1+i*8:], math.Float64bits(v))
	}
	return out
}
