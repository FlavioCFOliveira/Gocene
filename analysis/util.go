// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

// GetWordSetFromStrings converts a slice of strings into a CharArraySet.
// This is a utility function used by analyzers to create stop word sets.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.util.CharArraySet.
func GetWordSetFromStrings(words []string, ignoreCase bool) *CharArraySet {
	set := NewCharArraySet(len(words), ignoreCase)
	for _, word := range words {
		set.Add(word)
	}
	return set
}
