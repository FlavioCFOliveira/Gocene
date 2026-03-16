package join

import (
	"testing"
)

func TestNewJoinUtil(t *testing.T) {
	ju := NewJoinUtil()
	if ju == nil {
		t.Fatal("Expected JoinUtil to be created")
	}
}

func TestScoreModeString(t *testing.T) {
	tests := []struct {
		mode     ScoreMode
		expected string
	}{
		{None, "None"},
		{Avg, "Avg"},
		{Max, "Max"},
		{Total, "Total"},
		{Min, "Min"},
		{ScoreMode(999), "Unknown"},
	}

	for _, test := range tests {
		if test.mode.String() != test.expected {
			t.Errorf("Expected ScoreMode %d to be '%s', got '%s'", test.mode, test.expected, test.mode.String())
		}
	}
}

func TestNewDocIdBitSet(t *testing.T) {
	bs := NewDocIdBitSet(100)

	if bs == nil {
		t.Fatal("Expected DocIdBitSet to be created")
	}

	if bs.Length() != 100 {
		t.Errorf("Expected length 100, got %d", bs.Length())
	}
}

func TestDocIdBitSetSetAndGet(t *testing.T) {
	bs := NewDocIdBitSet(100)

	// Set some bits
	bs.Set(0)
	bs.Set(50)
	bs.Set(99)

	// Check they are set
	if !bs.Get(0) {
		t.Error("Expected bit 0 to be set")
	}
	if !bs.Get(50) {
		t.Error("Expected bit 50 to be set")
	}
	if !bs.Get(99) {
		t.Error("Expected bit 99 to be set")
	}

	// Check unset bits
	if bs.Get(1) {
		t.Error("Expected bit 1 to be unset")
	}
	if bs.Get(49) {
		t.Error("Expected bit 49 to be unset")
	}

	// Out of bounds
	if bs.Get(-1) {
		t.Error("Expected bit -1 to be unset (out of bounds)")
	}
	if bs.Get(100) {
		t.Error("Expected bit 100 to be unset (out of bounds)")
	}
}

func TestDocIdBitSetClear(t *testing.T) {
	bs := NewDocIdBitSet(100)

	bs.Set(50)
	if !bs.Get(50) {
		t.Error("Expected bit 50 to be set")
	}

	bs.Clear(50)
	if bs.Get(50) {
		t.Error("Expected bit 50 to be cleared")
	}
}

func TestDocIdBitSetCardinality(t *testing.T) {
	bs := NewDocIdBitSet(100)

	if bs.Cardinality() != 0 {
		t.Errorf("Expected cardinality 0, got %d", bs.Cardinality())
	}

	bs.Set(0)
	bs.Set(50)
	bs.Set(99)

	if bs.Cardinality() != 3 {
		t.Errorf("Expected cardinality 3, got %d", bs.Cardinality())
	}
}

func TestDocIdBitSetAnd(t *testing.T) {
	bs1 := NewDocIdBitSet(100)
	bs2 := NewDocIdBitSet(100)

	bs1.Set(0)
	bs1.Set(50)
	bs2.Set(50)
	bs2.Set(99)

	bs1.And(bs2)

	if bs1.Get(0) {
		t.Error("Expected bit 0 to be unset after AND")
	}
	if !bs1.Get(50) {
		t.Error("Expected bit 50 to be set after AND")
	}
	if bs1.Get(99) {
		t.Error("Expected bit 99 to be unset after AND")
	}
}

func TestDocIdBitSetOr(t *testing.T) {
	bs1 := NewDocIdBitSet(100)
	bs2 := NewDocIdBitSet(100)

	bs1.Set(0)
	bs2.Set(50)

	bs1.Or(bs2)

	if !bs1.Get(0) {
		t.Error("Expected bit 0 to be set after OR")
	}
	if !bs1.Get(50) {
		t.Error("Expected bit 50 to be set after OR")
	}
}

func TestPopCount(t *testing.T) {
	tests := []struct {
		value    uint64
		expected int
	}{
		{0, 0},
		{1, 1},
		{2, 1},
		{3, 2},
		{0xFF, 8},
		{0xFFFF, 16},
		{0xFFFFFFFFFFFFFFFF, 64},
	}

	for _, test := range tests {
		result := popCount(test.value)
		if result != test.expected {
			t.Errorf("popCount(%d) = %d, expected %d", test.value, result, test.expected)
		}
	}
}

func TestMatchNoDocsQuery(t *testing.T) {
	q := &MatchNoDocsQuery{}

	// Test Rewrite
	rewritten, err := q.Rewrite(nil)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if rewritten != q {
		t.Error("Expected Rewrite to return same query")
	}

	// Test Clone
	cloned := q.Clone()
	if cloned == nil {
		t.Error("Expected Clone to return non-nil")
	}

	// Test Equals
	if !q.Equals(&MatchNoDocsQuery{}) {
		t.Error("Expected Equals to return true for same type")
	}

	// Test HashCode
	if q.HashCode() != 0 {
		t.Errorf("Expected HashCode to be 0, got %d", q.HashCode())
	}
}

func TestTermsQuery(t *testing.T) {
	terms := [][]byte{[]byte("a"), []byte("b"), []byte("c")}
	q := NewTermsQuery("field", terms)

	// Test Clone
	cloned := q.Clone()
	if cloned == nil {
		t.Fatal("Expected Clone to return non-nil")
	}
	clonedTerms, ok := cloned.(*TermsQuery)
	if !ok {
		t.Fatal("Expected Clone to return *TermsQuery")
	}
	if len(clonedTerms.terms) != len(q.terms) {
		t.Error("Expected cloned terms to have same length")
	}

	// Test Equals
	if !q.Equals(cloned) {
		t.Error("Expected Equals to return true for cloned query")
	}

	// Test with different terms
	q2 := NewTermsQuery("field", [][]byte{[]byte("x")})
	if q.Equals(q2) {
		t.Error("Expected Equals to return false for different terms")
	}

	// Test with different field
	q3 := NewTermsQuery("other", terms)
	if q.Equals(q3) {
		t.Error("Expected Equals to return false for different field")
	}

	// Test HashCode
	hash := q.HashCode()
	if hash == 0 {
		t.Error("Expected non-zero HashCode")
	}
}
