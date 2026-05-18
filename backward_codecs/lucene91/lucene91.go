// Package lucene91 implements org.apache.lucene.backward_codecs.lucene91.
package lucene91

// Codec types in this package are read-only stubs that record the format
// metadata so the codec registry can resolve segments written by the
// matching Lucene version.

// Lucene91BoundsChecker mirrors org.apache.lucene.backward_codecs.lucene91.Lucene91BoundsChecker.
type Lucene91BoundsChecker struct { Name, Version string }

// NewLucene91BoundsChecker builds a Lucene91BoundsChecker with the supplied version.
func NewLucene91BoundsChecker(version string) *Lucene91BoundsChecker { return &Lucene91BoundsChecker{Name: "Lucene91BoundsChecker", Version: version} }

// Lucene91HnswVectorsFormat mirrors org.apache.lucene.backward_codecs.lucene91.Lucene91HnswVectorsFormat.
type Lucene91HnswVectorsFormat struct { Name, Version string }

// NewLucene91HnswVectorsFormat builds a Lucene91HnswVectorsFormat with the supplied version.
func NewLucene91HnswVectorsFormat(version string) *Lucene91HnswVectorsFormat { return &Lucene91HnswVectorsFormat{Name: "Lucene91HnswVectorsFormat", Version: version} }

// Lucene91NeighborArray mirrors org.apache.lucene.backward_codecs.lucene91.Lucene91NeighborArray.
type Lucene91NeighborArray struct { Name, Version string }

// NewLucene91NeighborArray builds a Lucene91NeighborArray with the supplied version.
func NewLucene91NeighborArray(version string) *Lucene91NeighborArray { return &Lucene91NeighborArray{Name: "Lucene91NeighborArray", Version: version} }

// Lucene91OnHeapHnswGraph mirrors org.apache.lucene.backward_codecs.lucene91.Lucene91OnHeapHnswGraph.
type Lucene91OnHeapHnswGraph struct { Name, Version string }

// NewLucene91OnHeapHnswGraph builds a Lucene91OnHeapHnswGraph with the supplied version.
func NewLucene91OnHeapHnswGraph(version string) *Lucene91OnHeapHnswGraph { return &Lucene91OnHeapHnswGraph{Name: "Lucene91OnHeapHnswGraph", Version: version} }

// Lucene91HnswVectorsReader mirrors org.apache.lucene.backward_codecs.lucene91.Lucene91HnswVectorsReader.
type Lucene91HnswVectorsReader struct { Name, Version string }

// NewLucene91HnswVectorsReader builds a Lucene91HnswVectorsReader with the supplied version.
func NewLucene91HnswVectorsReader(version string) *Lucene91HnswVectorsReader { return &Lucene91HnswVectorsReader{Name: "Lucene91HnswVectorsReader", Version: version} }

// Lucene91Codec mirrors org.apache.lucene.backward_codecs.lucene91.Lucene91Codec.
type Lucene91Codec struct { Name, Version string }

// NewLucene91Codec builds a Lucene91Codec with the supplied version.
func NewLucene91Codec(version string) *Lucene91Codec { return &Lucene91Codec{Name: "Lucene91Codec", Version: version} }

