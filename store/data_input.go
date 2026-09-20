// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package store

import "github.com/FlavioCFOliveira/Gocene/spi"

// DataInput is a port of org.apache.lucene.store.DataInput.
type DataInput = spi.DataInput

// DataInputCore represents the methods that must be implemented by the underlying storage.
type DataInputCore = spi.DataInputCore


// ReadVInt reads a variable-length integer from the input.
func ReadVInt(in DataInput) (int32, error) {
	return in.ReadVInt()
}
