// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package js

import "fmt"

// CustomFunction is a user-defined function that can be called from a
// JavaScript expression. The function receives its arguments as float64
// values and must return a float64 result.
//
// This is the Go equivalent of Java's java.lang.invoke.MethodHandle used
// by org.apache.lucene.expressions.js.JavascriptCompiler for custom
// function registration.
type CustomFunction func(args ...float64) (float64, error)

// FunctionRegistry holds named custom functions that can be invoked from
// compiled expressions. Register a function with AddFunction before
// compiling the source that calls it.
type FunctionRegistry struct {
	funcs map[string]CustomFunction
}

// NewFunctionRegistry creates an empty function registry.
func NewFunctionRegistry() *FunctionRegistry {
	return &FunctionRegistry{
		funcs: make(map[string]CustomFunction),
	}
}

// AddFunction registers a custom function under the given name. The name
// is normalised to lowercase so that expression calls are case-insensitive.
// If a function with the same name is already registered it is silently
// replaced.
func (r *FunctionRegistry) AddFunction(name string, fn CustomFunction) {
	r.funcs[normalizeName(name)] = fn
}

// Lookup returns the custom function registered under name, or nil if
// no function is registered.
func (r *FunctionRegistry) Lookup(name string) CustomFunction {
	return r.funcs[normalizeName(name)]
}

// HasFunction returns true when a custom function is registered under name.
func (r *FunctionRegistry) HasFunction(name string) bool {
	_, ok := r.funcs[normalizeName(name)]
	return ok
}

// Len returns the number of registered custom functions.
func (r *FunctionRegistry) Len() int {
	return len(r.funcs)
}

// RemoveFunction unregisters a custom function. It is a no-op when the
// function is not registered.
func (r *FunctionRegistry) RemoveFunction(name string) {
	delete(r.funcs, normalizeName(name))
}

func normalizeName(name string) string {
	n := make([]byte, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		n[i] = c
	}
	return string(n)
}

// --- JavascriptCompiler integration ---

// SetFunctions attaches a function registry to the compiler. Subsequent
// calls to Compile will resolve function calls through this registry
// before falling back to built-in functions.
//
// Passing nil clears the registry.
func (c *JavascriptCompiler) SetFunctions(reg *FunctionRegistry) {
	c.functions = reg
}

// GetFunctions returns the compiler's current function registry, or nil.
func (c *JavascriptCompiler) GetFunctions() *FunctionRegistry {
	return c.functions
}

// ErrUnknownFunction is returned when a function call in the expression
// does not match any built-in or custom function.
func ErrUnknownFunction(name string) error {
	return fmt.Errorf("javascript: unknown function %q", name)
}
