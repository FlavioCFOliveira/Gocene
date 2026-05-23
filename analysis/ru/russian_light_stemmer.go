// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package ru hosts the Go port of org.apache.lucene.analysis.ru.
package ru

// RussianLightStemmer implements light stemming for Russian.
//
// This stemmer implements the algorithm described in:
// "Indexing and Searching Strategies for the Russian Language."
// Ljiljana Dolamic and Jacques Savoy.
//
// Go port of org.apache.lucene.analysis.ru.RussianLightStemmer
// (Apache Lucene 10.4.0).
type RussianLightStemmer struct{}

// NewRussianLightStemmer creates a new RussianLightStemmer.
func NewRussianLightStemmer() *RussianLightStemmer {
	return &RussianLightStemmer{}
}

// Stem applies the light Russian stemming algorithm to s[:len] in-place and
// returns the new length.
func (st *RussianLightStemmer) Stem(s []rune, length int) int {
	length = removeCase(s, length)
	return normalize(s, length)
}

func normalize(s []rune, length int) int {
	if length > 3 {
		switch s[length-1] {
		case 'ь', 'и':
			return length - 1
		case 'н':
			if s[length-2] == 'н' {
				return length - 1
			}
		}
	}
	return length
}

func removeCase(s []rune, length int) int {
	if length > 6 {
		if endsWith(s, length, "иями") || endsWith(s, length, "оями") {
			return length - 4
		}
	}

	if length > 5 {
		if endsWith(s, length, "иям") ||
			endsWith(s, length, "иях") ||
			endsWith(s, length, "оях") ||
			endsWith(s, length, "ями") ||
			endsWith(s, length, "оям") ||
			endsWith(s, length, "оьв") ||
			endsWith(s, length, "ами") ||
			endsWith(s, length, "его") ||
			endsWith(s, length, "ему") ||
			endsWith(s, length, "ери") ||
			endsWith(s, length, "ими") ||
			endsWith(s, length, "ого") ||
			endsWith(s, length, "ому") ||
			endsWith(s, length, "ыми") ||
			endsWith(s, length, "оев") {
			return length - 3
		}
	}

	if length > 4 {
		if endsWith(s, length, "ая") ||
			endsWith(s, length, "яя") ||
			endsWith(s, length, "ях") ||
			endsWith(s, length, "юю") ||
			endsWith(s, length, "ах") ||
			endsWith(s, length, "ею") ||
			endsWith(s, length, "их") ||
			endsWith(s, length, "ия") ||
			endsWith(s, length, "ию") ||
			endsWith(s, length, "ьв") ||
			endsWith(s, length, "ою") ||
			endsWith(s, length, "ую") ||
			endsWith(s, length, "ям") ||
			endsWith(s, length, "ых") ||
			endsWith(s, length, "ея") ||
			endsWith(s, length, "ам") ||
			endsWith(s, length, "ем") ||
			endsWith(s, length, "ей") ||
			endsWith(s, length, "ём") ||
			endsWith(s, length, "ев") ||
			endsWith(s, length, "ий") ||
			endsWith(s, length, "им") ||
			endsWith(s, length, "ое") ||
			endsWith(s, length, "ой") ||
			endsWith(s, length, "ом") ||
			endsWith(s, length, "ов") ||
			endsWith(s, length, "ые") ||
			endsWith(s, length, "ый") ||
			endsWith(s, length, "ым") ||
			endsWith(s, length, "ми") {
			return length - 2
		}
	}

	if length > 3 {
		switch s[length-1] {
		case 'а', 'е', 'и', 'о', 'у', 'й', 'ы', 'я', 'ь':
			return length - 1
		}
	}

	return length
}

// endsWith returns true if s[:length] ends with the given suffix string.
func endsWith(s []rune, length int, suffix string) bool {
	suf := []rune(suffix)
	sufLen := len(suf)
	if sufLen > length {
		return false
	}
	for i := sufLen - 1; i >= 0; i-- {
		if s[length-(sufLen-i)] != suf[i] {
			return false
		}
	}
	return true
}
