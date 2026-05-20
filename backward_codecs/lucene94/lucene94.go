// Package lucene94 implements org.apache.lucene.backward_codecs.lucene94.
package lucene94

// Codec types in this package are read-only stubs that record the format
// metadata so the codec registry can resolve segments written by the
// matching Lucene version.

// Lucene94HnswVectorsFormat mirrors org.apache.lucene.backward_codecs.lucene94.Lucene94HnswVectorsFormat.
type Lucene94HnswVectorsFormat struct{ Name, Version string }

// NewLucene94HnswVectorsFormat builds a Lucene94HnswVectorsFormat with default settings.
func NewLucene94HnswVectorsFormat() *Lucene94HnswVectorsFormat {
	return &Lucene94HnswVectorsFormat{Name: "Lucene94HnswVectorsFormat"}
}

// NewLucene94HnswVectorsFormatWithVersion builds a Lucene94HnswVectorsFormat
// with the supplied version string.
func NewLucene94HnswVectorsFormatWithVersion(version string) *Lucene94HnswVectorsFormat {
	return &Lucene94HnswVectorsFormat{Name: "Lucene94HnswVectorsFormat", Version: version}
}

// Lucene94HnswVectorsReader mirrors org.apache.lucene.backward_codecs.lucene94.Lucene94HnswVectorsReader.
type Lucene94HnswVectorsReader struct{ Name, Version string }

// NewLucene94HnswVectorsReader builds a Lucene94HnswVectorsReader with the supplied version.
func NewLucene94HnswVectorsReader(version string) *Lucene94HnswVectorsReader {
	return &Lucene94HnswVectorsReader{Name: "Lucene94HnswVectorsReader", Version: version}
}

