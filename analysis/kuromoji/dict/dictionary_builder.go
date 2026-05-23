// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

// DictionaryFormat identifies the source dictionary format.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.dict.DictionaryBuilder.DictionaryFormat from
// Apache Lucene 10.4.0.
type DictionaryFormat int

const (
	// DictionaryFormatIPADIC is the IPADIC dictionary format.
	DictionaryFormatIPADIC DictionaryFormat = iota
	// DictionaryFormatUNIDIC is the UNIDIC dictionary format.
	DictionaryFormatUNIDIC
)

// DictionaryBuilder builds binary kuromoji dictionaries from source CSV/DEF
// files.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.dict.DictionaryBuilder from Apache Lucene
// 10.4.0.
//
// Deviation: the Java original orchestrates three sub-builders and writes
// binary files to disk. This Go port exposes the public contract; actual
// building from IPADIC/UNIDIC CSV sources is deferred to the codec sprint.
type DictionaryBuilder struct {
	format         DictionaryFormat
	encoding       string
	normalizeEntry bool
}

// NewDictionaryBuilder creates a DictionaryBuilder for the given format.
func NewDictionaryBuilder(format DictionaryFormat, encoding string, normalizeEntry bool) *DictionaryBuilder {
	return &DictionaryBuilder{
		format:         format,
		encoding:       encoding,
		normalizeEntry: normalizeEntry,
	}
}

// Format returns the dictionary format.
func (b *DictionaryBuilder) Format() DictionaryFormat { return b.format }

// Encoding returns the source file encoding.
func (b *DictionaryBuilder) Encoding() string { return b.encoding }

// NormalizeEntry reports whether NFC normalization of entries is enabled.
func (b *DictionaryBuilder) NormalizeEntry() bool { return b.normalizeEntry }
