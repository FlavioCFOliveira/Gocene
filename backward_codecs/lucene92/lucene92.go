// Package lucene92 implements org.apache.lucene.backward_codecs.lucene92.
package lucene92

// Codec types in this package are read-only stubs that record the format
// metadata so the codec registry can resolve segments written by the
// matching Lucene version.

// Lucene92HnswVectorsFormat mirrors org.apache.lucene.backward_codecs.lucene92.Lucene92HnswVectorsFormat.
type Lucene92HnswVectorsFormat struct { Name, Version string }

// NewLucene92HnswVectorsFormat builds a Lucene92HnswVectorsFormat with the supplied version.
func NewLucene92HnswVectorsFormat(version string) *Lucene92HnswVectorsFormat { return &Lucene92HnswVectorsFormat{Name: "Lucene92HnswVectorsFormat", Version: version} }

// Lucene92HnswVectorsReader mirrors org.apache.lucene.backward_codecs.lucene92.Lucene92HnswVectorsReader.
type Lucene92HnswVectorsReader struct { Name, Version string }

// NewLucene92HnswVectorsReader builds a Lucene92HnswVectorsReader with the supplied version.
func NewLucene92HnswVectorsReader(version string) *Lucene92HnswVectorsReader { return &Lucene92HnswVectorsReader{Name: "Lucene92HnswVectorsReader", Version: version} }

// Lucene92Codec mirrors org.apache.lucene.backward_codecs.lucene92.Lucene92Codec.
type Lucene92Codec struct { Name, Version string }

// NewLucene92Codec builds a Lucene92Codec with the supplied version.
func NewLucene92Codec(version string) *Lucene92Codec { return &Lucene92Codec{Name: "Lucene92Codec", Version: version} }

