package sqlm

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"
)

// LogLevel defines the severity of the log
type LogLevel int

const (
	FATAL LogLevel = iota
	ERROR
	WARN
	INFO
	DEBUG
	TRACE
)

// String returns the string representation of the log level
func (l LogLevel) String() string {
	switch l {
	case FATAL:
		return "FATAL"
	case ERROR:
		return "ERROR"
	case WARN:
		return "WARN"
	case INFO:
		return "INFO"
	case DEBUG:
		return "DEBUG"
	case TRACE:
		return "TRACE"
	default:
		return "UNKNOWN"
	}
}

// Logger represents the enhanced logger
type Logger struct {
	level   LogLevel
	prefix  string
	writer  io.Writer
	logger  *log.Logger
	enabled bool
}

// NewLogger creates a new instance of Logger
func NewLogger(level LogLevel, prefix string, writer io.Writer) *Logger {
	if writer == nil {
		writer = os.Stdout
	}

	l := &Logger{
		level:   level,
		prefix:  prefix,
		writer:  writer,
		enabled: true,
	}

	l.logger = log.New(l.writer, "", log.LstdFlags|log.Lmicroseconds)
	return l
}

// SetLevel sets the logging level
func (l *Logger) SetLevel(level LogLevel) {
	l.level = level
}

// SetEnabled enables or disables logging
func (l *Logger) SetEnabled(enabled bool) {
	l.enabled = enabled
}

// logMessage writes the log message if the level is enabled
func (l *Logger) logMessage(level LogLevel, msg string) {
	if !l.enabled || l.level < level {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05.000000")
	formattedMsg := fmt.Sprintf("[%s]%s[%s] %s", timestamp, l.prefix, level.String(), msg)
	l.logger.Output(3, formattedMsg) // Skip 3 stack frames to get the caller

	if level == FATAL {
		os.Exit(1)
	}
}

// Trace logs a trace message
func (l *Logger) Trace(s string) {
	l.logMessage(TRACE, s)
}

// Debug logs a debug message
func (l *Logger) Debug(s string) {
	l.logMessage(DEBUG, s)
}

// Info logs an info message
func (l *Logger) Info(s string) {
	l.logMessage(INFO, s)
}

// Warn logs a warning message
func (l *Logger) Warn(s string) {
	l.logMessage(WARN, s)
}

// Error logs an error message
func (l *Logger) Error(s string) {
	l.logMessage(ERROR, s)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(s string) {
	l.logMessage(FATAL, s)
}

// Panic logs a panic message and panics
func (l *Logger) Panic(s string) {
	l.logMessage(ERROR, s)
	panic(s)
}

// WithPrefix returns a new logger with the specified prefix
func (l *Logger) WithPrefix(prefix string) *Logger {
	newLogger := *l
	newLogger.prefix = prefix
	newLogger.logger = log.New(newLogger.writer, "", log.LstdFlags|log.Lmicroseconds)
	return &newLogger
}

// WithLevel returns a new logger with the specified level
func (l *Logger) WithLevel(level LogLevel) *Logger {
	newLogger := *l
	newLogger.level = level
	return &newLogger