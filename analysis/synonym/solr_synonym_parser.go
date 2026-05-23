// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package synonym

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// SolrSynonymParser parses the Solr synonyms format and populates a
// SynonymMap via [analysis.Parser].
//
// Format rules:
//  1. Blank lines and lines beginning with '#' are ignored.
//  2. Explicit mapping: "a, b => c, d" — every input maps to every output.
//     The expand flag is ignored; includeOriginal is always false.
//  3. Equivalent synonyms: "a, b, c" — mapping depends on expand.
//     When expand is true, every pair (i≠j) is added with includeOriginal=true.
//     When expand is false, every term maps to the first term with
//     includeOriginal=false.
//
// This is the Go port of
// org.apache.lucene.analysis.synonym.SolrSynonymParser from
// Apache Lucene 10.4.0.
type SolrSynonymParser struct {
	parser *analysis.Parser
	expand bool
}

// NewSolrSynonymParser constructs a parser.
//
//   - dedup: whether to deduplicate synonym pairs before building
//   - expand: controls bidirectional expansion for implicit mappings
//   - analyzer: tokenises synonym text (may be nil for whitespace split)
func NewSolrSynonymParser(dedup, expand bool, analyzer analysis.Analyzer) *SolrSynonymParser {
	return &SolrSynonymParser{
		parser: analysis.NewParserWithDedup(analyzer, dedup),
		expand: expand,
	}
}

// Parse reads a Solr synonyms stream and registers all mappings with
// the underlying Builder.
func (p *SolrSynonymParser) Parse(r io.Reader) error {
	scanner := bufio.NewScanner(r)
	lineNum := 0
	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		if len(line) == 0 || line[0] == '#' {
			continue
		}

		if err := p.addLine(line); err != nil {
			return fmt.Errorf("solr: line %d: %w", lineNum, err)
		}
	}
	return scanner.Err()
}

// Build finalises the synonym map.
func (p *SolrSynonymParser) Build() (*analysis.SynonymMap, error) {
	return p.parser.Build()
}

// addLine processes one non-empty, non-comment synonym line.
func (p *SolrSynonymParser) addLine(line string) error {
	sides := splitOn(line, "=>")
	if len(sides) > 1 {
		// Explicit mapping.
		if len(sides) != 2 {
			return fmt.Errorf("more than one explicit mapping specified on the same line")
		}
		inputs, err := p.analyzeTerms(sides[0])
		if err != nil {
			return err
		}
		outputs, err := p.analyzeTerms(sides[1])
		if err != nil {
			return err
		}
		for _, in := range inputs {
			for _, out := range outputs {
				if err := p.parser.Add(in, out, false); err != nil {
					return err
				}
			}
		}
		return nil
	}

	// Implicit (equivalent) mapping.
	inputs, err := p.analyzeTerms(line)
	if err != nil {
		return err
	}
	if p.expand {
		for i := range inputs {
			for j := range inputs {
				if i != j {
					if err := p.parser.Add(inputs[i], inputs[j], true); err != nil {
						return err
					}
				}
			}
		}
	} else {
		for i := range inputs {
			if err := p.parser.Add(inputs[i], inputs[0], false); err != nil {
				return err
			}
		}
	}
	return nil
}

// analyzeTerms splits a comma-delimited term list and analyses each entry.
func (p *SolrSynonymParser) analyzeTerms(s string) ([][]byte, error) {
	parts := splitOn(s, ",")
	out := make([][]byte, 0, len(parts))
	for _, part := range parts {
		analyzed, err := p.parser.Analyze(unescape(strings.TrimSpace(part)))
		if err != nil {
			return nil, err
		}
		out = append(out, analyzed)
	}
	return out, nil
}

// splitOn splits s at each non-escaped occurrence of sep, mirroring
// Lucene's SolrSynonymParser.split(). Backslash-escaping is preserved
// in the resulting segments (unescape is applied later).
func splitOn(s, sep string) []string {
	var list []string
	var sb strings.Builder
	pos, end := 0, len(s)
	for pos < end {
		if strings.HasPrefix(s[pos:], sep) {
			if sb.Len() > 0 {
				list = append(list, sb.String())
				sb.Reset()
			}
			pos += len(sep)
			continue
		}
		ch := s[pos]
		pos++
		if ch == '\\' {
			sb.WriteByte(ch)
			if pos < end {
				sb.WriteByte(s[pos])
				pos++
			}
			continue
		}
		sb.WriteByte(ch)
	}
	if sb.Len() > 0 {
		list = append(list, sb.String())
	}
	return list
}

// unescape collapses backslash-escape sequences in the way Lucene's
// SolrSynonymParser.unescape does: each backslash is dropped and the
// following character is kept literally.
func unescape(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '\\' && i < len(s)-1 {
			i++
			sb.WriteByte(s[i])
		} else {
			sb.WriteByte(ch)
		}
	}
	return sb.String()
}
