// Package lucene92 implements org.apache.lucene.backward_codecs.lucene92.
package lucene92

// Codec types in this package are read-only stubs that record the format
// metadata so the codec registry can resolve segments written by the
// matching Lucene version.


// Lucene92Codec mirrors org.apache.lucene.backward_codecs.lucene92.Lucene92Codec.
type Lucene92Codec struct{ Name, Version string }

// NewLucene92Codec builds a Lucene92Codec with the supplied version.
func NewLucene92Codec(version string) *Lucene92Codec {
	return &Lucene92Codec{Name: "Lucene92Codec", Version: version}
}

