package join

import "testing"

func TestCheckJoinIndexWellFormed(t *testing.T) {
	// child, child, parent, child, parent
	docs := []int{0, 1, 2, 3, 4}
	isParent := []bool{false, false, true, false, true}
	if err := CheckJoinIndex(docs, isParent); err != nil {
		t.Errorf("expected ok, got %v", err)
	}
}

func TestCheckJoinIndexTrailingChild(t *testing.T) {
	docs := []int{0, 1, 2}
	isParent := []bool{false, true, false}
	if err := CheckJoinIndex(docs, isParent); err == nil {
		t.Error("trailing child should fail")
	}
}
