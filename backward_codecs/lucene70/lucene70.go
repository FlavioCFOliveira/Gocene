// Package lucene70 implements org.apache.lucene.backward_codecs.lucene70.
package lucene70

// Codec types in this package are read-only stubs that record the format
// metadata so the codec registry can resolve segments written by the
// matching Lucene version.

// Lucene70SegmentInfoFormat mirrors org.apache.lucene.backward_codecs.lucene70.Lucene70SegmentInfoFormat.
type Lucene70SegmentInfoFormat struct { Name, Version string }

// NewLucene70SegmentInfoFormat builds a Lucene70SegmentInfoFormat with the supplied version.
func NewLucene70SegmentInfoFormat(version string) *Lucene70SegmentInfoFormat { return &Lucene70SegmentInfoFormat{Name: "Lucene70SegmentInfoFormat", Version: version} }

