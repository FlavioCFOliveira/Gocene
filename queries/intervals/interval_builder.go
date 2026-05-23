// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Ported from Apache Lucene 10.4.0:
//   lucene/queries/src/java/org/apache/lucene/queries/intervals/IntervalBuilder.java

package intervals

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
)

// noIntervalsSource is the sentinel returned when no terms are found.
var noIntervalsSource IntervalsSource = NewNoMatchIntervalsSource("No terms in analyzed text")

// AnalyzeText constructs an IntervalsSource from an analyzed text stream.
// It handles the simple phrase case (no synonyms, no graph).
//
// Mirrors org.apache.lucene.queries.intervals.IntervalBuilder.analyzeText.
//
// Deviations from Java:
//   - Graph token streams (hasSidePath) are not handled; returns an error.
//   - Uses analysis.CachingTokenFilter.
func AnalyzeText(stream *analysis.CachingTokenFilter, maxGaps int, ordered bool) (IntervalsSource, error) {
	if err := stream.Reset(); err != nil {
		return nil, err
	}

	// Phase 1: count tokens and detect synonyms/graphs.
	numTokens := 0
	hasSynonyms := false

	for {
		ok, err := stream.IncrementToken()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		numTokens++
		posIncAttr := stream.GetAttribute("PositionIncrementAttribute")
		if posIncAttr != nil {
			if pa, ok := posIncAttr.(analysis.PositionIncrementAttribute); ok {
				if pa.GetPositionIncrement() == 0 {
					hasSynonyms = true
				}
			}
		}
	}

	if numTokens == 0 {
		return noIntervalsSource, nil
	}

	// Phase 2: build source.
	if numTokens == 1 {
		return analyzeSingleTerm(stream)
	}
	if hasSynonyms {
		return analyzeSynonyms(stream, maxGaps, ordered)
	}
	return analyzeSimplePhrase(stream, maxGaps, ordered)
}

func analyzeSingleTerm(stream *analysis.CachingTokenFilter) (IntervalsSource, error) {
	if err := stream.Reset(); err != nil {
		return nil, err
	}
	ok, err := stream.IncrementToken()
	if err != nil || !ok {
		return noIntervalsSource, err
	}
	termBytes := getTermBytes(stream)
	if termBytes == nil {
		return noIntervalsSource, nil
	}
	return NewTermIntervalsSource(termBytes), nil
}

func analyzeSimplePhrase(stream *analysis.CachingTokenFilter, maxGaps int, ordered bool) (IntervalsSource, error) {
	if err := stream.Reset(); err != nil {
		return nil, err
	}
	var terms []IntervalsSource
	for {
		ok, err := stream.IncrementToken()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		termBytes := getTermBytes(stream)
		if termBytes == nil {
			continue
		}
		precedingSpaces := 0
		posIncAttr := stream.GetAttribute("PositionIncrementAttribute")
		if posIncAttr != nil {
			if pa, ok := posIncAttr.(analysis.PositionIncrementAttribute); ok {
				precedingSpaces = pa.GetPositionIncrement() - 1
			}
		}
		src := extendSource(NewTermIntervalsSource(termBytes), precedingSpaces)
		terms = append(terms, src)
	}
	return combineSources(terms, maxGaps, ordered), nil
}

func analyzeSynonyms(stream *analysis.CachingTokenFilter, maxGaps int, ordered bool) (IntervalsSource, error) {
	if err := stream.Reset(); err != nil {
		return nil, err
	}
	var terms []IntervalsSource
	var synonyms []IntervalsSource
	spaces := 0
	for {
		ok, err := stream.IncrementToken()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		posInc := 1
		posIncAttr := stream.GetAttribute("PositionIncrementAttribute")
		if posIncAttr != nil {
			if pa, ok := posIncAttr.(analysis.PositionIncrementAttribute); ok {
				posInc = pa.GetPositionIncrement()
			}
		}
		if posInc > 0 {
			// flush synonyms.
			if len(synonyms) == 1 {
				terms = append(terms, extendSource(synonyms[0], spaces))
			} else if len(synonyms) > 1 {
				orSrc := NewDisjunctionIntervalsSource(synonyms, true)
				terms = append(terms, extendSource(orSrc, spaces))
			}
			synonyms = synonyms[:0]
			spaces = posInc - 1
		}
		termBytes := getTermBytes(stream)
		if termBytes != nil {
			synonyms = append(synonyms, NewTermIntervalsSource(termBytes))
		}
	}
	if len(synonyms) == 1 {
		terms = append(terms, extendSource(synonyms[0], spaces))
	} else if len(synonyms) > 1 {
		orSrc := NewDisjunctionIntervalsSource(synonyms, true)
		terms = append(terms, extendSource(orSrc, spaces))
	}
	return combineSources(terms, maxGaps, ordered), nil
}

func combineSources(sources []IntervalsSource, maxGaps int, ordered bool) IntervalsSource {
	if len(sources) == 0 {
		return noIntervalsSource
	}
	if len(sources) == 1 {
		return sources[0]
	}
	if maxGaps == 0 && ordered {
		return BuildBlockIntervalsSource(sources)
	}
	var inner IntervalsSource
	if ordered {
		inner = BuildOrderedIntervalsSource(sources)
	} else {
		inner = BuildUnorderedIntervalsSource(sources)
	}
	if maxGaps == -1 {
		return inner
	}
	// Wrap with a fixed-gap filter — represented as ExtendedIntervalsSource with maxGaps.
	return NewExtendedIntervalsSource(inner, 0, maxGaps)
}

func extendSource(src IntervalsSource, precedingSpaces int) IntervalsSource {
	if precedingSpaces == 0 {
		return src
	}
	return NewExtendedIntervalsSource(src, precedingSpaces, 0)
}

// getTermBytes extracts the current token bytes from a CachingTokenFilter.
func getTermBytes(stream *analysis.CachingTokenFilter) []byte {
	attr := stream.GetAttribute("TermToBytesRefAttribute")
	if attr == nil {
		return nil
	}
	if tbr, ok := attr.(analysis.TermToBytesRefAttribute); ok {
		br := tbr.GetBytesRef()
		if br == nil {
			return nil
		}
		b := make([]byte, len(br.Bytes))
		copy(b, br.Bytes)
		return b
	}
	return nil
}
