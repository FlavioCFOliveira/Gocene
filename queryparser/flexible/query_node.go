// Package flexible provides the flexible query parser framework for Lucene-compatible query parsing.
// This package implements the QueryNode tree structure and processing pipeline.
package flexible

import (
	"fmt"
	"strings"
)

// QueryNode is the interface for all nodes in the query tree.
// It defines the basic operations that can be performed on any query node.
type QueryNode interface {
	// GetTag returns the value associated with the given tag key.
	GetTag(key string) interface{}

	// SetTag sets a tag value for the given key.
	SetTag(key string, value interface{})

	// HasTag returns true if the tag exists.
	HasTag(key string) bool

	// String returns a string representation of this node.
	String() string

	// ToQueryString returns the query string representation of this node.
	ToQueryString(escapeSpecialSyntax bool) string

	// CloneTree creates a deep copy of this node and its children.
	CloneTree() QueryNode

	// GetParent returns the parent node, or nil if this is the root.
	GetParent() QueryNode

	// SetParent sets the parent node.
	SetParent(parent QueryNode)

	// IsLeaf returns true if this node has no children.
	IsLeaf() bool

	// GetChildren returns the child nodes.
	GetChildren() []QueryNode

	// SetChildren replaces all children with the given slice.
	SetChildren(children []QueryNode)

	// AddChild adds a child node.
	AddChild(child QueryNode)

	// RemoveChild removes a child node.
	RemoveChild(child QueryNode) bool

	// ReplaceChild replaces an existing child with a new one.
	ReplaceChild(existingChild, newChild QueryNode) bool
}

// QueryNodeImpl is the base implementation of QueryNode.
// It provides common functionality for all query node types.
type QueryNodeImpl struct {
	parent   QueryNode
	children []QueryNode
	tags     map[string]interface{}
}

// NewQueryNodeImpl creates a new QueryNodeImpl with the given children.
func NewQueryNodeImpl(children []QueryNode) *QueryNodeImpl {
	node := &QueryNodeImpl{
		children: make([]QueryNode, 0, len(children)),
		tags:     make(map[string]interface{}),
	}
	for _, child := range children {
		node.AddChild(child)
	}
	return node
}

// GetTag returns the value associated with the given tag key.
func (n *QueryNodeImpl) GetTag(key string) interface{} {
	if n.tags == nil {
		return nil
	}
	return n.tags[key]
}

// SetTag sets a tag value for the given key.
func (n *QueryNodeImpl) SetTag(key string, value interface{}) {
	if n.tags == nil {
		n.tags = make(map[string]interface{})
	}
	n.tags[key] = value
}

// String returns a string representation of this node.
func (n *QueryNodeImpl) String() string {
	return n.ToQueryString(false)
}

// ToQueryString returns the query string representation of this node.
// This base implementation returns a generic representation.
// Subclasses should override this method.
func (n *QueryNodeImpl) ToQueryString(escapeSpecialSyntax bool) string {
	if n.IsLeaf() {
		return ""
	}

	var sb strings.Builder
	children := n.GetChildren()
	for i, child := range children {
		if i > 0 {
			sb.WriteString(" ")
		}
		sb.WriteString(child.ToQueryString(escapeSpecialSyntax))
	}
	return sb.String()
}

// CloneTree creates a deep copy of this node and its children.
// Subclasses must override this to properly clone their specific fields.
func (n *QueryNodeImpl) CloneTree() QueryNode {
	cloned := &QueryNodeImpl{
		tags: make(map[string]interface{}, len(n.tags)),
	}

	// Copy tags
	for k, v := range n.tags {
		cloned.tags[k] = v
	}

	// Clone children
	cloned.children = make([]QueryNode, 0, len(n.children))
	for _, child := range n.children {
		clonedChild := child.CloneTree()
		cloned.AddChild(clonedChild)
	}

	return cloned
}

// GetParent returns the parent node, or nil if this is the root.
func (n *QueryNodeImpl) GetParent() QueryNode {
	return n.parent
}

// SetParent sets the parent node.
func (n *QueryNodeImpl) SetParent(parent QueryNode) {
	n.parent = parent
}

// IsLeaf returns true if this node has no children.
func (n *QueryNodeImpl) IsLeaf() bool {
	return len(n.children) == 0
}

// GetChildren returns the child nodes.
func (n *QueryNodeImpl) GetChildren() []QueryNode {
	return n.children
}

// SetChildren replaces all children with the given slice.
func (n *QueryNodeImpl) SetChildren(children []QueryNode) {
	// Clear existing parent references
	for _, child := range n.children {
		if impl, ok := child.(*QueryNodeImpl); ok {
			impl.SetParent(nil)
		}
	}

	n.children = make([]QueryNode, 0, len(children))
	for _, child := range children {
		n.AddChild(child)
	}
}

// AddChild adds a child node.
func (n *QueryNodeImpl) AddChild(child QueryNode) {
	if child == nil {
		return
	}

	// Remove from old parent if exists
	if oldParent := child.GetParent(); oldParent != nil {
		oldParent.RemoveChild(child)
	}

	n.children = append(n.children, child)
	if impl, ok := child.(*QueryNodeImpl); ok {
		impl.SetParent(n)
	}
}

// RemoveChild removes a child node.
// Returns true if the child was found and removed.
func (n *QueryNodeImpl) RemoveChild(child QueryNode) bool {
	for i, c := range n.children {
		if c == child {
			// Remove from slice
			n.children = append(n.children[:i], n.children[i+1:]...)
			// Clear parent reference
			if impl, ok := c.(*QueryNodeImpl); ok {
				impl.SetParent(nil)
			}
			return true
		}
	}
	return false
}

// ReplaceChild replaces an existing child with a new one.
// Returns true if the existing child was found and replaced.
func (n *QueryNodeImpl) ReplaceChild(existingChild, newChild QueryNode) bool {
	for i, c := range n.children {
		if c == existingChild {
			// Clear parent of existing child
			if impl, ok := c.(*QueryNodeImpl); ok {
				impl.SetParent(nil)
			}

			// Remove new child from its old parent if exists
			if oldParent := newChild.GetParent(); oldParent != nil {
				oldParent.RemoveChild(newChild)
			}

			// Replace in slice
			n.children[i] = newChild
			if impl, ok := newChild.(*QueryNodeImpl); ok {
				impl.SetParent(n)
			}
			return true
		}
	}
	return false
}

// GetTagString returns the string value of a tag, or empty string if not found or not a string.
func (n *QueryNodeImpl) GetTagString(key string) string {
	val := n.GetTag(key)
	if val == nil {
		return ""
	}
	if s, ok := val.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", val)
}

// HasTag returns true if the tag exists.
func (n *QueryNodeImpl) HasTag(key string) bool {
	if n.tags == nil {
		return false
	}
	_, exists := n.tags[key]
	return exists
}

// RemoveTag removes a tag.
func (n *QueryNodeImpl) RemoveTag(key string) {
	if n.tags != nil {
		delete(n.tags, key)
	}
}

// ClearTags removes all tags.
func (n *QueryNodeImpl) ClearTags() {
	n.tags = make(map[string]interface{})
}

// GetTagKeys returns all tag keys.
func (n *QueryNodeImpl) GetTagKeys() []string {
	if n.tags == nil || len(n.tags) == 0 {
		return nil
	}
	keys := make([]string, 0, len(n.tags))
	for k := range n.tags {
		keys = append(keys, k)
	}
	return keys
}
