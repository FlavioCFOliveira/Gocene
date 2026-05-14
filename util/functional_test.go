// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"errors"
	"math"
	"testing"
)

// errSentinel is a shared error used by functional-interface tests to
// verify that error returns propagate through the func-type wrappers.
var errSentinel = errors.New("sentinel error")

// TestFloatToFloatFunction exercises the FloatToFloatFunction type as a
// drop-in replacement for the Java FunctionalInterface: literal,
// composition, identity, and special float values.
func TestFloatToFloatFunction(t *testing.T) {
	t.Run("identity", func(t *testing.T) {
		var id FloatToFloatFunction = func(v float32) float32 { return v }
		if got := id(1.5); got != 1.5 {
			t.Fatalf("identity(1.5) got %v want 1.5", got)
		}
	})

	t.Run("scale", func(t *testing.T) {
		var scale FloatToFloatFunction = func(v float32) float32 { return v * 2 }
		if got := scale(3); got != 6 {
			t.Fatalf("scale(3) got %v want 6", got)
		}
		if got := scale(-1.5); got != -3 {
			t.Fatalf("scale(-1.5) got %v want -3", got)
		}
	})

	t.Run("composition", func(t *testing.T) {
		var inc FloatToFloatFunction = func(v float32) float32 { return v + 1 }
		var sq FloatToFloatFunction = func(v float32) float32 { return v * v }
		composed := func(v float32) float32 { return sq(inc(v)) }
		if got := composed(3); got != 16 {
			t.Fatalf("composed(3) got %v want 16", got)
		}
	})

	t.Run("special values", func(t *testing.T) {
		var negate FloatToFloatFunction = func(v float32) float32 { return -v }
		if got := negate(float32(math.Inf(1))); !math.IsInf(float64(got), -1) {
			t.Fatalf("negate(+Inf) got %v want -Inf", got)
		}
		nan := float32(math.NaN())
		if got := negate(nan); !math.IsNaN(float64(got)) {
			t.Fatalf("negate(NaN) got %v want NaN", got)
		}
	})
}

func TestIOBooleanSupplier(t *testing.T) {
	t.Run("returns true with no error", func(t *testing.T) {
		var s IOBooleanSupplier = func() (bool, error) { return true, nil }
		v, err := s()
		if err != nil || !v {
			t.Fatalf("got (%v,%v) want (true,nil)", v, err)
		}
	})
	t.Run("propagates error", func(t *testing.T) {
		want := errSentinel
		var s IOBooleanSupplier = func() (bool, error) { return false, want }
		v, err := s()
		if err != want || v {
			t.Fatalf("got (%v,%v) want (false, errSentinel)", v, err)
		}
	})
}

func TestIOConsumer(t *testing.T) {
	t.Run("invocation captures value", func(t *testing.T) {
		var seen int
		var c IOConsumer[int] = func(v int) error {
			seen = v
			return nil
		}
		if err := c(42); err != nil {
			t.Fatalf("err: %v", err)
		}
		if seen != 42 {
			t.Fatalf("seen got %d want 42", seen)
		}
	})
	t.Run("propagates error", func(t *testing.T) {
		var c IOConsumer[string] = func(string) error { return errSentinel }
		if err := c("x"); err != errSentinel {
			t.Fatalf("got %v want errSentinel", err)
		}
	})
}

func TestIOFunction(t *testing.T) {
	t.Run("maps input", func(t *testing.T) {
		var f IOFunction[int, string] = func(v int) (string, error) {
			if v == 0 {
				return "", nil
			}
			return "x", nil
		}
		got, err := f(7)
		if err != nil || got != "x" {
			t.Fatalf("got (%q,%v) want (\"x\",nil)", got, err)
		}
	})
	t.Run("propagates error", func(t *testing.T) {
		var f IOFunction[int, int] = func(int) (int, error) { return 0, errSentinel }
		_, err := f(1)
		if err != errSentinel {
			t.Fatalf("got %v want errSentinel", err)
		}
	})
}

func TestIORunnable(t *testing.T) {
	t.Run("invocation runs", func(t *testing.T) {
		var ran bool
		var r IORunnable = func() error { ran = true; return nil }
		if err := r(); err != nil || !ran {
			t.Fatalf("got err=%v ran=%v", err, ran)
		}
	})
	t.Run("propagates error", func(t *testing.T) {
		var r IORunnable = func() error { return errSentinel }
		if err := r(); err != errSentinel {
			t.Fatalf("got %v want errSentinel", err)
		}
	})
}
