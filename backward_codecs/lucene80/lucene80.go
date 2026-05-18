// Package lucene80 implements org.apache.lucene.backward_codecs.lucene80.
package lucene80

// Codec types in this package are read-only stubs that record the format
// metadata so the codec registry can resolve segments written by the
// matching Lucene version.

// Lucene80NormsFormat mirrors org.apache.lucene.backward_codecs.lucene80.Lucene80NormsFormat.
type Lucene80NormsFormat struct { Name, Version string }

// NewLucene80NormsFormat builds a Lucene80NormsFormat with the supplied version.
func NewLucene80NormsFormat(version string) *Lucene80NormsFormat { return &Lucene80NormsFormat{Name: "Lucene80NormsFormat", Version: version} }

// Lucene80DocValuesFormat mirrors org.apache.lucene.backward_codecs.lucene80.Lucene80DocValuesFormat.
type Lucene80DocValuesFormat struct { Name, Version string }

// NewLucene80DocValuesFormat builds a Lucene80DocValuesFormat with the supplied version.
func NewLucene80DocValuesFormat(version string) *Lucene80DocValuesFormat { return &Lucene80DocValuesFormat{Name: "Lucene80DocValuesFormat", Version: version} }

// Lucene80Codec mirrors org.apache.lucene.backward_codecs.lucene80.Lucene80Codec.
type Lucene80Codec struct { Name, Version string }

// NewLucene80Codec builds a Lucene80Codec with the supplied version.
func NewLucene80Codec(version string) *Lucene80Codec { return &Lucene80Codec{Name: "Lucene80Codec", Version: version} }

