// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package index

import "github.com/FlavioCFOliveira/Gocene/spi"

// FieldDoc is a ScoreDoc which also contains information about how to sort the referenced document.
type FieldDoc = spi.FieldDoc

// NewFieldDoc creates a FieldDoc with empty sort information.
func NewFieldDoc(doc int, score float32) *FieldDoc {
	return spi.NewFieldDoc(doc, score)
}

// NewFieldDocWithFields creates a FieldDoc with the given sort information.
func NewFieldDocWithFields(doc int, score float32, fields []any) *FieldDoc {
	return spi.NewFieldDocWithFields(doc, score, fields)
}

// NewFieldDocWithShard creates a FieldDoc with the given sort information and shard index.
func NewFieldDocWithShard(doc int, score float32, fields []any, shardIndex int) *FieldDoc {
	return spi.NewFieldDocWithShard(doc, score, fields, shardIndex)
}
