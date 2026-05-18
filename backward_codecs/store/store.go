// Package store implements org.apache.lucene.backward_codecs.store.
package store

// Codec types in this package are read-only stubs that record the format
// metadata so the codec registry can resolve segments written by the
// matching Lucene version.

// EndiannessReverserUtil mirrors org.apache.lucene.backward_codecs.store.EndiannessReverserUtil.
type EndiannessReverserUtil struct { Name, Version string }

// NewEndiannessReverserUtil builds a EndiannessReverserUtil with the supplied version.
func NewEndiannessReverserUtil(version string) *EndiannessReverserUtil { return &EndiannessReverserUtil{Name: "EndiannessReverserUtil", Version: version} }

