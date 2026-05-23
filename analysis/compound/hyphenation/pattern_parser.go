// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package hyphenation

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"unicode"
)

// PatternParser parses FOP-style XML hyphenation pattern files and drives a
// PatternConsumer with the parsed data.
//
// This is the Go port of
// org.apache.lucene.analysis.compound.hyphenation.PatternParser from
// Apache Lucene 10.4.0. Taken originally from the Apache FOP project.
//
// Deviation: the Java reference uses a SAX XMLReader. This Go port uses
// encoding/xml streaming events to preserve the same event-driven logic
// without adding external dependencies.
type PatternParser struct {
	consumer    PatternConsumer
	hyphenChar  rune
	currElement int
	token       strings.Builder
	exception   []any
}

const (
	elemClasses   = 1
	elemExceptions = 2
	elemPatterns  = 3
	elemHyphen    = 4
)

// NewPatternParser creates a PatternParser with no consumer.
func NewPatternParser() *PatternParser {
	return &PatternParser{hyphenChar: '-'}
}

// NewPatternParserWithConsumer creates a PatternParser bound to consumer.
func NewPatternParserWithConsumer(consumer PatternConsumer) *PatternParser {
	return &PatternParser{consumer: consumer, hyphenChar: '-'}
}

// SetConsumer sets the PatternConsumer.
func (p *PatternParser) SetConsumer(consumer PatternConsumer) { p.consumer = consumer }

// Parse parses a hyphenation XML document from r, delivering events to the consumer.
func (p *PatternParser) Parse(r io.Reader) error {
	dec := xml.NewDecoder(r)
	dec.AutoClose = xml.HTMLAutoClose
	// Ignore DTD validation — FOP patterns reference an external DTD.
	dec.Entity = map[string]string{}

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("hyphenation: xml parse: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			p.startElement(t)
		case xml.EndElement:
			p.endElement(t)
		case xml.CharData:
			p.characters(string(t))
		}
	}
	return nil
}

func (p *PatternParser) startElement(e xml.StartElement) {
	p.token.Reset()
	switch e.Name.Local {
	case "hyphen-char":
		for _, a := range e.Attr {
			if a.Name.Local == "value" && len(a.Value) == 1 {
				p.hyphenChar = rune(a.Value[0])
			}
		}
	case "classes":
		p.currElement = elemClasses
	case "patterns":
		p.currElement = elemPatterns
	case "exceptions":
		p.currElement = elemExceptions
		p.exception = nil
	case "hyphen":
		if p.token.Len() > 0 {
			p.exception = append(p.exception, p.token.String())
		}
		pre := attrVal(e, "pre")
		no := attrVal(e, "no")
		post := attrVal(e, "post")
		p.exception = append(p.exception, NewHyphen(pre, no, post))
		p.currElement = elemHyphen
	}
}

func attrVal(e xml.StartElement, name string) string {
	for _, a := range e.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

func (p *PatternParser) endElement(e xml.EndElement) {
	if p.token.Len() > 0 {
		word := p.token.String()
		switch p.currElement {
		case elemClasses:
			p.consumer.AddClass(word)
		case elemExceptions:
			p.exception = append(p.exception, word)
			p.exception = p.normalizeException(p.exception)
			p.consumer.AddException(p.getExceptionWord(p.exception), cloneSlice(p.exception))
		case elemPatterns:
			p.consumer.AddPattern(getPattern(word), getInterletterValues(word))
		}
		if p.currElement != elemHyphen {
			p.token.Reset()
		}
	}
	if p.currElement == elemHyphen {
		p.currElement = elemExceptions
	} else {
		p.currElement = 0
	}
}

func (p *PatternParser) characters(s string) {
	chars := []rune(s)
	for len(chars) > 0 {
		word := p.readToken(&chars)
		if word == "" {
			break
		}
		switch p.currElement {
		case elemClasses:
			p.consumer.AddClass(word)
		case elemExceptions:
			p.exception = append(p.exception, word)
			p.exception = p.normalizeException(p.exception)
			p.consumer.AddException(p.getExceptionWord(p.exception), cloneSlice(p.exception))
			p.exception = p.exception[:0]
		case elemPatterns:
			p.consumer.AddPattern(getPattern(word), getInterletterValues(word))
		}
	}
}

// readToken extracts the next whitespace-delimited token from chars, modifying
// it in place to consume the processed runes. Returns "" if no complete token
// is available yet (more data expected).
func (p *PatternParser) readToken(chars *[]rune) string {
	// Skip leading whitespace; if we had a token buffered, return it.
	i := 0
	space := false
	for i < len(*chars) {
		if unicode.IsSpace((*chars)[i]) {
			space = true
		} else {
			break
		}
		i++
	}
	if space {
		*chars = (*chars)[i:]
		if p.token.Len() > 0 {
			word := p.token.String()
			p.token.Reset()
			return word
		}
	}

	// Find end of non-space run.
	j := 0
	for j < len(*chars) && !unicode.IsSpace((*chars)[j]) {
		j++
	}
	p.token.WriteString(string((*chars)[:j]))
	*chars = (*chars)[j:]
	if j < len(*chars)+j && len(*chars) == 0 {
		// no trailing space yet
		return ""
	}
	if j > 0 && (len(*chars) == 0 || unicode.IsSpace((*chars)[0])) {
		// we found a space after the token — return it
		word := p.token.String()
		p.token.Reset()
		return word
	}
	return ""
}

func (p *PatternParser) normalizeException(ex []any) []any {
	res := make([]any, 0, len(ex))
	for _, item := range ex {
		if s, ok := item.(string); ok {
			var buf strings.Builder
			for _, c := range s {
				if c != p.hyphenChar {
					buf.WriteRune(c)
				} else {
					if buf.Len() > 0 {
						res = append(res, buf.String())
						buf.Reset()
					}
					res = append(res, NewHyphenSimple(string(p.hyphenChar)))
				}
			}
			if buf.Len() > 0 {
				res = append(res, buf.String())
			}
		} else {
			res = append(res, item)
		}
	}
	return res
}

func (p *PatternParser) getExceptionWord(ex []any) string {
	var buf strings.Builder
	for _, item := range ex {
		switch v := item.(type) {
		case string:
			buf.WriteString(v)
		case *Hyphen:
			if v.NoBreak != "" {
				buf.WriteString(v.NoBreak)
			}
		}
	}
	return buf.String()
}

// getPattern strips digit characters from pat, returning only the letter part.
func getPattern(word string) string {
	var buf strings.Builder
	for _, c := range word {
		if !unicode.IsDigit(c) {
			buf.WriteRune(c)
		}
	}
	return buf.String()
}

// getInterletterValues extracts the digit interletter weight string from pat.
func getInterletterValues(pat string) string {
	var il strings.Builder
	word := pat + "a" // sentinel
	prev := false
	for _, c := range word {
		if unicode.IsDigit(c) {
			il.WriteRune(c)
			prev = true
		} else {
			if !prev {
				il.WriteRune('0')
			}
			prev = false
		}
	}
	return il.String()
}

func cloneSlice(s []any) []any {
	c := make([]any, len(s))
	copy(c, s)
	return c
}
