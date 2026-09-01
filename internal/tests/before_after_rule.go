package tests

import (
	"fmt"
	"testing"
)

// TB is an interface that mirrors the failure methods of testing.T.
// It allows the rule execution logic to be tested with a mock.
type TB interface {
	Fatalf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

// BeforeAfterRule is a port of Lucene's AbstractBeforeAfterRule.
// It defines the contract for a test rule that performs setup and teardown.
type BeforeAfterRule interface {
	Before() error
	After() error
}

// BaseRule provides a default empty implementation of BeforeAfterRule.
// This mimics the protected empty methods in the original Java class.
type BaseRule struct{}

func (b *BaseRule) Before() error { return nil }
func (b *BaseRule) After() error  { return nil }

// ExecuteRuleInternal implements the logic of Lucene's AbstractBeforeAfterRule.
// It guarantees the execution of the rule's After method even if the
// Before method or the test function itself fails or panics.
func ExecuteRuleInternal(t TB, rule BeforeAfterRule, testFn func(t TB)) {
	var errors []error

	// 1. Attempt to execute Before.
	beforeFailed := false
	if rule != nil {
		if err := rule.Before(); err != nil {
			errors = append(errors, fmt.Errorf("before: %w", err))
			beforeFailed = true
		}
	}

	// 2. Execute the test function only if Before succeeded.
	if !beforeFailed {
		func() {
			defer func() {
				if r := recover(); r != nil {
					errors = append(errors, fmt.Errorf("panic during test: %v", r))
				}
			}()
			testFn(t)
		}()
	}

	// 3. Attempt to execute After, regardless of whether Before or the test failed.
	if rule != nil {
		if err := rule.After(); err != nil {
			errors = append(errors, fmt.Errorf("after: %w", err))
		}
	}

	// 4. If any errors occurred, fail the test with all collected errors.
	if len(errors) > 0 {
		var errMsg string
		for i, err := range errors {
			if i > 0 {
				errMsg += "\n"
			}
			errMsg += fmt.Sprintf("Error %d: %v", i+1, err)
		}
		t.Fatalf("Rule execution failed with multiple errors:\n%s", errMsg)
	}
}

// ExecuteRule is the public API that works with standard testing.T.
func ExecuteRule(t *testing.T, rule BeforeAfterRule, testFn func(t *testing.T)) {
	ExecuteRuleInternal(t, rule, func(t TB) {
		testFn(t.(*testing.T))
	})
}
