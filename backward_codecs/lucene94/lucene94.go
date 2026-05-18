// Package lucene94 implements org.apache.lucene.backward_codecs.lucene94.
package lucene94

// Codec types in this package are read-only stubs that record the format
// metadata so the codec registry can resolve segments written by the
// matching Lucene version.

// Lucene94HnswVectorsFormat mirrors org.apache.lucene.backward_codecs.lucene94.Lucene94HnswVectorsFormat.
type Lucene94HnswVectorsFormat struct { Name, Version string }

// NewLucene94HnswVectorsFormat builds a Lucene94HnswVectorsFormat with the supplied version.
func NewLucene94HnswVectorsFormat(version string) *Lucene94HnswVectorsFormat { return &Lucene94HnswVectorsFormat{Name: "Lucene94HnswVectorsFormat", Version: version} }

// Lucene94HnswVectorsReader mirrors org.apache.lucene.backward_codecs.lucene94.Lucene94HnswVectorsReader.
type Lucene94HnswVectorsReader struct { Name, Version string }

// NewLucene94HnswVectorsReader builds a Lucene94HnswVectorsReader with the supplied version.
func NewLucene94HnswVectorsReader(version string) *Lucene94HnswVectorsReader { return &Lucene94HnswVectorsReader{Name: "Lucene94HnswVectorsReader", Version: version} }

// Lucene94Codec mirrors org.apache.lucene.backward_codecs.lucene94.Lucene94Codec.
type Lucene94Codec struct { Name, Version string }

// NewLucene94Codec builds a Lucene94Codec with the supplied version.
func NewLucene94Codec(version string) *Lucene94Codec { return &Lucene94Codec{Name: "Lucene94Codec", Version: version} }

