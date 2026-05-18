// Package spell implements org.apache.lucene.search.spell: spell-checking
// suggesters and the string-distance primitives they rely on.
package spell

// StringDistance is the contract every distance metric must satisfy.
// Mirrors org.apache.lucene.search.spell.StringDistance.
//
// GetDistance returns a value in [0.0, 1.0] where 1.0 means identical and
// 0.0 means completely different.
type StringDistance interface {
	GetDistance(s1, s2 string) float32
}
