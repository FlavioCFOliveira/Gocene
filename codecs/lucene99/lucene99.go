// Package lucene99 hosts the Sprint 49 ports for
// org.apache.lucene.codecs.lucene99.
package lucene99

// The Sprint 49 lucene99 port surfaces these types as typed stubs so
// dependent packages keep compiling; concrete behaviour ports (flat
// vector layout, HNSW graph traversal, scalar-quantised scoring,
// segment-info chunk format) land in follow-up deep-port sprints.

// Lucene99FlatVectorsFormat mirrors
// org.apache.lucene.codecs.lucene99.Lucene99FlatVectorsFormat.
type Lucene99FlatVectorsFormat struct{}

// NewLucene99FlatVectorsFormat builds a Lucene99FlatVectorsFormat.
func NewLucene99FlatVectorsFormat() *Lucene99FlatVectorsFormat { return &Lucene99FlatVectorsFormat{} }

// Lucene99FlatVectorsReader mirrors
// org.apache.lucene.codecs.lucene99.Lucene99FlatVectorsReader.
type Lucene99FlatVectorsReader struct{}

// NewLucene99FlatVectorsReader builds a Lucene99FlatVectorsReader.
func NewLucene99FlatVectorsReader() *Lucene99FlatVectorsReader { return &Lucene99FlatVectorsReader{} }

// Lucene99FlatVectorsWriter mirrors
// org.apache.lucene.codecs.lucene99.Lucene99FlatVectorsWriter.
type Lucene99FlatVectorsWriter struct{}

// NewLucene99FlatVectorsWriter builds a Lucene99FlatVectorsWriter.
func NewLucene99FlatVectorsWriter() *Lucene99FlatVectorsWriter { return &Lucene99FlatVectorsWriter{} }

// Lucene99HnswVectorsFormat mirrors
// org.apache.lucene.codecs.lucene99.Lucene99HnswVectorsFormat.
type Lucene99HnswVectorsFormat struct{}

// NewLucene99HnswVectorsFormat builds a Lucene99HnswVectorsFormat.
func NewLucene99HnswVectorsFormat() *Lucene99HnswVectorsFormat { return &Lucene99HnswVectorsFormat{} }

// Lucene99HnswVectorsReader mirrors
// org.apache.lucene.codecs.lucene99.Lucene99HnswVectorsReader.
type Lucene99HnswVectorsReader struct{}

// NewLucene99HnswVectorsReader builds a Lucene99HnswVectorsReader.
func NewLucene99HnswVectorsReader() *Lucene99HnswVectorsReader { return &Lucene99HnswVectorsReader{} }

// Lucene99HnswVectorsWriter mirrors
// org.apache.lucene.codecs.lucene99.Lucene99HnswVectorsWriter.
type Lucene99HnswVectorsWriter struct{}

// NewLucene99HnswVectorsWriter builds a Lucene99HnswVectorsWriter.
func NewLucene99HnswVectorsWriter() *Lucene99HnswVectorsWriter { return &Lucene99HnswVectorsWriter{} }

// Lucene99ScalarQuantizedVectorScorer mirrors
// org.apache.lucene.codecs.lucene99.Lucene99ScalarQuantizedVectorScorer.
type Lucene99ScalarQuantizedVectorScorer struct{}

// NewLucene99ScalarQuantizedVectorScorer builds a
// Lucene99ScalarQuantizedVectorScorer.
func NewLucene99ScalarQuantizedVectorScorer() *Lucene99ScalarQuantizedVectorScorer {
	return &Lucene99ScalarQuantizedVectorScorer{}
}

// Lucene99SegmentInfoFormat mirrors
// org.apache.lucene.codecs.lucene99.Lucene99SegmentInfoFormat.
type Lucene99SegmentInfoFormat struct{}

// NewLucene99SegmentInfoFormat builds a Lucene99SegmentInfoFormat.
func NewLucene99SegmentInfoFormat() *Lucene99SegmentInfoFormat { return &Lucene99SegmentInfoFormat{} }
