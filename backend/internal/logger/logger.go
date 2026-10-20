package logger

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"time"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

var levelNames = map[LogLevel]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
	FATAL: "FATAL",
}

type LogEntry struct {
	Timestamp  string         `json:"timestamp"`
	Level      string         `json:"level"`
	Message    string         `json:"message"`
	RequestID  string         `json:"request_id,omitempty"`
	UserID     int64          `json:"user_id,omitempty"`
	Method     string         `json:"method,omitempty"`
	Path       string         `json:"path,omitempty"`
	StatusCode int            `json:"status_code,omitempty"`
	Duration   string         `json:"duration,omitempty"`
	Fields     map[string]any `json:"fields,omitempty"`
	Error      string         `json:"error,omitempty"`
	StackTrace string         `json:"stack_trace,omitempty"`
	Caller     string         `json:"caller"`
}

type Logger struct {
	level LogLevel
	// output is shared by derived loggers so SetOutput reaches all of them.
	output    *atomic.Pointer[os.File]
	requestID string
	userID    int64
	fields    map[string]any
}

var defaultLogger *Logger

func init() {
	defaultLogger = New(os.Stdout, DEBUG)
}

func New(output *os.File, level LogLevel) *Logger {
	shared := &atomic.Pointer[os.File]{}
	shared.Store(output)
	return &Logger{
		level:  level,
		output: shared,
		fields: make(map[string]any),
	}
}

func (l *Logger) WithRequestID(requestID string) *Logger {
	newLogger := *l
	newLogger.requestID = requestID
	newLogger.fields = copyFields(l.fields)
	return &newLogger
}

func (l *Logger) WithUserID(userID int64) *Logger {
	newLogger := *l
	newLogger.userID = userID
	newLogger.fields = copyFields(l.fields)
	return &newLogger
}

func (l *Logger) WithField(key string, value any) *Logger {
	newLogger := *l
	newLogger.fields = copyFields(l.fields)
	newLogger.fields[key] = value
	return &newLogger
}

func (l *Logger) WithFields(fields map[string]any) *Logger {
	newLogger := *l
	newLogger.fields = copyFields(l.fields)
	for k, v := range fields {
		newLogger.fields[k] = v
	}
	return &newLogger
}

func copyFields(src map[string]any) map[string]any {
	dst := make(map[string]any)
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func (l *Logger) log(level LogLevel, message string, errs ...error) {
	if level < l.level {
		return
	}

	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     levelNames[level],
		Message:   message,
		RequestID: l.requestID,
		UserID:    l.userID,
		Fields:    l.fields,
		Caller:    getCaller(),
	}

	if len(errs) > 0 && errs[0] != nil {
		entry.Error = errs[0].Error()
		entry.StackTrace = getStackTrace()
	}

	jsonBytes, _ := json.Marshal(entry)
	_, writeErr := l.output.Load().Write(append(jsonBytes, '\n'))
	if writeErr != nil {
		log.Printf("Failed to write log: %v", writeErr)
	}

	if level == FATAL {
		os.Exit(1)
	}
}

func (l *Logger) Debug(message string, args ...any) {
	l.log(DEBUG, fmt.Sprintf(message, args...))
}

func (l *Logger) Debugf(format string, args ...any) {
	l.log(DEBUG, fmt.Sprintf(format, args...))
}

func (l *Logger) Info(message string, args ...any) {
	l.log(INFO, fmt.Sprintf(message, args...))
}

func (l *Logger) Infof(format string, args ...any) {
	l.log(INFO, fmt.Sprintf(format, args...))
}

func (l *Logger) Warn(message string, args ...any) {
	l.log(WARN, fmt.Sprintf(message, args...))
}

func (l *Logger) Warnf(format string, args ...any) {
	l.log(WARN, fmt.Sprintf(format, args...))
}

func (l *Logger) Error(message string, args ...any) {
	l.log(ERROR, fmt.Sprintf(message, args...))
}

func (l *Logger) Errorf(format string, args ...any) {
	l.log(ERROR, fmt.Sprintf(format, args...))
}

func (l *Logger) ErrorWithErr(err error, message string) {
	l.log(ERROR, message, err)
}

func (l *Logger) Fatal(message string, args ...any) {
	l.log(FATAL, fmt.Sprintf(message, args...))
}

func (l *Logger) Fatalf(format string, args ...any) {
	l.log(FATAL, fmt.Sprintf(format, args...))
}

func (l *Logger) RequestLog(method, path string, statusCode int, duration time.Duration) {
	entry := LogEntry{
		Timestamp:  time.Now().UTC().Format(time.RFC3339Nano),
		Level:      "INFO",
		Message:    "HTTP Request",
		RequestID:  l.requestID,
		UserID:     l.userID,
		Method:     method,
		Path:       path,
		StatusCode: statusCode,
		Duration:   duration.String(),
		Fields:     l.fields,
		Caller:     getCaller(),
	}

	if statusCode >= 400 {
		entry.Level = "WARN"
	}
	if statusCode >= 500 {
		entry.Level = "ERROR"
	}

	jsonBytes, _ := json.Marshal(entry)
	l.output.Load().Write(append(jsonBytes, '\n'))
}

func getCaller() string {
	pc, _, line, ok := runtime.Caller(2)
	if !ok {
		return "unknown:0"
	}
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return fmt.Sprintf("%d", line)
	}
	funcName := fn.Name()
	parts := strings.Split(funcName, ".")
	if len(parts) > 1 {
		funcName = parts[len(parts)-1]
	}
	return fmt.Sprintf("%s:%d", funcName, line)
}

func getStackTrace() string {
	buf := make([]byte, 2048)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

func GetDefaultLogger() *Logger {
	return defaultLogger
}

// SetOutput redirects the default logger, for example to capture log lines in tests.
func SetOutput(output *os.File) {
	defaultLogger.output.Store(output)
}

func SetLevel(level LogLevel) {
	defaultLogger.level = level
}
