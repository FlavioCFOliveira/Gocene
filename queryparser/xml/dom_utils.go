package xml

import (
	"strconv"
	"strings"
)

// DOMUtils provides the helper accessors over *Element that mirror Lucene's
// org.apache.lucene.queryparser.xml.DOMUtils. All helpers accept *Element to
// match the corresponding Java methods that take org.w3c.dom.Element.

// GetAttribute returns the attribute value for the given name or the
// defaultValue when the attribute is missing.
func GetAttribute(e *Element, name, defaultValue string) string {
	if v, ok := e.Attributes[name]; ok {
		return v
	}
	return defaultValue
}

// GetAttributeOrFail returns the named attribute value or a ParserException
// when the attribute is absent.
func GetAttributeOrFail(e *Element, name string) (string, error) {
	if v, ok := e.Attributes[name]; ok {
		return v, nil
	}
	return "", NewParserException("missing required attribute \"" + name + "\" on <" + e.TagName + ">")
}

// GetAttributeWithInheritance walks up the parent chain looking for the
// attribute. The xml.Element type does not currently store a parent pointer,
// so this helper falls back to GetAttribute. It is provided for API parity.
func GetAttributeWithInheritance(e *Element, name string) string {
	return GetAttribute(e, name, "")
}

// GetAttributeIntOrFail parses the named attribute as an int.
func GetAttributeIntOrFail(e *Element, name string) (int, error) {
	v, err := GetAttributeOrFail(e, name)
	if err != nil {
		return 0, err
	}
	n, perr := strconv.Atoi(v)
	if perr != nil {
		return 0, NewParserExceptionWithCause("attribute \""+name+"\" is not an integer", perr)
	}
	return n, nil
}

// GetAttributeInt parses the named attribute as an int, returning defaultValue
// when the attribute is missing or empty.
func GetAttributeInt(e *Element, name string, defaultValue int) int {
	v, ok := e.Attributes[name]
	if !ok || v == "" {
		return defaultValue
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultValue
	}
	return n
}

// GetAttributeFloat parses the named attribute as a float32, returning
// defaultValue when the attribute is missing or unparseable.
func GetAttributeFloat(e *Element, name string, defaultValue float32) float32 {
	v, ok := e.Attributes[name]
	if !ok || v == "" {
		return defaultValue
	}
	f, err := strconv.ParseFloat(v, 32)
	if err != nil {
		return defaultValue
	}
	return float32(f)
}

// GetAttributeBoolean parses the named attribute as a boolean. Lucene treats
// only the literal "true" (case-insensitive) as true.
func GetAttributeBoolean(e *Element, name string, defaultValue bool) bool {
	v, ok := e.Attributes[name]
	if !ok || v == "" {
		return defaultValue
	}
	return strings.EqualFold(v, "true")
}

// GetChildByTagName returns the first direct child whose tag matches name, or
// nil if no such child exists.
func GetChildByTagName(e *Element, name string) *Element {
	for _, c := range e.Children {
		if c.TagName == name {
			return c
		}
	}
	return nil
}

// GetChildByTagNameOrFail returns the first matching child or a ParserException.
func GetChildByTagNameOrFail(e *Element, name string) (*Element, error) {
	if c := GetChildByTagName(e, name); c != nil {
		return c, nil
	}
	return nil, NewParserException("missing required child <" + name + "> under <" + e.TagName + ">")
}

// GetChildrenByTagName returns all direct children whose tag matches name.
func GetChildrenByTagName(e *Element, name string) []*Element {
	out := make([]*Element, 0, len(e.Children))
	for _, c := range e.Children {
		if c.TagName == name {
			out = append(out, c)
		}
	}
	return out
}

// GetFirstChildElement returns the first direct child element, or nil if the
// parent has no element children. This mirrors Lucene's DOMUtils.getFirstChildElement.
func GetFirstChildElement(e *Element) *Element {
	if len(e.Children) == 0 {
		return nil
	}
	return e.Children[0]
}

// GetFirstChildOrFail returns the first child element or a ParserException
// if the parent has no children.
func GetFirstChildOrFail(e *Element) (*Element, error) {
	if c := GetFirstChildElement(e); c != nil {
		return c, nil
	}
	return nil, NewParserException("missing child element under <" + e.TagName + ">")
}

// GetText returns the trimmed text content of the element.
func GetText(e *Element) string {
	return strings.TrimSpace(e.TextContent())
}

// GetNonBlankTextOrFail returns the trimmed text content, or a ParserException
// when the text is empty.
func GetNonBlankTextOrFail(e *Element) (string, error) {
	t := GetText(e)
	if t == "" {
		return "", NewParserException("empty text content in <" + e.TagName + ">")
	}
	return t, nil
}
