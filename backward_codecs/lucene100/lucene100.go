// Package lucene100 implements org.apache.lucene.backward_codecs.lucene100.
package lucene100

// Codec types in this package are read-only stubs that record the format
// metadata so the codec registry can resolve segments written by the
// matching Lucene version.

// Lucene100Codec mirrors org.apache.lucene.backward_codecs.lucene100.Lucene100Codec.
type Lucene100Codec struct { Name, Version string }

// NewLucene100Codec builds a Lucene100Codec with the supplied version.
func NewLucene100Codec(version string) *Lucene100Codec { return &Lucene100Codec{Name: "Lucene100Codec", Version: version} }

