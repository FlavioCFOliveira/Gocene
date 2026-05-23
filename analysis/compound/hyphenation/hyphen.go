// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hyphenation

import "fmt"

// Hyphen represents a hyphen with pre-break, post-break and no-break text.
//
// This is the Go port of
// org.apache.lucene.analysis.compound.hyphenation.Hyphen from
// Apache Lucene 10.4.0. Taken originally from the Apache FOP project.
type Hyphen struct {
	PreBreak  string
	NoBreak   string
	PostBreak string
}

// NewHyphen creates a full Hyphen.
func NewHyphen(pre, no, post string) *Hyphen { return &Hyphen{PreBreak: pre, NoBreak: no, PostBreak: post} }

// NewHyphenSimple creates a Hyphen with only a pre-break string.
func NewHyphenSimple(pre string) *Hyphen { return &Hyphen{PreBreak: pre} }

// String returns a human-readable representation of the Hyphen.
func (h *Hyphen) String() string {
	if h.NoBreak == "" && h.PostBreak == "" && h.PreBreak == "-" {
		return "-"
	}
	return fmt.Sprintf("{%s}{%s}{%s}", h.PreBreak, h.PostBreak, h.NoBreak)
}

// Hyphenation holds the result of hyphenating a word: an ordered list of
// hyphenation-point indices within the word.
//
// This is the Go port of
// org.apache.lucene.analysis.compound.hyphenation.Hyphenation from
// Apache Lucene 10.4.0. Taken originally from the Apache FOP project.
type Hyphenation struct {
	hyphenPoints []int
}

// NewHyphenation creates a Hyphenation from a slice of hyphenation points.
func NewHyphenation(points []int) *Hyphenation { return &Hyphenation{hyphenPoints: points} }

// Length returns the number of hyphenation points.
func (h *Hyphenation) Length() int { return len(h.hyphenPoints) }

// GetHyphenationPoints returns the slice of hyphenation points.
func (h *Hyphenation) GetHyphenationPoints() []int { return h.hyphenPoints }
