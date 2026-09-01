// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package highlighter

import (
	"fmt"
)

// TextFragment represents a fragment of text that is highlighted.
//
// This is the Go port of Lucene's org.apache.lucene.search.highlight.TextFragment.
type TextFragment struct {
	StartOffset int
	StopOffset  int
	Snippet     string
}

func (tf *TextFragment) String() string {
	return fmt.Sprintf("TextFragment(start=%d, stop=%d, snippet=%q)", tf.StartOffset, tf.StopOffset, tf.Snippet)
}

// Formatter is used to format the highlighted fragments.
//
// This is the Go port of Lucene's org.apache.lucene.search.highlight.Formatter.
type Formatter interface {
	// Format formats the given fragment.
	Format(fragment *TextFragment) string
}

// Encoder is used to encode the highlighted terms.
//
// This is the Go port of Lucene's org.apache.lucene.search.highlight.Encoder.
type Encoder interface {
	// Encode encodes the given term.
	Encode(term string) string
}

// Fragmenter is used to break the text into fragments.
//
// This is the Go port of Lucene's org.apache.lucene.search.highlight.Fragmenter.
type Fragmenter interface {
	// Fragment fragments the text.
	Fragment(text string, offsets []int) []*TextFragment
}

// Highlighter is the common base for all highlighter implementations.
//
// This is the Go port of Lucene's org.apache.lucene.search.highlight.Highlighter.
type Highlighter interface {
	// Highlight highlights the given query.
	Highlight(query interface{}, doc int) ([]*TextFragment, error)
}
