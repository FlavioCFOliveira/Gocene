// Package blocktree implements org.apache.lucene.backward_codecs.lucene40.blocktree.
package blocktree

// Codec types in this package are read-only stubs that record the format
// metadata so the codec registry can resolve segments written by the
// matching Lucene version.

// Stats mirrors org.apache.lucene.backward_codecs.lucene40.blocktree.Stats.
type Stats struct{ Name, Version string }

// NewStats builds a Stats with the supplied version.
func NewStats(version string) *Stats { return &Stats{Name: "Stats", Version: version} }
