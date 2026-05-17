// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package docvalues

import (
	"github.com/FlavioCFOliveira/Gocene/queries/function"
)

// StrDocValues is the Go port of
// org.apache.lucene.queries.function.docvalues.StrDocValues. Concrete
// embedders supply StrFunc; bool/object accessors are derived from it.
type StrDocValues struct {
	function.BaseFunctionValues
	VS      function.ValueSource
	StrFunc func(doc int) (string, error)
}

// NewStrDocValues wires vs + strFunc into a ready-to-use StrDocValues.
func NewStrDocValues(vs function.ValueSource, strFunc func(doc int) (string, error)) *StrDocValues {
	v := &StrDocValues{VS: vs, StrFunc: strFunc}
	v.SetSelf(v)
	return v
}

// StrVal delegates to the embedded function.
func (s *StrDocValues) StrVal(doc int) (string, error) { return s.StrFunc(doc) }

// ObjectVal returns the string when the doc has a value.
func (s *StrDocValues) ObjectVal(doc int) (any, error) {
	ok, err := s.Exists(doc)
	if err != nil || !ok {
		return nil, err
	}
	return s.StrFunc(doc)
}

// BoolVal mirrors Lucene's "boolVal == exists" convention for string DV.
func (s *StrDocValues) BoolVal(doc int) (bool, error) { return s.Exists(doc) }

// ToString renders "<vs.description>='<value>'".
func (s *StrDocValues) ToString(doc int) (string, error) {
	v, err := s.StrFunc(doc)
	if err != nil {
		return "", err
	}
	return s.VS.Description() + "='" + v + "'", nil
}
