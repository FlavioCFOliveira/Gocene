// Package lucene102 implements org.apache.lucene.backward_codecs.lucene102.
package lucene102

// Codec types in this package are read-only stubs that record the format
// metadata so the codec registry can resolve segments written by the
// matching Lucene version.

// Lucene102BinaryQuantizedVectorsFormat mirrors org.apache.lucene.backward_codecs.lucene102.Lucene102BinaryQuantizedVectorsFormat.
type Lucene102BinaryQuantizedVectorsFormat struct{ Name, Version string }

// NewLucene102BinaryQuantizedVectorsFormat builds a Lucene102BinaryQuantizedVectorsFormat with the supplied version.
func NewLucene102BinaryQuantizedVectorsFormat(version string) *Lucene102BinaryQuantizedVectorsFormat {
	return &Lucene102BinaryQuantizedVectorsFormat{Name: "Lucene102BinaryQuantizedVectorsFormat", Version: version}
}

// Lucene102HnswBinaryQuantizedVectorsFormat mirrors org.apache.lucene.backward_codecs.lucene102.Lucene102HnswBinaryQuantizedVectorsFormat.
type Lucene102HnswBinaryQuantizedVectorsFormat struct{ Name, Version string }

// NewLucene102HnswBinaryQuantizedVectorsFormat builds a Lucene102HnswBinaryQuantizedVectorsFormat with the supplied version.
func NewLucene102HnswBinaryQuantizedVectorsFormat(version string) *Lucene102HnswBinaryQuantizedVectorsFormat {
	return &Lucene102HnswBinaryQuantizedVectorsFormat{Name: "Lucene102HnswBinaryQuantizedVectorsFormat", Version: version}
}

// OffHeapBinarizedVectorValues mirrors org.apache.lucene.backward_codecs.lucene102.OffHeapBinarizedVectorValues.
type OffHeapBinarizedVectorValues struct{ Name, Version string }

// NewOffHeapBinarizedVectorValues builds a OffHeapBinarizedVectorValues with the supplied version.
func NewOffHeapBinarizedVectorValues(version string) *OffHeapBinarizedVectorValues {
	return &OffHeapBinarizedVectorValues{Name: "OffHeapBinarizedVectorValues", Version: version}
}

// Lucene102BinaryFlatVectorsScorer mirrors org.apache.lucene.backward_codecs.lucene102.Lucene102BinaryFlatVectorsScorer.
type Lucene102BinaryFlatVectorsScorer struct{ Name, Version string }

// NewLucene102BinaryFlatVectorsScorer builds a Lucene102BinaryFlatVectorsScorer with the supplied version.
func NewLucene102BinaryFlatVectorsScorer(version string) *Lucene102BinaryFlatVectorsScorer {
	return &Lucene102BinaryFlatVectorsScorer{Name: "Lucene102BinaryFlatVectorsScorer", Version: version}
}

// Lucene102BinaryQuantizedVectorsReader mirrors org.apache.lucene.backward_codecs.lucene102.Lucene102BinaryQuantizedVectorsReader.
type Lucene102BinaryQuantizedVectorsReader struct{ Name, Version string }

// NewLucene102BinaryQuantizedVectorsReader builds a Lucene102BinaryQuantizedVectorsReader with the supplied version.
func NewLucene102BinaryQuantizedVectorsReader(version string) *Lucene102BinaryQuantizedVectorsReader {
	return &Lucene102BinaryQuantizedVectorsReader{Name: "Lucene102BinaryQuantizedVectorsReader", Version: version}
}
