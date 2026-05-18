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
type Lucene104Codec struct{}

// NewLucene104Codec builds a Lucene104Codec.
func NewLucene104Codec() *Lucene104Codec { return &Lucene104Codec{} }

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

// Lucene104ScalarQuantizedVectorScorer mirrors
// org.apache.lucene.codecs.lucene104.Lucene104ScalarQuantizedVectorScorer.
type Lucene104ScalarQuantizedVectorScorer struct{}

// NewLucene104ScalarQuantizedVectorScorer builds a
// Lucene104ScalarQuantizedVectorScorer.
func NewLucene104ScalarQuantizedVectorScorer() *Lucene104ScalarQuantizedVectorScorer {
	return &Lucene104ScalarQuantizedVectorScorer{}
}

// Lucene104ScalarQuantizedVectorsFormat mirrors
// org.apache.lucene.codecs.lucene104.Lucene104ScalarQuantizedVectorsFormat.
type Lucene104ScalarQuantizedVectorsFormat struct{}

// NewLucene104ScalarQuantizedVectorsFormat builds a
// Lucene104ScalarQuantizedVectorsFormat.
func NewLucene104ScalarQuantizedVectorsFormat() *Lucene104ScalarQuantizedVectorsFormat {
	return &Lucene104ScalarQuantizedVectorsFormat{}
}

// Lucene104ScalarQuantizedVectorsReader mirrors
// org.apache.lucene.codecs.lucene104.Lucene104ScalarQuantizedVectorsReader.
type Lucene104ScalarQuantizedVectorsReader struct{}

// NewLucene104ScalarQuantizedVectorsReader builds a
// Lucene104ScalarQuantizedVectorsReader.
func NewLucene104ScalarQuantizedVectorsReader() *Lucene104ScalarQuantizedVectorsReader {
	return &Lucene104ScalarQuantizedVectorsReader{}
}

// Lucene104ScalarQuantizedVectorsWriter mirrors
// org.apache.lucene.codecs.lucene104.Lucene104ScalarQuantizedVectorsWriter.
type Lucene104ScalarQuantizedVectorsWriter struct{}

// NewLucene104ScalarQuantizedVectorsWriter builds a
// Lucene104ScalarQuantizedVectorsWriter.
func NewLucene104ScalarQuantizedVectorsWriter() *Lucene104ScalarQuantizedVectorsWriter {
	return &Lucene104ScalarQuantizedVectorsWriter{}
}

// OffHeapScalarQuantizedVectorValues mirrors
// org.apache.lucene.codecs.lucene104.OffHeapScalarQuantizedVectorValues.
type OffHeapScalarQuantizedVectorValues struct{}

// NewOffHeapScalarQuantizedVectorValues builds an
// OffHeapScalarQuantizedVectorValues.
func NewOffHeapScalarQuantizedVectorValues() *OffHeapScalarQuantizedVectorValues {
	return &OffHeapScalarQuantizedVectorValues{}
}

// PostingIndexInput mirrors org.apache.lucene.codecs.lucene104.PostingIndexInput.
type PostingIndexInput struct{}

// NewPostingIndexInput builds a PostingIndexInput.
func NewPostingIndexInput() *PostingIndexInput { return &PostingIndexInput{} }

// QuantizedByteVectorValues mirrors
// org.apache.lucene.codecs.lucene104.QuantizedByteVectorValues.
type QuantizedByteVectorValues struct{}

// NewQuantizedByteVectorValues builds a QuantizedByteVectorValues.
func NewQuantizedByteVectorValues() *QuantizedByteVectorValues { return &QuantizedByteVectorValues{} }
