// Package expressions implements org.apache.lucene.expressions: arbitrary
// JavaScript-flavoured value computations evaluated against per-document
// variables.
package expressions

// Expression represents a parsed JavaScript expression that can be evaluated
// against a set of Bindings. Mirrors org.apache.lucene.expressions.Expression.
type Expression struct {
	SourceText string
	Variables  []string
	Evaluate   func(values map[string]float64) (float64, error)
}

// NewExpression builds an Expression with the supplied evaluator.
func NewExpression(source string, variables []string, evaluator func(values map[string]float64) (float64, error)) *Expression {
	clone := make([]string, len(variables))
	copy(clone, variables)
	return &Expression{SourceText: source, Variables: clone, Evaluate: evaluator}
}

// GetSourceText returns the source representation.
func (e *Expression) GetSourceText() string { return e.SourceText }

// GetVariables returns the list of free variables the expression depends on.
func (e *Expression) GetVariables() []string {
	out := make([]string, len(e.Variables))
	copy(out, e.Variables)
	return out
}
