package tests

import (
	"errors"
	"fmt"
	"testing"
)

// mockTB is a mock for testing.TB
type mockTB struct {
	fatalCalled bool
	errorCalled bool
	lastMsg    string
}

func (m *mockTB) Fatalf(format string, args ...interface{}) {
	m.fatalCalled = true
	m.lastMsg = fmt.Sprintf(format, args...)
}

func (m *mockTB) Errorf(format string, args ...interface{}) {
	m.errorCalled = true
	m.lastMsg = fmt.Sprintf(format, args...)
}

// mockRule is a mock implementation of BeforeAfterRule
type mockRule struct {
	beforeCalled bool
	afterCalled  bool
	beforeErr    error
	afterErr     error
}

func (m *mockRule) Before() error {
	m.beforeCalled = true
	return m.beforeErr
}

func (m *mockRule) After() error {
	m.afterCalled = true
	return m.afterErr
}

func TestExecuteRule_Success(t *testing.T) {
	rule := &mockRule{}
	testRan := false
	mt := &mockTB{}
	ExecuteRuleInternal(mt, rule, func(t TB) {
		testRan = true
	})

	if !rule.beforeCalled {
		t.Error("Expected Before to be called")
	}
	if !testRan {
		t.Error("Expected test function to be run")
	}
	if !rule.afterCalled {
		t.Error("Expected After to be called")
	}
	if mt.fatalCalled {
		t.Error("Expected no fatal failure")
	}
}

func TestExecuteRule_BeforeFails(t *testing.T) {
	rule := &mockRule{beforeErr: errors.New("before fail")}
	testRan := false
	mt := &mockTB{}
	ExecuteRuleInternal(mt, rule, func(t TB) {
		testRan = true
	})

	if !rule.beforeCalled {
		t.Error("Expected Before to be called")
	}
	if testRan {
		t.Error("Expected test function to be skipped because Before failed")
	}
	if !rule.afterCalled {
		t.Error("Expected After to be called even if Before failed")
	}
	if !mt.fatalCalled {
		t.Error("Expected fatal failure")
	}
}

func TestExecuteRule_TestFails(t *testing.T) {
	rule := &mockRule{}
	mt := &mockTB{}
	ExecuteRuleInternal(mt, rule, func(t TB) {
		t.Errorf("test failure")
	})

	if !rule.beforeCalled {
		t.Error("Expected Before to be called")
	}
	if !rule.afterCalled {
		t.Error("Expected After to be called even if test failed")
	}
	if !mt.errorCalled {
		t.Error("Expected error to be reported")
	}
}

func TestExecuteRule_TestPanics(t *testing.T) {
	rule := &mockRule{}
	mt := &mockTB{}
	ExecuteRuleInternal(mt, rule, func(t TB) {
		panic("boom")
	})

	if !rule.beforeCalled {
		t.Error("Expected Before to be called")
	}
	if !rule.afterCalled {
		t.Error("Expected After to be called even if test panicked")
	}
	if !mt.fatalCalled {
		t.Error("Expected fatal failure due to panic")
	}
}

func TestExecuteRule_AfterFails(t *testing.T) {
	rule := &mockRule{afterErr: errors.New("after fail")}
	mt := &mockTB{}
	ExecuteRuleInternal(mt, rule, func(t TB) {
		// success
	})

	if !rule.beforeCalled {
		t.Error("Expected Before to be called")
	}
	if !rule.afterCalled {
		t.Error("Expected After to be called")
	}
	if !mt.fatalCalled {
		t.Error("Expected fatal failure")
	}
}

func TestExecuteRule_MultipleFailures(t *testing.T) {
	rule := &mockRule{
		beforeErr: errors.New("before fail"),
		afterErr:  errors.New("after fail"),
	}
	mt := &mockTB{}
	ExecuteRuleInternal(mt, rule, func(t TB) {
		// should not be called
	})

	if !rule.beforeCalled {
		t.Error("Expected Before to be called")
	}
	if !rule.afterCalled {
		t.Error("Expected After to be called")
	}
	if !mt.fatalCalled {
		t.Error("Expected fatal failure")
	}
}
