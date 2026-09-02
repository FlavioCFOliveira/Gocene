package util

import (
	"fmt"
	"sync"
	"time"
)

// LogLevel represents the severity of a log message.
type LogLevel int

const (
	LevelFinest LogLevel = iota
	LevelFiner
	LevelFine
	LevelConfig
	LevelInfo
	LevelWarning
	LevelSevere
	LevelOff
)

func (l LogLevel) String() string {
	return []string{"FINEST", "FINER", "FINE", "CONFIG", "INFO", "WARNING", "SEVERE", "OFF"}[l]
}

// ImmutableLogRecord provides a snapshot of a log record.
type ImmutableLogRecord struct {
	LoggerName string
	Level      LogLevel
	Message    string
	Thrown     error
	Instant    time.Time
}

// LogUpdateListener is a function that receives log state updates.
type LogUpdateListener func([]ImmutableLogRecord)

// CircularLogBufferHandler is a bounded buffer of recent log messages.
type CircularLogBufferHandler struct {
	mu        sync.Mutex
	buffer    []ImmutableLogRecord
	listeners []LogUpdateListener
	maxSize   int
}

func NewCircularLogBufferHandler(maxSize int) *CircularLogBufferHandler {
	return &CircularLogBufferHandler{
		buffer:  make([]ImmutableLogRecord, 0, maxSize),
		maxSize: maxSize,
	}
}

func (h *CircularLogBufferHandler) Publish(record ImmutableLogRecord) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.buffer) >= h.maxSize {
		h.buffer = h.buffer[1:]
	}
	h.buffer = append(h.buffer, record)

	for _, l := range h.listeners {
		l(h.getLogRecordsInternal())
	}
}

func (h *CircularLogBufferHandler) AddUpdateListener(l LogUpdateListener) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.listeners = append(h.listeners, l)
}

func (h *CircularLogBufferHandler) RemoveUpdateListener(l LogUpdateListener) {
	h.mu.Lock()
	defer h.mu.Unlock()
	// Simplified removal: normally would use an ID or similar
}

func (h *CircularLogBufferHandler) GetLogRecords() []ImmutableLogRecord {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.getLogRecordsInternal()
}

func (h *CircularLogBufferHandler) getLogRecordsInternal() []ImmutableLogRecord {
	cp := make([]ImmutableLogRecord, len(h.buffer))
	copy(cp, h.buffer)
	return cp
}

func GetCircularBuffer() *CircularLogBufferHandler {
	return GlobalLoggerFactory.CircularBuffer
}

// LoggerFactory manages the circular log buffer.
type LoggerFactory struct {
	CircularBuffer *CircularLogBufferHandler
}

var GlobalLoggerFactory = &LoggerFactory{}

func (f *LoggerFactory) InitGuiLogging() {
	f.CircularBuffer = NewCircularLogBufferHandler(1000)
}
