// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.
//
// Port of org.apache.lucene.tests.analysis.TokenStreamToDot from
// Apache Lucene 10.4.0 (test-framework).

package analysis

import (
	"fmt"
	"io"
)

// TokenStreamToDot consumes a TokenStream and writes Graphviz DOT format.
//
// Port of org.apache.lucene.tests.analysis.TokenStreamToDot.
type TokenStreamToDot struct {
	in        TokenStream
	inputText string
	out       io.Writer
}

// NewTokenStreamToDot creates a TokenStreamToDot.
// If inputText is non-empty and the stream has offsets, the surface form
// is included in each arc's label.
func NewTokenStreamToDot(inputText string, in TokenStream, out io.Writer) *TokenStreamToDot {
	return &TokenStreamToDot{in: in, inputText: inputText, out: out}
}

// ToDot processes the TokenStream and writes the full DOT graph.
func (t *TokenStreamToDot) ToDot() error {
	// Reset if the stream supports it.
	type resetter interface{ Reset() error }
	if r, ok := t.in.(resetter); ok {
		if err := r.Reset(); err != nil {
			return err
		}
	}
	t.writeHeader()

	// Get attribute source via BaseTokenStream if available.
	type attrSourceProvider interface{ GetAttributeSource() interface{ GetAttribute(string) interface{} } }
	var termAttr CharTermAttribute
	var posIncrAttr PositionIncrementAttribute
	var posLenAttr PositionLengthAttribute
	var offsetAttr OffsetAttribute

	// Use type assertions to access attribute-typed helpers directly.
	if bts, ok := t.in.(*BaseTokenStream); ok && bts != nil {
		src := bts.GetAttributeSource()
		if src != nil {
			termAttr, _ = src.GetAttribute(CharTermAttributeType).(CharTermAttribute)
			posIncrAttr, _ = src.GetAttribute(PositionIncrementAttributeType).(PositionIncrementAttribute)
			posLenAttr, _ = src.GetAttribute(PositionLengthAttributeType).(PositionLengthAttribute)
			offsetAttr, _ = src.GetAttribute(OffsetAttributeType).(OffsetAttribute)
		}
	}
	_ = attrSourceProvider(nil)

	pos := -1
	lastEndPos := -1
	for {
		more, err := t.in.IncrementToken()
		if err != nil {
			return err
		}
		if !more {
			break
		}

		isFirst := pos == -1
		posInc := 1
		if posIncrAttr != nil {
			posInc = posIncrAttr.GetPositionIncrement()
		}
		if isFirst && posInc == 0 {
			posInc = 1
		}

		if posInc > 0 {
			pos += posInc
			t.writeNode(pos, fmt.Sprintf("%d", pos))
		}

		if posInc > 1 {
			t.writeArc(lastEndPos, pos, "", "dotted")
		}

		if isFirst {
			t.writeNode(-1, "")
			t.writeArc(-1, pos, "", "")
		}

		arcLabel := ""
		if termAttr != nil {
			arcLabel = termAttr.String()
		}

		if offsetAttr != nil {
			startOffset := offsetAttr.StartOffset()
			endOffset := offsetAttr.EndOffset()
			if t.inputText != "" && endOffset <= len(t.inputText) {
				fragment := t.inputText[startOffset:endOffset]
				if fragment != arcLabel {
					arcLabel += " / " + fragment
				}
			} else {
				arcLabel += fmt.Sprintf(" / %d-%d", startOffset, endOffset)
			}
		}

		posLen := 1
		if posLenAttr != nil {
			posLen = posLenAttr.GetPositionLength()
		}
		t.writeArc(pos, pos+posLen, arcLabel, "")
		lastEndPos = pos + posLen
	}

	_ = t.in.End()

	if lastEndPos != -1 {
		t.writeNode(-2, "")
		t.writeArc(lastEndPos, -2, "", "")
	}

	t.writeTrailer()
	return nil
}

func (t *TokenStreamToDot) writeArc(fromNode, toNode int, label, style string) {
	fmt.Fprintf(t.out, "  %d -> %d [", fromNode, toNode)
	if label != "" {
		fmt.Fprintf(t.out, " label=%q", label)
	}
	if style != "" {
		fmt.Fprintf(t.out, " style=%q", style)
	}
	fmt.Fprintln(t.out, "]")
}

func (t *TokenStreamToDot) writeNode(name int, label string) {
	fmt.Fprintf(t.out, "  %d", name)
	if label != "" {
		fmt.Fprintf(t.out, " [label=%q]", label)
	} else {
		fmt.Fprint(t.out, " [shape=point color=white]")
	}
	fmt.Fprintln(t.out)
}

const fontName = "Helvetica"

func (t *TokenStreamToDot) writeHeader() {
	fmt.Fprintln(t.out, "digraph tokens {")
	fmt.Fprintln(t.out, `  graph [ fontsize=30 labelloc="t" label="" splines=true overlap=false rankdir = "LR" ];`)
	fmt.Fprintln(t.out, `  // A2 paper size`)
	fmt.Fprintln(t.out, `  size = "34.4,16.5";`)
	fmt.Fprintf(t.out, "  edge [ fontname=%q fontcolor=\"red\" color=\"#606060\" ]\n", fontName)
	fmt.Fprintf(t.out, "  node [ style=\"filled\" fillcolor=\"#e8e8f0\" shape=\"Mrecord\" fontname=%q ]\n", fontName)
	fmt.Fprintln(t.out)
}

func (t *TokenStreamToDot) writeTrailer() {
	fmt.Fprintln(t.out, "}")
}
