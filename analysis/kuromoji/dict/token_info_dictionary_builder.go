// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

// TokenInfoDictionaryBuilder builds a TokenInfoDictionary from IPADIC/UNIDIC
// CSV source files.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.dict.TokenInfoDictionaryBuilder from Apache
// Lucene 10.4.0.
//
// Deviation: the Java original performs a full FST construction pass over all
// CSV entries. This Go port exposes the public contract; actual CSV parsing
// and FST construction is deferred to the codec sprint.
type TokenInfoDictionaryBuilder struct {
	format         DictionaryFormat
	encoding       string
	normalizeEntry bool
}

// NewTokenInfoDictionaryBuilder creates a TokenInfoDictionaryBuilder.
func NewTokenInfoDictionaryBuilder(format DictionaryFormat, encoding string, normalizeEntry bool) *TokenInfoDictionaryBuilder {
	return &TokenInfoDictionaryBuilder{
		format:         format,
		encoding:       encoding,
		normalizeEntry: normalizeEntry,
	}
}

// Format returns the dictionary format.
func (b *TokenInfoDictionaryBuilder) Format() DictionaryFormat { return b.format }

// Encoding returns the source file encoding.
func (b *TokenInfoDictionaryBuilder) Encoding() string { return b.encoding }

// NormalizeEntry reports whether NFC normalization of entries is enabled.
func (b *TokenInfoDictionaryBuilder) NormalizeEntry() bool { return b.normalizeEntry }
