// Package compressing implements org.apache.lucene.backward_codecs.compressing.
package compressing

// Codec types in this package are read-only stubs that record the format
// metadata so the codec registry can resolve segments written by the
// matching Lucene version.

// CompressionMode mirrors org.apache.lucene.backward_codecs.compressing.CompressionMode.
type CompressionMode struct { Name, Version string }

// NewCompressionMode builds a CompressionMode with the supplied version.
func NewCompressionMode(version string) *CompressionMode { return &CompressionMode{Name: "CompressionMode", Version: version} }

// Compressor mirrors org.apache.lucene.backward_codecs.compressing.Compressor.
type Compressor struct { Name, Version string }

// NewCompressor builds a Compressor with the supplied version.
func NewCompressor(version string) *Compressor { return &Compressor{Name: "Compressor", Version: version} }

// Decompressor mirrors org.apache.lucene.backward_codecs.compressing.Decompressor.
type Decompressor struct { Name, Version string }

// NewDecompressor builds a Decompressor with the supplied version.
func NewDecompressor(version string) *Decompressor { return &Decompressor{Name: "Decompressor", Version: version} }

// MatchingReaders mirrors org.apache.lucene.backward_codecs.compressing.MatchingReaders.
type MatchingReaders struct { Name, Version string }

// NewMatchingReaders builds a MatchingReaders with the supplied version.
func NewMatchingReaders(version string) *MatchingReaders { return &MatchingReaders{Name: "MatchingReaders", Version: version} }

