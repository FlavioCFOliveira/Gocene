// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package suggest_test

import (
	"fmt"
	"math"
)

// Average holds an average and its standard deviation. Mirrors
// org.apache.lucene.search.suggest.Average.
type Average struct {
	Avg    float64
	Stddev float64
}

// String returns a human-readable representation. Mirrors Average.toString().
func (a Average) String() string {
	return fmt.Sprintf("%.0f [+- %.2f]", a.Avg, a.Stddev)
}

// AverageFrom computes Average from a slice of float64 samples. Mirrors
// Average.from(List<Double>).
func AverageFrom(values []float64) Average {
	if len(values) == 0 {
		return Average{}
	}
	var sum, sumSquares float64
	for _, v := range values {
		sum += v
		sumSquares += v * v
	}
	n := float64(len(values))
	avg := sum / n
	variance := sumSquares/n - avg*avg
	if variance < 0 {
		variance = 0
	}
	return Average{Avg: avg, Stddev: math.Sqrt(variance)}
}
