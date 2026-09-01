// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package valuesource

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/function"
	"github.com/FlavioCFOliveira/Gocene/queries/function/docvalues"
)

// ScaleFloatFunction scales values to be between min and max.
//
// Go port of org.apache.lucene.queries.function.valuesource.ScaleFloatFunction.
type ScaleFloatFunction struct {
	function.BaseValueSource
	source function.ValueSource
	min    float32
	max    float32
}

type scaleInfo struct {
	minVal float32
	maxVal float32
}

// NewScaleFloatFunction creates a ScaleFloatFunction.
func NewScaleFloatFunction(source function.ValueSource, min, max float32) *ScaleFloatFunction {
	return &ScaleFloatFunction{
		source: source,
		min:    min,
		max:    max,
	}
}

func (s *ScaleFloatFunction) Description() string {
	return fmt.Sprintf("scale(%s,%f,%f)", s.source.Description(), s.min, s.max)
}

func (s *ScaleFloatFunction) CreateWeight(ctx function.Context, searcher any) error {
	return s.source.CreateWeight(ctx, searcher)
}

func (s *ScaleFloatFunction) createScaleInfo(ctx function.Context, readerContext *index.LeafReaderContext) *scaleInfo {
	topLevel := index.ReaderUtilGetTopLevelContext(readerContext)
	leaves, err := topLevel.Leaves()
	if err != nil {
		// This should not happen if readerContext is valid
		return &scaleInfo{minVal: 0, maxVal: 0}
	}

	minVal := float32(math.Inf(1))
	maxVal := float32(math.Inf(-1))

	for _, leaf := range leaves {
		maxDoc := leaf.Reader().MaxDoc()
		vals, err := s.source.GetValues(ctx, leaf)
		if err != nil {
			continue
		}
		for i := 0; i < maxDoc; i++ {
			exists, err := vals.Exists(i)
			if err != nil || !exists {
				continue
			}
			val, err := vals.FloatVal(i)
			if err != nil {
				continue
			}
			if math.IsInf(float64(val), 0) || math.IsNaN(float64(val)) {
				continue
			}
			if val < minVal {
				minVal = val
			}
			if val > maxVal {
				maxVal = val
			}
		}
	}

	if minVal == float32(math.Inf(1)) {
		minVal = 0
		maxVal = 0
	}

	info := &scaleInfo{
		minVal: minVal,
		maxVal: maxVal,
	}
	ctx[s] = info
	return info
}

func (s *ScaleFloatFunction) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	info, ok := ctx[s].(*scaleInfo)
	if !ok {
		info = s.createScaleInfo(ctx, readerContext)
	}

	scale := float32(0)
	if info.maxVal-info.minVal != 0 {
		scale = (s.max - s.min) / (info.maxVal - info.minVal)
	}
	minSource := info.minVal

	vals, err := s.source.GetValues(ctx, readerContext)
	if err != nil {
		return nil, err
	}

	v := &scaleFloatFunctionValues{
		FloatDocValues: *docvalues.NewFloatDocValues(s, func(doc int) (float32, error) {
			fv, err := vals.FloatVal(doc)
			if err != nil {
				return 0, err
			}
			return (fv-minSource)*scale + s.min, nil
		}),
		vals:      vals,
		scale:     scale,
		minSource: minSource,
		maxSource: info.maxVal,
		self:      s,
	}
	v.SetSelf(v)
	return v, nil
}

func (s *ScaleFloatFunction) Equals(other function.ValueSource) bool {
	o, ok := other.(*ScaleFloatFunction)
	if !ok || o == nil {
		return false
	}
	return s.min == o.min && s.max == o.max && s.source.Equals(o.source)
}

func (s *ScaleFloatFunction) HashCode() int32 {
	return hashFloat32(s.min) + hashFloat32(s.max) + s.source.HashCode()
}

type scaleFloatFunctionValues struct {
	docvalues.FloatDocValues
	vals      function.FunctionValues
	scale     float32
	minSource float32
	maxSource float32
	self      *ScaleFloatFunction
}

func (v *scaleFloatFunctionValues) ToString(doc int) (string, error) {
	fv, err := v.vals.ToString(doc)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("scale(%s,toMin=%f,toMax=%f,fromMin=%f,fromMax=%f)",
		fv, v.self.min, v.self.max, v.minSource, v.maxSource), nil
}

var _ function.ValueSource = (*ScaleFloatFunction)(nil)
