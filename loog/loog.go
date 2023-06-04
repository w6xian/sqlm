// short for "log"
package loog

import (
	"fmt"
	"log"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type AppLogFunc func(lvl LogLevel, f string, args ...interface{})

type Logger interface {
	Output(maxdepth int, s string) error
}

type NilLogger struct{}

func (l NilLogger) Output(maxdepth int, s string) error {
	return nil
}

type looger struct {
	lg     *zap.Logger
	Option *Options
}

func (l looger) Output(maxdepth int, s string) error {
	p := []rune(s)[0]
	if l.lg != nil {
		switch p {
		case 'D':
			l.lg.Debug(s)
		case 'I':
			l.lg.Info(s)
		case 'W':
			l.lg.Warn(s)
		case 'E':
			l.lg.Error(s)
		case 'P':
			l.lg.Panic(s)
		case 'F':
			l.lg.Fatal(s)
		}
	}
	return nil
}

var loog *looger = &looger{}

func Init(opt *Options) *looger {
	loog.Option = opt
	loog.initial()
	return loog
}

func (l *looger) initial() {
	opt := l.Option
	l.lg = newLogger(
		opt.ServiceName,
		opt.FilePath,
		zapcore.Level(opt.Level),
		opt.MaxSize,
		opt.MaxBackups,
		opt.MaxAge,
		opt.Compress)
}

func Debug(str string) {
	loog.lg.Debug(str)
}
func Info(str string) {
	loog.lg.Info(str)
}
func Warn(str string) {
	loog.lg.Warn(str)
}
func Error(str string) {
	loog.lg.Error(str)
}
func Panic(str string) {
	loog.lg.Panic(str)
}
func Fatal(str string) {
	loog.lg.Fatal(str)
}

const (
	DEBUG = LogLevel(1)
	INFO  = LogLevel(2)
	WARN  = LogLevel(3)
	ERROR = LogLevel(4)
	PANIC = LogLevel(5)
	FATAL = LogLevel(6)
)

type LogLevel int

func (l *LogLevel) String() string {
	switch *l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARNING"
	case ERROR:
		return "ERROR"
	case PANIC:
		return "PANIC"
	case FATAL:
		return "FATAL"
	}
	return "invalid"
}

func Logf(logger Logger, cfgLevel LogLevel, msgLevel LogLevel, f string, args ...interface{}) {
	if cfgLevel > msgLevel {
		return
	}

	if logger != nil {
		logger.Output(3, fmt.Sprintf(msgLevel.String()+": "+f, args...))
	}
}

func LogFatal(prefix string, f string, args ...interface{}) {
	logger := log.New(os.Stderr, prefix, log.Ldate|log.Ltime|log.Lmicroseconds)
	Logf(logger, FATAL, FATAL, f, args...)
	os.Exit(1)
}
