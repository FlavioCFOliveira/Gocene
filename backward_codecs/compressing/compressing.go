// Package compressing implements org.apache.lucene.backward_codecs.compressing.
package compressing

// Codec types in this package are read-only stubs that record the format
// metadata so the codec registry can resolve segments written by the
// matching Lucene version.

// MatchingReaders mirrors org.apache.lucene.backward_codecs.compressing.MatchingReaders.
type MatchingReaders struct{ Name, Version string }

// NewMatchingReaders builds a MatchingReaders with the supplied version.
func NewMatchingReaders(version string) *MatchingReaders {
	return &MatchingReaders{Name: "MatchingReaders", Version: version}
}
