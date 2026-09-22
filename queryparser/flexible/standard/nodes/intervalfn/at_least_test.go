package intervalfn

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queries/intervals"
	"github.com/FlavioCFOliveira/Gocene/search"
)

type mockSource struct{}

func (s *mockSource) Intervals(field string, ctx *index.LeafReaderContext) (intervals.IntervalIterator, error) {
	return nil, nil
}
func (s *mockSource) Matches(field string, ctx *index.LeafReaderContext, doc int) (intervals.IntervalMatchesIterator, error) {
	return nil, nil
}
func (s *mockSource) Visit(field string, visitor search.QueryVisitor) {}
func (s *mockSource) MinExtent() int                                  { return 0 }
func (s *mockSource) PullUpDisjunctions() []intervals.IntervalsSource { return nil }
func (s *mockSource) String() string                                  { return "mockSource" }

// Equals and HashCode keep Object's identity semantics, as a Java test mock
// that does not override them would.
func (s *mockSource) Equals(other intervals.IntervalsSource) bool {
	o, ok := other.(*mockSource)
	return ok && o == s
}
func (s *mockSource) HashCode() int { return 0 }

type mockIntervalFunction struct {
	name string
}

func (m *mockIntervalFunction) ToIntervalSource(field string, analyzer analysis.Analyzer) intervals.IntervalsSource {
	return &mockSource{}
}

func (m *mockIntervalFunction) String() string {
	return m.name
}

func TestAtLeast_String(t *testing.T) {
	sources := []IntervalFunction{
		&mockIntervalFunction{name: "src1"},
		&mockIntervalFunction{name: "src2"},
	}
	al := NewAtLeast(2, sources)
	expected := "fn:atLeast(2 src1 src2)"
	if al.String() != expected {
		t.Errorf("expected %q, got %q", expected, al.String())
	}
}

func TestAtLeast_SingleSource(t *testing.T) {
	sources := []IntervalFunction{
		&mockIntervalFunction{name: "src1"},
	}
	al := NewAtLeast(1, sources)
	expected := "fn:atLeast(1 src1)"
	if al.String() != expected {
		t.Errorf("expected %q, got %q", expected, al.String())
	}
}
