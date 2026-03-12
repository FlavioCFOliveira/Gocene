// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

// DocValuesMetadata holds metadata about DocValues.
type DocValuesMetadata struct {
	Type         DocValuesType
	NumDocs      int
	UniqueValues int
	MinValue     int64
	MaxValue     int64
}
