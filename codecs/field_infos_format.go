// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"github.com/FlavioCFOliveira/Gocene/spi"
)

// FieldInfosFormat is an alias of spi.FieldInfosFormat.
type FieldInfosFormat = spi.FieldInfosFormat

// Lucene104FieldInfosFormat is the codec wrapper used by the Lucene 10.4
// codec; the wire format itself is unchanged from Lucene 9.4 so we embed
// Lucene94FieldInfosFormat and only override the codec name.
type Lucene104FieldInfosFormat struct {
	*Lucene94FieldInfosFormat
}

// NewLucene104FieldInfosFormat returns a fresh Lucene104FieldInfosFormat.
func NewLucene104FieldInfosFormat() *Lucene104FieldInfosFormat {
	return &Lucene104FieldInfosFormat{
		Lucene94FieldInfosFormat: NewLucene94FieldInfosFormat(),
	}
}

// Name returns the canonical codec name for the Lucene 10.4 wrapper.
func (f *Lucene104FieldInfosFormat) Name() string {
	return "Lucene104FieldInfosFormat"
}
