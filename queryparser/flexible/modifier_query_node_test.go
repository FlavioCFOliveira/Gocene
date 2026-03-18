package flexible

import (
	"testing"
)

func TestModifierQueryNode_New(t *testing.T) {
	child := NewFieldQueryNode("field", "value", 0, 10)
	node := NewModifierQueryNode(child, ModifierRequired)

	if node.GetModifier() != ModifierRequired {
		t.Errorf("GetModifier() = %v, want ModifierRequired", node.GetModifier())
	}

	if len(node.GetChildren()) != 1 {
		t.Errorf("Expected 1 child, got %d", len(node.GetChildren()))
	}
}

func TestModifierQueryNode_SetModifier(t *testing.T) {
	node := NewModifierQueryNode(nil, ModifierNone)
	node.SetModifier(ModifierProhibited)

	if node.GetModifier() != ModifierProhibited {
		t.Errorf("SetModifier failed, got %v", node.GetModifier())
	}
}

func TestModifierQueryNode_ToQueryString(t *testing.T) {
	child := NewFieldQueryNode("", "value", 0, 0)

	tests := []struct {
		modifier Modifier
		want     string
	}{
		{ModifierNone, "value"},
		{ModifierRequired, "+value"},
		{ModifierProhibited, "-value"},
	}

	for _, tt := range tests {
		node := NewModifierQueryNode(child, tt.modifier)
		got := node.ToQueryString(false)
		if got != tt.want {
			t.Errorf("Modifier %v: ToQueryString() = %v, want %v", tt.modifier, got, tt.want)
		}
	}
}

func TestModifierQueryNode_CloneTree(t *testing.T) {
	child := NewFieldQueryNode("field", "value", 0, 10)
	original := NewModifierQueryNode(child, ModifierRequired)
	original.SetTag("key", "value")

	cloned := original.CloneTree()
	clonedNode := cloned.(*ModifierQueryNode)

	if clonedNode.GetModifier() != original.GetModifier() {
		t.Error("Clone should have same modifier")
	}

	if clonedNode.GetTag("key") != "value" {
		t.Error("Clone should have copied tags")
	}

	if cloned == original {
		t.Error("Clone should be a different object")
	}
}

func TestBoostQueryNode_New(t *testing.T) {
	child := NewFieldQueryNode("field", "value", 0, 10)
	node := NewBoostQueryNode(child, 2.5)

	if node.GetValue() != 2.5 {
		t.Errorf("GetValue() = %v, want 2.5", node.GetValue())
	}

	if len(node.GetChildren()) != 1 {
		t.Errorf("Expected 1 child, got %d", len(node.GetChildren()))
	}
}

func TestBoostQueryNode_SetValue(t *testing.T) {
	node := NewBoostQueryNode(nil, 1.0)
	node.SetValue(3.0)

	if node.GetValue() != 3.0 {
		t.Errorf("SetValue failed, got %v", node.GetValue())
	}
}

func TestBoostQueryNode_ToQueryString(t *testing.T) {
	child := NewFieldQueryNode("", "value", 0, 0)

	tests := []struct {
		value float64
		want  string
	}{
		{1.0, "value"},
		{2.0, "value^2"},
		{2.5, "value^2.5"},
	}

	for _, tt := range tests {
		node := NewBoostQueryNode(child, tt.value)
		got := node.ToQueryString(false)
		if got != tt.want {
			t.Errorf("Value %v: ToQueryString() = %v, want %v", tt.value, got, tt.want)
		}
	}
}

func TestBoostQueryNode_CloneTree(t *testing.T) {
	child := NewFieldQueryNode("field", "value", 0, 10)
	original := NewBoostQueryNode(child, 2.5)
	original.SetTag("key", "value")

	cloned := original.CloneTree()
	clonedNode := cloned.(*BoostQueryNode)

	if clonedNode.GetValue() != original.GetValue() {
		t.Error("Clone should have same value")
	}

	if clonedNode.GetTag("key") != "value" {
		t.Error("Clone should have copied tags")
	}

	if cloned == original {
		t.Error("Clone should be a different object")
	}
}

func TestFuzzyQueryNode_New(t *testing.T) {
	node := NewFuzzyQueryNode("field", "value", 0.7, 2, 0, 10)

	if node.GetField() != "field" {
		t.Errorf("GetField() = %v, want field", node.GetField())
	}

	if node.GetText() != "value" {
		t.Errorf("GetText() = %v, want value", node.GetText())
	}

	if node.GetMinSimilarity() != 0.7 {
		t.Errorf("GetMinSimilarity() = %v, want 0.7", node.GetMinSimilarity())
	}

	if node.GetPrefixLength() != 2 {
		t.Errorf("GetPrefixLength() = %v, want 2", node.GetPrefixLength())
	}
}

func TestFuzzyQueryNode_Setters(t *testing.T) {
	node := NewFuzzyQueryNode("", "", 0.5, 0, 0, 0)

	node.SetMinSimilarity(0.8)
	if node.GetMinSimilarity() != 0.8 {
		t.Errorf("SetMinSimilarity failed, got %v", node.GetMinSimilarity())
	}

	node.SetPrefixLength(3)
	if node.GetPrefixLength() != 3 {
		t.Errorf("SetPrefixLength failed, got %v", node.GetPrefixLength())
	}
}

func TestFuzzyQueryNode_ToQueryString(t *testing.T) {
	tests := []struct {
		field         string
		text          string
		minSimilarity float64
		want          string
	}{
		{"", "value", 0.5, "value~"},
		{"", "value", 0.7, "value~0.7"},
		{"field", "value", 0.5, "field:value~"},
	}

	for _, tt := range tests {
		node := NewFuzzyQueryNode(tt.field, tt.text, tt.minSimilarity, 0, 0, 0)
		got := node.ToQueryString(false)
		if got != tt.want {
			t.Errorf("ToQueryString() = %v, want %v", got, tt.want)
		}
	}
}

func TestFuzzyQueryNode_CloneTree(t *testing.T) {
	original := NewFuzzyQueryNode("field", "value", 0.7, 2, 0, 10)
	original.SetTag("key", "value")

	cloned := original.CloneTree()
	clonedNode := cloned.(*FuzzyQueryNode)

	if clonedNode.GetMinSimilarity() != original.GetMinSimilarity() {
		t.Error("Clone should have same minSimilarity")
	}

	if clonedNode.GetPrefixLength() != original.GetPrefixLength() {
		t.Error("Clone should have same prefixLength")
	}

	if clonedNode.GetTag("key") != "value" {
		t.Error("Clone should have copied tags")
	}

	if cloned == original {
		t.Error("Clone should be a different object")
	}
}

func TestRangeQueryNode_New(t *testing.T) {
	node := NewRangeQueryNode("field", "start", "end", BoundInclusive, BoundExclusive)

	if node.GetField() != "field" {
		t.Errorf("GetField() = %v, want field", node.GetField())
	}

	if node.GetLower() != "start" {
		t.Errorf("GetLower() = %v, want start", node.GetLower())
	}

	if node.GetUpper() != "end" {
		t.Errorf("GetUpper() = %v, want end", node.GetUpper())
	}

	if node.GetLowerBound() != BoundInclusive {
		t.Errorf("GetLowerBound() = %v, want BoundInclusive", node.GetLowerBound())
	}

	if node.GetUpperBound() != BoundExclusive {
		t.Errorf("GetUpperBound() = %v, want BoundExclusive", node.GetUpperBound())
	}
}

func TestRangeQueryNode_Setters(t *testing.T) {
	node := NewRangeQueryNode("", "", "", BoundExclusive, BoundExclusive)

	node.SetField("newfield")
	if node.GetField() != "newfield" {
		t.Errorf("SetField failed, got %v", node.GetField())
	}

	node.SetLower("newlower")
	if node.GetLower() != "newlower" {
		t.Errorf("SetLower failed, got %v", node.GetLower())
	}

	node.SetUpper("newupper")
	if node.GetUpper() != "newupper" {
		t.Errorf("SetUpper failed, got %v", node.GetUpper())
	}

	node.SetLowerBound(BoundInclusive)
	if node.GetLowerBound() != BoundInclusive {
		t.Errorf("SetLowerBound failed, got %v", node.GetLowerBound())
	}

	node.SetUpperBound(BoundInclusive)
	if node.GetUpperBound() != BoundInclusive {
		t.Errorf("SetUpperBound failed, got %v", node.GetUpperBound())
	}
}

func TestRangeQueryNode_Inclusive(t *testing.T) {
	node := NewRangeQueryNode("", "", "", BoundInclusive, BoundInclusive)

	if !node.IsLowerInclusive() {
		t.Error("IsLowerInclusive should return true")
	}

	if !node.IsUpperInclusive() {
		t.Error("IsUpperInclusive should return true")
	}
}

func TestRangeQueryNode_ToQueryString(t *testing.T) {
	tests := []struct {
		field      string
		lower      string
		upper      string
		lowerBound BoundType
		upperBound BoundType
		want       string
	}{
		{"", "start", "end", BoundInclusive, BoundInclusive, "[start TO end]"},
		{"", "start", "end", BoundExclusive, BoundExclusive, "{start TO end}"},
		{"", "start", "end", BoundInclusive, BoundExclusive, "[start TO end}"},
		{"field", "*", "end", BoundInclusive, BoundInclusive, "field:[* TO end]"},
		{"", "start", "*", BoundInclusive, BoundInclusive, "[start TO *]"},
	}

	for _, tt := range tests {
		node := NewRangeQueryNode(tt.field, tt.lower, tt.upper, tt.lowerBound, tt.upperBound)
		got := node.ToQueryString(false)
		if got != tt.want {
			t.Errorf("ToQueryString() = %v, want %v", got, tt.want)
		}
	}
}

func TestRangeQueryNode_CloneTree(t *testing.T) {
	original := NewRangeQueryNode("field", "start", "end", BoundInclusive, BoundExclusive)
	original.SetTag("key", "value")

	cloned := original.CloneTree()
	clonedNode := cloned.(*RangeQueryNode)

	if clonedNode.GetField() != original.GetField() {
		t.Error("Clone should have same field")
	}

	if clonedNode.GetLower() != original.GetLower() {
		t.Error("Clone should have same lower")
	}

	if clonedNode.GetUpper() != original.GetUpper() {
		t.Error("Clone should have same upper")
	}

	if clonedNode.GetLowerBound() != original.GetLowerBound() {
		t.Error("Clone should have same lowerBound")
	}

	if clonedNode.GetUpperBound() != original.GetUpperBound() {
		t.Error("Clone should have same upperBound")
	}

	if clonedNode.GetTag("key") != "value" {
		t.Error("Clone should have copied tags")
	}

	if cloned == original {
		t.Error("Clone should be a different object")
	}
}

func TestPhraseSlopQueryNode_New(t *testing.T) {
	node := NewPhraseSlopQueryNode("field", "phrase text", 2, 0, 20)

	if node.GetField() != "field" {
		t.Errorf("GetField() = %v, want field", node.GetField())
	}

	if node.GetText() != "phrase text" {
		t.Errorf("GetText() = %v, want 'phrase text'", node.GetText())
	}

	if node.GetSlop() != 2 {
		t.Errorf("GetSlop() = %v, want 2", node.GetSlop())
	}
}

func TestPhraseSlopQueryNode_SetSlop(t *testing.T) {
	node := NewPhraseSlopQueryNode("", "", 0, 0, 0)
	node.SetSlop(5)

	if node.GetSlop() != 5 {
		t.Errorf("SetSlop failed, got %v", node.GetSlop())
	}
}

func TestPhraseSlopQueryNode_ToQueryString(t *testing.T) {
	tests := []struct {
		field string
		text  string
		slop  int
		want  string
	}{
		{"", "phrase", 0, "\"phrase\""},
		{"", "phrase", 2, "\"phrase\"~2"},
		{"field", "phrase", 3, "field:\"phrase\"~3"},
	}

	for _, tt := range tests {
		node := NewPhraseSlopQueryNode(tt.field, tt.text, tt.slop, 0, 0)
		got := node.ToQueryString(false)
		if got != tt.want {
			t.Errorf("ToQueryString() = %v, want %v", got, tt.want)
		}
	}
}

func TestPhraseSlopQueryNode_CloneTree(t *testing.T) {
	original := NewPhraseSlopQueryNode("field", "phrase", 2, 0, 10)
	original.SetTag("key", "value")

	cloned := original.CloneTree()
	clonedNode := cloned.(*PhraseSlopQueryNode)

	if clonedNode.GetSlop() != original.GetSlop() {
		t.Error("Clone should have same slop")
	}

	if clonedNode.GetTag("key") != "value" {
		t.Error("Clone should have copied tags")
	}

	if cloned == original {
		t.Error("Clone should be a different object")
	}
}

func TestGroupQueryNode_New(t *testing.T) {
	child := NewFieldQueryNode("field", "value", 0, 10)
	node := NewGroupQueryNode(child)

	if len(node.GetChildren()) != 1 {
		t.Errorf("Expected 1 child, got %d", len(node.GetChildren()))
	}
}

func TestGroupQueryNode_ToQueryString(t *testing.T) {
	child := NewFieldQueryNode("", "value", 0, 0)
	node := NewGroupQueryNode(child)

	got := node.ToQueryString(false)
	want := "(value)"

	if got != want {
		t.Errorf("ToQueryString() = %v, want %v", got, want)
	}
}

func TestGroupQueryNode_CloneTree(t *testing.T) {
	child := NewFieldQueryNode("field", "value", 0, 10)
	original := NewGroupQueryNode(child)
	original.SetTag("key", "value")

	cloned := original.CloneTree()
	clonedNode := cloned.(*GroupQueryNode)

	if clonedNode.GetTag("key") != "value" {
		t.Error("Clone should have copied tags")
	}

	if len(clonedNode.GetChildren()) != len(original.GetChildren()) {
		t.Error("Clone should have same number of children")
	}

	if cloned == original {
		t.Error("Clone should be a different object")
	}
}

func TestMatchAllDocsQueryNode_New(t *testing.T) {
	node := NewMatchAllDocsQueryNode()

	if node.ToQueryString(false) != "*:*" {
		t.Errorf("ToQueryString() = %v, want *:*", node.ToQueryString(false))
	}
}

func TestMatchAllDocsQueryNode_CloneTree(t *testing.T) {
	original := NewMatchAllDocsQueryNode()
	original.SetTag("key", "value")

	cloned := original.CloneTree()
	clonedNode := cloned.(*MatchAllDocsQueryNode)

	if clonedNode.GetTag("key") != "value" {
		t.Error("Clone should have copied tags")
	}

	if cloned == original {
		t.Error("Clone should be a different object")
	}
}

func TestMatchNoDocsQueryNode_New(t *testing.T) {
	node := NewMatchNoDocsQueryNode()

	if node.ToQueryString(false) != "+ - + -" {
		t.Errorf("ToQueryString() = %v, want '+ - + -'", node.ToQueryString(false))
	}
}

func TestMatchNoDocsQueryNode_CloneTree(t *testing.T) {
	original := NewMatchNoDocsQueryNode()
	original.SetTag("key", "value")

	cloned := original.CloneTree()
	clonedNode := cloned.(*MatchNoDocsQueryNode)

	if clonedNode.GetTag("key") != "value" {
		t.Error("Clone should have copied tags")
	}

	if cloned == original {
		t.Error("Clone should be a different object")
	}
}
