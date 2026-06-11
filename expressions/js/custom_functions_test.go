// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

// Port of org.apache.lucene.expressions.js.TestCustomFunctions.
// The Java original tests passing custom MethodHandles to JavascriptCompiler.
// In Go, custom functions are registered as named CustomFunction values.

package js_test

import (
	"math"
	"testing"

	"github.com/FlavioCFOliveira/Gocene/expressions/js"
)

// TestCustomFunctions verifies that custom functions registered via
// FunctionRegistry are callable from compiled expressions and take
// precedence over built-in functions of the same name.
func TestCustomFunctions(t *testing.T) {
	reg := js.NewFunctionRegistry()

	// Register a custom function: double(x) = x * 2
	reg.AddFunction("double", func(args ...float64) (float64, error) {
		return args[0] * 2, nil
	})

	// Register a custom function: add3(x, y, z) = x + y + z
	reg.AddFunction("add3", func(args ...float64) (float64, error) {
		return args[0] + args[1] + args[2], nil
	})

	compiler := js.JavascriptCompiler{}
	compiler.SetFunctions(reg)

	// Test: double(5) = 10
	expr, err := compiler.Compile("double(5)")
	if err != nil {
		t.Fatalf("Compile(double(5)): %v", err)
	}
	result, err := expr.Evaluate(nil)
	if err != nil {
		t.Fatalf("Evaluate(double(5)): %v", err)
	}
	if result != 10.0 {
		t.Errorf("double(5) = %f, want 10.0", result)
	}

	// Test: add3(1, 2, 3) = 6
	expr2, err := compiler.Compile("add3(1, 2, 3)")
	if err != nil {
		t.Fatalf("Compile(add3(1, 2, 3)): %v", err)
	}
	result2, err := expr2.Evaluate(nil)
	if err != nil {
		t.Fatalf("Evaluate(add3(1, 2, 3)): %v", err)
	}
	if result2 != 6.0 {
		t.Errorf("add3(1, 2, 3) = %f, want 6.0", result2)
	}

	// Test: custom function overriding built-in
	// Register "abs" as our own version that negates instead
	reg.AddFunction("abs", func(args ...float64) (float64, error) {
		return -math.Abs(args[0]), nil
	})
	expr3, err := compiler.Compile("abs(5)")
	if err != nil {
		t.Fatalf("Compile(abs(5)) with custom abs: %v", err)
	}
	result3, err := expr3.Evaluate(nil)
	if err != nil {
		t.Fatalf("Evaluate(abs(5)) with custom abs: %v", err)
	}
	if result3 != -5.0 {
		t.Errorf("custom abs(5) = %f, want -5.0", result3)
	}

	// Test: unknown function returns error
	_, err = compiler.Compile("unknown_func(1)")
	if err == nil {
		t.Error("expected error for unknown function, got nil")
	} else {
		t.Logf("unknown function error (expected): %v", err)
	}

	t.Logf("Custom functions test passed")
}

// TestCustomFunctions_NoRegistry verifies that built-in functions still work
// when no custom registry is attached.
func TestCustomFunctions_NoRegistry(t *testing.T) {
	compiler := js.JavascriptCompiler{}

	expr, err := compiler.Compile("sqrt(16)")
	if err != nil {
		t.Fatalf("Compile(sqrt(16)): %v", err)
	}
	result, err := expr.Evaluate(nil)
	if err != nil {
		t.Fatalf("Evaluate(sqrt(16)): %v", err)
	}
	if result != 4.0 {
		t.Errorf("sqrt(16) = %f, want 4.0", result)
	}

	// Verify custom function fails without registry
	_, err = compiler.Compile("custom_thing(1)")
	if err == nil {
		t.Error("expected error for unregistered custom function, got nil")
	}

	t.Logf("NoRegistry test passed")
}

// TestCustomFunctions_RemoveFunction verifies function removal.
func TestCustomFunctions_RemoveFunction(t *testing.T) {
	reg := js.NewFunctionRegistry()
	reg.AddFunction("triple", func(args ...float64) (float64, error) {
		return args[0] * 3, nil
	})

	compiler := js.JavascriptCompiler{}
	compiler.SetFunctions(reg)

	// Works before removal
	expr, err := compiler.Compile("triple(7)")
	if err != nil {
		t.Fatalf("Compile(triple(7)): %v", err)
	}
	result, err := expr.Evaluate(nil)
	if err != nil {
		t.Fatalf("Evaluate(triple(7)): %v", err)
	}
	if result != 21.0 {
		t.Errorf("triple(7) = %f, want 21.0", result)
	}

	// Remove the function
	reg.RemoveFunction("triple")

	// Now compilation should fail
	_, err = compiler.Compile("triple(7)")
	if err == nil {
		t.Error("expected error for removed function, got nil")
	} else {
		t.Logf("removed function error (expected): %v", err)
	}

	t.Logf("RemoveFunction test passed")
}
