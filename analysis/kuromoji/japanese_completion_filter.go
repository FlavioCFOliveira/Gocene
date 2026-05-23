// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package kuromoji

import (
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/kuromoji/completion"
	"github.com/FlavioCFOliveira/Gocene/analysis/kuromoji/tokenattributes"
)

// CompletionMode controls how JapaneseCompletionFilter romanizes tokens.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.JapaneseCompletionFilter.Mode from Apache
// Lucene 10.4.0.
type CompletionMode int

const (
	// CompletionModeIndex performs simple romanization, expected for indexing.
	CompletionModeIndex CompletionMode = iota
	// CompletionModeQuery performs Input Method aware romanization, expected
	// for querying.
	CompletionModeQuery
)

// DefaultCompletionMode is the default completion mode.
const DefaultCompletionMode = CompletionModeIndex

// completionToken holds a single generated output token.
type completionToken struct {
	term        string
	isFirst     bool
	startOffset int
	endOffset   int
}

// completionTokenGenerator holds pending token state and generates output
// tokens.
type completionTokenGenerator struct {
	mode    CompletionMode
	outputs []completionToken

	pdgSurface      strings.Builder
	pdgReading      strings.Builder
	hasPdg          bool
	pdgStartOffset  int
	pdgEndOffset    int
}

func newCompletionTokenGenerator(mode CompletionMode) *completionTokenGenerator {
	return &completionTokenGenerator{mode: mode}
}

func (g *completionTokenGenerator) reset() {
	g.clearPending()
	g.outputs = g.outputs[:0]
}

func (g *completionTokenGenerator) hasNext() bool { return len(g.outputs) > 0 }

func (g *completionTokenGenerator) next() completionToken {
	tok := g.outputs[0]
	g.outputs = g.outputs[1:]
	return tok
}

func (g *completionTokenGenerator) hasPendingToken() bool { return g.hasPdg }

func (g *completionTokenGenerator) addToken(surface, reading string, startOffset, endOffset int) {
	if g.hasPdg {
		pdgSurf := g.pdgSurface.String()
		if g.mode == CompletionModeQuery &&
			g.pdgReading.Len() > 0 &&
			!completion.IsLowercaseAlphabets(pdgSurf) &&
			completion.IsLowercaseAlphabets(surface) {
			// Mid-IME composition: concatenate.
			g.pdgSurface.WriteString(surface)
			g.pdgReading.WriteString(surface)
			g.pdgEndOffset = endOffset
			g.generateOutputs()
			g.clearPending()
		} else if g.mode == CompletionModeQuery &&
			completion.IsKana(pdgSurf) &&
			completion.IsKana(surface) {
			// Concatenate kana tokens in query mode.
			g.pdgSurface.WriteString(surface)
			if reading != "" {
				g.pdgReading.WriteString(reading)
			} else {
				g.pdgReading.WriteString(surface)
			}
			g.pdgEndOffset = endOffset
		} else {
			g.generateOutputs()
			g.resetPending(surface, reading, startOffset, endOffset)
		}
	} else {
		g.resetPending(surface, reading, startOffset, endOffset)
	}
}

func (g *completionTokenGenerator) finish() {
	g.generateOutputs()
	g.clearPending()
}

func (g *completionTokenGenerator) generateOutputs() {
	surf := g.pdgSurface.String()
	g.outputs = append(g.outputs, completionToken{
		term:        surf,
		isFirst:     true,
		startOffset: g.pdgStartOffset,
		endOffset:   g.pdgEndOffset,
	})
	reading := g.pdgReading.String()
	if reading == "" || !completion.IsKatakanaOrHWAlphabets(reading) {
		return
	}
	romajis := completion.GetInstance().Romanize(reading)
	for _, r := range romajis {
		g.outputs = append(g.outputs, completionToken{
			term:        r,
			isFirst:     false,
			startOffset: g.pdgStartOffset,
			endOffset:   g.pdgEndOffset,
		})
	}
}

func (g *completionTokenGenerator) resetPending(surface, reading string, startOffset, endOffset int) {
	g.pdgSurface.Reset()
	g.pdgSurface.WriteString(surface)
	g.pdgReading.Reset()
	if reading != "" {
		g.pdgReading.WriteString(reading)
	} else {
		g.pdgReading.WriteString(surface)
	}
	g.pdgStartOffset = startOffset
	g.pdgEndOffset = endOffset
	g.hasPdg = true
}

func (g *completionTokenGenerator) clearPending() {
	g.pdgSurface.Reset()
	g.pdgReading.Reset()
	g.pdgStartOffset = 0
	g.pdgEndOffset = 0
	g.hasPdg = false
}

// JapaneseCompletionFilter adds romanized tokens to the term attribute stream.
// The original surface form is also preserved. Main usage is query
// auto-completion.
//
// This is the Go port of
// org.apache.lucene.analysis.ja.JapaneseCompletionFilter from Apache Lucene
// 10.4.0.
type JapaneseCompletionFilter struct {
	*analysis.BaseTokenFilter
	termAttr    analysis.CharTermAttribute
	offsetAttr  analysis.OffsetAttribute
	posIncrAttr analysis.PositionIncrementAttribute
	readingAttr tokenattributes.ReadingAttribute

	generator          *completionTokenGenerator
	inputStreamConsumed bool
}

// NewJapaneseCompletionFilter creates a JapaneseCompletionFilter with the
// given mode.
func NewJapaneseCompletionFilter(input analysis.TokenStream, mode CompletionMode) *JapaneseCompletionFilter {
	f := &JapaneseCompletionFilter{
		BaseTokenFilter: analysis.NewBaseTokenFilter(input),
		generator:       newCompletionTokenGenerator(mode),
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttribute(analysis.CharTermAttributeType); a != nil {
			f.termAttr = a.(analysis.CharTermAttribute)
		}
		if a := src.GetAttribute(analysis.OffsetAttributeType); a != nil {
			f.offsetAttr = a.(analysis.OffsetAttribute)
		}
		if a := src.GetAttribute(analysis.PositionIncrementAttributeType); a != nil {
			f.posIncrAttr = a.(analysis.PositionIncrementAttribute)
		}
		if a := src.GetAttribute(tokenattributes.ReadingAttributeType); a != nil {
			f.readingAttr = a.(tokenattributes.ReadingAttribute)
		}
	}
	return f
}

// NewJapaneseCompletionFilterDefault creates a JapaneseCompletionFilter in
// index mode.
func NewJapaneseCompletionFilterDefault(input analysis.TokenStream) *JapaneseCompletionFilter {
	return NewJapaneseCompletionFilter(input, DefaultCompletionMode)
}

// Reset resets the filter.
func (f *JapaneseCompletionFilter) Reset() error {
	if r, ok := f.GetInput().(interface{ Reset() error }); ok {
		if err := r.Reset(); err != nil {
			return err
		}
	}
	f.generator.reset()
	f.inputStreamConsumed = false
	return nil
}

// IncrementToken advances to the next token.
func (f *JapaneseCompletionFilter) IncrementToken() (bool, error) {
	if err := f.mayIncrementToken(); err != nil {
		return false, err
	}
	if !f.generator.hasNext() {
		return false, nil
	}
	tok := f.generator.next()
	if f.termAttr != nil {
		f.termAttr.SetValue(tok.term)
	}
	if f.posIncrAttr != nil {
		if tok.isFirst {
			f.posIncrAttr.SetPositionIncrement(1)
		} else {
			f.posIncrAttr.SetPositionIncrement(0)
		}
	}
	if f.offsetAttr != nil {
		f.offsetAttr.SetOffset(tok.startOffset, tok.endOffset)
	}
	return true, nil
}

func (f *JapaneseCompletionFilter) mayIncrementToken() error {
	for !f.generator.hasNext() {
		if !f.inputStreamConsumed {
			ok, err := f.GetInput().IncrementToken()
			if err != nil {
				return err
			}
			if ok {
				var surface, reading string
				startOffset, endOffset := 0, 0
				if f.termAttr != nil {
					surface = f.termAttr.String()
				}
				if f.readingAttr != nil {
					reading = f.readingAttr.Reading()
				}
				if reading == "" && completion.IsKana(surface) {
					reading = completion.ToKatakana(surface)
				}
				if f.offsetAttr != nil {
					startOffset = f.offsetAttr.StartOffset()
					endOffset = f.offsetAttr.EndOffset()
				}
				f.generator.addToken(surface, reading, startOffset, endOffset)
				continue
			}
		}
		f.inputStreamConsumed = true
		if f.generator.hasPendingToken() {
			f.generator.finish()
		} else {
			break
		}
	}
	return nil
}

// Ensure JapaneseCompletionFilter implements analysis.TokenFilter.
var _ analysis.TokenFilter = (*JapaneseCompletionFilter)(nil)
