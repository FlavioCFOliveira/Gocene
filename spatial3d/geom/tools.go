// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import "math"

// SafeAcos is a numerically robust arc-cosine that clamps the input to [-1, 1].
//
// Port of org.apache.lucene.spatial3d.geom.Tools.safeAcos.
func SafeAcos(value float64) float64 {
	if value > 1.0 {
		value = 1.0
	} else if value < -1.0 {
		value = -1.0
	}
	return math.Acos(value)
}
