package uhighlight

import (
	"strings"
)

// DefaultPassageFormatter creates a formatted snippet from the top passages.
// The default implementation marks the query terms as bold, and places ellipses between
// unconnected passages.
type DefaultPassageFormatter struct {
	preTag    string
	postTag   string
	ellipsis  string
	escape    bool
}

// NewDefaultPassageFormatter creates a new DefaultPassageFormatter with the default tags.
func NewDefaultPassageFormatter() *DefaultPassageFormatter {
	return &DefaultPassageFormatter{
		preTag:   "<b>",
		postTag:  "</b>",
		ellipsis: "... ",
		escape:   false,
	}
}

// NewDefaultPassageFormatterWithParams creates a new DefaultPassageFormatter with custom tags.
func NewDefaultPassageFormatterWithParams(preTag, postTag, ellipsis string, escape bool) *DefaultPassageFormatter {
	return &DefaultPassageFormatter{
		preTag:   preTag,
		postTag:  postTag,
		ellipsis: ellipsis,
		escape:   escape,
	}
}

func (f *DefaultPassageFormatter) Format(passages []*Passage, content string) interface{} {
	var sb strings.Builder
	pos := 0
	for _, passage := range passages {
		// don't add ellipsis if it's the first one, or if it's connected.
		if sb.Len() > 0 && passage.StartOffset() != pos {
			sb.WriteString(f.ellipsis)
		}
		pos = passage.StartOffset()
		for i := 0; i < passage.NumMatches(); i++ {
			start := passage.MatchStarts()[i]
			// append content before this start
			f.append(&sb, content, pos, start)

			end := passage.MatchEnds()[i]
			// It's possible to have overlapping terms.
			// Look ahead to expand 'end' past all overlapping.
			for i+1 < passage.NumMatches() && passage.MatchStarts()[i+1] < end {
				if passage.MatchEnds()[i+1] > end {
					end = passage.MatchEnds()[i+1]
				}
				i++
			}
			if end > passage.EndOffset() {
				end = passage.EndOffset()
			}

			sb.WriteString(f.preTag)
			f.append(&sb, content, start, end)
			sb.WriteString(f.postTag)

			pos = end
		}
		// its possible a "term" from the analyzer could span a sentence boundary.
		endPos := passage.EndOffset()
		if pos < endPos {
			f.append(&sb, content, pos, endPos)
		}
		pos = passage.EndOffset()
	}
	return sb.String()
}

func (f *DefaultPassageFormatter) append(sb *strings.Builder, content string, start, end int) {
	if start < 0 {
		start = 0
	}
	if end > len(content) {
		end = len(content)
	}
	if start >= end {
		return
	}

	if f.escape {
		for i := start; i < end; i++ {
			ch := content[i]
			switch ch {
			case '&':
				sb.WriteString("&amp;")
			case '<':
				sb.WriteString("&lt;")
			case '>':
				sb.WriteString("&gt;")
			case '"':
				sb.WriteString("&quot;")
			case '\'':
				sb.WriteString("&#x27;")
			case '/':
				sb.WriteString("&#x2F;")
			default:
				sb.WriteByte(ch)
			}
		}
	} else {
		sb.WriteString(content[start:end])
	}
}
