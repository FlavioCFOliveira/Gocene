// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/queries/function/docvalues"
)

// LiteralValueSource passes a string literal through as the value for
// every document, regardless of indexed type.
//
// Go port of org.apache.lucene.queries.function.valuesource.LiteralValueSource.
type LiteralValueSource struct {
	function.BaseValueSource
	Literal string
}

// NewLiteralValueSource builds a LiteralValueSource for s.
func NewLiteralValueSource(s string) *LiteralValueSource {
	return &LiteralValueSource{Literal: s}
}

// GetValue returns the literal.
func (l *LiteralValueSource) GetValue() string { return l.Literal }

// Description renders "literal(<value>)".
func (l *LiteralValueSource) Description() string { return "literal(" + l.Literal + ")" }

// String implements fmt.Stringer.
func (l *LiteralValueSource) String() string { return l.Description() }

// Equals reports value equality.
func (l *LiteralValueSource) Equals(other function.ValueSource) bool {
	o, ok := other.(*LiteralValueSource)
	return ok && o.Literal == l.Literal
}

// HashCode mirrors Lucene's `hash + string.hashCode()`. Gocene uses the
// FNV-32 hash from the helper rather than Java's polynomial hash; the
// scheme is consistent within Gocene.
const literalHashBase int32 = 0x4c_69_56_53 // "LiVS" marker

// HashCode returns a stable hash for the literal.
func (l *LiteralValueSource) HashCode() int32 { return literalHashBase + hashString(l.Literal) }

// GetValues returns a StrDocValues view of the literal.
func (l *LiteralValueSource) GetValues(_ function.Context, _ *index.LeafReaderContext) (function.FunctionValues, error) {
	value := l.Literal
	bytes := []byte(value)
	v := &literalStrFunctionValues{
		StrDocValues: *docvalues.NewStrDocValues(l, func(_ int) (string, error) { return value, nil }),
		bytes:        bytes,
		desc:         value,
	}
	v.SetSelf(v)
	return v, nil
}

type literalStrFunctionValues struct {
	docvalues.StrDocValues
	bytes []byte
	desc  string
}

func (v *literalStrFunctionValues) BytesVal(_ int, target *[]byte) (bool, error) {
	if target != nil {
		*target = append((*target)[:0], v.bytes...)
	}
	return true, nil
}

func (v *literalStrFunctionValues) ToString(_ int) (string, error) { return v.desc, nil }
