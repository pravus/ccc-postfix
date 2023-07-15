package volatile

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"ccc-mail-api/internal/model"
)

type Logger struct {
	level model.LogLevel
}

func NewLogger(level model.LogLevel) *Logger {
	logger := &Logger{
		level: level,
	}
	return logger
}

func (logger Logger) Log(level model.LogLevel, format string, args ...any) {
	fmt.Println(time.Now().UTC().Format(`2006-01-02 15:04:05`) + fmt.Sprintf(` [%-5s] `, level.String()) + fmt.Sprintf(format, args...))
}

func (logger *Logger) Level() model.LogLevel {
	return logger.level
}

func (logger *Logger) SetLevel(level model.LogLevel) {
	logger.level = level
}

func (logger *Logger) SetLevelFromString(want string) {
	logger.level = model.LogLevelFromString(want)
}

func (logger Logger) Trace(format string, args ...any) {
	if logger.level <= model.LogLevelTrace {
		logger.Log(model.LogLevelTrace, format, args...)
	}
}

func (logger Logger) Debug(format string, args ...any) {
	if logger.level <= model.LogLevelDebug {
		logger.Log(model.LogLevelDebug, format, args...)
	}
}

func (logger Logger) Info(format string, args ...any) {
	if logger.level <= model.LogLevelInfo {
		logger.Log(model.LogLevelInfo, format, args...)
	}
}

func (logger Logger) Warn(format string, args ...any) {
	if logger.level <= model.LogLevelWarn {
		logger.Log(model.LogLevelWarn, format, args...)
	}
}

func (logger Logger) Error(format string, args ...any) {
	if logger.level <= model.LogLevelError {
		logger.Log(model.LogLevelError, format, args...)
	}
}

func (logger Logger) Panic(format string, args ...any) {
	logger.Log(model.LogLevelPanic, format, args...)
}

func (logger Logger) Fatal(format string, args ...any) {
	logger.Log(model.LogLevelFatal, format, args...)
	os.Exit(1)
}

func (logger Logger) Serve(format string, args ...any) {
	logger.Log(model.LogLevelServe, format, args...)
}

func (logger Logger) Audit(format string, args ...any) {
	logger.Log(model.LogLevelAudit, format, args...)
}

func (logger Logger) Help(format string, args ...any) {
	logger.Log(model.LogLevelHelp, format, args...)
}

type LogFormatter struct {
	label  string
	logger model.Logger
}

var _ middleware.LogFormatter = (*LogFormatter)(nil)

func NewLogFormatter(label string, logger model.Logger) LogFormatter {
	formatter := LogFormatter{
		label:  label,
		logger: logger,
	}
	return formatter
}

func (formatter LogFormatter) NewLogEntry(r *http.Request) middleware.LogEntry {
	return LogEntry{LogFormatter: formatter, request: r}
}

type LogEntry struct {
	LogFormatter
	request *http.Request
}

var _ middleware.LogEntry = (*LogEntry)(nil)

func (entry LogEntry) Write(code int, written int, header http.Header, elapsed time.Duration, extra any) {
	req := entry.request
	sig := '.'
	if req.TLS != nil {
		sig = '^'
	}
	// FIXME: there are 3 slots left to move the path to the correct column
	// FIXME: need stripped path here? what is correct through the entire cycle?
	entry.logger.Serve(`%-5s %c %s %d %-7s %-21s %9d %9d %15s    %s`,
		entry.label, sig, middleware.GetReqID(req.Context()),
		code, req.Method, req.RemoteAddr, req.ContentLength, written, elapsed.String(), req.URL.String())
}

func (entry LogEntry) Panic(v any, stack []byte) {
	entry.logger.Panic(`%T %+v %s`, v, v, string(stack))
}
