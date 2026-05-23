// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package word2vec

import (
	"errors"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// typeSynonym is the token type emitted for synonym tokens, mirroring
// SynonymGraphFilter.TYPE_SYNONYM from Apache Lucene.
const typeSynonym = "SYNONYM"

// Word2VecSynonymFilter applies single-token synonyms produced by a
// Word2Vec model to an incoming TokenStream. For each input token that
// the model recognises, it emits the k nearest-neighbour synonyms as
// extra tokens at the same position (positionIncrement = 0).
//
// This is the Go port of
// org.apache.lucene.analysis.synonym.word2vec.Word2VecSynonymFilter
// from Apache Lucene 10.4.0.
//
// Deviation: Java stores token state using Lucene's
// AttributeSource.captureState / restoreState, which snapshots all
// attribute impls at once. Gocene's AttributeSource exposes the same
// mechanism through CaptureState / RestoreState; the port uses them
// identically.
type Word2VecSynonymFilter struct {
	*analysis.BaseTokenFilter

	termAttr    analysis.CharTermAttribute
	posIncrAttr analysis.PositionIncrementAttribute
	posLenAttr  analysis.PositionLengthAttribute
	typeAttr    analysis.TypeAttribute

	synonymProvider       *Word2VecSynonymProvider
	maxSynonymsPerTerm    int
	minAcceptedSimilarity float32

	// synonymBuffer holds pending synonyms to emit before reading the
	// next input token. The head is consumed first (FIFO).
	synonymBuffer []*TermAndBoost

	// lastState is the attribute snapshot captured after reading the
	// source token for which synonyms were queued.
	lastState *util.AttributeState
}

// NewWord2VecSynonymFilter wraps input with synonym injection.
//
// Parameters:
//   - input: the upstream TokenStream
//   - synonymProvider: pre-built Word2VecSynonymProvider (must not be nil)
//   - maxSynonymsPerTerm: maximum number of synonyms returned per token
//   - minAcceptedSimilarity: minimum cosine similarity (0 < x ≤ 1)
func NewWord2VecSynonymFilter(
	input analysis.TokenStream,
	synonymProvider *Word2VecSynonymProvider,
	maxSynonymsPerTerm int,
	minAcceptedSimilarity float32,
) (*Word2VecSynonymFilter, error) {
	if synonymProvider == nil {
		return nil, errors.New("word2vec: synonym provider must not be nil")
	}
	bf := analysis.NewBaseTokenFilter(input)
	src := bf.GetAttributeSource()

	termAttr := src.AddAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	posIncrAttr := src.AddAttribute(analysis.PositionIncrementAttributeType).(analysis.PositionIncrementAttribute)
	posLenAttr := src.AddAttribute(analysis.PositionLengthAttributeType).(analysis.PositionLengthAttribute)
	typeAttr := src.AddAttribute(analysis.TypeAttributeType).(analysis.TypeAttribute)

	return &Word2VecSynonymFilter{
		BaseTokenFilter:       bf,
		termAttr:              termAttr,
		posIncrAttr:           posIncrAttr,
		posLenAttr:            posLenAttr,
		typeAttr:              typeAttr,
		synonymProvider:       synonymProvider,
		maxSynonymsPerTerm:    maxSynonymsPerTerm,
		minAcceptedSimilarity: minAcceptedSimilarity,
	}, nil
}

// IncrementToken advances the stream. It first drains any queued
// synonyms (emitting them at positionIncrement=0), then reads the next
// input token.
func (f *Word2VecSynonymFilter) IncrementToken() (bool, error) {
	if len(f.synonymBuffer) > 0 {
		synonym := f.synonymBuffer[0]
		f.synonymBuffer = f.synonymBuffer[1:]

		f.ClearAttributes()
		f.GetAttributeSource().RestoreState(f.lastState)

		f.termAttr.SetEmpty()
		f.termAttr.AppendString(synonym.Term.String())
		f.typeAttr.SetType(typeSynonym)
		f.posLenAttr.SetPositionLength(1)
		f.posIncrAttr.SetPositionIncrement(0)
		return true, nil
	}

	ok, err := f.GetInput().IncrementToken()
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}

	// Build a BytesRef from the current term.
	text := f.termAttr.String()
	term := util.NewBytesRef([]byte(text))

	synonyms, err := f.synonymProvider.GetSynonyms(term, f.maxSynonymsPerTerm, f.minAcceptedSimilarity)
	if err != nil {
		return false, err
	}
	if len(synonyms) > 0 {
		f.lastState = f.GetAttributeSource().CaptureState()
		f.synonymBuffer = synonyms
	}
	return true, nil
}

// Reset clears the synonym buffer and delegates to the input.
func (f *Word2VecSynonymFilter) Reset() error {
	f.synonymBuffer = f.synonymBuffer[:0]
	f.lastState = nil
	if r, ok := f.GetInput().(interface{ Reset() error }); ok {
		return r.Reset()
	}
	return nil
}

// Ensure Word2VecSynonymFilter implements analysis.TokenFilter.
var _ analysis.TokenFilter = (*Word2VecSynonymFilter)(nil)
