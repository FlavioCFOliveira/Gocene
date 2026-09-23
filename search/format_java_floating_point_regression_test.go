// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package search

// This file pins the Double.toString / Float.toString rendering the
// {Double,Float}RangeSlowRangeQuery toString uses (Arrays.toString): Go's %g
// printed "296" for Java's "296.0" and "1e+07" for Java's "1.0E7".

import (
	"math"
	"testing"
)

func TestFormatJavaFloatingPointRegression(t *testing.T) {
	for _, c := range []struct {
		v       float64
		bitSize int
		want    string
	}{
		{296.0, 64, "296.0"},
		{float64(float32(512.4)), 64, "512.4000244140625"},
		{1e7, 64, "1.0E7"},
		{12345678.9, 64, "1.23456789E7"},
		{0.001, 64, "0.001"},
		{1.0e-4, 64, "1.0E-4"},
		{-0.0015, 64, "-0.0015"},
		{math.Copysign(0, -1), 64, "-0.0"},
		{0, 64, "0.0"},
		{math.Inf(-1), 64, "-Infinity"},
		{float64(float32(1.0) / 3), 32, "0.33333334"},
		{float64(float32(11.0)), 32, "11.0"},
		{float64(float32(3.7)), 32, "3.7"},
	} {
		if got := formatJavaFloatingPoint(c.v, c.bitSize); got != c.want {
			t.Errorf("formatJavaFloatingPoint(%v, %d) = %q, want %q", c.v, c.bitSize, got, c.want)
		}
	}
}
