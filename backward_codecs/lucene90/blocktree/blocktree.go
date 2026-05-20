// Package blocktree implements org.apache.lucene.backward_codecs.lucene90.blocktree.
package blocktree

// Codec types in this package are read-only stubs that record the format
// metadata so the codec registry can resolve segments written by the
// matching Lucene version.

// CompressionAlgorithm mirrors org.apache.lucene.backward_codecs.lucene90.blocktree.CompressionAlgorithm.
type CompressionAlgorithm struct{ Name, Version string }

// NewCompressionAlgorithm builds a CompressionAlgorithm with the supplied version.
func NewCompressionAlgorithm(version string) *CompressionAlgorithm {
	return &CompressionAlgorithm{Name: "CompressionAlgorithm", Version: version}
}

// Lucene90BlockTreeTermsReader mirrors org.apache.lucene.backward_codecs.lucene90.blocktree.Lucene90BlockTreeTermsReader.
type Lucene90BlockTreeTermsReader struct{ Name, Version string }

// NewLucene90BlockTreeTermsReader builds a Lucene90BlockTreeTermsReader with the supplied version.
func NewLucene90BlockTreeTermsReader(version string) *Lucene90BlockTreeTermsReader {
	return &Lucene90BlockTreeTermsReader{Name: "Lucene90BlockTreeTermsReader", Version: version}
}

// Stats mirrors org.apache.lucene.backward_codecs.lucene90.blocktree.Stats.
type Stats struct{ Name, Version string }

// NewStats builds a Stats with the supplied version.
func NewStats(version string) *Stats { return &Stats{Name: "Stats", Version: version} }

// FieldReader mirrors org.apache.lucene.backward_codecs.lucene90.blocktree.FieldReader.
type FieldReader struct{ Name, Version string }

// NewFieldReader builds a FieldReader with the supplied version.
func NewFieldReader(version string) *FieldReader {
	return &FieldReader{Name: "FieldReader", Version: version}
}
