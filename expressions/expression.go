// Package expressions implements org.apache.lucene.expressions: arbitrary
// JavaScript-flavoured value computations evaluated against per-document
// variables.
package expressions

// Expression represents a parsed JavaScript expression that can be evaluated
// against a set of Bindings. Mirrors org.apache.lucene.expressions.Expression.
//
// The Java original is an abstract class; in Go the evaluation function is
// supplied at construction time via the evaluator func fields.
type Expression struct {
	// SourceText is the original source text (e.g. "sqrt(_score) + ln(popularity)").
	SourceText string

	// Variables holds the names of external variables referenced by the expression.
	Variables []string

	// Evaluate is the legacy evaluator operating on a map of named values.
	// Kept for backward compatibility with existing callers in the Gocene tree.
	Evaluate func(values map[string]float64) (float64, error)

	// evaluateDoubleValues is the Lucene-compatible evaluator that receives a
	// []DoubleValues — one per variable in the same order as Variables.
	evaluateDoubleValues func(functionValues []DoubleValues) (float64, error)
}

// NewExpression builds an Expression with a legacy map-based evaluator.
// This is kept for backward compatibility; prefer NewExpressionWithDoubleValues
// when writing new code that uses the full Lucene API.
func NewExpression(source string, variables []string, evaluator func(values map[string]float64) (float64, error)) *Expression {
	clone := make([]string, len(variables))
	copy(clone, variables)
	return &Expression{SourceText: source, Variables: clone, Evaluate: evaluator}
}

// NewExpressionWithDoubleValues builds an Expression with a Lucene-compatible
// evaluator that receives per-variable DoubleValues instances. This is the
// preferred constructor for new code; it enables ExpressionFunctionValues,
// ExpressionValueSource, and ExpressionRescorer.
func NewExpressionWithDoubleValues(
	source string,
	variables []string,
	evaluator func(functionValues []DoubleValues) (float64, error),
) *Expression {
	clone := make([]string, len(variables))
	copy(clone, variables)
	return &Expression{
		SourceText:           source,
		Variables:            clone,
		evaluateDoubleValues: evaluator,
	}
}

// EvaluateDoubleValues evaluates the expression against the provided per-variable
// DoubleValues instances. It is the Go counterpart of Expression.evaluate(DoubleValues[]).
func (e *Expression) EvaluateDoubleValues(functionValues []DoubleValues) (float64, error) {
	if e.evaluateDoubleValues != nil {
		return e.evaluateDoubleValues(functionValues)
	}
	// Fallback: use the legacy map-based evaluator, reading each variable value.
	if e.Evaluate == nil {
		return 0, nil
	}
	m := make(map[string]float64, len(e.Variables))
	for i, name := range e.Variables {
		if i < len(functionValues) && functionValues[i] != nil {
			v, err := functionValues[i].DoubleValue()
			if err != nil {
				return 0, err
			}
			m[name] = v
		}
	}
	return e.Evaluate(m)
}

// GetSourceText returns the source representation.
func (e *Expression) GetSourceText() string { return e.SourceText }

// GetVariables returns the list of free variables the expression depends on.
func (e *Expression) GetVariables() []string {
	out := make([]string, len(e.Variables))
	copy(out, e.Variables)
	return out
}
