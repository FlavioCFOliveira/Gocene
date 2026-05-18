// Package lucene90 implements org.apache.lucene.backward_codecs.lucene90.
package lucene90

// Codec types in this package are read-only stubs that record the format
// metadata so the codec registry can resolve segments written by the
// matching Lucene version.

// Lucene90BoundsChecker mirrors org.apache.lucene.backward_codecs.lucene90.Lucene90BoundsChecker.
type Lucene90BoundsChecker struct { Name, Version string }

// NewLucene90BoundsChecker builds a Lucene90BoundsChecker with the supplied version.
func NewLucene90BoundsChecker(version string) *Lucene90BoundsChecker { return &Lucene90BoundsChecker{Name: "Lucene90BoundsChecker", Version: version} }

// Lucene90Codec mirrors org.apache.lucene.backward_codecs.lucene90.Lucene90Codec.
type Lucene90Codec struct { Name, Version string }

// NewLucene90Codec builds a Lucene90Codec with the supplied version.
func NewLucene90Codec(version string) *Lucene90Codec { return &Lucene90Codec{Name: "Lucene90Codec", Version: version} }

// Lucene90FieldInfosFormat mirrors org.apache.lucene.backward_codecs.lucene90.Lucene90FieldInfosFormat.
type Lucene90FieldInfosFormat struct { Name, Version string }

// NewLucene90FieldInfosFormat builds a Lucene90FieldInfosFormat with the supplied version.
func NewLucene90FieldInfosFormat(version string) *Lucene90FieldInfosFormat { return &Lucene90FieldInfosFormat{Name: "Lucene90FieldInfosFormat", Version: version} }

// Lucene90HnswGraphBuilder mirrors org.apache.lucene.backward_codecs.lucene90.Lucene90HnswGraphBuilder.
type Lucene90HnswGraphBuilder struct { Name, Version string }

// NewLucene90HnswGraphBuilder builds a Lucene90HnswGraphBuilder with the supplied version.
func NewLucene90HnswGraphBuilder(version string) *Lucene90HnswGraphBuilder { return &Lucene90HnswGraphBuilder{Name: "Lucene90HnswGraphBuilder", Version: version} }

// Lucene90HnswVectorsFormat mirrors org.apache.lucene.backward_codecs.lucene90.Lucene90HnswVectorsFormat.
type Lucene90HnswVectorsFormat struct { Name, Version string }

// NewLucene90HnswVectorsFormat builds a Lucene90HnswVectorsFormat with the supplied version.
func NewLucene90HnswVectorsFormat(version string) *Lucene90HnswVectorsFormat { return &Lucene90HnswVectorsFormat{Name: "Lucene90HnswVectorsFormat", Version: version} }

// Lucene90NeighborArray mirrors org.apache.lucene.backward_codecs.lucene90.Lucene90NeighborArray.
type Lucene90NeighborArray struct { Name, Version string }

// NewLucene90NeighborArray builds a Lucene90NeighborArray with the supplied version.
func NewLucene90NeighborArray(version string) *Lucene90NeighborArray { return &Lucene90NeighborArray{Name: "Lucene90NeighborArray", Version: version} }

// Lucene90OnHeapHnswGraph mirrors org.apache.lucene.backward_codecs.lucene90.Lucene90OnHeapHnswGraph.
type Lucene90OnHeapHnswGraph struct { Name, Version string }

// NewLucene90OnHeapHnswGraph builds a Lucene90OnHeapHnswGraph with the supplied version.
func NewLucene90OnHeapHnswGraph(version string) *Lucene90OnHeapHnswGraph { return &Lucene90OnHeapHnswGraph{Name: "Lucene90OnHeapHnswGraph", Version: version} }

// Lucene90SegmentInfoFormat mirrors org.apache.lucene.backward_codecs.lucene90.Lucene90SegmentInfoFormat.
type Lucene90SegmentInfoFormat struct { Name, Version string }

// NewLucene90SegmentInfoFormat builds a Lucene90SegmentInfoFormat with the supplied version.
func NewLucene90SegmentInfoFormat(version string) *Lucene90SegmentInfoFormat { return &Lucene90SegmentInfoFormat{Name: "Lucene90SegmentInfoFormat", Version: version} }

// Lucene90HnswVectorsReader mirrors org.apache.lucene.backward_codecs.lucene90.Lucene90HnswVectorsReader.
type Lucene90HnswVectorsReader struct { Name, Version string }

// NewLucene90HnswVectorsReader builds a Lucene90HnswVectorsReader with the supplied version.
func NewLucene90HnswVectorsReader(version string) *Lucene90HnswVectorsReader { return &Lucene90HnswVectorsReader{Name: "Lucene90HnswVectorsReader", Version: version} }

// Lucene90PostingsFormat mirrors org.apache.lucene.backward_codecs.lucene90.Lucene90PostingsFormat.
type Lucene90PostingsFormat struct { Name, Version string }

// NewLucene90PostingsFormat builds a Lucene90PostingsFormat with the supplied version.
func NewLucene90PostingsFormat(version string) *Lucene90PostingsFormat { return &Lucene90PostingsFormat{Name: "Lucene90PostingsFormat", Version: version} }

// Lucene90PostingsReader mirrors org.apache.lucene.backward_codecs.lucene90.Lucene90PostingsReader.
type Lucene90PostingsReader struct { Name, Version string }

// NewLucene90PostingsReader builds a Lucene90PostingsReader with the supplied version.
func NewLucene90PostingsReader(version string) *Lucene90PostingsReader { return &Lucene90PostingsReader{Name: "Lucene90PostingsReader", Version: version} }

// Lucene90PostingsWriter mirrors org.apache.lucene.backward_codecs.lucene90.Lucene90PostingsWriter.
type Lucene90PostingsWriter struct { Name, Version string }

// NewLucene90PostingsWriter builds a Lucene90PostingsWriter with the supplied version.
func NewLucene90PostingsWriter(version string) *Lucene90PostingsWriter { return &Lucene90PostingsWriter{Name: "Lucene90PostingsWriter", Version: version} }

