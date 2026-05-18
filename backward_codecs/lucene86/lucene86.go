// Package lucene86 implements org.apache.lucene.backward_codecs.lucene86.
package lucene86

// Codec types in this package are read-only stubs that record the format
// metadata so the codec registry can resolve segments written by the
// matching Lucene version.

// Lucene86PointsFormat mirrors org.apache.lucene.backward_codecs.lucene86.Lucene86PointsFormat.
type Lucene86PointsFormat struct { Name, Version string }

// NewLucene86PointsFormat builds a Lucene86PointsFormat with the supplied version.
func NewLucene86PointsFormat(version string) *Lucene86PointsFormat { return &Lucene86PointsFormat{Name: "Lucene86PointsFormat", Version: version} }

// Lucene86PointsReader mirrors org.apache.lucene.backward_codecs.lucene86.Lucene86PointsReader.
type Lucene86PointsReader struct { Name, Version string }

// NewLucene86PointsReader builds a Lucene86PointsReader with the supplied version.
func NewLucene86PointsReader(version string) *Lucene86PointsReader { return &Lucene86PointsReader{Name: "Lucene86PointsReader", Version: version} }

// Lucene86SegmentInfoFormat mirrors org.apache.lucene.backward_codecs.lucene86.Lucene86SegmentInfoFormat.
type Lucene86SegmentInfoFormat struct { Name, Version string }

// NewLucene86SegmentInfoFormat builds a Lucene86SegmentInfoFormat with the supplied version.
func NewLucene86SegmentInfoFormat(version string) *Lucene86SegmentInfoFormat { return &Lucene86SegmentInfoFormat{Name: "Lucene86SegmentInfoFormat", Version: version} }

// Lucene86Codec mirrors org.apache.lucene.backward_codecs.lucene86.Lucene86Codec.
type Lucene86Codec struct { Name, Version string }

// NewLucene86Codec builds a Lucene86Codec with the supplied version.
func NewLucene86Codec(version string) *Lucene86Codec { return &Lucene86Codec{Name: "Lucene86Codec", Version: version} }

