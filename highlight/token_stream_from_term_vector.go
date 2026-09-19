// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package highlight

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/analysis/tokenattributes"
	"github.com/FlavioCFOliveira/Gocene/spi"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TokenStreamFromTermVector is a TokenStream created from a term vector field.
// The term vector requires positions and/or offsets (either). If you want
// payloads add PayloadAttributeImpl (as you would normally) but don't assume
// the attribute is already added just because you know the term vector has
// payloads, since the first call to IncrementToken will observe if you asked
// for them and if not then won't get them. This TokenStream supports an
// efficient Reset, so there's no need to wrap with a caching impl.
//
// The implementation will create an array of tokens indexed by token position.
// As long as there aren't massive jumps in positions, this is fine. And it
// assumes there aren't large numbers of tokens at the same position, since it
// adds them to a linked-list per position in O(N^2) complexity. When there
// aren't positions in the term vector, it divides the startOffset by 8 to use
// as a temporary substitute. In that case, tokens with the same startOffset
// will occupy the same final position; otherwise tokens become adjacent.
//
// This is the Go port of
// org.apache.lucene.search.highlight.TokenStreamFromTermVector (Apache Lucene
// 10.5.0,
// lucene/highlighter/src/java/org/apache/lucene/search/highlight/TokenStreamFromTermVector.java).
type TokenStreamFromTermVector struct {
	*analysis.BaseTokenStream

	vector spi.Terms

	termAttribute analysis.CharTermAttribute

	positionIncrementAttribute tokenattributes.PositionIncrementAttribute

	maxStartOffset int

	offsetAttribute analysis.OffsetAttribute // maybe nil

	payloadAttribute analysis.PayloadAttribute // maybe nil

	// termCharsBuilder holds the term data. Java uses a CharsRefBuilder of
	// UTF-16 chars; Gocene's CharTermAttribute buffer is UTF-8 bytes, so the
	// UnicodeUtil.UTF8toUTF16 conversion of Java's init() is the identity here
	// and the builder holds the term bytes verbatim.
	termCharsBuilder []byte

	payloadsBytesRefArray *util.BytesRefArray // only used when payloadAttribute is non-nil
	spareBytesRefBuilder  *util.BytesRef      // only used when payloadAttribute is non-nil

	firstToken *tokenLL // the head of a linked-list

	incrementToken *tokenLL

	initialized bool // lazy
}

var _ analysis.TokenStream = (*TokenStreamFromTermVector)(nil)

// NewTokenStreamFromTermVector is the constructor. The uninversion doesn't
// happen here; it's delayed till the first call to IncrementToken.
//
// vector is the Terms that contains the data for creating the TokenStream; it
// must have positions and/or offsets. maxStartOffset is the threshold above
// which a token is not added; -1 disables the limit.
//
// Renders `public TokenStreamFromTermVector(Terms vector, int maxStartOffset)`
// (TokenStreamFromTermVector.java:85).
func NewTokenStreamFromTermVector(vector spi.Terms, maxStartOffset int) (*TokenStreamFromTermVector, error) {
	ts := &TokenStreamFromTermVector{BaseTokenStream: analysis.NewBaseTokenStream()}
	if maxStartOffset < 0 {
		ts.maxStartOffset = math.MaxInt32
	} else {
		ts.maxStartOffset = maxStartOffset
	}
	if !vector.HasPositions() && !vector.HasOffsets() {
		return nil, fmt.Errorf("The term vector needs positions and/or offsets.")
	}
	ts.vector = vector
	ts.termAttribute = ts.GetAttributeSource().
		AddAttribute(analysis.CharTermAttributeType).(analysis.CharTermAttribute)
	ts.positionIncrementAttribute = ts.GetAttributeSource().
		AddAttribute(tokenattributes.PositionIncrementAttributeType).(tokenattributes.PositionIncrementAttribute)
	return ts, nil
}

// GetTermVectorTerms renders `public Terms getTermVectorTerms()`
// (TokenStreamFromTermVector.java:99).
func (ts *TokenStreamFromTermVector) GetTermVectorTerms() spi.Terms { return ts.vector }

// Reset renders `public void reset()` (TokenStreamFromTermVector.java:104).
func (ts *TokenStreamFromTermVector) Reset() error {
	ts.incrementToken = nil
	return ts.BaseTokenStream.Reset()
}

// init delays initialization because we can see which attributes the consumer
// wants, particularly payloads.
//
// Renders `private void init()` (TokenStreamFromTermVector.java:112).
func (ts *TokenStreamFromTermVector) init() error {
	source := ts.GetAttributeSource()
	dpEnumFlags := spi.PostingsFlagPositions
	if ts.vector.HasOffsets() {
		dpEnumFlags |= spi.PostingsFlagOffsets
		ts.offsetAttribute = source.AddAttribute(analysis.OffsetAttributeType).(analysis.OffsetAttribute)
	}
	if ts.vector.HasPayloads() && source.HasAttribute(analysis.PayloadAttributeType) {
		dpEnumFlags |= spi.PostingsFlagOffsets | spi.PostingsFlagPayloads // must ask for offsets too
		ts.payloadAttribute = source.GetAttribute(analysis.PayloadAttributeType).(analysis.PayloadAttribute)
		ts.payloadsBytesRefArray = util.NewBytesRefArray(0)
		ts.spareBytesRefBuilder = util.NewBytesRefEmpty()
	}

	// We put term data here
	ts.termCharsBuilder = make([]byte, 0, ts.vector.Size()*7) // 7 is over-estimate of average term len

	// Step 1: iterate termsEnum and create a token, placing into an array of tokens by position

	positionedTokens, err := ts.initTokensArray()
	if err != nil {
		return err
	}

	lastPosition := -1

	termsEnum, err := ts.vector.Iterator()
	if err != nil {
		return err
	}
	for {
		termBytesRef, err := termsEnum.Next()
		if err != nil {
			return err
		}
		if termBytesRef == nil {
			break
		}
		// Grab the term. Java converts the UTF-8 bytes to UTF-16 chars through
		// UnicodeUtil.UTF8toUTF16 because CharTermAttribute holds chars;
		// Gocene's CharTermAttribute buffer is UTF-8 bytes, so the term bytes
		// are appended verbatim and termCharsLen counts bytes, not UTF-16
		// code units.
		termBytes := termBytesRef.Bytes.ValidBytes()
		termCharsLen := len(termBytes)
		termCharsOff := len(ts.termCharsBuilder)
		ts.termCharsBuilder = append(ts.termCharsBuilder, termBytes...)

		dpEnum, err := termsEnum.Postings(dpEnumFlags)
		if err != nil {
			return err
		}
		if _, err := dpEnum.NextDoc(); err != nil {
			return err
		}
		freq, err := dpEnum.Freq()
		if err != nil {
			return err
		}
		for j := 0; j < freq; j++ {
			pos, err := dpEnum.NextPosition()
			if err != nil {
				return err
			}
			token := &tokenLL{}
			token.termCharsOff = termCharsOff
			token.termCharsLen = int16(min(termCharsLen, math.MaxInt16))
			if ts.offsetAttribute != nil {
				startOffset, err := dpEnum.StartOffset()
				if err != nil {
					return err
				}
				token.startOffset = startOffset
				if token.startOffset > ts.maxStartOffset {
					continue // filter this token out; exceeds threshold
				}
				endOffset, err := dpEnum.EndOffset()
				if err != nil {
					return err
				}
				token.endOffsetInc = int16(min(endOffset-token.startOffset, math.MaxInt16))
				if pos == -1 {
					pos = token.startOffset >> 3 // divide by 8
				}
			}

			if ts.payloadAttribute != nil {
				payload, err := dpEnum.GetPayload()
				if err != nil {
					return err
				}
				if payload == nil {
					token.payloadIndex = -1
				} else {
					token.payloadIndex = ts.payloadsBytesRefArray.Append(util.NewBytesRef(payload))
				}
			}

			// Add token to an array indexed by position
			if len(positionedTokens) <= pos {
				// grow, but not 2x since we think our original length estimate is close
				newPositionedTokens := make([]*tokenLL, int(float32(pos+1)*1.5))
				copy(newPositionedTokens, positionedTokens[0:lastPosition+1])
				positionedTokens = newPositionedTokens
			}
			positionedTokens[pos] = token.insertIntoSortedLinkedList(positionedTokens[pos])

			lastPosition = max(lastPosition, pos)
		}
	}

	// Step 2:  Link all Tokens into a linked-list and set position increments as we go

	prevTokenPos := -1
	var prevToken *tokenLL
	for pos := 0; pos <= lastPosition; pos++ {
		token := positionedTokens[pos]
		if token == nil {
			continue
		}
		// link
		if prevToken != nil {
			prevToken.next = token // concatenate linked-list
		} else {
			ts.firstToken = token
		}
		// set increments
		if ts.vector.HasPositions() {
			token.positionIncrement = pos - prevTokenPos
			for token.next != nil {
				token = token.next
				token.positionIncrement = 0
			}
		} else {
			token.positionIncrement = 1
			for token.next != nil {
				prevToken = token
				token = token.next
				if prevToken.startOffset == token.startOffset {
					token.positionIncrement = 0
				} else {
					token.positionIncrement = 1
				}
			}
		}
		prevTokenPos = pos
		prevToken = token
	}

	ts.initialized = true
	return nil
}

// initTokensArray renders `private TokenLL[] initTokensArray()`
// (TokenStreamFromTermVector.java:232).
func (ts *TokenStreamFromTermVector) initTokensArray() ([]*tokenLL, error) {
	// Estimate the number of position slots we need from term stats.  We use some estimation
	// factors taken from
	//  Wikipedia that reduce the likelihood of needing to expand the array.
	sum, err := ts.vector.GetSumTotalTermFreq()
	if err != nil {
		return nil, err
	}
	sumTotalTermFreq := int(sum)

	originalPositionEstimate := int(float32(sumTotalTermFreq) * 1.5) // less than 1 in 10 docs exceed this

	// This estimate is based on maxStartOffset. Err on the side of this being larger than needed.
	offsetLimitPositionEstimate := int(float64(ts.maxStartOffset) / 5.0)

	// Take the smaller of the two estimates, but no smaller than 64
	return make([]*tokenLL, max(64, min(originalPositionEstimate, offsetLimitPositionEstimate))), nil
}

// IncrementToken renders `public boolean incrementToken()`
// (TokenStreamFromTermVector.java:249).
func (ts *TokenStreamFromTermVector) IncrementToken() (bool, error) {
	if ts.incrementToken == nil {
		if !ts.initialized {
			if err := ts.init(); err != nil {
				return false, err
			}
		}
		ts.incrementToken = ts.firstToken
		if ts.incrementToken == nil {
			return false, nil
		}
	} else if ts.incrementToken.next != nil {
		ts.incrementToken = ts.incrementToken.next
	} else {
		return false, nil
	}
	ts.ClearAttributes()
	// Java's termAttribute.copyBuffer(chars, off, len) replaces the whole
	// buffer; SetEmpty followed by AppendChars is its Gocene rendering.
	ts.termAttribute.SetEmpty()
	ts.termAttribute.AppendChars(
		ts.termCharsBuilder[ts.incrementToken.termCharsOff : ts.incrementToken.termCharsOff+int(ts.incrementToken.termCharsLen)])
	ts.positionIncrementAttribute.SetPositionIncrement(ts.incrementToken.positionIncrement)
	if ts.offsetAttribute != nil {
		ts.offsetAttribute.SetOffset(
			ts.incrementToken.startOffset, ts.incrementToken.startOffset+int(ts.incrementToken.endOffsetInc))
	}
	if ts.payloadAttribute != nil {
		if ts.incrementToken.payloadIndex == -1 {
			ts.payloadAttribute.SetPayload(nil)
		} else {
			ts.payloadsBytesRefArray.Get(ts.incrementToken.payloadIndex, ts.spareBytesRefBuilder)
			ts.payloadAttribute.SetPayload(ts.spareBytesRefBuilder.ValidBytes())
		}
	}
	return true, nil
}

// tokenLL renders the private static class
// TokenStreamFromTermVector.TokenLL (TokenStreamFromTermVector.java:282).
type tokenLL struct {
	// This class should weigh 32 bytes, including object header

	termCharsOff int // see termCharsBuilder
	termCharsLen int16

	positionIncrement int
	startOffset       int
	endOffsetInc      int16 // add to startOffset to get endOffset
	payloadIndex      int

	next *tokenLL
}

// insertIntoSortedLinkedList takes the head of a linked-list (possibly nil),
// inserts the token at the correct spot to maintain the desired order, and
// returns the head (which could be this token if it's the smallest). O(N^2)
// complexity but N should be a handful at most.
//
// Renders `TokenLL insertIntoSortedLinkedList(final TokenLL head)`
// (TokenStreamFromTermVector.java:300).
func (t *tokenLL) insertIntoSortedLinkedList(head *tokenLL) *tokenLL {
	if head == nil {
		return t
	} else if t.compareOffsets(head) <= 0 {
		t.next = head
		return t
	}
	prev := head
	for prev.next != nil && t.compareOffsets(prev.next) > 0 {
		prev = prev.next
	}
	t.next = prev.next
	prev.next = t
	return head
}

// compareOffsets orders by startOffset then endOffset.
//
// Renders `int compareOffsets(TokenLL tokenB)` (TokenStreamFromTermVector.java:316).
func (t *tokenLL) compareOffsets(tokenB *tokenLL) int {
	cmp := cmpInt(t.startOffset, tokenB.startOffset)
	if cmp == 0 {
		cmp = cmpInt(int(t.endOffsetInc), int(tokenB.endOffsetInc))
	}
	return cmp
}

// cmpInt renders java.lang.Integer.compare / java.lang.Short.compare.
func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
