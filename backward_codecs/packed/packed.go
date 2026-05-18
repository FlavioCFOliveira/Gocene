// Package packed implements org.apache.lucene.backward_codecs.packed.
package packed

// Codec types in this package are read-only stubs that record the format
// metadata so the codec registry can resolve segments written by the
// matching Lucene version.

// LegacyDirectMonotonicReader mirrors org.apache.lucene.backward_codecs.packed.LegacyDirectMonotonicReader.
type LegacyDirectMonotonicReader struct { Name, Version string }

// NewLegacyDirectMonotonicReader builds a LegacyDirectMonotonicReader with the supplied version.
func NewLegacyDirectMonotonicReader(version string) *LegacyDirectMonotonicReader { return &LegacyDirectMonotonicReader{Name: "LegacyDirectMonotonicReader", Version: version} }

// LegacyDirectMonotonicWriter mirrors org.apache.lucene.backward_codecs.packed.LegacyDirectMonotonicWriter.
type LegacyDirectMonotonicWriter struct { Name, Version string }

// NewLegacyDirectMonotonicWriter builds a LegacyDirectMonotonicWriter with the supplied version.
func NewLegacyDirectMonotonicWriter(version string) *LegacyDirectMonotonicWriter { return &LegacyDirectMonotonicWriter{Name: "LegacyDirectMonotonicWriter", Version: version} }

// LegacyDirectReader mirrors org.apache.lucene.backward_codecs.packed.LegacyDirectReader.
type LegacyDirectReader struct { Name, Version string }

// NewLegacyDirectReader builds a LegacyDirectReader with the supplied version.
func NewLegacyDirectReader(version string) *LegacyDirectReader { return &LegacyDirectReader{Name: "LegacyDirectReader", Version: version} }

// LegacyDirectWriter mirrors org.apache.lucene.backward_codecs.packed.LegacyDirectWriter.
type LegacyDirectWriter struct { Name, Version string }

// NewLegacyDirectWriter builds a LegacyDirectWriter with the supplied version.
func NewLegacyDirectWriter(version string) *LegacyDirectWriter { return &LegacyDirectWriter{Name: "LegacyDirectWriter", Version: version} }

// LegacyPackedInts mirrors org.apache.lucene.backward_codecs.packed.LegacyPackedInts.
type LegacyPackedInts struct { Name, Version string }

// NewLegacyPackedInts builds a LegacyPackedInts with the supplied version.
func NewLegacyPackedInts(version string) *LegacyPackedInts { return &LegacyPackedInts{Name: "LegacyPackedInts", Version: version} }

