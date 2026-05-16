// Tests for XYEncodingUtils. Lucene 10.4.0 ships no
// TestXYEncodingUtils.java peer; behavioural coverage is provided
// indirectly through the XY shape tests. The tests below verify the
// validation and encode/decode round-trip directly.

package geo

import (
	"errors"
	"math"
	"testing"
)

func TestXYEncodingUtils_CheckValAcceptsFiniteFloats(t *testing.T) {
	t.Parallel()
	for _, v := range []float32{0, 1, -1, math.MaxFloat32, -math.MaxFloat32, 0.5} {
		if _, err := XYCheckVal(v); err != nil {
			t.Errorf("XYCheckVal(%v) = %v; want nil", v, err)
		}
	}
}

func TestXYEncodingUtils_CheckValRejectsNonFinite(t *testing.T) {
	t.Parallel()
	for _, v := range []float32{float32(math.NaN()), float32(math.Inf(1)), float32(math.Inf(-1))} {
		_, err := XYCheckVal(v)
		if err == nil {
			t.Errorf("XYCheckVal(%v) = nil; want error", v)
		}
		if !errors.Is(err, ErrInvalidXYValue) {
			t.Errorf("XYCheckVal(%v) = %v; want wrap ErrInvalidXYValue", v, err)
		}
	}
}

func TestXYEncodingUtils_EncodeDecodeRoundTrip(t *testing.T) {
	t.Parallel()
	for _, v := range []float32{0, 1, -1, 1e10, -1e10, 1e-10, math.MaxFloat32} {
		enc := XYEncode(v)
		dec := XYDecode(enc)
		if dec != v {
			t.Errorf("round-trip %v -> %d -> %v", v, enc, dec)
		}
	}
}

func TestXYEncodingUtils_FloatArrayToDoubleArray(t *testing.T) {
	t.Parallel()
	in := []float32{1.5, -2.5, 0}
	out := XYFloatArrayToDoubleArray(in)
	want := []float64{1.5, -2.5, 0}
	for i, v := range out {
		if v != want[i] {
			t.Errorf("[%d] = %v; want %v", i, v, want[i])
		}
	}
	// Ensure independence: mutating the input doesn't affect the
	// output.
	in[0] = 999
	if out[0] == 999 {
		t.Error("XYFloatArrayToDoubleArray did not deep-copy")
	}
}

func TestXYEncodingUtils_EncodePanicsOnInvalid(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Error("XYEncode(NaN) should panic")
		}
	}()
	XYEncode(float32(math.NaN()))
}
