// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package ko

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/ko/dict"
)

// KoreanPartOfSpeechStopFilterFactory creates KoreanPartOfSpeechStopFilter
// instances.
//
// Supported args:
//
//	"tags" — comma-separated POS tag names; defaults to DefaultStopTags.
//
// This is the Go port of
// org.apache.lucene.analysis.ko.KoreanPartOfSpeechStopFilterFactory from
// Apache Lucene 10.4.0.
//
// Deviation: Gocene does not have a TokenFilterFactory SPI; this factory is a
// plain struct with a Create method.
type KoreanPartOfSpeechStopFilterFactory struct {
	stopTags map[dict.POSTag]struct{}
}

// NewKoreanPartOfSpeechStopFilterFactory creates the factory from the given
// args map. Recognised key: "tags" (comma-separated POS tag names). Any
// unrecognised key causes an error.
func NewKoreanPartOfSpeechStopFilterFactory(args map[string]string) (*KoreanPartOfSpeechStopFilterFactory, error) {
	stopTags := DefaultStopTags
	if tagsStr, ok := args["tags"]; ok {
		delete(args, "tags")
		stopTags = make(map[dict.POSTag]struct{})
		for _, name := range strings.Split(tagsStr, ",") {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			tag := dict.ResolveTagByName(name)
			if tag == dict.POSTagUNKNOWN {
				return nil, fmt.Errorf("KoreanPartOfSpeechStopFilterFactory: unknown POS tag %q", name)
			}
			stopTags[tag] = struct{}{}
		}
	}
	if len(args) != 0 {
		return nil, fmt.Errorf("KoreanPartOfSpeechStopFilterFactory: unknown parameters: %v", args)
	}
	return &KoreanPartOfSpeechStopFilterFactory{stopTags: stopTags}, nil
}

// Create wraps input in a KoreanPartOfSpeechStopFilter.
func (f *KoreanPartOfSpeechStopFilterFactory) Create(input analysis.TokenStream) analysis.TokenStream {
	return NewKoreanPartOfSpeechStopFilterWithTags(input, f.stopTags)
}
