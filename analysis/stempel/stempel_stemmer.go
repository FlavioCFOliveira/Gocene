// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Package stempel provides the Stempel algorithmic stemmer for Polish and
// other languages using the Egothor stemmer tables.
package stempel

import (
	"bufio"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/FlavioCFOliveira/Gocene/analysis/egothor"
)

// StempelStemmer is a convenient facade for the Egothor stemmer. It loads a
// stemmer table from a stream and applies patch commands to reduce words to
// their stems.
//
// This is the Go port of
// org.apache.lucene.analysis.stempel.StempelStemmer (Lucene 10.4.0).
type StempelStemmer struct {
	stemmer egothorTrie
}

// egothorTrie is the subset of the Trie/MultiTrie2 API used by StempelStemmer.
type egothorTrie interface {
	GetLastOnPath(key []rune) string
}

// NewStempelStemmerFromStream creates a StempelStemmer by loading the stemmer
// table from r.
func NewStempelStemmerFromStream(r io.Reader) (*StempelStemmer, error) {
	trie, err := Load(r)
	if err != nil {
		return nil, err
	}
	return &StempelStemmer{stemmer: trie}, nil
}

// NewStempelStemmer creates a StempelStemmer using a pre-loaded trie.
func NewStempelStemmer(trie egothorTrie) *StempelStemmer {
	return &StempelStemmer{stemmer: trie}
}

// Load reads and returns a stemmer Trie (or MultiTrie2) from an io.Reader.
// The stream must be in Java DataInput format. The method identifier in the
// header (read via readUTF) determines whether to use a MultiTrie2 (contains
// 'M') or a plain Trie.
//
// This mirrors org.apache.lucene.analysis.stempel.StempelStemmer.load().
func Load(r io.Reader) (egothorTrie, error) {
	br := bufio.NewReader(r)
	method, err := readUTFFromReader(br)
	if err != nil {
		return nil, err
	}
	method = strings.ToUpper(method)
	if strings.ContainsRune(method, 'M') {
		return egothor.NewMultiTrie2FromReader(br)
	}
	return egothor.NewTrieFromReader(br)
}

// Stem reduces word to its root using the loaded stemmer table.
// Returns the stemmed word, or "" if no stem could be found.
func (s *StempelStemmer) Stem(word []rune) []rune {
	cmd := s.stemmer.GetLastOnPath(word)
	if cmd == "" {
		return nil
	}
	result := make([]rune, len(word))
	copy(result, word)
	egothor.ApplyToBuilder(&result, cmd)
	if len(result) > 0 {
		return result
	}
	return nil
}

// StemString stems a string word and returns the stem, or "" if no stem.
func (s *StempelStemmer) StemString(word string) string {
	runes := []rune(word)
	stem := s.Stem(runes)
	if stem == nil {
		return ""
	}
	return string(stem)
}

// readUTFFromReader reads a Java modified-UTF-8 string from a bufio.Reader.
// The string is prefixed with a big-endian uint16 length.
func readUTFFromReader(r *bufio.Reader) (string, error) {
	hi, err := r.ReadByte()
	if err != nil {
		return "", err
	}
	lo, err := r.ReadByte()
	if err != nil {
		return "", err
	}
	length := int(hi)<<8 | int(lo)
	if length == 0 {
		return "", nil
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return decodeModifiedUTF8(buf), nil
}

// decodeModifiedUTF8 converts Java modified-UTF-8 bytes to a Go string.
func decodeModifiedUTF8(b []byte) string {
	var runes []rune
	for i := 0; i < len(b); {
		c := b[i]
		switch {
		case c&0x80 == 0:
			runes = append(runes, rune(c))
			i++
		case c&0xE0 == 0xC0 && i+1 < len(b):
			r := rune(c&0x1F)<<6 | rune(b[i+1]&0x3F)
			runes = append(runes, r)
			i += 2
		case c&0xF0 == 0xE0 && i+2 < len(b):
			r := rune(c&0x0F)<<12 | rune(b[i+1]&0x3F)<<6 | rune(b[i+2]&0x3F)
			runes = append(runes, r)
			i += 3
		default:
			r, size := utf8.DecodeRune(b[i:])
			runes = append(runes, r)
			i += size
		}
	}
	return string(runes)
}
