package search

import (
	"fmt"
	"strings"
)

// ComplexExplanation provides detailed explanation of scoring calculations.
// This is the Go port of Lucene's org.apache.lucene.search.ComplexExplanation.
type ComplexExplanation struct {
	// Value is the score value
	Value float32

	// Description explains what this score represents
	Description string

	// Details contains sub-explanations
	Details []*ComplexExplanation

	// Match indicates if this is a match
	Match bool
}

// NewComplexExplanation creates a new ComplexExplanation.
func NewComplexExplanation() *ComplexExplanation {
	return &ComplexExplanation{
		Details: make([]*ComplexExplanation, 0),
		Match:   true,
	}
}

// NewComplexExplanationWithValue creates a new ComplexExplanation with a value.
func NewComplexExplanationWithValue(value float32, description string) *ComplexExplanation {
	return &ComplexExplanation{
		Value:       value,
		Description: description,
		Details:     make([]*ComplexExplanation, 0),
		Match:       true,
	}
}

// SetValue sets the score value.
func (e *ComplexExplanation) SetValue(value float32) {
	e.Value = value
}

// GetValue returns the score value.
func (e *ComplexExplanation) GetValue() float32 {
	return e.Value
}

// SetDescription sets the description.
func (e *ComplexExplanation) SetDescription(description string) {
	e.Description = description
}

// GetDescription returns the description.
func (e *ComplexExplanation) GetDescription() string {
	return e.Description
}

// AddDetail adds a sub-explanation.
func (e *ComplexExplanation) AddDetail(detail *ComplexExplanation) {
	e.Details = append(e.Details, detail)
}

// GetDetails returns all sub-explanations.
func (e *ComplexExplanation) GetDetails() []*ComplexExplanation {
	result := make([]*ComplexExplanation, len(e.Details))
	copy(result, e.Details)
	return result
}

// GetDetailCount returns the number of sub-explanations.
func (e *ComplexExplanation) GetDetailCount() int {
	return len(e.Details)
}

// SetMatch sets whether this is a match.
func (e *ComplexExplanation) SetMatch(match bool) {
	e.Match = match
}

// IsMatch returns whether this is a match.
func (e *ComplexExplanation) IsMatch() bool {
	return e.Match
}

// IsValid returns true if this explanation has a valid value.
func (e *ComplexExplanation) IsValid() bool {
	return !isNaN(float64(e.Value))
}

// String returns a string representation of this explanation.
func (e *ComplexExplanation) String() string {
	var sb strings.Builder
	e.toString(&sb, 0)
	return sb.String()
}

// toString recursively builds the string representation.
func (e *ComplexExplanation) toString(sb *strings.Builder, indent int) {
	// Add indentation
	for i := 0; i < indent; i++ {
		sb.WriteString("  ")
	}

	// Add value and description
	sb.WriteString(fmt.Sprintf("%.5f", e.Value))
	sb.WriteString(" = ")
	sb.WriteString(e.Description)

	// Add match indicator
	if !e.Match {
		sb.WriteString(" [NON-MATCH]")
	}

	sb.WriteString("\n")

	// Add details
	for _, detail := range e.Details {
		detail.toString(sb, indent+1)
	}
}

// Summary returns a one-line summary of this explanation.
func (e *ComplexExplanation) Summary() string {
	return fmt.Sprintf("%.5f = %s", e.Value, e.Description)
}

// FindExplanation finds a sub-explanation by description.
func (e *ComplexExplanation) FindExplanation(description string) *ComplexExplanation {
	if strings.Contains(e.Description, description) {
		return e
	}

	for _, detail := range e.Details {
		if found := detail.FindExplanation(description); found != nil {
			return found
		}
	}

	return nil
}

// GetTotalValue returns the sum of all detail values.
func (e *ComplexExplanation) GetTotalValue() float32 {
	total := e.Value
	for _, detail := range e.Details {
		total += detail.GetValue()
	}
	return total
}

// ExplanationBuilder helps build complex explanations.
type ExplanationBuilder struct {
	explanation *ComplexExplanation
}

// NewExplanationBuilder creates a new ExplanationBuilder.
func NewExplanationBuilder() *ExplanationBuilder {
	return &ExplanationBuilder{
		explanation: NewComplexExplanation(),
	}
}

// SetValue sets the value.
func (b *ExplanationBuilder) SetValue(value float32) *ExplanationBuilder {
	b.explanation.SetValue(value)
	return b
}

// SetDescription sets the description.
func (b *ExplanationBuilder) SetDescription(description string) *ExplanationBuilder {
	b.explanation.SetDescription(description)
	return b
}

// SetMatch sets whether this is a match.
func (b *ExplanationBuilder) SetMatch(match bool) *ExplanationBuilder {
	b.explanation.SetMatch(match)
	return b
}

// AddDetail adds a detail explanation.
func (b *ExplanationBuilder) AddDetail(detail *ComplexExplanation) *ExplanationBuilder {
	b.explanation.AddDetail(detail)
	return b
}

// AddDetailWithValue adds a detail explanation with value and description.
func (b *ExplanationBuilder) AddDetailWithValue(value float32, description string) *ExplanationBuilder {
	b.explanation.AddDetail(NewComplexExplanationWithValue(value, description))
	return b
}

// Build builds and returns the ComplexExplanation.
func (b *ExplanationBuilder) Build() *ComplexExplanation {
	return b.explanation
}

// isNaN checks if a float64 is NaN.
func isNaN(f float64) bool {
	return f != f
}

// ExplanationFormatter formats explanations for display.
type ExplanationFormatter struct {
	// MaxDepth is the maximum depth to display
	MaxDepth int

	// IncludeNonMatches indicates whether to include non-matching explanations
	IncludeNonMatches bool
}

// NewExplanationFormatter creates a new ExplanationFormatter.
func NewExplanationFormatter() *ExplanationFormatter {
	return &ExplanationFormatter{
		MaxDepth:          -1, // Unlimited
		IncludeNonMatches: true,
	}
}

// Format formats an explanation for display.
func (f *ExplanationFormatter) Format(explanation *ComplexExplanation) string {
	return f.formatWithDepth(explanation, 0)
}

// formatWithDepth formats an explanation with a specific depth.
func (f *ExplanationFormatter) formatWithDepth(explanation *ComplexExplanation, depth int) string {
	if f.MaxDepth >= 0 && depth > f.MaxDepth {
		return ""
	}

	if !f.IncludeNonMatches && !explanation.IsMatch() {
		return ""
	}

	var sb strings.Builder

	// Add indentation
	for i := 0; i < depth; i++ {
		sb.WriteString("  ")
	}

	// Add value and description
	sb.WriteString(fmt.Sprintf("%.5f", explanation.GetValue()))
	sb.WriteString(" = ")
	sb.WriteString(explanation.GetDescription())

	if !explanation.IsMatch() {
		sb.WriteString(" [NON-MATCH]")
	}

	sb.WriteString("\n")

	// Add details
	for _, detail := range explanation.GetDetails() {
		formatted := f.formatWithDepth(detail, depth+1)
		if formatted != "" {
			sb.WriteString(formatted)
		}
	}

	return sb.String()
}
