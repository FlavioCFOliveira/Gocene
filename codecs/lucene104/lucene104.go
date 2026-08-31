// Package lucene104 hosts the Sprint 47 ports for
// org.apache.lucene.codecs.lucene104.
package lucene104

// The Sprint 47 lucene104-codec port surfaces these types as typed
// stubs so dependent packages keep compiling; concrete behaviour ports
// (e.g. PostingsReader/Writer byte-format round-trip, HNSW scalar
// quantization) land progressively in follow-up deep-port sprints.

// ForUtil mirrors org.apache.lucene.codecs.lucene104.ForUtil.
type ForUtil struct{}

// NewForUtil builds a ForUtil.
func NewForUtil() *ForUtil { return &ForUtil{} }

// Lucene104Codec mirrors org.apache.lucene.codecs.lucene104.Lucene104Codec.
// It is implemented in lucene104_codec.go.


// Lucene104HnswScalarQuantizedVectorsFormat mirrors
// org.apache.lucene.codecs.lucene104.Lucene104HnswScalarQuantizedVectorsFormat.
type Lucene104HnswScalarQuantizedVectorsFormat struct{}

// NewLucene104HnswScalarQuantizedVectorsFormat builds a
// Lucene104HnswScalarQuantizedVectorsFormat.
func NewLucene104HnswScalarQuantizedVectorsFormat() *Lucene104HnswScalarQuantizedVectorsFormat {
	return &Lucene104HnswScalarQuantizedVectorsFormat{}
}

// Lucene104PostingsFormat mirrors
// org.apache.lucene.codecs.lucene104.Lucene104PostingsFormat.
type Lucene104PostingsFormat struct{}

// NewLucene104PostingsFormat builds a Lucene104PostingsFormat.
func NewLucene104PostingsFormat() *Lucene104PostingsFormat { return &Lucene104PostingsFormat{} }

// Lucene104PostingsReader mirrors
// org.apache.lucene.codecs.lucene104.Lucene104PostingsReader.
type Lucene104PostingsReader struct{}

// NewLucene104PostingsReader builds a Lucene104PostingsReader.
func NewLucene104PostingsReader() *Lucene104PostingsReader { return &Lucene104PostingsReader{} }

// Lucene104PostingsWriter mirrors
// org.apache.lucene.codecs.lucene104.Lucene104PostingsWriter.
type Lucene104PostingsWriter struct{}

// NewLucene104PostingsWriter builds a Lucene104PostingsWriter.
func NewLucene104PostingsWriter() *Lucene104PostingsWriter { return &Lucene104PostingsWriter{} }

