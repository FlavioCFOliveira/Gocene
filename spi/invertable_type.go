// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package spi

// InvertableType describes how a field is indexed.
// Mirrors org.apache.lucene.index.Field.InvertableType.
type InvertableType int

const (
	InvertableTypeTokenStream InvertableType = iota
	InvertableTypeNone
)
