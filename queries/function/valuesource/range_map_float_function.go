// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/queries/function/docvalues"
)

// RangeMapFloatFunction maps source values within [min, max] to a target
// value, with optional default for values outside the range.
//
// Go port of org.apache.lucene.queries.function.valuesource.RangeMapFloatFunction.
type RangeMapFloatFunction struct {
	function.BaseValueSource
	Source     function.ValueSource
	Min, Max   float32
	Target     function.ValueSource
	DefaultVal function.ValueSource // may be nil
}

// NewRangeMapFloatFunction creates a RangeMapFloatFunction with float
// target and optional float default.
func NewRangeMapFloatFunction(source function.ValueSource, min, max float32, target float32, def *float32) *RangeMapFloatFunction {
	var defVS function.ValueSource
	if def != nil {
		defVS = NewConstValueSource(*def)
	}
	return &RangeMapFloatFunction{
		Source:     source,
		Min:        min,
		Max:        max,
		Target:     NewConstValueSource(target),
		DefaultVal: defVS,
	}
}

// NewRangeMapFloatFunctionWithSources creates a RangeMapFloatFunction with
// ValueSource target and optional default.
func NewRangeMapFloatFunctionWithSources(source function.ValueSource, min, max float32, target, def function.ValueSource) *RangeMapFloatFunction {
	return &RangeMapFloatFunction{
		Source:     source,
		Min:        min,
		Max:        max,
		Target:     target,
		DefaultVal: def,
	}
}

// Description returns "map(source,min,max,target,default)".
func (s *RangeMapFloatFunction) Description() string {
	defStr := "null"
	if s.DefaultVal != nil {
		defStr = s.DefaultVal.Description()
	}
	return fmt.Sprintf("map(%s,%v,%v,%s,%s)", s.Source.Description(), s.Min, s.Max, s.Target.Description(), defStr)
}

// CreateWeight delegates to the source.
func (s *RangeMapFloatFunction) CreateWeight(ctx function.Context, searcher any) error {
	if err := s.Source.CreateWeight(ctx, searcher); err != nil {
		return err
	}
	return nil
}

// GetValues returns FunctionValues that map values in [min,max] to target.
func (s *RangeMapFloatFunction) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	vals, err := s.Source.GetValues(ctx, readerContext)
	if err != nil {
		return nil, err
	}
	targets, err := s.Target.GetValues(ctx, readerContext)
	if err != nil {
		return nil, err
	}
	var defaults function.FunctionValues
	if s.DefaultVal != nil {
		defaults, err = s.DefaultVal.GetValues(ctx, readerContext)
		if err != nil {
			return nil, err
		}
	}

	v := &rangeMapFloatValues{
		FloatDocValues: *docvalues.NewFloatDocValues(s, func(doc int) (float32, error) {
			fv, err := vals.FloatVal(doc)
			if err != nil {
				return 0, err
			}
			if fv >= s.Min && fv <= s.Max {
				return targets.FloatVal(doc)
			}
			if defaults != nil {
				return defaults.FloatVal(doc)
			}
			return fv, nil
		}),
		vals:     vals,
		targets:  targets,
		defaults: defaults,
		vs:       s,
	}
	v.SetSelf(v)
	return v, nil
}

// Equals reports value equality.
func (s *RangeMapFloatFunction) Equals(other function.ValueSource) bool {
	o, ok := other.(*RangeMapFloatFunction)
	if !ok || o == nil {
		return false
	}
	if s.Min != o.Min || s.Max != o.Max {
		return false
	}
	if !s.Target.Equals(o.Target) {
		return false
	}
	if !s.Source.Equals(o.Source) {
		return false
	}
	if s.DefaultVal == nil && o.DefaultVal == nil {
		return true
	}
	if s.DefaultVal == nil || o.DefaultVal == nil {
		return false
	}
	return s.DefaultVal.Equals(o.DefaultVal)
}

// HashCode returns a stable hash.
func (s *RangeMapFloatFunction) HashCode() int32 {
	h := s.Source.HashCode()
	h ^= (h << 10) | (h >> 23)
	h += hashFloat32(s.Min)
	h ^= (h << 14) | (h >> 19)
	h += hashFloat32(s.Max)
	h += s.Target.HashCode()
	if s.DefaultVal != nil {
		h += s.DefaultVal.HashCode()
	}
	return h
}

type rangeMapFloatValues struct {
	docvalues.FloatDocValues
	vals, targets, defaults function.FunctionValues
	vs                      *RangeMapFloatFunction
}

func (v *rangeMapFloatValues) ToString(doc int) (string, error) {
	vs, err := v.vals.ToString(doc)
	if err != nil {
		return "", err
	}
	ts, err := v.targets.ToString(doc)
	if err != nil {
		return "", err
	}
	defStr := "null"
	if v.defaults != nil {
		ds, err := v.defaults.ToString(doc)
		if err == nil {
			defStr = ds
		}
	}
	return fmt.Sprintf("map(%s,min=%v,max=%v,target=%s,defaultVal=%s)", vs, v.vs.Min, v.vs.Max, ts, defStr), nil
}

var _ function.ValueSource = (*RangeMapFloatFunction)(nil)
