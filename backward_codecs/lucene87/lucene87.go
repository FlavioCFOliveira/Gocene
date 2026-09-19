// Package lucene87 implements org.apache.lucene.backward_codecs.lucene87.
package lucene87

// Codec types in this package are read-only stubs that record the format
// metadata so the codec registry can resolve segments written by the
// matching Lucene version.

// Lucene87Codec mirrors org.apache.lucene.backward_codecs.lucene87.Lucene87Codec.
type Lucene87Codec struct{ Name, Version string }

// NewLucene87Codec builds a Lucene87Codec with the supplied version.
func NewLucene87Codec(version string) *Lucene87Codec {
	return &Lucene87Codec{Name: "Lucene87Codec", Version: version}
}
