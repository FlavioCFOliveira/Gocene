// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hyphenation

// PatternConsumer is the callback interface used by PatternParser to deliver
// parsed hyphenation-pattern data to a consumer (typically HyphenationTree).
//
// This is the Go port of
// org.apache.lucene.analysis.compound.hyphenation.PatternConsumer from
// Apache Lucene 10.4.0. Taken originally from the Apache FOP project.
type PatternConsumer interface {
	// AddClass adds a character equivalence class for hyphenation lookup
	// (e.g. "aA" means 'a' and 'A' are equivalent).
	AddClass(chargroup string)

	// AddException adds a hand-crafted hyphenation exception for word.
	// hyphenatedWord is a slice of alternating strings and *Hyphen values.
	AddException(word string, hyphenatedWord []any)

	// AddPattern adds a hyphenation pattern with its interletter digit string.
	AddPattern(pattern, values string)
}
