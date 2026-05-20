// Package lucene95 implements org.apache.lucene.backward_codecs.lucene95.
package lucene95

// Lucene95HnswVectorsReader mirrors org.apache.lucene.backward_codecs.lucene95.Lucene95HnswVectorsReader.
type Lucene95HnswVectorsReader struct{ Name, Version string }

// NewLucene95HnswVectorsReader builds a Lucene95HnswVectorsReader with the supplied version.
func NewLucene95HnswVectorsReader(version string) *Lucene95HnswVectorsReader {
	return &Lucene95HnswVectorsReader{Name: "Lucene95HnswVectorsReader", Version: version}
}

// Lucene95Codec mirrors org.apache.lucene.backward_codecs.lucene95.Lucene95Codec.
type Lucene95Codec struct{ Name, Version string }

// NewLucene95Codec builds a Lucene95Codec with the supplied version.
func NewLucene95Codec(version string) *Lucene95Codec {
	return &Lucene95Codec{Name: "Lucene95Codec", Version: version}
}
