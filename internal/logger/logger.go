package logger

import (
	"context"
	"io"
	"os"

	"github.com/sirupsen/logrus"
)

type contextKey string

const traceIDKey contextKey = "trace_id"

var log *logrus.Logger

func init() {
	log = logrus.New()
	log.SetOutput(os.Stdout)
	log.SetLevel(logrus.InfoLevel)
	log.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
	})
}

func Setup(level, mode string) {
	parsedLevel, err := logrus.ParseLevel(level)
	if err != nil {
		parsedLevel = logrus.InfoLevel
	}
	log.SetLevel(parsedLevel)

	if mode == "development" {
		log.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:    true,
			TimestampFormat: "15:04:05",
		})
	}
}

func SetOutput(w io.Writer) {
	log.SetOutput(w)
}

func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

func TraceIDFromContext(ctx context.Context) string {
	if traceID, ok := ctx.Value(traceIDKey).(string); ok {
		return traceID
	}
	return ""
}

func withContext(ctx context.Context) *logrus.Entry {
	entry := log.WithFields(logrus.Fields{})
	if traceID := TraceIDFromContext(ctx); traceID != "" {
		entry = entry.WithField("trace_id", traceID)
	}
	return entry
}

func WithField(key string, value interface{}) *logrus.Entry {
	return log.WithField(key, value)
}

func WithFields(fields map[string]interface{}) *logrus.Entry {
	return log.WithFields(fields)
}

func WithCluster(clusterID string) *logrus.Entry {
	return log.WithField("cluster_id", clusterID)
}

func Debug(msg string) {
	log.Debug(msg)
}

func Info(msg string) {
	log.Info(msg)
}

func Warn(msg string) {
	log.Warn(msg)
}

func Error(msg string) {
	log.Error(msg)
}

func Fatal(msg string) {
	log.Fatal(msg)
}

func Debugf(format string, args ...interface{}) {
	log.Debugf(format, args...)
}

func Infof(format string, args ...interface{}) {
	log.Infof(format, args...)
}

func Warnf(format string, args ...interface{}) {
	log.Warnf(format, args...)
}

func Errorf(format string, args ...interface{}) {
	log.Errorf(format, args...)
}

func Fatalf(format string, args ...interface{}) {
	log.Fatalf(format, args...)
}

// Context-aware logging
func DebugCtx(ctx context.Context, msg string) {
	withContext(ctx).Debug(msg)
}

func InfoCtx(ctx context.Context, msg string) {
	withContext(ctx).Info(msg)
}

func WarnCtx(ctx context.Context, msg string) {
	withContext(ctx).Warn(msg)
}

func ErrorCtx(ctx context.Context, msg string) {
	withContext(ctx).Error(msg)
}

func DebugCtxf(ctx context.Context, format string, args ...interface{}) {
	withContext(ctx).Debugf(format, args...)
}

func InfoCtxf(ctx context.Context, format string, args ...interface{}) {
	withContext(ctx).Infof(format, args...)
}

func WarnCtxf(ctx context.Context, format string, args ...interface{}) {
	withContext(ctx).Warnf(format, args...)
}

func ErrorCtxf(ctx context.Context, format string, args ...interface{}) {
	withContext(ctx).Errorf(format, args...)
}