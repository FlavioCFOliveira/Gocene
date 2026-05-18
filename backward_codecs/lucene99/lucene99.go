// Package lucene99 implements org.apache.lucene.backward_codecs.lucene99.
package lucene99

// Codec types in this package are read-only stubs that record the format
// metadata so the codec registry can resolve segments written by the
// matching Lucene version.

// ForDeltaUtil mirrors org.apache.lucene.backward_codecs.lucene99.ForDeltaUtil.
type ForDeltaUtil struct { Name, Version string }

// NewForDeltaUtil builds a ForDeltaUtil with the supplied version.
func NewForDeltaUtil(version string) *ForDeltaUtil { return &ForDeltaUtil{Name: "ForDeltaUtil", Version: version} }

// Lucene99HnswScalarQuantizedVectorsFormat mirrors org.apache.lucene.backward_codecs.lucene99.Lucene99HnswScalarQuantizedVectorsFormat.
type Lucene99HnswScalarQuantizedVectorsFormat struct { Name, Version string }

// NewLucene99HnswScalarQuantizedVectorsFormat builds a Lucene99HnswScalarQuantizedVectorsFormat with the supplied version.
func NewLucene99HnswScalarQuantizedVectorsFormat(version string) *Lucene99HnswScalarQuantizedVectorsFormat { return &Lucene99HnswScalarQuantizedVectorsFormat{Name: "Lucene99HnswScalarQuantizedVectorsFormat", Version: version} }

// Lucene99ScalarQuantizedVectorsFormat mirrors org.apache.lucene.backward_codecs.lucene99.Lucene99ScalarQuantizedVectorsFormat.
type Lucene99ScalarQuantizedVectorsFormat struct { Name, Version string }

// NewLucene99ScalarQuantizedVectorsFormat builds a Lucene99ScalarQuantizedVectorsFormat with the supplied version.
func NewLucene99ScalarQuantizedVectorsFormat(version string) *Lucene99ScalarQuantizedVectorsFormat { return &Lucene99ScalarQuantizedVectorsFormat{Name: "Lucene99ScalarQuantizedVectorsFormat", Version: version} }

// Lucene99SkipReader mirrors org.apache.lucene.backward_codecs.lucene99.Lucene99SkipReader.
type Lucene99SkipReader struct { Name, Version string }

// NewLucene99SkipReader builds a Lucene99SkipReader with the supplied version.
func NewLucene99SkipReader(version string) *Lucene99SkipReader { return &Lucene99SkipReader{Name: "Lucene99SkipReader", Version: version} }

// Lucene99SkipWriter mirrors org.apache.lucene.backward_codecs.lucene99.Lucene99SkipWriter.
type Lucene99SkipWriter struct { Name, Version string }

// NewLucene99SkipWriter builds a Lucene99SkipWriter with the supplied version.
func NewLucene99SkipWriter(version string) *Lucene99SkipWriter { return &Lucene99SkipWriter{Name: "Lucene99SkipWriter", Version: version} }

// OffHeapQuantizedByteVectorValues mirrors org.apache.lucene.backward_codecs.lucene99.OffHeapQuantizedByteVectorValues.
type OffHeapQuantizedByteVectorValues struct { Name, Version string }

// NewOffHeapQuantizedByteVectorValues builds a OffHeapQuantizedByteVectorValues with the supplied version.
func NewOffHeapQuantizedByteVectorValues(version string) *OffHeapQuantizedByteVectorValues { return &OffHeapQuantizedByteVectorValues{Name: "OffHeapQuantizedByteVectorValues", Version: version} }

// Lucene99PostingsFormat mirrors org.apache.lucene.backward_codecs.lucene99.Lucene99PostingsFormat.
type Lucene99PostingsFormat struct { Name, Version string }

// NewLucene99PostingsFormat builds a Lucene99PostingsFormat with the supplied version.
func NewLucene99PostingsFormat(version string) *Lucene99PostingsFormat { return &Lucene99PostingsFormat{Name: "Lucene99PostingsFormat", Version: version} }

// Lucene99ScalarQuantizedVectorsReader mirrors org.apache.lucene.backward_codecs.lucene99.Lucene99ScalarQuantizedVectorsReader.
type Lucene99ScalarQuantizedVectorsReader struct { Name, Version string }

// NewLucene99ScalarQuantizedVectorsReader builds a Lucene99ScalarQuantizedVectorsReader with the supplied version.
func NewLucene99ScalarQuantizedVectorsReader(version string) *Lucene99ScalarQuantizedVectorsReader { return &Lucene99ScalarQuantizedVectorsReader{Name: "Lucene99ScalarQuantizedVectorsReader", Version: version} }

// Lucene99Codec mirrors org.apache.lucene.backward_codecs.lucene99.Lucene99Codec.
type Lucene99Codec struct { Name, Version string }

// NewLucene99Codec builds a Lucene99Codec with the supplied version.
func NewLucene99Codec(version string) *Lucene99Codec { return &Lucene99Codec{Name: "Lucene99Codec", Version: version} }

// Lucene99PostingsReader mirrors org.apache.lucene.backward_codecs.lucene99.Lucene99PostingsReader.
type Lucene99PostingsReader struct { Name, Version string }

// NewLucene99PostingsReader builds a Lucene99PostingsReader with the supplied version.
func NewLucene99PostingsReader(version string) *Lucene99PostingsReader { return &Lucene99PostingsReader{Name: "Lucene99PostingsReader", Version: version} }

