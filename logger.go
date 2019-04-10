package dao

import (
	"context"
	"fmt"
	"time"
)

type Logger interface {
	Debugf(c context.Context, format string, args ...interface{})
	Infof(c context.Context, format string, args ...interface{})
	Warningf(c context.Context, format string, args ...interface{})
	Errorf(c context.Context, format string, args ...interface{})
	Error(c context.Context, err error, format string, args ...interface{})
	Criticalf(c context.Context, format string, args ...interface{})
}

func timestampFormatted(c context.Context) string {
	return time.Now().Format("2006-01-02 15:04:05.999")
}

type stdoutLogger struct{}

func (l *stdoutLogger) Debugf(c context.Context, format string, args ...interface{}) {
	fmt.Printf("DEBUG "+timestampFormatted(c)+" "+format+"\n", args...)
}
func (l *stdoutLogger) Infof(c context.Context, format string, args ...interface{}) {
	fmt.Printf("INFO "+timestampFormatted(c)+" "+format+"\n", args...)
}
func (l *stdoutLogger) Warningf(c context.Context, format string, args ...interface{}) {
	fmt.Printf("WARNING "+timestampFormatted(c)+" "+format+"\n", args...)
}
func (l *stdoutLogger) Errorf(c context.Context, format string, args ...interface{}) {
	fmt.Printf("ERROR "+timestampFormatted(c)+" "+format+"\n", args...)
}
func (l *stdoutLogger) Error(c context.Context, err error, format string, args ...interface{}) {
	fmt.Printf("ERROR "+timestampFormatted(c)+" "+fmt.Sprintf(format, args...)+": %s", err.Error())
}
func (l *stdoutLogger) Criticalf(c context.Context, format string, args ...interface{}) {
	fmt.Printf("ERROR "+timestampFormatted(c)+" "+fmt.Sprintf(format, args...)+": %s, %s\n%s", args...)
}

func NewStdoutLogger() Logger {
	return new(stdoutLogger)
}
