// Package blocktree implements org.apache.lucene.backward_codecs.lucene40.blocktree.
package blocktree

// Codec types in this package are read-only stubs that record the format
// metadata so the codec registry can resolve segments written by the
// matching Lucene version.

// FieldReader mirrors org.apache.lucene.backward_codecs.lucene40.blocktree.FieldReader.
type FieldReader struct { Name, Version string }

// NewFieldReader builds a FieldReader with the supplied version.
func NewFieldReader(version string) *FieldReader { return &FieldReader{Name: "FieldReader", Version: version} }

// Stats mirrors org.apache.lucene.backward_codecs.lucene40.blocktree.Stats.
type Stats struct { Name, Version string }

// NewStats builds a Stats with the supplied version.
func NewStats(version string) *Stats { return &Stats{Name: "Stats", Version: version} }

// Lucene40BlockTreeTermsReader mirrors org.apache.lucene.backward_codecs.lucene40.blocktree.Lucene40BlockTreeTermsReader.
type Lucene40BlockTreeTermsReader struct { Name, Version string }

// NewLucene40BlockTreeTermsReader builds a Lucene40BlockTreeTermsReader with the supplied version.
func NewLucene40BlockTreeTermsReader(version string) *Lucene40BlockTreeTermsReader { return &Lucene40BlockTreeTermsReader{Name: "Lucene40BlockTreeTermsReader", Version: version} }

