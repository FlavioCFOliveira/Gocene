// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/analysis/tokenattributes"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// MaxGraphStackSize is the maximum permitted number of routes through a graph
// (GraphTokenFilter.MAX_GRAPH_STACK_SIZE).
const MaxGraphStackSize = 1000

// MaxTokenCacheSize is the maximum permitted read-ahead in the token stream
// (GraphTokenFilter.MAX_TOKEN_CACHE_SIZE).
const MaxTokenCacheSize = 100

// GraphTokenFilter is an abstract TokenFilter that exposes its input stream
// as a graph.
//
// This is the Go port of Lucene's org.apache.lucene.analysis.GraphTokenFilter
// (lucene/core/src/java/org/apache/lucene/analysis/GraphTokenFilter.java).
//
// Call IncrementBaseToken to move the root of the graph to the next position
// in the TokenStream, IncrementGraphToken to move along the current graph, and
// IncrementGraph to reset to the next graph based at the current root.
//
// For example, given the stream 'a b/c:2 d e', then with the base token at
// 'a', IncrementGraphToken will produce the stream 'a b d e', and then after
// calling IncrementGraph will produce the stream 'a c e'.
//
// The Java class is abstract: a concrete filter embeds *GraphTokenFilter and
// implements IncrementToken. Java's IllegalStateException, which is
// unchecked, is raised as a panic.
type GraphTokenFilter struct {
	*BaseTokenFilter

	tokenPool    []*graphToken
	currentGraph []*graphToken

	baseToken         *graphToken
	graphDepth        int
	graphPos          int
	trailingPositions int
	finalOffsets      int

	stackSize int
	cacheSize int

	posIncAtt tokenattributes.PositionIncrementAttribute
	offsetAtt OffsetAttribute
}

// NewGraphTokenFilter creates a new GraphTokenFilter wrapping input
// (GraphTokenFilter(TokenStream)).
func NewGraphTokenFilter(input TokenStream) *GraphTokenFilter {
	f := &GraphTokenFilter{
		BaseTokenFilter:   NewBaseTokenFilter(input),
		trailingPositions: -1,
		finalOffsets:      -1,
	}
	src := input.GetAttributeSource()
	f.posIncAtt = src.AddAttribute(tokenattributes.PositionIncrementAttributeType).(tokenattributes.PositionIncrementAttribute)
	f.offsetAtt = src.AddAttribute(OffsetAttributeType).(OffsetAttribute)
	return f
}

// IncrementBaseToken moves the root of the graph to the next token in the
// wrapped TokenStream. It returns false if the underlying stream is
// exhausted.
func (f *GraphTokenFilter) IncrementBaseToken() (bool, error) {
	f.stackSize = 0
	f.graphDepth = 0
	f.graphPos = 0
	oldBase := f.baseToken
	var err error
	f.baseToken, err = f.nextTokenInStream(f.baseToken)
	if err != nil {
		return false, err
	}
	if f.baseToken == nil {
		return false, nil
	}
	f.currentGraph = f.currentGraph[:0]
	f.currentGraph = append(f.currentGraph, f.baseToken)
	f.baseToken.attSource.CopyTo(f.AttributeSource)
	f.recycleToken(oldBase)
	return true, nil
}

// IncrementGraphToken moves to the next token in the current route through
// the graph. It returns false if there are no more tokens in the current
// graph.
func (f *GraphTokenFilter) IncrementGraphToken() (bool, error) {
	if f.graphPos < f.graphDepth {
		f.graphPos++
		f.currentGraph[f.graphPos].attSource.CopyTo(f.AttributeSource)
		return true, nil
	}
	token, err := f.nextTokenInGraph(f.currentGraph[f.graphDepth])
	if err != nil {
		return false, err
	}
	if token == nil {
		return false, nil
	}
	f.graphDepth++
	f.graphPos++
	// currentGraph.add(graphDepth, token)
	f.currentGraph = append(f.currentGraph, nil)
	copy(f.currentGraph[f.graphDepth+1:], f.currentGraph[f.graphDepth:])
	f.currentGraph[f.graphDepth] = token
	token.attSource.CopyTo(f.AttributeSource)
	return true, nil
}

// IncrementGraph resets to the root token again, and moves down the next
// route through the graph. It returns false if there are no more routes
// through the graph.
func (f *GraphTokenFilter) IncrementGraph() (bool, error) {
	if f.baseToken == nil {
		return false, nil
	}
	f.graphPos = 0
	for i := f.graphDepth; i >= 1; i-- {
		last, err := f.lastInStack(f.currentGraph[i])
		if err != nil {
			return false, err
		}
		if !last {
			next, err := f.nextTokenInStream(f.currentGraph[i])
			if err != nil {
				return false, err
			}
			f.currentGraph[i] = next
			for j := i + 1; j < f.graphDepth; j++ {
				next, err := f.nextTokenInGraph(f.currentGraph[j])
				if err != nil {
					return false, err
				}
				f.currentGraph[j] = next
			}
			stackSize := f.stackSize
			f.stackSize++
			if stackSize > MaxGraphStackSize {
				panic(fmt.Sprintf("Too many graph paths (> %d)", MaxGraphStackSize))
			}
			f.currentGraph[0].attSource.CopyTo(f.AttributeSource)
			f.graphDepth = i
			return true, nil
		}
	}
	return false, nil
}

// GetTrailingPositions returns the number of trailing positions at the end
// of the graph.
//
// NB this should only be called after IncrementGraphToken has returned false.
func (f *GraphTokenFilter) GetTrailingPositions() int {
	return f.trailingPositions
}

// End performs end-of-stream operations (GraphTokenFilter.end()).
func (f *GraphTokenFilter) End() error {
	if f.trailingPositions == -1 {
		if err := f.input.End(); err != nil {
			return err
		}
		f.trailingPositions = f.posIncAtt.GetPositionIncrement()
		f.finalOffsets = f.offsetAtt.EndOffset()
	} else {
		f.EndAttributes()
		f.posIncAtt.SetPositionIncrement(f.trailingPositions)
		f.offsetAtt.SetOffset(f.finalOffsets, f.finalOffsets)
	}
	return nil
}

// Reset resets the filter and its input (GraphTokenFilter.reset()).
func (f *GraphTokenFilter) Reset() error {
	if err := f.input.Reset(); err != nil {
		return err
	}
	// new attributes can be added between reset() calls, so we can't reuse
	// token objects from a previous run
	f.tokenPool = f.tokenPool[:0]
	f.cacheSize = 0
	f.graphDepth = 0
	f.trailingPositions = -1
	f.finalOffsets = -1
	f.baseToken = nil
	return nil
}

// cachedTokenCount returns the number of tokens created for the read-ahead
// cache (package-private cachedTokenCount()).
func (f *GraphTokenFilter) cachedTokenCount() int {
	return f.cacheSize
}

func (f *GraphTokenFilter) newToken() *graphToken {
	if len(f.tokenPool) == 0 {
		f.cacheSize++
		if f.cacheSize > MaxTokenCacheSize {
			panic(fmt.Sprintf("Too many cached tokens (> %d)", MaxTokenCacheSize))
		}
		return newGraphToken(f.CloneAttributes())
	}
	token := f.tokenPool[0]
	f.tokenPool[0] = nil
	f.tokenPool = f.tokenPool[1:]
	token.reset(f.input.GetAttributeSource())
	return token
}

func (f *GraphTokenFilter) recycleToken(token *graphToken) {
	if token == nil {
		return
	}
	token.nextToken = nil
	f.tokenPool = append(f.tokenPool, token)
}

func (f *GraphTokenFilter) nextTokenInGraph(token *graphToken) (*graphToken, error) {
	remaining := token.length()
	for {
		var err error
		token, err = f.nextTokenInStream(token)
		if err != nil {
			return nil, err
		}
		if token == nil {
			return nil, nil
		}
		remaining -= token.posInc()
		if remaining <= 0 {
			return token, nil
		}
	}
}

// lastInStack checks if the next token in the tokenstream is at the same
// position as this one.
func (f *GraphTokenFilter) lastInStack(token *graphToken) (bool, error) {
	next, err := f.nextTokenInStream(token)
	if err != nil {
		return false, err
	}
	return next == nil || next.posInc() != 0, nil
}

func (f *GraphTokenFilter) nextTokenInStream(token *graphToken) (*graphToken, error) {
	if token != nil && token.nextToken != nil {
		return token.nextToken, nil
	}
	if f.trailingPositions != -1 {
		// already hit the end
		return nil, nil
	}
	ok, err := f.input.IncrementToken()
	if err != nil {
		return nil, err
	}
	if !ok {
		if err := f.input.End(); err != nil {
			return nil, err
		}
		f.trailingPositions = f.posIncAtt.GetPositionIncrement()
		f.finalOffsets = f.offsetAtt.EndOffset()
		return nil, nil
	}
	if token == nil {
		return f.newToken(), nil
	}
	token.nextToken = f.newToken()
	return token.nextToken, nil
}

// graphToken is the port of the private static inner class
// GraphTokenFilter.Token.
type graphToken struct {
	attSource *util.AttributeSource
	posIncAtt tokenattributes.PositionIncrementAttribute
	lengthAtt PositionLengthAttribute
	nextToken *graphToken
}

func newGraphToken(attSource *util.AttributeSource) *graphToken {
	t := &graphToken{attSource: attSource}
	t.posIncAtt = attSource.AddAttribute(tokenattributes.PositionIncrementAttributeType).(tokenattributes.PositionIncrementAttribute)
	if attSource.HasAttribute(PositionLengthAttributeType) {
		t.lengthAtt = attSource.AddAttribute(PositionLengthAttributeType).(PositionLengthAttribute)
	}
	return t
}

func (t *graphToken) posInc() int {
	return t.posIncAtt.GetPositionIncrement()
}

func (t *graphToken) length() int {
	if t.lengthAtt == nil {
		return 1
	}
	return t.lengthAtt.GetPositionLength()
}

func (t *graphToken) reset(attSource *util.AttributeSource) {
	attSource.CopyTo(t.attSource)
	t.nextToken = nil
}

func (t *graphToken) String() string {
	return t.attSource.String()
}
