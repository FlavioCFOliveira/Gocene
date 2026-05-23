// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

// TokenInfoDictionaryBuilder builds a TokenInfoDictionary from mecab-ko-dic
// CSV source files.
//
// This is the Go port of
// org.apache.lucene.analysis.ko.dict.TokenInfoDictionaryBuilder from Apache
// Lucene 10.4.0.
//
// Deviation: the Java original performs a full FST construction pass over all
// CSV entries and writes binary output via TokenInfoDictionaryWriter. This Go
// port provides the public struct; actual CSV parsing and FST construction is
// deferred to the nori codec sprint.
type TokenInfoDictionaryBuilder struct {
	encoding       string
	normalizeEntry bool
}

// NewTokenInfoDictionaryBuilder creates a TokenInfoDictionaryBuilder.
//
//   - encoding is the charset name used to read the CSV source files.
//   - normalizeEntry controls whether NFKC normalisation is applied to
//     dictionary surface forms.
func NewTokenInfoDictionaryBuilder(encoding string, normalizeEntry bool) *TokenInfoDictionaryBuilder {
	return &TokenInfoDictionaryBuilder{encoding: encoding, normalizeEntry: normalizeEntry}
}

// Encoding returns the source file encoding.
func (b *TokenInfoDictionaryBuilder) Encoding() string { return b.encoding }

// NormalizeEntry reports whether NFKC normalisation of entries is enabled.
func (b *TokenInfoDictionaryBuilder) NormalizeEntry() bool { return b.normalizeEntry }
