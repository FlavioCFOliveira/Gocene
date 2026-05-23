// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

// DictionaryBuilder builds binary Nori dictionaries from mecab-ko-dic source
// files.
//
// This is the Go port of
// org.apache.lucene.analysis.ko.dict.DictionaryBuilder from Apache Lucene
// 10.4.0.
//
// Deviation: the Java original orchestrates three sub-builders and writes
// codec-formatted binary files to disk. This Go port exposes the public
// contract; actual building from mecab-ko-dic CSV/DEF sources is deferred to
// the nori codec sprint.
type DictionaryBuilder struct {
	encoding       string
	normalizeEntry bool
}

// NewDictionaryBuilder creates a DictionaryBuilder.
func NewDictionaryBuilder(encoding string, normalizeEntry bool) *DictionaryBuilder {
	return &DictionaryBuilder{encoding: encoding, normalizeEntry: normalizeEntry}
}

// Encoding returns the source file encoding.
func (b *DictionaryBuilder) Encoding() string { return b.encoding }

// NormalizeEntry reports whether NFKC normalization of entries is enabled.
func (b *DictionaryBuilder) NormalizeEntry() bool { return b.normalizeEntry }
