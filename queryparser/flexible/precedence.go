// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible

import (
	"github.com/FlavioCFOliveira/Gocene/analysis"
	"github.com/FlavioCFOliveira/Gocene/search"
)

// BooleanModifiersQueryNodeProcessor converts ModifierQueryNode children into
// BooleanModifierNode so that downstream processors can handle them distinctly.
// This is the Go equivalent of Lucene's BooleanModifiersQueryNodeProcessor.
type BooleanModifiersQueryNodeProcessor struct {
	*QueryNodeProcessorImpl
}

// NewBooleanModifiersQueryNodeProcessor creates a new BooleanModifiersQueryNodeProcessor.
func NewBooleanModifiersQueryNodeProcessor() *BooleanModifiersQueryNodeProcessor {
	return &BooleanModifiersQueryNodeProcessor{QueryNodeProcessorImpl: NewQueryNodeProcessorImpl()}
}

// Process wraps any ModifierQueryNode children in a BooleanModifierNode.
func (p *BooleanModifiersQueryNodeProcessor) Process(queryTree QueryNode) (QueryNode, error) {
	if queryTree == nil {
		return nil, nil
	}

	// Replace ModifierQueryNode children with BooleanModifierNode
	children := queryTree.GetChildren()
	for i, child := range children {
		if modNode, ok := child.(*ModifierQueryNode); ok {
			boolMod := NewBooleanModifierNode(nil, modNode.GetModifier())
			for _, grandchild := range modNode.GetChildren() {
				boolMod.AddChild(grandchild)
			}
			for _, k := range modNode.GetTagKeys() {
				boolMod.SetTag(k, modNode.GetTag(k))
			}
			children[i] = boolMod
		}
	}
	queryTree.SetChildren(children)

	// Process children recursively
	for _, child := range queryTree.GetChildren() {
		_, err := p.Process(child)
		if err != nil {
			return nil, err
		}
	}

	return queryTree, nil
}

// PrecedenceQueryParser is a StandardQueryParser variant that applies AND-before-OR
// precedence rules, matching Lucene's PrecedenceQueryParser.
type PrecedenceQueryParser struct {
	config     *StandardQueryConfigHandler
	syntax     *StandardSyntaxParser
	processors *QueryNodeProcessorPipeline
	builder    *StandardQueryTreeBuilder
}

// NewPrecedenceQueryParser creates a new PrecedenceQueryParser.
func NewPrecedenceQueryParser() *PrecedenceQueryParser {
	config := NewStandardQueryConfigHandler()

	processors := NewQueryNodeProcessorPipeline([]QueryNodeProcessor{
		NewBooleanModifiersQueryNodeProcessor(),
		NewPrecedenceQueryNodeProcessor(),
		NewPhraseSlopQueryNodeProcessor(),
		NewBoostQueryNodeProcessor(),
		NewNoChildOptimizationProcessor(),
	})

	return &PrecedenceQueryParser{
		config:     config,
		syntax:     NewStandardSyntaxParser(config),
		processors: processors,
		builder:    NewStandardQueryTreeBuilder(),
	}
}

// Parse parses a query string with AND-before-OR precedence.
func (p *PrecedenceQueryParser) Parse(query string) (search.Query, error) {
	queryTree, err := p.syntax.Parse(query)
	if err != nil {
		return nil, err
	}

	processedTree, err := p.processors.Process(queryTree)
	if err != nil {
		return nil, err
	}

	if processedTree == nil {
		return search.NewMatchNoDocsQuery(), nil
	}

	return p.builder.Build(processedTree)
}

// SetAnalyzer sets the analyzer on the underlying config.
func (p *PrecedenceQueryParser) SetAnalyzer(analyzer analysis.Analyzer) {
	p.config.SetAnalyzer(analyzer)
}

// SetDefaultField sets the default field on the underlying config.
func (p *PrecedenceQueryParser) SetDefaultField(field string) {
	p.config.SetDefaultField(field)
}

// GetConfig returns the underlying configuration handler.
func (p *PrecedenceQueryParser) GetConfig() *StandardQueryConfigHandler { return p.config }

// PrecedenceQueryNodeProcessorPipeline extends the standard pipeline by
// replacing BooleanQuery2ModifierNodeProcessor with
// BooleanModifiersQueryNodeProcessor, enabling AND-before-OR precedence
// semantics.
//
// Mirrors
// org.apache.lucene.queryparser.flexible.precedence.processors.PrecedenceQueryNodeProcessorPipeline.
type PrecedenceQueryNodeProcessorPipeline struct {
	*QueryNodeProcessorPipeline
}

// NewPrecedenceQueryNodeProcessorPipeline constructs the pipeline for the
// given configuration. It builds a pipeline equivalent to
// StandardQueryNodeProcessorPipelineFull but substitutes
// BooleanModifiersQueryNodeProcessor for the standard
// BooleanQuery2ModifierNodeProcessor, implementing precedence rules.
func NewPrecedenceQueryNodeProcessorPipeline(config *StandardQueryConfigHandler) *PrecedenceQueryNodeProcessorPipeline {
	processors := []QueryNodeProcessor{
		NewBooleanModifiersQueryNodeProcessor(),
		NewPrecedenceQueryNodeProcessor(),
		NewPhraseSlopQueryNodeProcessor(),
		NewBoostQueryNodeProcessor(),
		NewNoChildOptimizationProcessor(),
	}
	return &PrecedenceQueryNodeProcessorPipeline{
		QueryNodeProcessorPipeline: NewQueryNodeProcessorPipeline(processors),
	}
}
