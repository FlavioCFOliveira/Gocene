// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package codecs

import (
	"github.com/FlavioCFOliveira/Gocene/index"
)

// Apache Lucene 10.5.0 has no BaseTermVectorsReader, TermVectorsReaderImpl or
// EmptyTermVectorsReader; the three types that used to live here were invented
// by this port. org.apache.lucene.codecs.TermVectorsReader is abstract in
// get(int), checkIntegrity(), clone() and close(), so there is no default body
// for a base type to carry, and giving one a checkIntegrity() that does nothing
// let an invented type satisfy the real interface while hiding that the port of
// the reader is missing. The reader Lucene actually uses is
// org.apache.lucene.codecs.lucene90.compressing.Lucene90CompressingTermVectorsReader
// (reached through Lucene90TermVectorsFormat, which is what Lucene104Codec
// returns from vectorsFormat()); Gocene's counterpart in
// codecs/lucene90/compressing is still an empty struct.

// TermVectorDocument represents term vectors for a document.
type TermVectorDocument struct {
	Fields map[string]index.Terms
}

// termVectorFields implements index.Fields for term vectors.
type termVectorFields struct {
	fields map[string]index.Terms
}

// Iterator returns an iterator over all field names.
func (f *termVectorFields) Iterator() (index.FieldIterator, error) {
	fieldNames := make([]string, 0, len(f.fields))
	for name := range f.fields {
		fieldNames = append(fieldNames, name)
	}
	return &termVectorFieldIterator{fields: fieldNames, index: -1}, nil
}

// Get returns the terms for a field.
func (f *termVectorFields) Terms(name string) (index.Terms, error) {
	terms, ok := f.fields[name]
	if !ok {
		return nil, nil
	}
	return terms, nil
}

// Size returns the number of fields.
func (f *termVectorFields) Size() int {
	return len(f.fields)
}

// termVectorFieldIterator implements index.FieldIterator for term vectors.
type termVectorFieldIterator struct {
	fields []string
	index  int
}

// Next advances to the next field name.
func (i *termVectorFieldIterator) Next() (string, error) {
	i.index++
	if i.index >= len(i.fields) {
		return "", nil
	}
	return i.fields[i.index], nil
}

// HasNext returns true if there are more field names.
func (i *termVectorFieldIterator) HasNext() bool {
	return i.index+1 < len(i.fields)
}
