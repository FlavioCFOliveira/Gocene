// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package ko

import (
	"fmt"
	"io"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis/ko/dict"
)

// KoreanTokenizerFactory creates KoreanTokenizer instances.
//
// Supported args:
//
//   - "decompoundMode"        — "none", "discard" (default), or "mixed".
//   - "outputUnknownUnigrams" — "true" or "false" (default false).
//   - "discardPunctuation"    — "true" (default) or "false".
//
// Deviation: the Java factory also supports "userDictionary" and
// "userDictionaryEncoding" that load a user dictionary from a resource path
// via a ResourceLoader. In Go, callers provide the user dictionary directly
// via the CreateWithUserDict method.
//
// This is the Go port of
// org.apache.lucene.analysis.ko.KoreanTokenizerFactory from Apache Lucene
// 10.4.0.
type KoreanTokenizerFactory struct {
	mode                  DecompoundMode
	outputUnknownUnigrams bool
	discardPunctuation    bool
}

// NewKoreanTokenizerFactory creates a KoreanTokenizerFactory from the given
// args map. Any unrecognised key causes an error.
func NewKoreanTokenizerFactory(args map[string]string) (*KoreanTokenizerFactory, error) {
	f := &KoreanTokenizerFactory{
		mode:               DefaultDecompoundMode,
		discardPunctuation: true,
	}

	if v, ok := args["decompoundMode"]; ok {
		delete(args, "decompoundMode")
		switch strings.ToLower(v) {
		case "none":
			f.mode = DecompoundModeNone
		case "discard":
			f.mode = DecompoundModeDiscard
		case "mixed":
			f.mode = DecompoundModeMixed
		default:
			return nil, fmt.Errorf("KoreanTokenizerFactory: unknown decompoundMode %q; expected none|discard|mixed", v)
		}
	}

	if v, ok := args["outputUnknownUnigrams"]; ok {
		delete(args, "outputUnknownUnigrams")
		f.outputUnknownUnigrams = strings.EqualFold(v, "true")
	}

	if v, ok := args["discardPunctuation"]; ok {
		delete(args, "discardPunctuation")
		f.discardPunctuation = !strings.EqualFold(v, "false")
	}

	if len(args) != 0 {
		return nil, fmt.Errorf("KoreanTokenizerFactory: unknown parameters: %v", args)
	}
	return f, nil
}

// Create returns a KoreanTokenizer with no user dictionary.
func (f *KoreanTokenizerFactory) Create() *KoreanTokenizer {
	return f.CreateWithUserDict(nil)
}

// CreateWithUserDict returns a KoreanTokenizer that reads user-dictionary
// entries from r at startup. Pass nil for no user dictionary.
func (f *KoreanTokenizerFactory) CreateWithUserDict(r io.Reader) *KoreanTokenizer {
	var userDict *dict.UserDictionary
	if r != nil {
		var err error
		userDict, err = dict.Open(r)
		if err != nil {
			userDict = nil
		}
	}
	return NewKoreanTokenizerWithOptions(userDict, f.mode, f.outputUnknownUnigrams, f.discardPunctuation)
}

// Mode returns the configured decompound mode.
func (f *KoreanTokenizerFactory) Mode() DecompoundMode { return f.mode }

// OutputUnknownUnigrams reports whether unknown-word unigram output is enabled.
func (f *KoreanTokenizerFactory) OutputUnknownUnigrams() bool { return f.outputUnknownUnigrams }

// DiscardPunctuation reports whether punctuation tokens are discarded.
func (f *KoreanTokenizerFactory) DiscardPunctuation() bool { return f.discardPunctuation }
