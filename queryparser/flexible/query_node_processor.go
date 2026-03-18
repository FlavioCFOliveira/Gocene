// Package flexible provides the flexible query parser framework for Lucene-compatible query parsing.
package flexible

import (
	"fmt"
)

// QueryNodeProcessor is the interface for all query node processors.
// Processors transform the query tree by analyzing and modifying nodes.
type QueryNodeProcessor interface {
	// Process processes the query node tree and returns the modified tree.
	Process(queryTree QueryNode) (QueryNode, error)
}

// QueryNodeProcessorImpl is the base implementation of QueryNodeProcessor.
// It provides common functionality for query node processors.
type QueryNodeProcessorImpl struct {
	// Children processors that will be applied in sequence
	children []QueryNodeProcessor
}

// NewQueryNodeProcessorImpl creates a new QueryNodeProcessorImpl.
func NewQueryNodeProcessorImpl() *QueryNodeProcessorImpl {
	return &QueryNodeProcessorImpl{
		children: make([]QueryNodeProcessor, 0),
	}
}

// AddChildProcessor adds a child processor.
func (p *QueryNodeProcessorImpl) AddChildProcessor(processor QueryNodeProcessor) {
	p.children = append(p.children, processor)
}

// GetChildProcessors returns the child processors.
func (p *QueryNodeProcessorImpl) GetChildProcessors() []QueryNodeProcessor {
	return p.children
}

// Process processes the query node tree.
// This base implementation applies all child processors in sequence.
func (p *QueryNodeProcessorImpl) Process(queryTree QueryNode) (QueryNode, error) {
	var err error
	result := queryTree

	for _, child := range p.children {
		result, err = child.Process(result)
		if err != nil {
			return nil, fmt.Errorf("child processor failed: %w", err)
		}
	}

	return result, nil
}

// QueryNodeProcessorPipeline is a pipeline of query node processors.
// It processes the query tree through a sequence of processors.
type QueryNodeProcessorPipeline struct {
	*QueryNodeProcessorImpl
	processors []QueryNodeProcessor
}

// NewQueryNodeProcessorPipeline creates a new QueryNodeProcessorPipeline.
func NewQueryNodeProcessorPipeline(processors []QueryNodeProcessor) *QueryNodeProcessorPipeline {
	return &QueryNodeProcessorPipeline{
		QueryNodeProcessorImpl: NewQueryNodeProcessorImpl(),
		processors:             processors,
	}
}

// GetPipeline returns the pipeline of processors.
func (p *QueryNodeProcessorPipeline) GetPipeline() []QueryNodeProcessor {
	return p.processors
}

// SetPipeline sets the pipeline of processors.
func (p *QueryNodeProcessorPipeline) SetPipeline(processors []QueryNodeProcessor) {
	p.processors = processors
}

// AddProcessor adds a processor to the pipeline.
func (p *QueryNodeProcessorPipeline) AddProcessor(processor QueryNodeProcessor) {
	p.processors = append(p.processors, processor)
}

// Process processes the query node tree through the pipeline.
func (p *QueryNodeProcessorPipeline) Process(queryTree QueryNode) (QueryNode, error) {
	var err error
	result := queryTree

	for _, processor := range p.processors {
		result, err = processor.Process(result)
		if err != nil {
			return nil, fmt.Errorf("pipeline processor failed: %w", err)
		}
	}

	return result, nil
}

// NoChildOptimizationProcessor removes nodes that have no children.
// This is useful for cleaning up the query tree after processing.
type NoChildOptimizationProcessor struct {
	*QueryNodeProcessorImpl
}

// NewNoChildOptimizationProcessor creates a new NoChildOptimizationProcessor.
func NewNoChildOptimizationProcessor() *NoChildOptimizationProcessor {
	return &NoChildOptimizationProcessor{
		QueryNodeProcessorImpl: NewQueryNodeProcessorImpl(),
	}
}

// Process processes the query node tree and removes nodes with no children.
func (p *NoChildOptimizationProcessor) Process(queryTree QueryNode) (QueryNode, error) {
	if queryTree == nil {
		return nil, nil
	}

	// Process children first (post-order traversal)
	children := queryTree.GetChildren()
	newChildren := make([]QueryNode, 0, len(children))

	for _, child := range children {
		processedChild, err := p.Process(child)
		if err != nil {
			return nil, err
		}
		if processedChild != nil {
			newChildren = append(newChildren, processedChild)
		}
	}

	// Update children
	queryTree.SetChildren(newChildren)

	// Remove this node if it has no children and is not a leaf node type
	// Leaf nodes like FieldQueryNode are valid even without children
	if len(newChildren) == 0 && !isLeafNodeType(queryTree) {
		return nil, nil
	}

	return queryTree, nil
}

// isLeafNodeType returns true if the node is a leaf node type.
func isLeafNodeType(node QueryNode) bool {
	switch node.(type) {
	case *FieldQueryNode, *FuzzyQueryNode, *PhraseSlopQueryNode,
		*MatchAllDocsQueryNode, *MatchNoDocsQueryNode:
		return true
	default:
		return false
	}
}

// RemoveDeletedQueryNodesProcessor removes nodes marked as deleted.
// Nodes can be marked as deleted by setting a special tag.
type RemoveDeletedQueryNodesProcessor struct {
	*QueryNodeProcessorImpl
}

// TagDeleted is the tag key used to mark nodes as deleted.
const TagDeleted = "deleted"

// NewRemoveDeletedQueryNodesProcessor creates a new RemoveDeletedQueryNodesProcessor.
func NewRemoveDeletedQueryNodesProcessor() *RemoveDeletedQueryNodesProcessor {
	return &RemoveDeletedQueryNodesProcessor{
		QueryNodeProcessorImpl: NewQueryNodeProcessorImpl(),
	}
}

// Process processes the query node tree and removes deleted nodes.
func (p *RemoveDeletedQueryNodesProcessor) Process(queryTree QueryNode) (QueryNode, error) {
	if queryTree == nil {
		return nil, nil
	}

	// Check if this node is marked as deleted
	if queryTree.HasTag(TagDeleted) {
		if deleted := queryTree.GetTag(TagDeleted); deleted != nil {
			if isDeleted, ok := deleted.(bool); ok && isDeleted {
				return nil, nil
			}
		}
	}

	// Process children
	children := queryTree.GetChildren()
	newChildren := make([]QueryNode, 0, len(children))

	for _, child := range children {
		processedChild, err := p.Process(child)
		if err != nil {
			return nil, err
		}
		if processedChild != nil {
			newChildren = append(newChildren, processedChild)
		}
	}

	queryTree.SetChildren(newChildren)

	return queryTree, nil
}

// PrecedenceQueryNodeProcessor processes nodes to ensure proper precedence.
// It groups nodes according to operator precedence rules.
type PrecedenceQueryNodeProcessor struct {
	*QueryNodeProcessorImpl
}

// NewPrecedenceQueryNodeProcessor creates a new PrecedenceQueryNodeProcessor.
func NewPrecedenceQueryNodeProcessor() *PrecedenceQueryNodeProcessor {
	return &PrecedenceQueryNodeProcessor{
		QueryNodeProcessorImpl: NewQueryNodeProcessorImpl(),
	}
}

// Process processes the query node tree to ensure proper precedence.
func (p *PrecedenceQueryNodeProcessor) Process(queryTree QueryNode) (QueryNode, error) {
	if queryTree == nil {
		return nil, nil
	}

	// Process children first
	children := queryTree.GetChildren()
	for _, child := range children {
		processedChild, err := p.Process(child)
		if err != nil {
			return nil, err
		}
		if processedChild != child {
			queryTree.ReplaceChild(child, processedChild)
		}
	}

	return queryTree, nil
}

// AnalyzerQueryNodeProcessor analyzes text fields using an analyzer.
// It expands field queries into multiple tokens if the analyzer produces multiple terms.
type AnalyzerQueryNodeProcessor struct {
	*QueryNodeProcessorImpl
}

// NewAnalyzerQueryNodeProcessor creates a new AnalyzerQueryNodeProcessor.
func NewAnalyzerQueryNodeProcessor() *AnalyzerQueryNodeProcessor {
	return &AnalyzerQueryNodeProcessor{
		QueryNodeProcessorImpl: NewQueryNodeProcessorImpl(),
	}
}

// Process processes the query node tree and analyzes text fields.
func (p *AnalyzerQueryNodeProcessor) Process(queryTree QueryNode) (QueryNode, error) {
	if queryTree == nil {
		return nil, nil
	}

	// Process children first
	children := queryTree.GetChildren()
	for _, child := range children {
		processedChild, err := p.Process(child)
		if err != nil {
			return nil, err
		}
		if processedChild != child {
			queryTree.ReplaceChild(child, processedChild)
		}
	}

	return queryTree, nil
}

// PhraseSlopQueryNodeProcessor processes phrase slop queries.
// It validates and normalizes slop values.
type PhraseSlopQueryNodeProcessor struct {
	*QueryNodeProcessorImpl
}

// NewPhraseSlopQueryNodeProcessor creates a new PhraseSlopQueryNodeProcessor.
func NewPhraseSlopQueryNodeProcessor() *PhraseSlopQueryNodeProcessor {
	return &PhraseSlopQueryNodeProcessor{
		QueryNodeProcessorImpl: NewQueryNodeProcessorImpl(),
	}
}

// Process processes the query node tree and validates phrase slop values.
func (p *PhraseSlopQueryNodeProcessor) Process(queryTree QueryNode) (QueryNode, error) {
	if queryTree == nil {
		return nil, nil
	}

	// Process phrase slop nodes
	if phraseNode, ok := queryTree.(*PhraseSlopQueryNode); ok {
		// Ensure slop is non-negative
		if phraseNode.GetSlop() < 0 {
			phraseNode.SetSlop(0)
		}
	}

	// Process children
	children := queryTree.GetChildren()
	for _, child := range children {
		processedChild, err := p.Process(child)
		if err != nil {
			return nil, err
		}
		if processedChild != child {
			queryTree.ReplaceChild(child, processedChild)
		}
	}

	return queryTree, nil
}

// BoostQueryNodeProcessor processes boost queries.
// It validates and normalizes boost values.
type BoostQueryNodeProcessor struct {
	*QueryNodeProcessorImpl
}

// NewBoostQueryNodeProcessor creates a new BoostQueryNodeProcessor.
func NewBoostQueryNodeProcessor() *BoostQueryNodeProcessor {
	return &BoostQueryNodeProcessor{
		QueryNodeProcessorImpl: NewQueryNodeProcessorImpl(),
	}
}

// Process processes the query node tree and validates boost values.
func (p *BoostQueryNodeProcessor) Process(queryTree QueryNode) (QueryNode, error) {
	if queryTree == nil {
		return nil, nil
	}

	// Process boost nodes
	if boostNode, ok := queryTree.(*BoostQueryNode); ok {
		// Ensure boost is positive
		if boostNode.GetValue() <= 0 {
			boostNode.SetValue(1.0)
		}
	}

	// Process children
	children := queryTree.GetChildren()
	for _, child := range children {
		processedChild, err := p.Process(child)
		if err != nil {
			return nil, err
		}
		if processedChild != child {
			queryTree.ReplaceChild(child, processedChild)
		}
	}

	return queryTree, nil
}
