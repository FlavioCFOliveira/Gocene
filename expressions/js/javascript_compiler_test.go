package js

import "testing"

func TestJavascriptCompilerArithmetic(t *testing.T) {
	expr, err := JavascriptCompiler{}.Compile("(a + b) * 2")
	if err != nil {
		t.Fatal(err)
	}
	v, err := expr.Evaluate(map[string]float64{"a": 3, "b": 4})
	if err != nil || v != 14 {
		t.Errorf("got %v %v", v, err)
	}
	if len(expr.Variables) != 2 {
		t.Errorf("variables = %v", expr.Variables)
	}
}

func TestJavascriptCompilerFunction(t *testing.T) {
	expr, err := JavascriptCompiler{}.Compile("sqrt(x)")
	if err != nil {
		t.Fatal(err)
	}
	v, _ := expr.Evaluate(map[string]float64{"x": 9})
	if v != 3 {
		t.Errorf("sqrt(9) = %v", v)
	}
}
