package sqlm

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
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

// NewNullLogger returns a logger writing nowhere.
func NewNullLogger() *BaseLogger {
	return NewBaseLogger(FATAL, "", io.Discard)
}

// NoopLogger implements StdLog and swallows everything. It is the default
// logger used when none has been configured, so sqlm never panics because of a
// missing logger.
type NoopLogger struct{}

func (l NoopLogger) Debug(s string) {}
func (l NoopLogger) Info(s string)  {}
func (l NoopLogger) Warn(s string)  {}
func (l NoopLogger) Error(s string) {}
func (l NoopLogger) Panic(s string) {}
func (l NoopLogger) Fatal(s string) {}

func NewNoopLogger() *NoopLogger { return &NoopLogger{} }

// MemLogger is a StdLog implementation storing everything in memory.
// Handy for tests and for asserting emitted SQL.
type MemLogger struct {
	mu     sync.Mutex
	Level  LogLevel
	Debug_ []string
	Info_  []string
	Warn_  []string
	Error_ []string
	Panic_ []string
	Fatal_ []string
}

func NewMemLogger(level LogLevel) *MemLogger {
	return &MemLogger{Level: level}
}

func (l *MemLogger) Debug(s string) { l.append(DEBUG, &l.Debug_, s) }
func (l *MemLogger) Info(s string)  { l.append(INFO, &l.Info_, s) }
func (l *MemLogger) Warn(s string)  { l.append(WARN, &l.Warn_, s) }
func (l *MemLogger) Error(s string) { l.append(ERROR, &l.Error_, s) }
func (l *MemLogger) Panic(s string) { l.append(ERROR, &l.Panic_, s) }
func (l *MemLogger) Fatal(s string) { l.append(FATAL, &l.Fatal_, s) }

func (l *MemLogger) append(level LogLevel, dst *[]string, s string) {
	if l.Level < level {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	*dst = append(*dst, s)
}

// All returns every logged message, level prefixed, in emission order is not
// guaranteed: use Messages(level) instead.
func (l *MemLogger) Messages(level LogLevel) []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	switch level {
	case DEBUG:
		return append([]string(nil), l.Debug_...)
	case INFO:
		return append([]string(nil), l.Info_...)
	case WARN:
		return append([]string(nil), l.Warn_...)
	case FATAL:
		return append([]string(nil), l.Fatal_...)
	default:
		// Panic() 记录在指针自己的桶里，但语义上属于 ERROR
		out := append([]string(nil), l.Error_...)
		return append(out, l.Panic_...)
	}
}

// Last returns the most recent message of the highest available level and is
// handy to assert on the last statement emitted by sqlm.
func (l *MemLogger) Last() string {
	for _, level := range []LogLevel{DEBUG, TRACE, INFO, WARN, ERROR, FATAL} {
		if msgs := l.Messages(level); len(msgs) > 0 {
			return msgs[len(msgs)-1]
		}
	}
	return ""
}

func (l *MemLogger) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Debug_ = nil
	l.Info_ = nil
	l.Warn_ = nil
	l.Error_ = nil
	l.Panic_ = nil
	l.Fatal_ = nil
}

// 1
func (l baseLog) Fatal(s string) {
	if l.Level >= 1 {
		fmt.Printf("[FATA]%s%s\n", l.prefix, s)
	}
}
