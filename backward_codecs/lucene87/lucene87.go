// Package lucene87 implements org.apache.lucene.backward_codecs.lucene87.
package lucene87

// Codec types in this package are read-only stubs that record the format
// metadata so the codec registry can resolve segments written by the
// matching Lucene version.

// DeflateWithPresetDictCompressionMode mirrors org.apache.lucene.backward_codecs.lucene87.DeflateWithPresetDictCompressionMode.
type DeflateWithPresetDictCompressionMode struct { Name, Version string }

// NewDeflateWithPresetDictCompressionMode builds a DeflateWithPresetDictCompressionMode with the supplied version.
func NewDeflateWithPresetDictCompressionMode(version string) *DeflateWithPresetDictCompressionMode { return &DeflateWithPresetDictCompressionMode{Name: "DeflateWithPresetDictCompressionMode", Version: version} }

// LZ4WithPresetDictCompressionMode mirrors org.apache.lucene.backward_codecs.lucene87.LZ4WithPresetDictCompressionMode.
type LZ4WithPresetDictCompressionMode struct { Name, Version string }

// NewLZ4WithPresetDictCompressionMode builds a LZ4WithPresetDictCompressionMode with the supplied version.
func NewLZ4WithPresetDictCompressionMode(version string) *LZ4WithPresetDictCompressionMode { return &LZ4WithPresetDictCompressionMode{Name: "LZ4WithPresetDictCompressionMode", Version: version} }

// Lucene87StoredFieldsFormat mirrors org.apache.lucene.backward_codecs.lucene87.Lucene87StoredFieldsFormat.
type Lucene87StoredFieldsFormat struct { Name, Version string }

// NewLucene87StoredFieldsFormat builds a Lucene87StoredFieldsFormat with the supplied version.
func NewLucene87StoredFieldsFormat(version string) *Lucene87StoredFieldsFormat { return &Lucene87StoredFieldsFormat{Name: "Lucene87StoredFieldsFormat", Version: version} }

// Lucene87Codec mirrors org.apache.lucene.backward_codecs.lucene87.Lucene87Codec.
type Lucene87Codec struct { Name, Version string }

// NewLucene87Codec builds a Lucene87Codec with the supplied version.
func NewLucene87Codec(version string) *Lucene87Codec { return &Lucene87Codec{Name: "Lucene87Codec", Version: version} }

