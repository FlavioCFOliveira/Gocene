/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package analysis

import (
	"bufio"
	"io"
	"strings"
)

// InitialCapacity is the default initial capacity for word sets.
const initialCapacity = 16

// WordlistLoader is a loader for text files that represent a list of stopwords.
// This is a port of Lucene's WordlistLoader.
type WordlistLoader struct{}

// NewWordlistLoader creates a new WordlistLoader instance.
func NewWordlistLoader() *WordlistLoader {
	return &WordlistLoader{}
}

// GetWordSet reads lines from a reader and adds every non-blank line as an entry
// to a CharArraySet (omitting leading and trailing whitespace).
// Every line of the reader should contain only one word.
// The words need to be in lowercase if you make use of an Analyzer which uses LowerCaseFilter.
func GetWordSet(reader io.Reader, result *CharArraySet) (*CharArraySet, error) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		word := strings.TrimSpace(scanner.Text())
		if word == "" {
			continue
		}
		result.Add(word)
	}
	return result, scanner.Err()
}

// GetWordSetFromReader reads lines from a reader and returns an unmodifiable CharArraySet
// with all non-blank lines (omitting leading and trailing whitespace).
func GetWordSetFromReader(reader io.Reader) (*UnmodifiableCharArraySet, error) {
	set := NewCharArraySet(initialCapacity, false)
	_, err := GetWordSet(reader, set)
	if err != nil {
		return nil, err
	}
	return UnmodifiableSet(set), nil
}

// GetWordSetWithComment reads lines from a reader and adds every non-blank non-comment line
// as an entry to a CharArraySet (omitting leading and trailing whitespace).
// Lines starting with the comment string are ignored.
func GetWordSetWithComment(reader io.Reader, comment string, result *CharArraySet) (*CharArraySet, error) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, comment) {
			continue
		}
		word := strings.TrimSpace(line)
		if word == "" {
			continue
		}
		result.Add(word)
	}
	return result, scanner.Err()
}

// GetWordSetFromReaderWithComment reads lines from a reader and returns an unmodifiable CharArraySet
// with all non-blank, non-comment lines.
func GetWordSetFromReaderWithComment(reader io.Reader, comment string) (*UnmodifiableCharArraySet, error) {
	set := NewCharArraySet(initialCapacity, false)
	_, err := GetWordSetWithComment(reader, comment, set)
	if err != nil {
		return nil, err
	}
	return UnmodifiableSet(set), nil
}

// GetSnowballWordSet reads stopwords from a stopword list in Snowball format.
// The snowball format is:
// - Lines may contain multiple words separated by whitespace
// - The comment character is the vertical line (|)
// - Lines may contain trailing comments
func GetSnowballWordSet(reader io.Reader, result *CharArraySet) (*CharArraySet, error) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		// Remove comments (everything after |)
		if idx := strings.Index(line, "|"); idx >= 0 {
			line = line[:idx]
		}
		// Split by whitespace
		words := strings.Fields(line)
		for _, word := range words {
			if word != "" {
				result.Add(word)
			}
		}
	}
	return result, scanner.Err()
}

// GetSnowballWordSetFromReader reads stopwords from a stopword list in Snowball format
// and returns an unmodifiable CharArraySet.
func GetSnowballWordSetFromReader(reader io.Reader) (*UnmodifiableCharArraySet, error) {
	set := NewCharArraySet(initialCapacity, false)
	_, err := GetSnowballWordSet(reader, set)
	if err != nil {
		return nil, err
	}
	return UnmodifiableSet(set), nil
}

// GetStemDict reads a stem dictionary. Each line contains: word\tstem
// (i.e., two tab-separated words).
// Returns a stem dictionary that overrules the stemming algorithm.
func GetStemDict(reader io.Reader, result *CharArrayMap[string]) (*CharArrayMap[string], error) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) == 2 {
			result.Put(parts[0], parts[1])
		}
	}
	return result, scanner.Err()
}

// GetLines accesses a resource by name and returns the (non comment) lines
// containing data using the given character encoding.
// A comment line is any line that starts with the character "#".
// Returns a list of non-blank non-comment lines with whitespace trimmed.
func GetLines(reader io.Reader) ([]string, error) {
	scanner := bufio.NewScanner(reader)
	lines := make([]string, 0)
	firstLine := true
	bom := "\uFEFF"

	for scanner.Scan() {
		line := scanner.Text()

		// Skip initial BOM marker (U+FEFF)
		if firstLine && strings.HasPrefix(line, bom) {
			line = strings.TrimPrefix(line, bom)
		}
		firstLine = false

		// Skip comments
		if strings.HasPrefix(line, "#") {
			continue
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		lines = append(lines, line)
	}

	return lines, scanner.Err()
}

// GetWordSetFromStrings creates a CharArraySet from a slice of strings.
// This is a convenience function for creating word sets from static data.
func GetWordSetFromStrings(words []string, ignoreCase bool) *CharArraySet {
	set := NewCharArraySet(len(words), ignoreCase)
	for _, word := range words {
		set.Add(word)
	}
	return set
}

// GetWordSetFromStringsUnmodifiable creates an unmodifiable CharArraySet from a slice of strings.
func GetWordSetFromStringsUnmodifiable(words []string, ignoreCase bool) *UnmodifiableCharArraySet {
	set := GetWordSetFromStrings(words, ignoreCase)
	return UnmodifiableSet(set)
}