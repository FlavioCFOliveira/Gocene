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

// WordnetSynonymParser parses the Princeton WordNet prolog format and
// populates a SynonymMap via [analysis.Parser].
//
// Format description: https://wordnet.princeton.edu/documentation/prologdb5wn
//
// Each line has the form:
//
//	s(SynSetID,WordNum,'term',pos,sense,frameCount).
//
// Lines sharing the same SynSetID form one synonym group. Within a group,
// single-quoted terms are extracted. Escaped single quotes (”) are
// collapsed to a literal apostrophe.
//
// This is the Go port of
// org.apache.lucene.analysis.synonym.WordnetSynonymParser from
// Apache Lucene 10.4.0.
type WordnetSynonymParser struct {
	parser *analysis.Parser
	expand bool
}

// NewWordnetSynonymParser constructs a parser.
//
//   - dedup: whether to deduplicate synonym pairs before building
//   - expand: when true every pair (i, j) where i≠j is added; when false each
//     term maps to the first term in the synset
//   - analyzer: tokenises synonym text (may be nil for whitespace split)
func NewWordnetSynonymParser(dedup, expand bool, analyzer analysis.Analyzer) *WordnetSynonymParser {
	return &WordnetSynonymParser{
		parser: analysis.NewParserWithDedup(analyzer, dedup),
		expand: expand,
	}
}

// Parse reads a WordNet prolog stream and registers all synonym groups
// with the underlying Builder.
func (p *WordnetSynonymParser) Parse(r io.Reader) error {
	scanner := bufio.NewScanner(r)
	var synset [][]byte
	lastSynSetID := ""
	lineNum := 0

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		if len(line) < 11 {
			continue
		}
		synSetID := line[2:11]

		if synSetID != lastSynSetID {
			if err := p.addInternal(synset); err != nil {
				return fmt.Errorf("wordnet: line %d: %w", lineNum, err)
			}
			synset = synset[:0]
		}

		term, err := p.parseSynonym(line)
		if err != nil {
			return fmt.Errorf("wordnet: line %d: %w", lineNum, err)
		}
		synset = append(synset, term)
		lastSynSetID = synSetID
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("wordnet: scan: %w", err)
	}
	// Flush final synset.
	return p.addInternal(synset)
}

// Build finalises the synonym map.
func (p *WordnetSynonymParser) Build() (*analysis.SynonymMap, error) {
	return p.parser.Build()
}

// parseSynonym extracts and analyses the single-quoted term from a line.
func (p *WordnetSynonymParser) parseSynonym(line string) ([]byte, error) {
	start := strings.Index(line, "'") + 1
	end := strings.LastIndex(line, "'")
	if start <= 0 || end < start {
		return nil, fmt.Errorf("no quoted term in %q", line)
	}
	text := strings.ReplaceAll(line[start:end], "''", "'")
	return p.parser.Analyze(text)
}

// addInternal registers all pairs in the synset with the Builder.
func (p *WordnetSynonymParser) addInternal(synset [][]byte) error {
	if len(synset) <= 1 {
		return nil
	}
	if p.expand {
		for i := range synset {
			for j := range synset {
				if i != j {
					if err := p.parser.Add(synset[i], synset[j], true); err != nil {
						return err
					}
				}
			}
		}
	} else {
		for i := range synset {
			if err := p.parser.Add(synset[i], synset[0], false); err != nil {
				return err
			}
		}
	}
	return nil
}
