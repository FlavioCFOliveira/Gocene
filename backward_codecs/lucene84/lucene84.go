// Package lucene84 implements org.apache.lucene.backward_codecs.lucene84.
package lucene84

// Codec types in this package are read-only stubs that record the format
// metadata so the codec registry can resolve segments written by the
// matching Lucene version.

// ForDeltaUtil mirrors org.apache.lucene.backward_codecs.lucene84.ForDeltaUtil.
type ForDeltaUtil struct { Name, Version string }

// NewForDeltaUtil builds a ForDeltaUtil with the supplied version.
func NewForDeltaUtil(version string) *ForDeltaUtil { return &ForDeltaUtil{Name: "ForDeltaUtil", Version: version} }

// PForUtil mirrors org.apache.lucene.backward_codecs.lucene84.PForUtil.
type PForUtil struct { Name, Version string }

// NewPForUtil builds a PForUtil with the supplied version.
func NewPForUtil(version string) *PForUtil { return &PForUtil{Name: "PForUtil", Version: version} }

// Lucene84PostingsFormat mirrors org.apache.lucene.backward_codecs.lucene84.Lucene84PostingsFormat.
type Lucene84PostingsFormat struct { Name, Version string }

// NewLucene84PostingsFormat builds a Lucene84PostingsFormat with the supplied version.
func NewLucene84PostingsFormat(version string) *Lucene84PostingsFormat { return &Lucene84PostingsFormat{Name: "Lucene84PostingsFormat", Version: version} }

// Lucene84PostingsReader mirrors org.apache.lucene.backward_codecs.lucene84.Lucene84PostingsReader.
type Lucene84PostingsReader struct { Name, Version string }

// NewLucene84PostingsReader builds a Lucene84PostingsReader with the supplied version.
func NewLucene84PostingsReader(version string) *Lucene84PostingsReader { return &Lucene84PostingsReader{Name: "Lucene84PostingsReader", Version: version} }

// Lucene84Codec mirrors org.apache.lucene.backward_codecs.lucene84.Lucene84Codec.
type Lucene84Codec struct { Name, Version string }

// NewLucene84Codec builds a Lucene84Codec with the supplied version.
func NewLucene84Codec(version string) *Lucene84Codec { return &Lucene84Codec{Name: "Lucene84Codec", Version: version} }

