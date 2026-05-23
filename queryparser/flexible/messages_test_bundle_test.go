// Copyright 2026 Gocene. All rights reserved.
// Use of this source code is governed by the Apache License 2.0
// that can be found in the LICENSE file.

package flexible_test

import (
	"testing"

	"github.com/FlavioCFOliveira/Gocene/queryparser/flexible"
)

// TestMessagesTestBundle verifies that the messages constants defined in the
// flexible package cover the keys required by the Java MessagesTestBundle.
//
// In Java, MessagesTestBundle is an NLS sub-class that carries two test message
// keys (Q0001E_INVALID_SYNTAX, Q0004E_INVALID_SYNTAX_ESCAPE_UNICODE_TRUNCATION)
// and a deliberately absent key (Q0005E_MESSAGE_NOT_IN_BUNDLE) to test
// NLS fallback behaviour.
//
// In Gocene, message keys are Go constants rather than NLS-style resource
// bundles, so the concept of a missing bundle key does not apply.  The
// meaningful assertion is that MsgInvalidSyntaxCannotParse and
// MsgInvalidSyntaxEscapeUnicodeTruncation are defined and non-empty.
//
// Port of: queryparser/src/test/.../flexible/messages/MessagesTestBundle.java
func TestMessagesTestBundle(t *testing.T) {
	if flexible.MsgInvalidSyntaxCannotParse == "" {
		t.Error("MsgInvalidSyntaxCannotParse must be a non-empty constant")
	}
	if flexible.MsgInvalidSyntaxEscapeUnicodeTruncation == "" {
		t.Error("MsgInvalidSyntaxEscapeUnicodeTruncation must be a non-empty constant")
	}
}
