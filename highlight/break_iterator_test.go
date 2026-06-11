package highlight

import (
	"testing"
)

func TestTextFragment_New(t *testing.T) {
	tf := NewTextFragment("text", 0, 0, 10)
	if tf == nil {
		t.Fatal("NewTextFragment returned nil")
	}
	if tf.GetFragNum() != 0 {
		t.Fatalf("GetFragNum=%d", tf.GetFragNum())
	}
}

func TestTextFragment_Score(t *testing.T) {
	tf := NewTextFragment("marked", 1, 5, 15)
	tf.SetScore(0.75)
	if tf.GetScore() != 0.75 {
		t.Fatalf("GetScore=%v", tf.GetScore())
	}
}

func TestTextFragment_DifferentFragNums(t *testing.T) {
	a := NewTextFragment("a", 0, 0, 5)
	b := NewTextFragment("b", 1, 6, 10)
	if a.GetFragNum() == b.GetFragNum() {
		t.Error("different fragments should have different frag nums")
	}
	if a.GetScore() == b.GetScore() && a.GetScore() != 0 {
		t.Error("new fragments should start with score 0")
	}
}
