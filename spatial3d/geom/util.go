// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package geom

import "strconv"

// fmtFloat formats a float64 for human-readable output.
func fmtFloat(f float64) string { return strconv.FormatFloat(f, 'g', -1, 64) }
