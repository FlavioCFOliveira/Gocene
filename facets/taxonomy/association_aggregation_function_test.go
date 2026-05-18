package taxonomy

import "testing"

func TestSumAggregation(t *testing.T) {
	if SUM.Name() != "SUM" {
		t.Error("name")
	}
	if SUM.Aggregate(1, 2) != 3 {
		t.Error("float")
	}
	if SUM.AggregateInt(1, 2) != 3 {
		t.Error("int")
	}
}

func TestMaxAggregation(t *testing.T) {
	if MAX.Name() != "MAX" {
		t.Error("name")
	}
	if MAX.Aggregate(1, 2) != 2 {
		t.Error("float")
	}
	if MAX.AggregateInt(5, 2) != 5 {
		t.Error("int")
	}
}
