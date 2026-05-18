// Package lucene912 implements org.apache.lucene.backward_codecs.lucene912.
package lucene912

// Codec types in this package are read-only stubs that record the format
// metadata so the codec registry can resolve segments written by the
// matching Lucene version.

// Lucene912Codec mirrors org.apache.lucene.backward_codecs.lucene912.Lucene912Codec.
type Lucene912Codec struct { Name, Version string }

// NewLucene912Codec builds a Lucene912Codec with the supplied version.
func NewLucene912Codec(version string) *Lucene912Codec { return &Lucene912Codec{Name: "Lucene912Codec", Version: version} }

// Lucene912PostingsFormat mirrors org.apache.lucene.backward_codecs.lucene912.Lucene912PostingsFormat.
type Lucene912PostingsFormat struct { Name, Version string }

// NewLucene912PostingsFormat builds a Lucene912PostingsFormat with the supplied version.
func NewLucene912PostingsFormat(version string) *Lucene912PostingsFormat { return &Lucene912PostingsFormat{Name: "Lucene912PostingsFormat", Version: version} }

// Lucene912PostingsReader mirrors org.apache.lucene.backward_codecs.lucene912.Lucene912PostingsReader.
type Lucene912PostingsReader struct { Name, Version string }

// NewLucene912PostingsReader builds a Lucene912PostingsReader with the supplied version.
func NewLucene912PostingsReader(version string) *Lucene912PostingsReader { return &Lucene912PostingsReader{Name: "Lucene912PostingsReader", Version: version} }

