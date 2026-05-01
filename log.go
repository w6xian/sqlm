package sqlm

import (
	"fmt"
	"io"
	"log"
	"os"
)

// StdLog interface defines the logging contract
type StdLog interface {
	Debug(s string)
	Info(s string)
	Warn(s string)
	Error(s string)
	Panic(s string)
	Fatal(s string)
}

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

// BaseLogger represents the enhanced logger
type BaseLogger struct {
	level   LogLevel
	prefix  string
	writer  io.Writer
	logger  *log.Logger
	enabled bool
}

// NewBaseLogger creates a new instance of BaseLogger
func NewBaseLogger(level LogLevel, prefix string, writer io.Writer) *BaseLogger {
	if writer == nil {
		writer = os.Stdout
	}

	l := &BaseLogger{
		level:   level,
		prefix:  prefix,
		writer:  writer,
		enabled: true,
	}

	l.logger = log.New(l.writer, "", log.LstdFlags|log.Lmicroseconds)
	return l
}

// SetLevel sets the logging level
func (l *BaseLogger) SetLevel(level LogLevel) {
	l.level = level
}

// SetEnabled enables or disables logging
func (l *BaseLogger) SetEnabled(enabled bool) {
	l.enabled = enabled
}

// logMessage writes the log message if the level is enabled
func (l *BaseLogger) logMessage(level LogLevel, msg string) {
	if !l.enabled || l.level < level {
		return
	}

	l.logger.Output(3, fmt.Sprintf("%s[%s] %s", l.prefix, level.String(), msg))

	if level == FATAL {
		os.Exit(1)
	}
}

// Trace logs a trace message (not implemented in StdLog but kept for completeness)
func (l *BaseLogger) Trace(s string) {
	l.logMessage(TRACE, s)
}

// Debug implements the StdLog interface
func (l *BaseLogger) Debug(s string) {
	l.logMessage(DEBUG, s)
}

// Info implements the StdLog interface
func (l *BaseLogger) Info(s string) {
	l.logMessage(INFO, s)
}

// Warn implements the StdLog interface
func (l *BaseLogger) Warn(s string) {
	l.logMessage(WARN, s)
}

// Error implements the StdLog interface
func (l *BaseLogger) Error(s string) {
	l.logMessage(ERROR, s)
}

// Fatal implements the StdLog interface
func (l *BaseLogger) Fatal(s string) {
	l.logMessage(FATAL, s)
}

// Panic implements the StdLog interface
func (l *BaseLogger) Panic(s string) {
	l.logMessage(ERROR, s)
	panic(s)
}

// WithPrefix returns a new logger with the specified prefix
func (l *BaseLogger) WithPrefix(prefix string) *BaseLogger {
	newLogger := *l
	newLogger.prefix = prefix
	newLogger.logger = log.New(newLogger.writer, "", log.LstdFlags|log.Lmicroseconds)
	return &newLogger
}

// WithLevel returns a new logger with the specified level
func (l *BaseLogger) WithLevel(level LogLevel) *BaseLogger {
	newLogger := *l
	newLogger.level = level
	return &newLogger
}

// baseLog maintains backward compatibility
type baseLog struct {
	prefix string
	Level  int
}

//6
func (l baseLog) Debug(s string) {
	if l.Level >= 6 {
		fmt.Printf("[DEBU]%s%s\n", l.prefix, s)
	}
}

//5
func (l baseLog) Info(s string) {
	if l.Level >= 5 {
		fmt.Printf("[INFO]%s%s\n", l.prefix, s)
	}
}

//4
func (l baseLog) Warn(s string) {
	if l.Level >= 4 {
		fmt.Printf("[WARN]%s%s\n", l.prefix, s)
	}
}

//3
func (l baseLog) Error(s string) {
	if l.Level >= 3 {
		fmt.Printf("[ERRO]%s%s\n", l.prefix, s)
	}
}

//2
func (l baseLog) Panic(s string) {
	if l.Level >= 2 {
		fmt.Printf("[PANI]%s%s\n", l.prefix, s)
	}
}

// 1
func (l baseLog) Fatal(s string) {
	if l.Level >= 1 {
		fmt.Printf("[FATA]%s%s\n", l.prefix, s)
	}
}
