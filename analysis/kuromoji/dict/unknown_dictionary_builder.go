// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package dict

// UnknownDictionaryBuilder builds an UnknownDictionary from source DEF files.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.dict.UnknownDictionaryBuilder from Apache
// Lucene 10.4.0.
//
// Deviation: the Java original reads unk.def and char.def source files and
// writes binary output. This Go port provides the public struct; actual
// building from source files is deferred to the codec sprint.
type UnknownDictionaryBuilder struct {
	encoding string
}

// NewUnknownDictionaryBuilder creates an UnknownDictionaryBuilder.
func NewUnknownDictionaryBuilder(encoding string) *UnknownDictionaryBuilder {
	return &UnknownDictionaryBuilder{encoding: encoding}
}

// Encoding returns the source file encoding.
func (b *UnknownDictionaryBuilder) Encoding() string { return b.encoding }
