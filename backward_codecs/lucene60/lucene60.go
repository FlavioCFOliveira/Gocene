// Package lucene60 implements org.apache.lucene.backward_codecs.lucene60.
package lucene60

// Codec types in this package are read-only stubs that record the format
// metadata so the codec registry can resolve segments written by the
// matching Lucene version.

// Lucene60PointsFormat mirrors org.apache.lucene.backward_codecs.lucene60.Lucene60PointsFormat.
type Lucene60PointsFormat struct { Name, Version string }

// NewLucene60PointsFormat builds a Lucene60PointsFormat with the supplied version.
func NewLucene60PointsFormat(version string) *Lucene60PointsFormat { return &Lucene60PointsFormat{Name: "Lucene60PointsFormat", Version: version} }

// Lucene60FieldInfosFormat mirrors org.apache.lucene.backward_codecs.lucene60.Lucene60FieldInfosFormat.
type Lucene60FieldInfosFormat struct { Name, Version string }

// NewLucene60FieldInfosFormat builds a Lucene60FieldInfosFormat with the supplied version.
func NewLucene60FieldInfosFormat(version string) *Lucene60FieldInfosFormat { return &Lucene60FieldInfosFormat{Name: "Lucene60FieldInfosFormat", Version: version} }

// Lucene60PointsReader mirrors org.apache.lucene.backward_codecs.lucene60.Lucene60PointsReader.
type Lucene60PointsReader struct { Name, Version string }

// NewLucene60PointsReader builds a Lucene60PointsReader with the supplied version.
func NewLucene60PointsReader(version string) *Lucene60PointsReader { return &Lucene60PointsReader{Name: "Lucene60PointsReader", Version: version} }

