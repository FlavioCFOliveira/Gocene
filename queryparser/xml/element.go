package xml

import (
	"encoding/xml"
	"io"
	"strings"
)

// Element is the minimal DOM node the xml package operates on. It is built
// from an encoding/xml stream and provides the subset of W3C DOM operations
// the Lucene XML query parser actually needs (tag name, attributes, child
// elements, text content).
//
// Text and Tail follow lxml's convention so mixed content can be reconstructed
// in document order: Text is the character data that appears immediately
// inside the element before any child element; Tail is the character data
// that appears after the element's end tag but before the next sibling.
type Element struct {
	TagName    string
	Attributes map[string]string
	Children   []*Element
	Text       string
	Tail       string
}

// ParseDocument parses an XML document from the reader into an *Element tree.
// Whitespace-only character data is preserved on the Text field for downstream
// trimming; mixed-content trees keep child order so query builders can scan
// children deterministically.
func ParseDocument(r io.Reader) (*Element, error) {
	dec := xml.NewDecoder(r)
	var root *Element
	stack := make([]*Element, 0, 8)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, NewParserExceptionWithCause("xml decode error", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			node := newElementFromStart(t)
			if len(stack) == 0 {
				root = node
			} else {
				parent := stack[len(stack)-1]
				parent.Children = append(parent.Children, node)
			}
			stack = append(stack, node)
		case xml.EndElement:
			if len(stack) == 0 {
				return nil, NewParserException("unexpected end element </" + t.Name.Local + ">")
			}
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) == 0 {
				continue
			}
			top := stack[len(stack)-1]
			if len(top.Children) == 0 {
				top.Text += string(t)
			} else {
				last := top.Children[len(top.Children)-1]
				last.Tail += string(t)
			}
		}
	}
	if root == nil {
		return nil, NewParserException("empty xml document")
	}
	return root, nil
}

func newElementFromStart(t xml.StartElement) *Element {
	e := &Element{
		TagName:    t.Name.Local,
		Attributes: make(map[string]string, len(t.Attr)),
	}
	for _, a := range t.Attr {
		e.Attributes[a.Name.Local] = a.Value
	}
	return e
}

// TextContent returns the concatenated text content of this element and all
// descendants in document order (analogous to W3C DOM's textContent).
func (e *Element) TextContent() string {
	var b strings.Builder
	e.writeText(&b)
	return b.String()
}

func (e *Element) writeText(b *strings.Builder) {
	b.WriteString(e.Text)
	for _, c := range e.Children {
		c.writeText(b)
		b.WriteString(c.Tail)
	}
}
