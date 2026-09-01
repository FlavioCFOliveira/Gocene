// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package highlighter

import (
	"fmt"
	"strings"
)

// DefaultEncoder is a simple encoder that wraps terms in <b> tags.
//
// This is the Go port of Lucene's org.apache.lucene.search.highlight.DefaultEncoder.
type DefaultEncoder struct {
	startTag string
	endTag   string
}

func NewDefaultEncoder() *DefaultEncoder {
	return &DefaultEncoder{
		startTag: "<b>",
		endTag:   "</b>",
	}
}

func (e *DefaultEncoder) Encode(term string) string {
	return e.startTag + term + e.endTag
}

// SimpleHTMLFormatter is a simple HTML formatter.
//
// This is the Go port of Lucene's org.apache.lucene.search.highlight.SimpleHTMLFormatter.
type SimpleHTMLFormatter struct {
	encoder Encoder
}

func NewSimpleHTMLFormatter(encoder Encoder) *SimpleHTMLFormatter {
	return &SimpleHTMLFormatter{
		encoder: encoder,
	}
}

func (f *SimpleHTMLFormatter) Format(fragment *TextFragment) string {
	return fragment.Snippet
}

// SimpleFragmenter is a simple fragmenter.
//
// This is the Go port of Lucene's org.apache.lucene.search.highlight.SimpleFragmenter.
type SimpleFragmenter struct {
	fragmentSize int
}

func NewSimpleFragmenter(fragmentSize int) *SimpleFragmenter {
	return &SimpleFragmenter{
		fragmentSize: fragmentSize,
	}
}

func (f *SimpleFragmenter) Fragment(text string, offsets []int) []*TextFragment {
	if len(offsets) == 0 {
		return nil
	}

	// Simple logic to create one fragment around the first offset.
	start := offsets[0] - f.fragmentSize
	if start < 0 {
		start = 0
	}
	stop := offsets[0] + f.fragmentSize
	if stop > len(text) {
		stop = len(text)
	}

	return []*TextFragment{
		{
			StartOffset: start,
			StopOffset:  stop,
			Snippet:     text[start:stop],
		},
	}
}
