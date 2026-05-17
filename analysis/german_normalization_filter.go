// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package analysis

import "reflect"

// State machine constants for GermanNormalizationFilter, matching
// the N/U/V state symbols in the Lucene reference.
const (
	germanStateN = 0
	germanStateU = 1
	germanStateV = 2
)

// GermanNormalizationFilter normalises German text by removing the
// "e" that follows certain umlaut vowels, stripping umlaut diacritics
// (ä→a, ö→o, ü→u), and expanding the sharp s (ß→ss).
//
// This is the Go port of
// org.apache.lucene.analysis.de.GermanNormalizationFilter from
// Apache Lucene 10.4.0.
type GermanNormalizationFilter struct {
	*BaseTokenFilter

	termAttr CharTermAttribute
}

// NewGermanNormalizationFilter wraps input with the normaliser.
func NewGermanNormalizationFilter(input TokenStream) *GermanNormalizationFilter {
	f := &GermanNormalizationFilter{
		BaseTokenFilter: NewBaseTokenFilter(input),
	}
	src := f.GetAttributeSource()
	if src != nil {
		if a := src.GetAttributeByType(reflect.TypeOf(&charTermAttribute{})); a != nil {
			f.termAttr = a.(CharTermAttribute)
		}
	}
	return f
}

// IncrementToken applies the German normalisation rules.
func (f *GermanNormalizationFilter) IncrementToken() (bool, error) {
	ok, err := f.input.IncrementToken()
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	if f.termAttr == nil {
		return true, nil
	}
	runes := []rune(f.termAttr.String())
	state := germanStateN
	length := len(runes)
	i := 0
	for i < length {
		c := runes[i]
		switch c {
		case 'a', 'o':
			state = germanStateU
		case 'u':
			if state == germanStateN {
				state = germanStateU
			} else {
				state = germanStateV
			}
		case 'e':
			if state == germanStateU {
				length = runeDelete(runes, i, length)
				state = germanStateV
				continue // do not advance i; current index now holds next char
			}
			state = germanStateV
		case 'i', 'q', 'y':
			state = germanStateV
		case 'ä':
			runes[i] = 'a'
			state = germanStateV
		case 'ö':
			runes[i] = 'o'
			state = germanStateV
		case 'ü':
			runes[i] = 'u'
			state = germanStateV
		case 'ß':
			// Expand into two 's': overwrite current rune and insert
			// another at i+1, growing the buffer by one.
			runes[i] = 's'
			runes = append(runes, 0)
			copy(runes[i+2:], runes[i+1:length])
			runes[i+1] = 's'
			length++
			i++ // consume the inserted 's'
			state = germanStateN
		default:
			state = germanStateN
		}
		i++
	}
	res := string(runes[:length])
	if res != f.termAttr.String() {
		f.termAttr.SetEmpty()
		f.termAttr.AppendString(res)
	}
	return true, nil
}

// Ensure GermanNormalizationFilter implements TokenFilter.
var _ TokenFilter = (*GermanNormalizationFilter)(nil)

// GermanNormalizationFilterFactory creates instances.
type GermanNormalizationFilterFactory struct{}

// NewGermanNormalizationFilterFactory returns a fresh factory.
func NewGermanNormalizationFilterFactory() *GermanNormalizationFilterFactory {
	return &GermanNormalizationFilterFactory{}
}

// Create wraps input.
func (f *GermanNormalizationFilterFactory) Create(input TokenStream) TokenFilter {
	return NewGermanNormalizationFilter(input)
}

// Ensure factory implements TokenFilterFactory.
var _ TokenFilterFactory = (*GermanNormalizationFilterFactory)(nil)
