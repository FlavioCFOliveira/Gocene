package flexible

import (
	"fmt"
	"strings"
)

// BooleanQueryNode represents a boolean query node that can contain multiple clauses.
// It is the base class for AND, OR, and NOT query nodes.
type BooleanQueryNode struct {
	*QueryNodeImpl
	operator string
}

// NewBooleanQueryNode creates a new BooleanQueryNode with the given operator and children.
func NewBooleanQueryNode(operator string, children []QueryNode) *BooleanQueryNode {
	return &BooleanQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(children),
		operator:      operator,
	}
}

// GetOperator returns the boolean operator (AND, OR, NOT).
func (n *BooleanQueryNode) GetOperator() string {
	return n.operator
}

// SetOperator sets the boolean operator.
func (n *BooleanQueryNode) SetOperator(operator string) {
	n.operator = operator
}

// ToQueryString returns the query string representation.
func (n *BooleanQueryNode) ToQueryString(escapeSpecialSyntax bool) string {
	var sb strings.Builder

	children := n.GetChildren()
	for i, child := range children {
		if i > 0 {
			sb.WriteString(" ")
			sb.WriteString(strings.ToUpper(n.operator))
			sb.WriteString(" ")
		}
		sb.WriteString(child.ToQueryString(escapeSpecialSyntax))
	}

	return sb.String()
}

// CloneTree creates a deep copy of this node.
func (n *BooleanQueryNode) CloneTree() QueryNode {
	cloned := &BooleanQueryNode{
		QueryNodeImpl: NewQueryNodeImpl(nil),
		operator:      n.operator,
	}

	// Copy tags
	for _, key := range n.GetTagKeys() {
		cloned.SetTag(key, n.GetTag(key))
	}

	// Clone children
	for _, child := range n.GetChildren() {
		cloned.AddChild(child.CloneTree())
	}

	return cloned
}

// String returns a string representation of this node.
func (n *BooleanQueryNode) String() string {
	return fmt.Sprintf("<boolean operator=%s children=%d>", n.operator, len(n.GetChildren()))
}

// AndQueryNode represents an AND query node.
type AndQueryNode struct {
	*BooleanQueryNode
}

// NewAndQueryNode creates a new AndQueryNode with the given children.
func NewAndQueryNode(children []QueryNode) *AndQueryNode {
	return &AndQueryNode{
		BooleanQueryNode: NewBooleanQueryNode("AND", children),
	}
}

// CloneTree creates a deep copy of this node.
func (n *AndQueryNode) CloneTree() QueryNode {
	cloned := &AndQueryNode{
		BooleanQueryNode: NewBooleanQueryNode("AND", nil),
	}

	// Copy tags
	for _, key := range n.GetTagKeys() {
		cloned.SetTag(key, n.GetTag(key))
	}

	// Clone children
	for _, child := range n.GetChildren() {
		cloned.AddChild(child.CloneTree())
	}

	return cloned
}

// String returns a string representation of this node.
func (n *AndQueryNode) String() string {
	return fmt.Sprintf("<and children=%d>", len(n.GetChildren()))
}

// OrQueryNode represents an OR query node.
type OrQueryNode struct {
	*BooleanQueryNode
}

// NewOrQueryNode creates a new OrQueryNode with the given children.
func NewOrQueryNode(children []QueryNode) *OrQueryNode {
	return &OrQueryNode{
		BooleanQueryNode: NewBooleanQueryNode("OR", children),
	}
}

// CloneTree creates a deep copy of this node.
func (n *OrQueryNode) CloneTree() QueryNode {
	cloned := &OrQueryNode{
		BooleanQueryNode: NewBooleanQueryNode("OR", nil),
	}

	// Copy tags
	for _, key := range n.GetTagKeys() {
		cloned.SetTag(key, n.GetTag(key))
	}

	// Clone children
	for _, child := range n.GetChildren() {
		cloned.AddChild(child.CloneTree())
	}

	return cloned
}

// String returns a string representation of this node.
func (n *OrQueryNode) String() string {
	return fmt.Sprintf("<or children=%d>", len(n.GetChildren()))
}
