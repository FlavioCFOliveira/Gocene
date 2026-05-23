package grouping

import (
	"testing"
)

// buildLongFactory creates a simple factory with three [0,10) [10,20) [20,30) ranges.
func buildLongFactory() *LongRangeFactory {
	return NewLongRangeFactory(
		NewLongRange("0-10", 0, 10),
		NewLongRange("10-20", 10, 20),
		NewLongRange("20-30", 20, 30),
	)
}

// TestLongRangeGroupSelectorSelectsCorrectRange verifies that Select returns
// the expected range bucket. Mirrors the intent of TestLongRangeGroupSelector
// from Lucene 10.4.0.
func TestLongRangeGroupSelectorSelectsCorrectRange(t *testing.T) {
	values := map[int]int64{0: 5, 1: 15, 2: 25}
	fn := func(doc int) (int64, bool) {
		v, ok := values[doc]
		return v, ok
	}
	s := NewLongRangeGroupSelector(buildLongFactory(), fn)

	tests := []struct {
		doc     int
		wantMin int64
		wantMax int64
	}{
		{0, 0, 10},
		{1, 10, 20},
		{2, 20, 30},
	}

	for _, tt := range tests {
		got := s.Select(tt.doc)
		r, ok := got.(*LongRange)
		if !ok {
			t.Fatalf("doc %d: Select returned %T, want *LongRange", tt.doc, got)
		}
		if r.Min != tt.wantMin || r.Max != tt.wantMax {
			t.Errorf("doc %d: range [%d,%d), want [%d,%d)",
				tt.doc, r.Min, r.Max, tt.wantMin, tt.wantMax)
		}
	}
}

// TestLongRangeGroupSelectorNoValue verifies that missing doc values produce nil.
func TestLongRangeGroupSelectorNoValue(t *testing.T) {
	fn := func(doc int) (int64, bool) { return 0, false }
	s := NewLongRangeGroupSelector(buildLongFactory(), fn)
	if s.Select(0) != nil {
		t.Fatalf("expected nil for doc without value, got %v", s.Select(0))
	}
}

// TestLongRangeGroupSelectorAdvanceTo verifies the AdvanceTo / CurrentValue /
// CopyValue lifecycle.
func TestLongRangeGroupSelectorAdvanceTo(t *testing.T) {
	fn := func(doc int) (int64, bool) {
		if doc == 0 {
			return 12, true
		}
		return 0, false
	}
	s := NewLongRangeGroupSelector(buildLongFactory(), fn)

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
		t.Errorf("CurrentValue = [%d,%d), want [10,20)", cv.Min, cv.Max)
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

// TestLongRangeGroupSelectorSetGroupsSecondPass verifies second-pass filtering.
func TestLongRangeGroupSelectorSetGroupsSecondPass(t *testing.T) {
	values := map[int]int64{0: 5, 1: 15, 2: 25}
	fn := func(doc int) (int64, bool) {
		v, ok := values[doc]
		return v, ok
	}
	s := NewLongRangeGroupSelector(buildLongFactory(), fn)
	// Second pass: only accept [10,20)
	accepted := NewLongRange("10-20", 10, 20)
	s.SetGroups([]*SearchGroup[*LongRange]{
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

// TestLongRangeGroupSelectorSetGroupsIncludesEmpty verifies that a nil group
// value in SetGroups causes documents without a value to be included.
func TestLongRangeGroupSelectorSetGroupsIncludesEmpty(t *testing.T) {
	fn := func(doc int) (int64, bool) { return 0, false }
	s := NewLongRangeGroupSelector(buildLongFactory(), fn)
	s.SetGroups([]*SearchGroup[*LongRange]{
		{GroupValue: nil},
	})
	if !s.AdvanceTo(0) {
		t.Error("doc without value should be accepted when empty group is in second pass")
	}
}

// TestLongRangeGroupSelectorGroupSelectorInterface is a belt-and-suspenders
// runtime check that the type satisfies GroupSelector.
func TestLongRangeGroupSelectorGroupSelectorInterface(t *testing.T) {
	fn := func(doc int) (int64, bool) { return 5, true }
	var gs GroupSelector = NewLongRangeGroupSelector(buildLongFactory(), fn)
	if gs.Select(0) == nil {
		t.Error("Select on a doc with value must not return nil")
	}
}
