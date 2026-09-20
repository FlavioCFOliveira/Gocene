package components

import (
	"github.com/FlavioCFOliveira/Gocene/luke/util"
)

// LogsPanel provides access to system logs.
type LogsPanel struct {
	logBuffer *util.CircularLogBufferHandler
	filterLevel util.LogLevel // simplified Level representation
}

func NewLogsPanel() *LogsPanel {
	return &LogsPanel{
		logBuffer: util.GetCircularBuffer(),
	}
}

func (p *LogsPanel) SetFilterLevel(level util.LogLevel) {
	p.filterLevel = level
}

func (p *LogsPanel) GetLogs() []string {
	records := p.logBuffer.GetLogRecords()
	var formatted []string
	for _, r := range records {
		if r.Level >= p.filterLevel {
			formatted = append(formatted, r.Message)
		}
	}
	return formatted
}
