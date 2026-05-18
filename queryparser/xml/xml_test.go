package xml

import (
	"errors"
	"strings"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/search"
)

func TestParserExceptionMessage(t *testing.T) {
	e := NewParserException("boom")
	if e.Error() != "boom" {
		t.Errorf("got %q", e.Error())
	}
	if e.Unwrap() != nil {
		t.Error("Unwrap should be nil")
	}
}

func TestParserExceptionWithCause(t *testing.T) {
	root := errors.New("io")
	e := NewParserExceptionWithCause("wrap", root)
	if !errors.Is(e, root) {
		t.Error("errors.Is")
	}
}

func TestParseDocument(t *testing.T) {
	xmlStr := `<BooleanQuery><Clause occurs="must"><TermQuery fieldName="title">hello</TermQuery></Clause></BooleanQuery>`
	root, err := ParseDocument(strings.NewReader(xmlStr))
	if err != nil {
		t.Fatal(err)
	}
	if root.TagName != "BooleanQuery" {
		t.Errorf("tag = %q", root.TagName)
	}
	if len(root.Children) != 1 || root.Children[0].TagName != "Clause" {
		t.Errorf("children = %#v", root.Children)
	}
	clause := root.Children[0]
	if clause.Attributes["occurs"] != "must" {
		t.Errorf("attr = %q", clause.Attributes["occurs"])
	}
}

func TestParseDocumentEmpty(t *testing.T) {
	_, err := ParseDocument(strings.NewReader(""))
	if err == nil {
		t.Error("expected error for empty doc")
	}
}

func TestDOMUtilsAttributes(t *testing.T) {
	e := &Element{
		TagName:    "TermQuery",
		Attributes: map[string]string{"fieldName": "title", "boost": "2.5", "active": "true", "n": "7"},
	}
	if got := GetAttribute(e, "fieldName", "default"); got != "title" {
		t.Errorf("GetAttribute = %q", got)
	}
	if got := GetAttribute(e, "missing", "default"); got != "default" {
		t.Errorf("default fallback = %q", got)
	}
	if got, _ := GetAttributeOrFail(e, "fieldName"); got != "title" {
		t.Errorf("OrFail = %q", got)
	}
	if _, err := GetAttributeOrFail(e, "missing"); err == nil {
		t.Error("expected fail")
	}
	if got := GetAttributeInt(e, "n", -1); got != 7 {
		t.Errorf("int = %d", got)
	}
	if got := GetAttributeInt(e, "missing", 9); got != 9 {
		t.Errorf("int default = %d", got)
	}
	if got := GetAttributeFloat(e, "boost", 1.0); got != 2.5 {
		t.Errorf("float = %v", got)
	}
	if !GetAttributeBoolean(e, "active", false) {
		t.Error("bool")
	}
}

func TestDOMUtilsChildren(t *testing.T) {
	e := &Element{
		TagName: "Root",
		Children: []*Element{
			{TagName: "A", Text: "1"},
			{TagName: "B", Text: "2"},
			{TagName: "A", Text: "3"},
		},
	}
	first := GetChildByTagName(e, "A")
	if first == nil || first.Text != "1" {
		t.Errorf("first child")
	}
	if _, err := GetChildByTagNameOrFail(e, "missing"); err == nil {
		t.Error("expected error")
	}
	all := GetChildrenByTagName(e, "A")
	if len(all) != 2 {
		t.Errorf("count = %d", len(all))
	}
	fc := GetFirstChildElement(e)
	if fc == nil || fc.TagName != "A" {
		t.Error("first")
	}
}

func TestDOMUtilsText(t *testing.T) {
	e := &Element{TagName: "T", Text: "  hello  "}
	if GetText(e) != "hello" {
		t.Errorf("trimmed = %q", GetText(e))
	}
	if _, err := GetNonBlankTextOrFail(&Element{}); err == nil {
		t.Error("expected error for blank")
	}
}

func TestQueryBuilderFunc(t *testing.T) {
	called := false
	var b QueryBuilder = QueryBuilderFunc(func(e *Element) (search.Query, error) {
		called = true
		return search.NewMatchAllDocsQuery(), nil
	})
	q, err := b.GetQuery(&Element{TagName: "X"})
	if err != nil {
		t.Fatal(err)
	}
	if q == nil || !called {
		t.Error("not invoked")
	}
}

func TestQueryBuilderFactoryDispatch(t *testing.T) {
	f := NewQueryBuilderFactory()
	f.AddBuilder("MatchAllDocsQuery", QueryBuilderFunc(func(e *Element) (search.Query, error) {
		return search.NewMatchAllDocsQuery(), nil
	}))
	q, err := f.GetQuery(&Element{TagName: "MatchAllDocsQuery"})
	if err != nil {
		t.Fatal(err)
	}
	if q == nil {
		t.Fatal("nil")
	}
	if f.GetBuilder("MatchAllDocsQuery") == nil {
		t.Error("GetBuilder")
	}
}

func TestQueryBuilderFactoryUnknown(t *testing.T) {
	f := NewQueryBuilderFactory()
	_, err := f.GetQuery(&Element{TagName: "Unknown"})
	if err == nil {
		t.Error("expected error")
	}
	var pe *ParserException
	if !errors.As(err, &pe) {
		t.Errorf("err type %T", err)
	}
}

func TestElementTextContent(t *testing.T) {
	root, err := ParseDocument(strings.NewReader("<a>hello <b>world</b>!</a>"))
	if err != nil {
		t.Fatal(err)
	}
	if got := root.TextContent(); got != "hello world!" {
		t.Errorf("TextContent = %q", got)
	}
}
