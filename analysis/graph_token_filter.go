// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import "fmt"

// GraphTokenFilter is an abstract token filter that exposes its input stream as a graph.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.GraphTokenFilter.
type GraphTokenFilter struct {
	source TokenStream

	tokenPool []Token
	currentGraph []Token

	baseToken Token
	graphDepth int
	graphPos int
	trailingPositions int
	finalOffsets int

	stackSize int
	cacheSize int
}

const MaxGraphStackSize = 1000
const MaxTokenCacheSize = 100

func NewGraphTokenFilter(source TokenStream) *GraphTokenFilter {
	return &GraphTokenFilter{
		source:            source,
		trailingPositions: -1,
		finalOffsets:      -1,
	}
}

func (f *GraphTokenFilter) IncrementBaseToken() (Token, bool) {
	f.stackSize = 0
	f.graphDepth = 0
	f.graphPos = 0

	oldBase := f.baseToken
	token, ok := f.nextTokenInStream(f.baseToken)
	if !ok {
		return Token{}, false
	}
	f.baseToken = token
	f.currentGraph = []Token{f.baseToken}
	f.recycleToken(oldBase)
	return f.baseToken, true
}

func (f *GraphTokenFilter) IncrementGraphToken() (Token, bool) {
	if f.graphPos < f.graphDepth {
		f.graphPos++
		return f.currentGraph[f.graphPos], true
	}

	token, ok := f.nextTokenInGraph(f.currentGraph[f.graphDepth])
	if !ok {
		return Token{}, false
	}

	f.graphDepth++
	f.graphPos++
	f.currentGraph = append(f.currentGraph, token)
	return token, true
}

func (f *GraphTokenFilter) IncrementGraph() (Token, bool) {
	if f.baseToken == (Token{}) {
		return Token{}, false
	}

	f.graphPos = 0
	for i := f.graphDepth; i >= 1; i-- {
		if !f.lastInStack(f.currentGraph[i]) {
			token, ok := f.nextTokenInStream(f.currentGraph[i])
			if !ok {
				continue
			}
			f.currentGraph[i] = token
			for j := i + 1; j < f.graphDepth; j++ {
				t, ok := f.nextTokenInGraph(f.currentGraph[j])
				if !ok {
					// This should not happen in a well-formed graph
					continue
				}
				f.currentGraph[j] = t
			}
			if f.stackSize > MaxGraphStackSize {
				panic("Too many graph paths (> " + fmt.Sprint(MaxGraphStackSize) + ")")
			}
			f.stackSize++
			f.currentGraph[0] = f.currentGraph[0] // just to be explicit
			f.graphDepth = i
			return f.currentGraph[0], true
		}
	}
	return Token{}, false
}

func (f *GraphTokenFilter) GetTrailingPositions() int {
	return f.trailingPositions
}

func (f *GraphTokenFilter) Reset() error {
	if err := ResetTokenStream(f.source); err != nil {
		return err
	}
	f.tokenPool = nil
	f.cacheSize = 0
	f.graphDepth = 0
	f.trailingPositions = -1
	f.finalOffsets = -1
	f.baseToken = Token{}
	return nil
}

func (f *GraphTokenFilter) Close() error {
	return f.source.Close()
}

func (f *GraphTokenFilter) nextTokenInGraph(token Token) (Token, bool) {
	remaining := token.PositionLength
	if remaining == 0 {
		remaining = 1
	}
	for {
		t, ok := f.nextTokenInStream(token)
		if !ok {
			return Token{}, false
		}
		token = t
		remaining -= token.PositionInc
		if remaining <= 0 {
			return token, true
		}
	}
}

func (f *GraphTokenFilter) lastInStack(token Token) bool {
	next, ok := f.nextTokenInStream(token)
	return !ok || next.PositionInc != 0
}

func (f *GraphTokenFilter) nextTokenInStream(token Token) (Token, bool) {
	// In Lucene, the Token object has a nextToken reference.
	// In our Go port, we don't have that.
	// We would need to store the sequence of tokens to support this.
	// However, for now, let's assume we just call the source.
	if f.trailingPositions != -1 {
		return Token{}, false
	}
	t, ok := NextFromTokenStream(f.source)
	if !ok {
		f.trailingPositions = t.PositionInc
		f.finalOffsets = t.EndOffset
		return Token{}, false
	}
	return t, true
}

func (f *GraphTokenFilter) recycleToken(token Token) {
	// In our Go port, Token is a value type, so we don't need a pool.
}
