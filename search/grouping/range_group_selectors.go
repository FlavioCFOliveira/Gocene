package grouping

import (
	"fmt"
	"math"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// LongRange represents a contiguous range of long values, with an inclusive minimum and exclusive maximum.
type LongRange struct {
	Min int64
	Max int64
}

func (r LongRange) String() string {
	return fmt.Sprintf("LongRange(%d, %d)", r.Min, r.Max)
}

// LongRangeFactory groups long values into ranges.
type LongRangeFactory struct {
	min   int64
	width int64
	max   int64
}

func NewLongRangeFactory(min, width, max int64) *LongRangeFactory {
	return &LongRangeFactory{
		min:   min,
		width: width,
		max:   max,
	}
}

func (f *LongRangeFactory) GetRange(value int64, reuse *LongRange) *LongRange {
	if reuse == nil {
		reuse = &LongRange{Min: math.MinInt64, Max: math.MaxInt64}
	}
	if value < f.min {
		reuse.Max = f.min
		reuse.Min = math.MinInt64
		return reuse
	}
	if value >= f.max {
		reuse.Min = f.max
		reuse.Max = math.MaxInt64
		return reuse
	}
	bucket := (value - f.min) / f.width
	reuse.Min = f.min + (bucket * f.width)
	reuse.Max = reuse.Min + f.width
	return reuse
}

// LongRangeGroupSelector implements GroupSelector for long values.
type LongRangeGroupSelector struct {
	source       search.LongValuesSource
	rangeFactory *LongRangeFactory
	inSecondPass map[LongRange]struct{}
	includeEmpty bool
	positioned   bool
	current      *LongRange
	context      index.LeafReaderContext
	values       search.LongValues
}

func NewLongRangeGroupSelector(source search.LongValuesSource, rangeFactory *LongRangeFactory) *LongRangeGroupSelector {
	return &LongRangeGroupSelector{
		source:       source,
		rangeFactory: rangeFactory,
	}
}

func (s *LongRangeGroupSelector) SetNextReader(readerContext index.LeafReaderContext) error {
	s.context = readerContext
	return nil
}

func (s *LongRangeGroupSelector) SetScorer(scorer search.Scorable) error {
	var err error
	s.values, err = s.source.GetValues(s.context, search.DoubleValuesSourceFromScorer(scorer))
	return err
}

func (s *LongRangeGroupSelector) AdvanceTo(doc int) (State, error) {
	positioned, err := s.values.AdvanceExact(doc)
	if err != nil {
		return StateSkip, err
	}
	s.positioned = positioned
	if !positioned {
		if s.includeEmpty {
			return StateAccept, nil
		}
		return StateSkip, nil
	}
	s.current = s.rangeFactory.GetRange(s.values.LongValue(), s.current)
	if s.inSecondPass == nil {
		return StateAccept, nil
	}
	if _, ok := s.inSecondPass[*s.current]; ok {
		return StateAccept, nil
	}
	return StateSkip, nil
}

func (s *LongRangeGroupSelector) CurrentValue() (LongRange, error) {
	if !s.positioned || s.current == nil {
		return LongRange{}, fmt.Errorf("selector not positioned on a value")
	}
	return *s.current, nil
}

func (s *LongRangeGroupSelector) CopyValue() (LongRange, error) {
	if !s.positioned || s.current == nil {
		return LongRange{}, fmt.Errorf("selector not positioned on a value")
	}
	return LongRange{Min: s.current.Min, Max: s.current.Max}, nil
}

func (s *LongRangeGroupSelector) SetGroups(groups []SearchGroup[LongRange]) {
	s.inSecondPass = make(map[LongRange]struct{})
	for _, group := range groups {
		if group.GroupValue == nil { // Note: this assumes GroupValue is a pointer or can be nil.
                                   // In Go, for a struct, we might need a pointer if it can be null.
			s.includeEmpty = true
		} else {
			s.inSecondPass[*group.GroupValue] = struct{}{}
		}
	}
}

// DoubleRange represents a contiguous range of double values, with an inclusive minimum and exclusive maximum.
type DoubleRange struct {
	Min float64
	Max float64
}

func (r DoubleRange) String() string {
	return fmt.Sprintf("DoubleRange(%f, %f)", r.Min, r.Max)
}

// DoubleRangeFactory groups double values into ranges.
type DoubleRangeFactory struct {
	min   float64
	width float64
	max   float64
}

func NewDoubleRangeFactory(min, width, max float64) *DoubleRangeFactory {
	return &DoubleRangeFactory{
		min:   min,
		width: width,
		max:   max,
	}
}

func (f *DoubleRangeFactory) GetRange(value float64, reuse *DoubleRange) *DoubleRange {
	if reuse == nil {
		reuse = &DoubleRange{Min: -math.MaxFloat64, Max: math.MaxFloat64}
	}
	if value < f.min {
		reuse.Max = f.min
		reuse.Min = -math.MaxFloat64
		return reuse
	}
	if value >= f.max {
		reuse.Min = f.max
		reuse.Max = math.MaxFloat64
		return reuse
	}
	bucket := math.Floor((value - f.min) / f.width)
	reuse.Min = f.min + (bucket * f.width)
	reuse.Max = reuse.Min + f.width
	return reuse
}

// DoubleRangeGroupSelector implements GroupSelector for double values.
type DoubleRangeGroupSelector struct {
	source       search.DoubleValuesSource
	rangeFactory *DoubleRangeFactory
	inSecondPass map[DoubleRange]struct{}
	includeEmpty bool
	positioned   bool
	current      *DoubleRange
	context      index.LeafReaderContext
	values       search.DoubleValues
}

func NewDoubleRangeGroupSelector(source search.DoubleValuesSource, rangeFactory *DoubleRangeFactory) *DoubleRangeGroupSelector {
	return &DoubleRangeGroupSelector{
		source:       source,
		rangeFactory: rangeFactory,
	}
}

func (s *DoubleRangeGroupSelector) SetNextReader(readerContext index.LeafReaderContext) error {
	s.context = readerContext
	return nil
}

func (s *DoubleRangeGroupSelector) SetScorer(scorer search.Scorable) error {
	var err error
	s.values, err = s.source.GetValues(s.context, search.DoubleValuesSourceFromScorer(scorer))
	return err
}

func (s *DoubleRangeGroupSelector) AdvanceTo(doc int) (State, error) {
	positioned, err := s.values.AdvanceExact(doc)
	if err != nil {
		return StateSkip, err
	}
	s.positioned = positioned
	if !positioned {
		if s.includeEmpty {
			return StateAccept, nil
		}
		return StateSkip, nil
	}
	s.current = s.rangeFactory.GetRange(s.values.DoubleValue(), s.current)
	if s.inSecondPass == nil {
		return StateAccept, nil
	}
	if _, ok := s.inSecondPass[*s.current]; ok {
		return StateAccept, nil
	}
	return StateSkip, nil
}

func (s *DoubleRangeGroupSelector) CurrentValue() (DoubleRange, error) {
	if !s.positioned || s.current == nil {
		return DoubleRange{}, fmt.Errorf("selector not positioned on a value")
	}
	return *s.current, nil
}

func (s *DoubleRangeGroupSelector) CopyValue() (DoubleRange, error) {
	if !s.positioned || s.current == nil {
		return DoubleRange{}, fmt.Errorf("selector not positioned on a value")
	}
	return DoubleRange{Min: s.current.Min, Max: s.current.Max}, nil
}

func (s *DoubleRangeGroupSelector) SetGroups(groups []SearchGroup[DoubleRange]) {
	s.inSecondPass = make(map[DoubleRange]struct{})
	for _, group := range groups {
		if group.GroupValue == nil {
			s.includeEmpty = true
		} else {
			s.inSecondPass[*group.GroupValue] = struct{}{}
		}
	}
}
