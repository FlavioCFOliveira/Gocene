// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package kuromoji

import (
	"io"

	"github.com/FlavioCFOliveira/Gocene/analysis/kuromoji/dict"
)

// JapaneseTokenizerFactory creates JapaneseTokenizer instances.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.JapaneseTokenizerFactory from Apache Lucene
// 10.4.0.
type JapaneseTokenizerFactory struct {
	mode               Mode
	userDictionary     *dict.UserDictionary
	discardPunctuation bool
	discardCompound    bool
}

// NewJapaneseTokenizerFactory creates a factory with the given options.
func NewJapaneseTokenizerFactory(
	mode Mode,
	userDictionary *dict.UserDictionary,
	discardPunctuation bool,
	discardCompoundToken bool,
) *JapaneseTokenizerFactory {
	return &JapaneseTokenizerFactory{
		mode:               mode,
		userDictionary:     userDictionary,
		discardPunctuation: discardPunctuation,
		discardCompound:    discardCompoundToken,
	}
}

// NewJapaneseTokenizerFactoryDefault creates a factory with default settings:
// ModeSearch, no user dictionary, punctuation discarded, compound discarded.
func NewJapaneseTokenizerFactoryDefault() *JapaneseTokenizerFactory {
	return NewJapaneseTokenizerFactory(DefaultMode, nil, true, true)
}

// Create returns a new JapaneseTokenizer and sets its input to r.
func (f *JapaneseTokenizerFactory) Create(r io.Reader) *JapaneseTokenizer {
	t := NewJapaneseTokenizer(f.userDictionary, f.discardPunctuation, f.discardCompound, f.mode)
	_ = t.SetReader(r)
	return t
}
