package document

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

func TestContextQuery_Automaton(t *testing.T) {
	inner := NewCompletionQuery("test", nil)
	q := NewContextQuery(inner, "ctx1", "ctx2")

	auto := q.toContextAutomaton()
	if auto == nil {
		t.Fatal("Automaton should not be nil")
	}

	// Test matching
	// "ctx1" + SEP(0x1F)
	input := []int{'c', 't', 'x', '1', 0x1F}
	if !auto.Accepts(input) {
		t.Errorf("Automaton should accept ctx1")
	}

	input = []int{'c', 't', 'x', '2', 0x1F}
	if !auto.Accepts(input) {
		t.Errorf("Automaton should accept ctx2")
	}

	input = []int{'c', 't', 'x', '3', 0x1F}
	if auto.Accepts(input) {
		t.Errorf("Automaton should NOT accept ctx3")
	}
}

func TestContextQuery_CreateWeight(t *testing.T) {
	inner := NewCompletionQuery("test", nil)
	q := NewContextQuery(inner, "ctx1")
	q.AddContext("ctx2", 2.0, true)

	weight, err := q.CreateWeight(nil, search.ScoreModeDefault, 1.0)
	if err != nil {
		t.Fatalf("CreateWeight failed: %v", err)
	}

	// Test boosts
	weight.SetNextMatch([]int{'c', 't', 'x', '1', 0x1F})
	if weight.Boost() != 1.0+1.0 { // ctx1 boost (1.0) + inner boost (1.0)
		t.Errorf("Expected boost 2.0, got %f", weight.Boost())
	}

	weight.SetNextMatch([]int{'c', 't', 'x', '2', 0x1F})
	if weight.Boost() != 2.0+1.0 { // ctx2 boost (2.0) + inner boost (1.0)
		t.Errorf("Expected boost 3.0, got %f", weight.Boost())
	}

	weight.SetNextMatch([]int{'u', 'n', 'k', 'n', 'o', 'w', 'n', 0x1F})
	if weight.Boost() != 0+1.0 {
		t.Errorf("Expected boost 1.0, got %f", weight.Boost())
	}
}
