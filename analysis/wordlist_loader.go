// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"bufio"
	"io"
	"strings"
)

// WordlistLoader is a utility for loading stopword lists.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.WordlistLoader.
type WordlistLoader struct{}

// GetWordSet reads lines from a reader and returns a set of words.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.WordlistLoader.getWordSet.
func GetWordSet(reader io.Reader) (*CharArraySet, error) {
	return GetWordSetWithComment(reader, "")
}

// GetWordSetWithComment reads lines from a reader and returns a set of words,
// omitting lines that start with the given comment string.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.WordlistLoader.getWordSet(Reader, String, CharArraySet).
func GetWordSetWithComment(reader io.Reader, comment string) (*CharArraySet, error) {
	set := NewCharArraySet(16, false)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if comment != "" && strings.HasPrefix(line, comment) {
			continue
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		set.Add(line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return set, nil
}

// GetSnowballWordSet reads stopwords from a stopword list in Snowball format.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.WordlistLoader.getSnowballWordSet.
func GetSnowballWordSet(reader io.Reader) (*CharArraySet, error) {
	set := NewCharArraySet(16, false)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if idx := strings.Index(line, "|"); idx != -1 {
			line = line[:idx]
		}
		parts := strings.Fields(line)
		for _, p := range parts {
			if p != "" {
				set.Add(p)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return set, nil
}
