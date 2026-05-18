// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"errors"

	"github.com/FlavioCFOliveira/Gocene/util"
)

// MaxGraphStackSize is the maximum permitted number of routes through a
// graph. Mirrors Lucene's GraphTokenFilter.MAX_GRAPH_STACK_SIZE.
const MaxGraphStackSize = 1000

// MaxTokenCacheSize is the maximum permitted read-ahead in the token stream.
// Mirrors Lucene's GraphTokenFilter.MAX_TOKEN_CACHE_SIZE.
const MaxTokenCacheSize = 100

// GraphTokenFilter is an abstract TokenFilter that exposes its input stream
// as a token graph. Concrete filters call IncrementBaseToken to advance to
// the next root of the graph, IncrementGraphToken to advance along the
// current path, and IncrementGraph to move to the next path rooted at the
// same base token.
//
// This is the Go port of Lucene's
// org.apache.lucene.analysis.GraphTokenFilter.
//
// Implementation notes:
//
//   - The shared AttributeSource is captured per cached token via
//     AttributeSource.CaptureState, mirroring Java's attSource.copyTo + clone.
//   - The PositionLengthAttribute lookup is best-effort: if the underlying
//     stream does not expose one, a token's length defaults to 1.
//   - The cached-token pool is bounded by MaxTokenCacheSize; the per-graph
//     stack is bounded by MaxGraphStackSize. Both bound violations return
//     errors instead of panicking, to match Go-idiomatic error propagation.
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

	posIncAtt PositionIncrementAttribute
	offsetAtt OffsetAttribute
	posLenAtt PositionLengthAttribute
}

// NewGraphTokenFilter wraps the given input stream and initializes the
// graph-walk state. The PositionIncrementAttribute, OffsetAttribute and
// (optional) PositionLengthAttribute are cached for fast access.
func NewGraphTokenFilter(input TokenStream) *GraphTokenFilter {
	f := &GraphTokenFilter{
		BaseTokenFilter:   NewBaseTokenFilter(input),
		trailingPositions: -1,
		finalOffsets:      -1,
	}
	if src := f.GetAttributeSource(); src != nil {
		if attr := src.GetAttribute(PositionIncrementAttributeType); attr != nil {
			if pi, ok := attr.(PositionIncrementAttribute); ok {
				f.posIncAtt = pi
			}
		}
		if attr := src.GetAttribute(OffsetAttributeType); attr != nil {
			if off, ok := attr.(OffsetAttribute); ok {
				f.offsetAtt = off
			}
		}
		if attr := src.GetAttribute(PositionLengthAttributeType); attr != nil {
			if pl, ok := attr.(PositionLengthAttribute); ok {
				f.posLenAtt = pl
			}
		}
	}
	return f
}

// IncrementBaseToken moves the root of the graph to the next token in the
// wrapped TokenStream. Returns false when the underlying stream is
// exhausted.
func (f *GraphTokenFilter) IncrementBaseToken() (bool, error) {
	f.stackSize = 0
	f.graphDepth = 0
	f.graphPos = 0
	oldBase := f.baseToken
	next, err := f.nextTokenInStream(f.baseToken)
	if err != nil {
		return false, err
	}
	f.baseToken = next
	if f.baseToken == nil {
		return false, nil
	}
	f.currentGraph = f.currentGraph[:0]
	f.currentGraph = append(f.currentGraph, f.baseToken)
	f.GetAttributeSource().RestoreState(f.baseToken.state)
	f.recycleToken(oldBase)
	return true, nil
}

// IncrementGraphToken moves to the next token in the current route through
// the graph. Returns false when no more tokens are reachable on the current
// route.
func (f *GraphTokenFilter) IncrementGraphToken() (bool, error) {
	if f.graphPos < f.graphDepth {
		f.graphPos++
		f.GetAttributeSource().RestoreState(f.currentGraph[f.graphPos].state)
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
	// Grow currentGraph as needed; mirrors ArrayList.add(index, element).
	if f.graphDepth >= len(f.currentGraph) {
		f.currentGraph = append(f.currentGraph, token)
	} else {
		f.currentGraph[f.graphDepth] = token
	}
	f.GetAttributeSource().RestoreState(token.state)
	return true, nil
}

// IncrementGraph resets to the root token and moves down the next available
// route through the graph. Returns false when no more routes exist.
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
				nx, err := f.nextTokenInGraph(f.currentGraph[j])
				if err != nil {
					return false, err
				}
				f.currentGraph[j] = nx
			}
			f.stackSize++
			if f.stackSize > MaxGraphStackSize {
				return false, errors.New("graph: too many paths (> MaxGraphStackSize)")
			}
			f.GetAttributeSource().RestoreState(f.currentGraph[0].state)
			f.graphDepth = i
			return true, nil
		}
	}
	return false, nil
}

// GetTrailingPositions returns the number of trailing positions at the end
// of the graph. Should only be called after IncrementGraphToken has returned
// false.
func (f *GraphTokenFilter) GetTrailingPositions() int {
	return f.trailingPositions
}

// End performs end-of-stream operations. On the first call, the underlying
// stream's End() runs and the trailing PositionIncrement / final offsets
// are captured. On subsequent calls, the stored state is replayed on the
// shared AttributeSource.
func (f *GraphTokenFilter) End() error {
	if f.trailingPositions == -1 {
		if err := f.input.End(); err != nil {
			return err
		}
		if f.posIncAtt != nil {
			f.trailingPositions = f.posIncAtt.GetPositionIncrement()
		} else {
			f.trailingPositions = 0
		}
		if f.offsetAtt != nil {
			f.finalOffsets = f.offsetAtt.EndOffset()
		}
		return nil
	}
	if f.posIncAtt != nil {
		f.posIncAtt.SetPositionIncrement(f.trailingPositions)
	}
	if f.offsetAtt != nil {
		f.offsetAtt.SetStartOffset(f.finalOffsets)
		f.offsetAtt.SetEndOffset(f.finalOffsets)
	}
	return nil
}

// Reset clears the cached graph state and forwards the call to the input
// stream.
func (f *GraphTokenFilter) Reset() error {
	if resetter, ok := f.input.(interface{ Reset() error }); ok {
		if err := resetter.Reset(); err != nil {
			return err
		}
	}
	f.tokenPool = f.tokenPool[:0]
	f.currentGraph = f.currentGraph[:0]
	f.cacheSize = 0
	f.graphDepth = 0
	f.trailingPositions = -1
	f.finalOffsets = -1
	f.baseToken = nil
	return nil
}

// CachedTokenCount returns the number of currently cached tokens (for tests
// and instrumentation). Mirrors Lucene's package-private cachedTokenCount().
func (f *GraphTokenFilter) CachedTokenCount() int {
	return f.cacheSize
}

// newToken constructs or recycles a graphToken that captures the current
// attribute state of the underlying stream. The cache-size limit is enforced
// on the wax side (allocation), not on the recycle path.
func (f *GraphTokenFilter) newToken() (*graphToken, error) {
	if len(f.tokenPool) == 0 {
		f.cacheSize++
		if f.cacheSize > MaxTokenCacheSize {
			return nil, errors.New("graph: too many cached tokens (> MaxTokenCacheSize)")
		}
		return &graphToken{
			state:    f.GetAttributeSource().CaptureState(),
			posInc:   f.currentPosInc(),
			posLen:   f.currentPosLen(),
			nextRef:  nil,
			refValid: false,
		}, nil
	}
	// Pop the back of the pool (ArrayDeque.removeFirst -> Lucene behavior).
	t := f.tokenPool[0]
	f.tokenPool = f.tokenPool[1:]
	t.state = f.GetAttributeSource().CaptureState()
	t.posInc = f.currentPosInc()
	t.posLen = f.currentPosLen()
	t.nextRef = nil
	t.refValid = false
	return t, nil
}

// recycleToken returns a token to the pool for later reuse.
func (f *GraphTokenFilter) recycleToken(t *graphToken) {
	if t == nil {
		return
	}
	t.nextRef = nil
	t.refValid = false
	f.tokenPool = append(f.tokenPool, t)
}

// nextTokenInGraph follows the wrapped stream until it reaches a token that
// completes the current graph row (advances by token.length positions).
func (f *GraphTokenFilter) nextTokenInGraph(token *graphToken) (*graphToken, error) {
	remaining := token.posLen
	for {
		next, err := f.nextTokenInStream(token)
		if err != nil {
			return nil, err
		}
		if next == nil {
			return nil, nil
		}
		remaining -= next.posInc
		token = next
		if remaining <= 0 {
			return token, nil
		}
	}
}

// lastInStack reports whether the next token in the underlying stream lives
// at the same position as the given token (posInc == 0). When that is the
// case there is a sibling on the alternative path; otherwise the token is
// the last on its stack.
func (f *GraphTokenFilter) lastInStack(token *graphToken) (bool, error) {
	next, err := f.nextTokenInStream(token)
	if err != nil {
		return false, err
	}
	return next == nil || next.posInc != 0, nil
}

// nextTokenInStream returns the cached successor of token (if any) or reads
// a fresh one from the input stream, caching its attribute state.
func (f *GraphTokenFilter) nextTokenInStream(token *graphToken) (*graphToken, error) {
	if token != nil && token.refValid {
		return token.nextRef, nil
	}
	if f.trailingPositions != -1 {
		return nil, nil
	}
	ok, err := f.input.IncrementToken()
	if err != nil {
		return nil, err
	}
	if !ok {
		if endErr := f.input.End(); endErr != nil {
			return nil, endErr
		}
		if f.posIncAtt != nil {
			f.trailingPositions = f.posIncAtt.GetPositionIncrement()
		} else {
			f.trailingPositions = 0
		}
		if f.offsetAtt != nil {
			f.finalOffsets = f.offsetAtt.EndOffset()
		}
		return nil, nil
	}
	if token == nil {
		return f.newToken()
	}
	next, err := f.newToken()
	if err != nil {
		return nil, err
	}
	token.nextRef = next
	token.refValid = true
	return next, nil
}

// currentPosInc returns the current PositionIncrement of the shared
// AttributeSource (defaults to 1 when no attribute is present).
func (f *GraphTokenFilter) currentPosInc() int {
	if f.posIncAtt == nil {
		return 1
	}
	return f.posIncAtt.GetPositionIncrement()
}

// currentPosLen returns the current PositionLength of the shared
// AttributeSource (defaults to 1 when no attribute is present).
func (f *GraphTokenFilter) currentPosLen() int {
	if f.posLenAtt == nil {
		return 1
	}
	return f.posLenAtt.GetPositionLength()
}

// graphToken caches the attribute state of a single token together with the
// position-increment and position-length needed for graph traversal.
type graphToken struct {
	state    *util.AttributeState
	posInc   int
	posLen   int
	nextRef  *graphToken
	refValid bool
}

// Ensure GraphTokenFilter implements TokenFilter.
var _ TokenFilter = (*GraphTokenFilter)(nil)
