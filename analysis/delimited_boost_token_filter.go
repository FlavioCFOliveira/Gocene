// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import (
	"strconv"
)

// DefaultBoostDelimiter is the delimiter byte separating the token
// text from its boost factor in DelimitedBoostTokenFilter input.
// The Lucene reference uses '|'.
const DefaultBoostDelimiter byte = '|'

// DelimitedBoostTokenFilter splits each token at the first
// occurrence of delimiter and parses the trailing substring as a
// float32 boost. The boost is written to the BoostAttribute; the
// token text is truncated to the bytes preceding the delimiter.
//
// This is the Go port of
// org.apache.lucene.analysis.boost.DelimitedBoostTokenFilter from
// Apache Lucene 10.4.0.
type DelimitedBoostTokenFilter struct {
	*BaseTokenFilter

	delimiter byte
	termAttr  CharTermAttribute
	boostAttr BoostAttribute
}

// NewDelimitedBoostTokenFilter wraps input with the given delimiter.
func NewDelimitedBoostTokenFilter(input TokenStream, delimiter byte) *DelimitedBoostTokenFilter {
	f := &DelimitedBoostTokenFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
		delimiter:       delimiter,
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttribute(CharTermAttributeType); a != nil {
			f.termAttr = a.(CharTermAttribute)
		}
		if a := src.GetAttribute(BoostAttributeType); a != nil {
			f.boostAttr = a.(BoostAttribute)
		}
	}
	return f
}

// IncrementToken splits at the delimiter and assigns the boost.
// Tokens without a delimiter are emitted unchanged.
func (f *DelimitedBoostTokenFilter) IncrementToken() (bool, error) {
	ok, err := f.input.IncrementToken()
	if err != nil || !ok {
		return ok, err
	}
	if f.termAttr == nil {
		return true, nil
	}
	buf := f.termAttr.Buffer()
	length := f.termAttr.Length()
	for i := 0; i < length; i++ {
		if buf[i] == f.delimiter {
			if f.boostAttr != nil {
				s := string(buf[i+1 : length])
				if v, err := strconv.ParseFloat(s, 32); err == nil {
					f.boostAttr.SetBoost(float32(v))
				}
			}
			f.termAttr.SetLength(i)
			return true, nil
		}
	}
	return true, nil
}

// Ensure DelimitedBoostTokenFilter implements TokenFilter.
var _ TokenFilter = (*DelimitedBoostTokenFilter)(nil)

// DelimitedBoostTokenFilterFactory creates instances.
type DelimitedBoostTokenFilterFactory struct {
	delimiter byte
}

// NewDelimitedBoostTokenFilterFactory returns a factory using the
// default delimiter.
func NewDelimitedBoostTokenFilterFactory() *DelimitedBoostTokenFilterFactory {
	return &DelimitedBoostTokenFilterFactory{delimiter: DefaultBoostDelimiter}
}

// NewDelimitedBoostTokenFilterFactoryWithDelimiter returns a
// configured factory.
func NewDelimitedBoostTokenFilterFactoryWithDelimiter(delimiter byte) *DelimitedBoostTokenFilterFactory {
	return &DelimitedBoostTokenFilterFactory{delimiter: delimiter}
}

// Create wraps input.
func (f *DelimitedBoostTokenFilterFactory) Create(input TokenStream) TokenFilter {
	return NewDelimitedBoostTokenFilter(input, f.delimiter)
}

// Ensure factory satisfies TokenFilterFactory.
var _ TokenFilterFactory = (*DelimitedBoostTokenFilterFactory)(nil)
