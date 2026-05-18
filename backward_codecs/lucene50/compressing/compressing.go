// Package compressing implements org.apache.lucene.backward_codecs.lucene50.compressing.
package compressing

// Codec types in this package are read-only stubs that record the format
// metadata so the codec registry can resolve segments written by the
// matching Lucene version.

// Lucene50CompressingTermVectorsFormat mirrors org.apache.lucene.backward_codecs.lucene50.compressing.Lucene50CompressingTermVectorsFormat.
type Lucene50CompressingTermVectorsFormat struct { Name, Version string }

// NewLucene50CompressingTermVectorsFormat builds a Lucene50CompressingTermVectorsFormat with the supplied version.
func NewLucene50CompressingTermVectorsFormat(version string) *Lucene50CompressingTermVectorsFormat { return &Lucene50CompressingTermVectorsFormat{Name: "Lucene50CompressingTermVectorsFormat", Version: version} }

// Lucene50CompressingStoredFieldsFormat mirrors org.apache.lucene.backward_codecs.lucene50.compressing.Lucene50CompressingStoredFieldsFormat.
type Lucene50CompressingStoredFieldsFormat struct { Name, Version string }

// NewLucene50CompressingStoredFieldsFormat builds a Lucene50CompressingStoredFieldsFormat with the supplied version.
func NewLucene50CompressingStoredFieldsFormat(version string) *Lucene50CompressingStoredFieldsFormat { return &Lucene50CompressingStoredFieldsFormat{Name: "Lucene50CompressingStoredFieldsFormat", Version: version} }

// Lucene50CompressingStoredFieldsReader mirrors org.apache.lucene.backward_codecs.lucene50.compressing.Lucene50CompressingStoredFieldsReader.
type Lucene50CompressingStoredFieldsReader struct { Name, Version string }

// NewLucene50CompressingStoredFieldsReader builds a Lucene50CompressingStoredFieldsReader with the supplied version.
func NewLucene50CompressingStoredFieldsReader(version string) *Lucene50CompressingStoredFieldsReader { return &Lucene50CompressingStoredFieldsReader{Name: "Lucene50CompressingStoredFieldsReader", Version: version} }

// Lucene50CompressingTermVectorsReader mirrors org.apache.lucene.backward_codecs.lucene50.compressing.Lucene50CompressingTermVectorsReader.
type Lucene50CompressingTermVectorsReader struct { Name, Version string }

// NewLucene50CompressingTermVectorsReader builds a Lucene50CompressingTermVectorsReader with the supplied version.
func NewLucene50CompressingTermVectorsReader(version string) *Lucene50CompressingTermVectorsReader { return &Lucene50CompressingTermVectorsReader{Name: "Lucene50CompressingTermVectorsReader", Version: version} }

