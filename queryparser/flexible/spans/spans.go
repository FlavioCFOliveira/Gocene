// Package spans is a self-contained example of how to extend the Gocene
// flexible query parser framework to produce span queries.
//
// It is a port of the Java package
// org.apache.lucene.queryparser.flexible.spans, which lives under the
// queryparser test sources and serves as both a working demo and a set of
// regression tests for the extension points of the flexible parser.
//
// Execution of the tests in this package is deferred until:
//   - SpanOrQuery / SpanTermQuery in queries/spans have functional implementations
//   - The flexible parser pipeline (processors, builders) supports custom
//     query-node type registration
//
// Port of: queryparser/src/test/.../flexible/spans/
package spans

import (
	"fmt"

	"github.com/FlavioCFOliveira/Gocene/index"
	"github.com/FlavioCFOliveira/Gocene/queryparser/flexible"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// UniqueFieldAttribute is a query-node attribute that carries the unique field
// name required by the spans query parser.
//
// Port of: UniqueFieldAttribute.java
type UniqueFieldAttribute interface {
	flexible.QueryNode
	// SetUniqueField sets the unique field name.
	SetUniqueField(field string)
	// GetUniqueField returns the unique field name.
	GetUniqueField() string
}

// UniqueFieldAttributeImpl is the concrete implementation of
// UniqueFieldAttribute.
//
// Port of: UniqueFieldAttributeImpl.java
type UniqueFieldAttributeImpl struct {
	flexible.FieldQueryNode
	uniqueField string
}

// SetUniqueField stores the unique field name.
func (u *UniqueFieldAttributeImpl) SetUniqueField(field string) { u.uniqueField = field }

// GetUniqueField returns the stored unique field name.
func (u *UniqueFieldAttributeImpl) GetUniqueField() string { return u.uniqueField }

// SpansQueryConfigHandler is the query configuration handler for the spans
// query parser example.
//
// Port of: SpansQueryConfigHandler.java
type SpansQueryConfigHandler struct{}

// NewSpansQueryConfigHandler creates an empty configuration handler.
func NewSpansQueryConfigHandler() *SpansQueryConfigHandler {
	return &SpansQueryConfigHandler{}
}

// SpansValidatorQueryNodeProcessor validates that the query node tree only
// contains single-term nodes acceptable by the span query model.
//
// Port of: SpansValidatorQueryNodeProcessor.java
type SpansValidatorQueryNodeProcessor struct{}

// Process validates the tree.  Returns an error for unsupported constructs.
func (p *SpansValidatorQueryNodeProcessor) Process(node flexible.QueryNode) (flexible.QueryNode, error) {
	// Stub: full validation deferred until SpanQuery infrastructure is complete.
	return node, nil
}

// UniqueFieldQueryNodeProcessor ensures that every query node carries a
// unique field name.
//
// Port of: UniqueFieldQueryNodeProcessor.java
type UniqueFieldQueryNodeProcessor struct{}

// Process assigns a unique field to each node.
func (p *UniqueFieldQueryNodeProcessor) Process(node flexible.QueryNode) (flexible.QueryNode, error) {
	// Stub: deferred.
	return node, nil
}

// SpanTermQueryNodeBuilder builds a SpanTermQuery from a FieldQueryNode.
//
// Port of: SpanTermQueryNodeBuilder.java
type SpanTermQueryNodeBuilder struct{}

// Build builds the SpanTermQuery.
func (b *SpanTermQueryNodeBuilder) Build(node flexible.QueryNode) (search.Query, error) {
	fqn, ok := node.(*flexible.FieldQueryNode)
	if !ok {
		return nil, fmt.Errorf("SpanTermQueryNodeBuilder expects FieldQueryNode, got %T", node)
	}
	term := index.NewTerm(fqn.GetField(), fqn.GetText())
	return search.NewSpanTermQuery(term), nil
}

// SpanOrQueryNodeBuilder builds a SpanOrQuery from an OrQueryNode.
//
// Port of: SpanOrQueryNodeBuilder.java
type SpanOrQueryNodeBuilder struct{}

// Build builds the SpanOrQuery.
func (b *SpanOrQueryNodeBuilder) Build(node flexible.QueryNode) (search.Query, error) {
	orNode, ok := node.(*flexible.OrQueryNode)
	if !ok {
		return nil, fmt.Errorf("SpanOrQueryNodeBuilder expects OrQueryNode, got %T", node)
	}
	children := orNode.GetChildren()
	if len(children) == 0 {
		return nil, fmt.Errorf("OrQueryNode has no children")
	}

	builder := NewSpansQueryTreeBuilder()
	clauses := make([]search.SpanQuery, 0, len(children))
	for _, child := range children {
		q, err := builder.Build(child)
		if err != nil {
			return nil, err
		}
		sq, ok := q.(search.SpanQuery)
		if !ok {
			return nil, fmt.Errorf("child query %T is not a SpanQuery", q)
		}
		clauses = append(clauses, sq)
	}
	return search.NewSpanOrQuery(clauses...), nil
}

// SpansQueryTreeBuilder assembles the builder registry for the span query pipeline.
//
// Port of: SpansQueryTreeBuilder.java
type SpansQueryTreeBuilder struct{}

// NewSpansQueryTreeBuilder creates the builder.
func NewSpansQueryTreeBuilder() *SpansQueryTreeBuilder {
	return &SpansQueryTreeBuilder{}
}

// Build dispatches to the registered node builders.
func (b *SpansQueryTreeBuilder) Build(node flexible.QueryNode) (search.Query, error) {
	if node == nil {
		return nil, fmt.Errorf("nil query node")
	}
	switch n := node.(type) {
	case *flexible.FieldQueryNode:
		return (&SpanTermQueryNodeBuilder{}).Build(n)
	case *flexible.OrQueryNode:
		return (&SpanOrQueryNodeBuilder{}).Build(n)
	default:
		return nil, fmt.Errorf("unsupported query node type %T", node)
	}
}
