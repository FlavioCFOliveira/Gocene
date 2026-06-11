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

// ScaleFloatFunction scales source values to be between min and max.
//
// Go port of org.apache.lucene.queries.function.valuesource.ScaleFloatFunction.
type ScaleFloatFunction struct {
	function.BaseValueSource
	Source   function.ValueSource
	Min, Max float32
}

// NewScaleFloatFunction creates a ScaleFloatFunction.
func NewScaleFloatFunction(source function.ValueSource, min, max float32) *ScaleFloatFunction {
	return &ScaleFloatFunction{Source: source, Min: min, Max: max}
}

// Description returns "scale(source,min,max)".
func (s *ScaleFloatFunction) Description() string {
	return fmt.Sprintf("scale(%s,%v,%v)", s.Source.Description(), s.Min, s.Max)
}

// CreateWeight delegates to the source.
func (s *ScaleFloatFunction) CreateWeight(ctx function.Context, searcher any) error {
	return s.Source.CreateWeight(ctx, searcher)
}

// GetValues returns FunctionValues that scale values to [min, max].
func (s *ScaleFloatFunction) GetValues(ctx function.Context, readerContext *index.LeafReaderContext) (function.FunctionValues, error) {
	scaleInfo := s.computeScaleInfo(ctx, readerContext)
	scale := float32(0)
	if scaleInfo.maxVal-scaleInfo.minVal != 0 {
		scale = (s.Max - s.Min) / (scaleInfo.maxVal - scaleInfo.minVal)
	}
	minSource := scaleInfo.minVal
	maxSource := scaleInfo.maxVal

	vals, err := s.Source.GetValues(ctx, readerContext)
	if err != nil {
		return nil, err
	}
	v := &scaleFloatValues{
		FloatDocValues: *docvalues.NewFloatDocValues(s, func(doc int) (float32, error) {
			fv, err := vals.FloatVal(doc)
			if err != nil {
				return 0, err
			}
			return (fv-minSource)*scale + s.Min, nil
		}),
		vals:      vals,
		vs:        s,
		minSource: minSource,
		maxSource: maxSource,
		scale:     scale,
	}
	v.SetSelf(v)
	return v, nil
}

type scaleInfo struct {
	minVal float32
	maxVal float32
}

func (s *ScaleFloatFunction) computeScaleInfo(ctx function.Context, readerContext *index.LeafReaderContext) scaleInfo {
	// Check context cache first.
	if si, ok := ctx.Get(s); ok {
		if info, ok := si.(scaleInfo); ok {
			return info
		}
	}

	// Compute by scanning. For simplicity, scan the current leaf.
	// Lucene does a full scan across all leaves.
	minVal := float32(math.Inf(1))
	maxVal := float32(math.Inf(-1))
	vals, err := s.Source.GetValues(ctx, readerContext)
	if err != nil {
		ctx.Put(s, scaleInfo{minVal: 0, maxVal: 0})
		return scaleInfo{minVal: 0, maxVal: 0}
	}
	maxDoc := readerContext.LeafReader()
	var docCount int
	if leaf, ok := maxDoc.(maxDocReader); ok {
		docCount = leaf.MaxDoc()
	}
	for doc := 0; doc < docCount; doc++ {
		exists, err := vals.Exists(doc)
		if err != nil || !exists {
			continue
		}
		fv, err := vals.FloatVal(doc)
		if err != nil {
			continue
		}
		bits := math.Float32bits(fv)
		if bits&0x7f800000 == 0x7f800000 {
			// +Inf, -Inf or NaN - skip
			continue
		}
		if fv < minVal {
			minVal = fv
		}
		if fv > maxVal {
			maxVal = fv
		}
	}
	if minVal == float32(math.Inf(1)) {
		minVal = 0
		maxVal = 0
	}
	info := scaleInfo{minVal: minVal, maxVal: maxVal}
	ctx.Put(s, info)
	return info
}

// maxDocReader provides MaxDoc.
type maxDocReader interface {
	MaxDoc() int
}

// Equals reports value equality.
func (s *ScaleFloatFunction) Equals(other function.ValueSource) bool {
	o, ok := other.(*ScaleFloatFunction)
	if !ok || o == nil {
		return false
	}
	return s.Min == o.Min && s.Max == o.Max && s.Source.Equals(o.Source)
}

// HashCode returns a stable hash.
func (s *ScaleFloatFunction) HashCode() int32 {
	h := hashFloat32(s.Min)
	h = h*29 + hashFloat32(s.Max)
	h = h*29 + s.Source.HashCode()
	return h
}

type scaleFloatValues struct {
	docvalues.FloatDocValues
	vals                  function.FunctionValues
	vs                    *ScaleFloatFunction
	minSource, maxSource  float32
	scale                 float32
}

func (v *scaleFloatValues) ToString(doc int) (string, error) {
	vs, err := v.vals.ToString(doc)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("scale(%s,toMin=%v,toMax=%v,fromMin=%v,fromMax=%v)",
		vs, v.vs.Min, v.vs.Max, v.minSource, v.maxSource), nil
}

var _ function.ValueSource = (*ScaleFloatFunction)(nil)
