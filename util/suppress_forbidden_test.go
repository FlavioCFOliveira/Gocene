// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package util

import (
	"strings"
	"testing"
)

func TestSuppressForbidden_ReturnsReason(t *testing.T) {
	reason := "System.out used before logger init"
	if got := SuppressForbidden(reason); got != reason {
		t.Fatalf("SuppressForbidden(%q)=%q want %q", reason, got, reason)
	}
}

func TestSuppressForbiddenDirective_FormatStable(t *testing.T) {
	if !strings.HasPrefix(SuppressForbiddenDirective, "//gocene:suppress-forbidden=") {
		t.Fatalf("directive prefix changed: %q", SuppressForbiddenDirective)
	}
}
