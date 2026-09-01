package flexible; import "fmt"; func main() { e := NewEscapeQuerySyntaxImpl(); fmt.Println(e.Escape("foo:bar", "en", EscapeNormal)) }
