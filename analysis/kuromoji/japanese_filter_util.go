// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package kuromoji

// JapaneseFilterUtil provides utility functions for Japanese analysis filters.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.JapaneseFilterUtil from Apache Lucene 10.4.0.
type JapaneseFilterUtil struct{}

// CreateCharMap creates a map from individual rune pairs.
// It is the Go analogue of JapaneseFilterUtil.createCharMap in the Java
// source, which used the HPPC library.
func CreateCharMap(pairs [][2]rune) map[rune]rune {
	m := make(map[rune]rune, len(pairs))
	for _, p := range pairs {
		m[p[0]] = p[1]
	}
	return m
}
