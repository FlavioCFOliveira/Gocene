package uhighlight

// CharArrayMatcher is used to match a sequence of characters.
type CharArrayMatcher interface {
	Match(text string) bool
	Label() string
}

// LabelledCharArrayMatcher is a matcher that also has a label.
type LabelledCharArrayMatcher struct {
	label string
	match func(string) bool
}

func NewLabelledCharArrayMatcher(label string, match func(string) bool) *LabelledCharArrayMatcher {
	return &LabelledCharArrayMatcher{
		label: label,
		match: match,
	}
}

func (m *LabelledCharArrayMatcher) Match(text string) bool {
	return m.match(text)
}

func (m *LabelledCharArrayMatcher) Label() string {
	return m.label
}
