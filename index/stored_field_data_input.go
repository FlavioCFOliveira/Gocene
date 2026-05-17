// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "github.com/FlavioCFOliveira/Gocene/store"

// StoredFieldDataInput pairs a length with the DataInput from which the
// stored-field bytes can be read. Mirrors
// org.apache.lucene.index.StoredFieldDataInput from Apache Lucene 10.4.0.
//
// Used by codecs that decode stored fields lazily — the visitor receives the
// triple (length, dataInput) and may consume exactly length bytes itself.
type StoredFieldDataInput struct {
	// Length is the number of bytes available for this stored field.
	Length int

	// In is the source DataInput positioned at the first byte of the field.
	In store.DataInput
}

// NewStoredFieldDataInput constructs a StoredFieldDataInput.
func NewStoredFieldDataInput(in store.DataInput, length int) *StoredFieldDataInput {
	return &StoredFieldDataInput{Length: length, In: in}
}
