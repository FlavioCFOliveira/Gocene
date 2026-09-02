// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"time"
)

// Now returns the current time.
func Now() time.Time {
	return time.Now()
}

// Since returns the time elapsed since the given time.
func Since(t time.Time) time.Duration {
	return time.Since(t)
}
