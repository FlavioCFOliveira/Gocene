// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package monitor

import (
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/util"
)

// TermWeightor calculates the weight of an index.Term.
//
// Port of org.apache.lucene.monitor.TermWeightor.
type TermWeightor interface {
	// ApplyAsDouble returns the weight for the given term.
	ApplyAsDouble(term *index.Term) float64
}

// TermWeightorFunc is a function-based TermWeightor.
type TermWeightorFunc func(term *index.Term) float64

// ApplyAsDouble implements TermWeightor.
func (f TermWeightorFunc) ApplyAsDouble(term *index.Term) float64 { return f(term) }

// DefaultTermWeightor is a length-based weightor (a=3, k=0.3), matching Java's DEFAULT.
var DefaultTermWeightor TermWeightor = LengthWeightor(3, 0.3)

// CombineWeightors returns a weightor that multiplies the results of all supplied weightors.
func CombineWeightors(weightors ...TermWeightor) TermWeightor {
	return TermWeightorFunc(func(term *index.Term) float64 {
		r := 1.0
		for _, w := range weightors {
			r *= w.ApplyAsDouble(term)
		}
		return r
	})
}

// FieldWeightor assigns a fixed weight to terms from any of the supplied fields, 1.0 otherwise.
func FieldWeightor(weight float64, fields ...string) TermWeightor {
	set := make(map[string]struct{}, len(fields))
	for _, f := range fields {
		set[f] = struct{}{}
	}
	return TermWeightorFunc(func(term *index.Term) float64 {
		if _, ok := set[term.Field]; ok {
			return weight
		}
		return 1.0
	})
}

// TermWeightorByBytes assigns a fixed weight to terms whose bytes are in the supplied set.
func TermWeightorByBytes(weight float64, terms ...*util.BytesRef) TermWeightor {
	set := make(map[string]struct{}, len(terms))
	for _, t := range terms {
		if t != nil {
			set[string(t.ValidBytes())] = struct{}{}
		}
	}
	return TermWeightorFunc(func(term *index.Term) float64 {
		if term.Bytes == nil {
			return 1.0
		}
		if _, ok := set[string(term.Bytes.ValidBytes())]; ok {
			return weight
		}
		return 1.0
	})
}

// TermAndFieldWeightor assigns a fixed weight to terms matching one of the supplied (field+bytes)
// pairs, 1.0 otherwise.
func TermAndFieldWeightor(weight float64, terms ...*index.Term) TermWeightor {
	type key struct{ field, bytes string }
	set := make(map[key]struct{}, len(terms))
	for _, t := range terms {
		var b string
		if t.Bytes != nil {
			b = string(t.Bytes.ValidBytes())
		}
		set[key{t.Field, b}] = struct{}{}
	}
	return TermWeightorFunc(func(term *index.Term) float64 {
		var b string
		if term.Bytes != nil {
			b = string(term.Bytes.ValidBytes())
		}
		if _, ok := set[key{term.Field, b}]; ok {
			return weight
		}
		return 1.0
	})
}

// TermFreqWeightor assigns a weight based on the term's frequency in the supplied map.
//
// w = (n / freq) + k.  Terms with no recorded frequency receive weight 1.0.
func TermFreqWeightor(frequencies map[string]int, n, k float64) TermWeightor {
	return TermWeightorFunc(func(term *index.Term) float64 {
		if f, ok := frequencies[term.Text()]; ok {
			return (n / float64(f)) + k
		}
		return 1.0
	})
}

// LengthWeightor assigns a weight based on term length: a * e^(-k * length).
// Longer terms are weighted higher; terms ≥ 32 bytes share the same weight.
func LengthWeightor(a, k float64) TermWeightor {
	norms := make([]float64, 32)
	for i := range norms {
		norms[i] = a * math.Exp(-k*float64(i))
	}
	return TermWeightorFunc(func(term *index.Term) float64 {
		var l int
		if term.Bytes != nil {
			l = term.Bytes.Length
		}
		if l >= 32 {
			return 4 - norms[31]
		}
		return 4 - norms[l]
	})
}
