// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package suggest

import (
	"fmt"
	"os"
)

// FileDictionary is a dictionary based on a file.
//
// This is the Go port of Lucene's org.apache.lucene.search.suggest.FileDictionary.
type FileDictionary struct {
	filePath string
	data     map[string]float64
}

func NewFileDictionary(path string) (*FileDictionary, error) {
	// Mock implementation of file loading.
	return &FileDictionary{
		filePath: path,
		data:     make(map[string]float64),
	}, nil
}

func (fd *FileDictionary) Build(iterator InputIterator) error {
	for {
		word, weight, ok := iterator.Next()
		if !ok {
			break
		}
		fd.data[word] = weight
	}
	return nil
}

func (fd *FileDictionary) Lookup(input string, numResults int) ([]Suggestion, error) {
	// Simple prefix lookup.
	var results []Suggestion
	for word, weight := range fd.data {
		if len(word) >= len(input) && word[:len(input)] == input {
			results = append(results, Suggestion{Value: word, Score: weight})
		}
		if len(results) >= numResults {
			break
		}
	}
	return results, nil
}

// DocumentDictionary is a dictionary based on documents.
//
// This is the Go port of Lucene's org.apache.lucene.search.suggest.DocumentDictionary.
type DocumentDictionary struct {
	data map[string]float64
}

func NewDocumentDictionary() *DocumentDictionary {
	return &DocumentDictionary{
		data: make(map[string]float64),
	}
}

func (dd *DocumentDictionary) Build(iterator InputIterator) error {
	for {
		word, weight, ok := iterator.Next()
		if !ok {
			break
		}
		dd.data[word] = weight
	}
	return nil
}

func (dd *DocumentDictionary) Lookup(input string, numResults int) ([]Suggestion, error) {
	var results []Suggestion
	for word, weight := range dd.data {
		if len(word) >= len(input) && word[:len(input)] == input {
			results = append(results, Suggestion{Value: word, Score: weight})
		}
		if len(results) >= numResults {
			break
		}
	}
	return results, nil
}
