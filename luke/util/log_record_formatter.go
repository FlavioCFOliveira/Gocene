package util

import (
	"fmt"
)

// LogRecordFormatter formats an ImmutableLogRecord to string.
type LogRecordFormatter struct{}

// Format formats an ImmutableLogRecord to string.
func (f *LogRecordFormatter) Format(r ImmutableLogRecord) string {
	return fmt.Sprintf(
		"%s [%s] %s: %s",
		r.Instant.Format("15:04:05"),
		r.Level.String(),
		r.LoggerName,
		r.Message+f.toString(r.Thrown),
	)
}

func (f *LogRecordFormatter) toString(t error) string {
	if t == nil {
		return ""
	}
	// In Java, t.printStackTrace() is used to dump the full stack trace.
	// Go's standard error does not carry a stack trace by default.
	// We use t.Error() as the closest equivalent for the error message.
	return "\n" + t.Error()
}
