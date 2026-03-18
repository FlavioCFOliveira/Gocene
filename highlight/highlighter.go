package highlight

import (
	"fmt"
	"strings"

	"github.com/FlavioCFOliveira/Gocene/search"
)

// Highlighter is the base interface for all highlighters.
// It provides methods to highlight query terms in text.
//
// This is the Go port of Lucene's org.apache.lucene.search.highlight.Highlighter.
type Highlighter interface {
	// GetBestFragment returns the best fragment of text with query terms highlighted.
	// Parameters:
	//   - text: the text to highlight
	//   - maxNumFragments: the maximum number of fragments to return
	// Returns:
	//   - the highlighted text fragment, or error if highlighting fails
	GetBestFragment(text string, maxNumFragments int) (string, error)

	// GetBestFragments returns the best fragments of text with query terms highlighted.
	// Parameters:
	//   - text: the text to highlight
	//   - maxNumFragments: the maximum number of fragments to return
	// Returns:
	//   - the highlighted text fragments, or error if highlighting fails
	GetBestFragments(text string, maxNumFragments int) ([]string, error)

	// SetTextFragmenter sets the fragmenter for this highlighter.
	SetTextFragmenter(fragmenter Fragmenter)

	// SetFormatter sets the formatter for this highlighter.
	SetFormatter(formatter Formatter)
}

// SimpleHighlighter is a simple implementation of the Highlighter interface.
type SimpleHighlighter struct {
	// scorer scores fragments
	scorer FragmentScorer

	// fragmenter breaks text into fragments
	fragmenter Fragmenter

	// formatter formats highlighted text
	formatter Formatter

	// maxDocBytesToAnalyze limits how much of the document to analyze
	maxDocBytesToAnalyze int
}

// NewSimpleHighlighter creates a new SimpleHighlighter with the given scorer.
func NewSimpleHighlighter(scorer FragmentScorer) *SimpleHighlighter {
	return &SimpleHighlighter{
		scorer:               scorer,
		fragmenter:           NewSimpleFragmenter(100),
		formatter:            NewSimpleHTMLFormatter("<b>", "</b>"),
		maxDocBytesToAnalyze: 50 * 1024, // 50KB default
	}
}

// GetBestFragment returns the best fragment of text with query terms highlighted.
func (h *SimpleHighlighter) GetBestFragment(text string, maxNumFragments int) (string, error) {
	fragments, err := h.GetBestFragments(text, maxNumFragments)
	if err != nil {
		return "", err
	}

	if len(fragments) == 0 {
		return "", nil
	}

	return fragments[0], nil
}

// GetBestFragments returns the best fragments of text with query terms highlighted.
func (h *SimpleHighlighter) GetBestFragments(text string, maxNumFragments int) ([]string, error) {
	if text == "" {
		return []string{}, nil
	}

	// Limit text size
	if len(text) > h.maxDocBytesToAnalyze {
		text = text[:h.maxDocBytesToAnalyze]
	}

	// Get fragments
	fragments := h.fragmenter.GetFragments(text, maxNumFragments)

	// Score and sort fragments
	scoredFragments := make([]*ScoredFragment, len(fragments))
	for i, fragment := range fragments {
		score := h.scorer.GetFragmentScore(fragment)
		scoredFragments[i] = &ScoredFragment{
			Text:  fragment,
			Score: score,
		}
	}

	// Sort by score (highest first)
	sortScoredFragments(scoredFragments)

	// Format top fragments
	result := make([]string, 0, maxNumFragments)
	for i := 0; i < len(scoredFragments) && i < maxNumFragments; i++ {
		formatted := h.formatter.Highlight(scoredFragments[i].Text, h.scorer.GetQueryTerms())
		result = append(result, formatted)
	}

	return result, nil
}

// SetTextFragmenter sets the fragmenter for this highlighter.
func (h *SimpleHighlighter) SetTextFragmenter(fragmenter Fragmenter) {
	h.fragmenter = fragmenter
}

// SetFormatter sets the formatter for this highlighter.
func (h *SimpleHighlighter) SetFormatter(formatter Formatter) {
	h.formatter = formatter
}

// SetMaxDocBytesToAnalyze sets the maximum number of bytes to analyze.
func (h *SimpleHighlighter) SetMaxDocBytesToAnalyze(maxBytes int) {
	h.maxDocBytesToAnalyze = maxBytes
}

// ScoredFragment represents a fragment with its score.
type ScoredFragment struct {
	Text  string
	Score float32
}

// sortScoredFragments sorts fragments by score (highest first).
func sortScoredFragments(fragments []*ScoredFragment) {
	// Simple bubble sort for now
	for i := 0; i < len(fragments); i++ {
		for j := i + 1; j < len(fragments); j++ {
			if fragments[j].Score > fragments[i].Score {
				fragments[i], fragments[j] = fragments[j], fragments[i]
			}
		}
	}
}

// Fragmenter breaks text into fragments for highlighting.
type Fragmenter interface {
	// GetFragments returns fragments of the given text.
	GetFragments(text string, maxNumFragments int) []string
}

// NullFragmenter is a fragmenter that returns the entire text as a single fragment.
// This is useful when you want to highlight the entire text without breaking it into pieces.
//
// This is the Go port of Lucene's org.apache.lucene.search.highlight.NullFragmenter.
type NullFragmenter struct{}

// NewNullFragmenter creates a new NullFragmenter.
func NewNullFragmenter() *NullFragmenter {
	return &NullFragmenter{}
}

// GetFragments returns the entire text as a single fragment.
func (f *NullFragmenter) GetFragments(text string, maxNumFragments int) []string {
	if text == "" {
		return []string{}
	}
	return []string{text}
}

// Ensure NullFragmenter implements Fragmenter
var _ Fragmenter = (*NullFragmenter)(nil)

// SimpleFragmenter is a simple implementation of Fragmenter.
type SimpleFragmenter struct {
	// fragmentSize is the target size of each fragment
	fragmentSize int
}

// NewSimpleFragmenter creates a new SimpleFragmenter with the given fragment size.
func NewSimpleFragmenter(fragmentSize int) *SimpleFragmenter {
	return &SimpleFragmenter{
		fragmentSize: fragmentSize,
	}
}

// GetFragments returns fragments of the given text.
func (f *SimpleFragmenter) GetFragments(text string, maxNumFragments int) []string {
	if text == "" {
		return []string{}
	}

	// Split text into sentences (simple approach)
	sentences := strings.Split(text, ". ")

	fragments := make([]string, 0, maxNumFragments)
	currentFragment := ""

	for _, sentence := range sentences {
		if len(currentFragment)+len(sentence) > f.fragmentSize && currentFragment != "" {
			fragments = append(fragments, strings.TrimSpace(currentFragment))
			currentFragment = sentence + ". "
			if len(fragments) >= maxNumFragments {
				break
			}
		} else {
			currentFragment += sentence + ". "
		}
	}

	// Add remaining text
	if currentFragment != "" && len(fragments) < maxNumFragments {
		fragments = append(fragments, strings.TrimSpace(currentFragment))
	}

	return fragments
}

// Formatter formats highlighted text.
type Formatter interface {
	// Highlight highlights the given text with the specified terms.
	Highlight(text string, terms []string) string
}

// SimpleHTMLFormatter is a simple HTML formatter.
type SimpleHTMLFormatter struct {
	// preTag is the tag to use before highlighted terms
	preTag string

	// postTag is the tag to use after highlighted terms
	postTag string
}

// NewSimpleHTMLFormatter creates a new SimpleHTMLFormatter.
func NewSimpleHTMLFormatter(preTag, postTag string) *SimpleHTMLFormatter {
	return &SimpleHTMLFormatter{
		preTag:  preTag,
		postTag: postTag,
	}
}

// Highlight highlights the given text with the specified terms.
func (f *SimpleHTMLFormatter) Highlight(text string, terms []string) string {
	result := text
	for _, term := range terms {
		// Simple case-insensitive replacement
		lowerText := strings.ToLower(result)
		lowerTerm := strings.ToLower(term)

		// Find all occurrences
		var sb strings.Builder
		start := 0
		for {
			idx := strings.Index(lowerText[start:], lowerTerm)
			if idx == -1 {
				sb.WriteString(result[start:])
				break
			}
			idx += start
			sb.WriteString(result[start:idx])
			sb.WriteString(f.preTag)
			sb.WriteString(result[idx : idx+len(term)])
			sb.WriteString(f.postTag)
			start = idx + len(term)
		}
		result = sb.String()
	}
	return result
}

// FragmentScorer scores text fragments.
type FragmentScorer interface {
	// GetFragmentScore returns the score for the given fragment.
	GetFragmentScore(fragment string) float32

	// GetQueryTerms returns the query terms being highlighted.
	GetQueryTerms() []string
}

// SimpleFragmentScorer is a simple implementation of FragmentScorer.
type SimpleFragmentScorer struct {
	// queryTerms are the terms to highlight
	queryTerms []string
}

// NewSimpleFragmentScorer creates a new SimpleFragmentScorer.
func NewSimpleFragmentScorer(queryTerms []string) *SimpleFragmentScorer {
	return &SimpleFragmentScorer{
		queryTerms: queryTerms,
	}
}

// GetFragmentScore returns the score for the given fragment.
func (s *SimpleFragmentScorer) GetFragmentScore(fragment string) float32 {
	score := float32(0)
	lowerFragment := strings.ToLower(fragment)

	for _, term := range s.queryTerms {
		lowerTerm := strings.ToLower(term)
		count := strings.Count(lowerFragment, lowerTerm)
		score += float32(count)
	}

	return score
}

// GetQueryTerms returns the query terms being highlighted.
func (s *SimpleFragmentScorer) GetQueryTerms() []string {
	return s.queryTerms
}

// HighlighterFactory creates Highlighter instances.
type HighlighterFactory struct {
	// query is the query to highlight
	query search.Query

	// defaultField is the default field to highlight
	defaultField string
}

// NewHighlighterFactory creates a new HighlighterFactory.
func NewHighlighterFactory(query search.Query, defaultField string) *HighlighterFactory {
	return &HighlighterFactory{
		query:        query,
		defaultField: defaultField,
	}
}

// CreateHighlighter creates a Highlighter for the given query.
func (hf *HighlighterFactory) CreateHighlighter() (Highlighter, error) {
	// Extract query terms
	terms := hf.extractTerms(hf.query)

	// Create scorer
	scorer := NewSimpleFragmentScorer(terms)

	// Create highlighter
	highlighter := NewSimpleHighlighter(scorer)

	return highlighter, nil
}

// extractTerms extracts terms from a query.
func (hf *HighlighterFactory) extractTerms(query search.Query) []string {
	// In a full implementation, this would recursively extract terms
	// from the query tree
	// For now, return empty slice
	return []string{}
}

// String returns a string representation of this HighlighterFactory.
func (hf *HighlighterFactory) String() string {
	return fmt.Sprintf("HighlighterFactory(field=%s)", hf.defaultField)
}

// Encoder encodes text for output.
//
// This is the Go port of Lucene's org.apache.lucene.search.highlight.Encoder.
//
// Encoders transform text to make it safe for the output format,
// such as escaping HTML special characters.
type Encoder interface {
	// EncodeText encodes the given text for output.
	// Returns the encoded text.
	EncodeText(originalText string) string
}

// SimpleHTMLEncoder encodes text for HTML output.
//
// This is the Go port of Lucene's org.apache.lucene.search.highlight.SimpleHTMLEncoder.
// It escapes HTML special characters to prevent XSS attacks and ensure valid HTML.
type SimpleHTMLEncoder struct{}

// NewSimpleHTMLEncoder creates a new SimpleHTMLEncoder.
func NewSimpleHTMLEncoder() *SimpleHTMLEncoder {
	return &SimpleHTMLEncoder{}
}

// EncodeText encodes the given text for HTML output.
// It escapes the following characters:
//   - & becomes &amp;
//   - < becomes &lt;
//   - > becomes &gt;
//   - " becomes &quot;
//   - ' becomes &#x27;
func (e *SimpleHTMLEncoder) EncodeText(originalText string) string {
	var result strings.Builder
	result.Grow(len(originalText))

	for _, ch := range originalText {
		switch ch {
		case '&':
			result.WriteString("&amp;")
		case '<':
			result.WriteString("&lt;")
		case '>':
			result.WriteString("&gt;")
		case '"':
			result.WriteString("&quot;")
		case '\'':
			result.WriteString("&#x27;")
		default:
			result.WriteRune(ch)
		}
	}

	return result.String()
}

// DefaultEncoder is a no-op encoder that returns text unchanged.
//
// Use this when no encoding is needed or when the output format
// doesn't require special character escaping.
type DefaultEncoder struct{}

// NewDefaultEncoder creates a new DefaultEncoder.
func NewDefaultEncoder() *DefaultEncoder {
	return &DefaultEncoder{}
}

// EncodeText returns the original text unchanged.
func (e *DefaultEncoder) EncodeText(originalText string) string {
	return originalText
}

// Ensure interfaces are implemented
var (
	_ Encoder = (*SimpleHTMLEncoder)(nil)
	_ Encoder = (*DefaultEncoder)(nil)
)
