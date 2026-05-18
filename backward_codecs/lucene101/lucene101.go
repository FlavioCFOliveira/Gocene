// Package lucene101 implements org.apache.lucene.backward_codecs.lucene101.
package lucene101

// Codec types in this package are read-only stubs that record the format
// metadata so the codec registry can resolve segments written by the
// matching Lucene version.

// ForDeltaUtil mirrors org.apache.lucene.backward_codecs.lucene101.ForDeltaUtil.
type ForDeltaUtil struct { Name, Version string }

// NewForDeltaUtil builds a ForDeltaUtil with the supplied version.
func NewForDeltaUtil(version string) *ForDeltaUtil { return &ForDeltaUtil{Name: "ForDeltaUtil", Version: version} }

// ForUtil mirrors org.apache.lucene.backward_codecs.lucene101.ForUtil.
type ForUtil struct { Name, Version string }

// NewForUtil builds a ForUtil with the supplied version.
func NewForUtil(version string) *ForUtil { return &ForUtil{Name: "ForUtil", Version: version} }

// Lucene101Codec mirrors org.apache.lucene.backward_codecs.lucene101.Lucene101Codec.
type Lucene101Codec struct { Name, Version string }

// NewLucene101Codec builds a Lucene101Codec with the supplied version.
func NewLucene101Codec(version string) *Lucene101Codec { return &Lucene101Codec{Name: "Lucene101Codec", Version: version} }

// Lucene101PostingsFormat mirrors org.apache.lucene.backward_codecs.lucene101.Lucene101PostingsFormat.
type Lucene101PostingsFormat struct { Name, Version string }

// NewLucene101PostingsFormat builds a Lucene101PostingsFormat with the supplied version.
func NewLucene101PostingsFormat(version string) *Lucene101PostingsFormat { return &Lucene101PostingsFormat{Name: "Lucene101PostingsFormat", Version: version} }

// Lucene101PostingsReader mirrors org.apache.lucene.backward_codecs.lucene101.Lucene101PostingsReader.
type Lucene101PostingsReader struct { Name, Version string }

// NewLucene101PostingsReader builds a Lucene101PostingsReader with the supplied version.
func NewLucene101PostingsReader(version string) *Lucene101PostingsReader { return &Lucene101PostingsReader{Name: "Lucene101PostingsReader", Version: version} }

