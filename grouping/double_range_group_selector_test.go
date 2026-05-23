package grouping

import (
	"testing"
)

// buildDoubleFactory creates a simple factory with three [0,10) [10,20) [20,30) ranges.
func buildDoubleFactory() *DoubleRangeFactory {
	return NewDoubleRangeFactory(
		NewDoubleRange("0-10", 0, 10),
		NewDoubleRange("10-20", 10, 20),
		NewDoubleRange("20-30", 20, 30),
	)
}

// TestDoubleRangeGroupSelectorSelectsCorrectRange verifies that Select returns
// the expected range bucket. Mirrors the intent of TestDoubleRangeGroupSelector
// from Lucene 10.4.0.
func TestDoubleRangeGroupSelectorSelectsCorrectRange(t *testing.T) {
	values := map[int]float64{0: 5.0, 1: 15.0, 2: 25.0}
	fn := func(doc int) (float64, bool) {
		v, ok := values[doc]
		return v, ok
	}
	s := NewDoubleRangeGroupSelector(buildDoubleFactory(), fn)

	tests := []struct {
		doc     int
		wantMin float64
		wantMax float64
	}{
		{0, 0, 10},
		{1, 10, 20},
		{2, 20, 30},
	}

	for _, tt := range tests {
		got := s.Select(tt.doc)
		r, ok := got.(*DoubleRange)
		if !ok {
			t.Fatalf("doc %d: Select returned %T, want *DoubleRange", tt.doc, got)
		}
		if r.Min != tt.wantMin || r.Max != tt.wantMax {
			t.Errorf("doc %d: range [%g,%g), want [%g,%g)",
				tt.doc, r.Min, r.Max, tt.wantMin, tt.wantMax)
		}
	}
}

// TestDoubleRangeGroupSelectorNoValue verifies that missing doc values produce
// nil — the "empty group" bucket.
func TestDoubleRangeGroupSelectorNoValue(t *testing.T) {
	fn := func(doc int) (float64, bool) { return 0, false }
	s := NewDoubleRangeGroupSelector(buildDoubleFactory(), fn)
	if s.Select(0) != nil {
		t.Fatalf("expected nil for doc without value, got %v", s.Select(0))
	}
}

// TestDoubleRangeGroupSelectorAdvanceTo verifies the AdvanceTo / CurrentValue /
// CopyValue lifecycle, matching DoubleRangeGroupSelector.advanceTo /
// currentValue / copyValue semantics.
func TestDoubleRangeGroupSelectorAdvanceTo(t *testing.T) {
	fn := func(doc int) (float64, bool) {
		if doc == 0 {
			return 12.5, true
		}
		return 0, false
	}
	s := NewDoubleRangeGroupSelector(buildDoubleFactory(), fn)

	// Doc with a value.
	accepted := s.AdvanceTo(0)
	if !accepted {
		t.Fatal("expected doc 0 to be accepted (first pass)")
	}
	cv := s.CurrentValue()
	if cv == nil {
		t.Fatal("CurrentValue must not be nil")
	}
	if cv.Min != 10 || cv.Max != 20 {
		t.Errorf("CurrentValue = [%g,%g), want [10,20)", cv.Min, cv.Max)
	}
	cp := s.CopyValue()
	if cp == cv {
		t.Error("CopyValue must return a distinct pointer")
	}
	if cp.Min != cv.Min || cp.Max != cv.Max {
		t.Error("CopyValue fields must equal CurrentValue fields")
	}

	// Doc without a value.
	s.AdvanceTo(99)
	if s.CurrentValue() != nil {
		t.Error("CurrentValue must be nil for doc without value")
	}
	if s.CopyValue() != nil {
		t.Error("CopyValue must be nil for doc without value")
	}
}

// TestDoubleRangeGroupSelectorSetGroupsSecondPass verifies that SetGroups
// restricts the selector to only the supplied group values, mirroring
// DoubleRangeGroupSelector.setGroups.
func TestDoubleRangeGroupSelectorSetGroupsSecondPass(t *testing.T) {
	// Values: doc 0 → [0,10), doc 1 → [10,20), doc 2 → [20,30)
	values := map[int]float64{0: 5.0, 1: 15.0, 2: 25.0}
	fn := func(doc int) (float64, bool) {
		v, ok := values[doc]
		return v, ok
	}
	s := NewDoubleRangeGroupSelector(buildDoubleFactory(), fn)
	// Second pass: only accept [10,20)
	accepted := NewDoubleRange("10-20", 10, 20)
	s.SetGroups([]*SearchGroup[*DoubleRange]{
		{GroupValue: accepted},
	})

	if s.AdvanceTo(0) {
		t.Error("doc 0 ([0,10)) should be rejected in second pass")
	}
	if !s.AdvanceTo(1) {
		t.Error("doc 1 ([10,20)) should be accepted in second pass")
	}
	if s.AdvanceTo(2) {
		t.Error("doc 2 ([20,30)) should be rejected in second pass")
	}
}

// TestDoubleRangeGroupSelectorSetGroupsIncludesEmpty verifies that a nil
// group value in SetGroups causes documents without a value to be included.
func TestDoubleRangeGroupSelectorSetGroupsIncludesEmpty(t *testing.T) {
	fn := func(doc int) (float64, bool) { return 0, false }
	s := NewDoubleRangeGroupSelector(buildDoubleFactory(), fn)
	// Second pass includes the empty group (nil groupValue)
	s.SetGroups([]*SearchGroup[*DoubleRange]{
		{GroupValue: nil},
	})
	if !s.AdvanceTo(0) {
		t.Error("doc without value should be accepted when empty group is in second pass")
	}
}

// TestDoubleRangeGroupSelectorGroupSelectorInterface ensures the type satisfies
// the GroupSelector interface (compile-time enforced via var _ assertion in
// source; this test is a belt-and-suspenders runtime check).
func TestDoubleRangeGroupSelectorGroupSelectorInterface(t *testing.T) {
	fn := func(doc int) (float64, bool) { return 5.0, true }
	var gs GroupSelector = NewDoubleRangeGroupSelector(buildDoubleFactory(), fn)
	if gs.Select(0) == nil {
		t.Error("Select on a doc with value must not return nil")
	}
}
