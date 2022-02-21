package dao

import "context"

type Logger interface {
	Debugf(c context.Context, format string, args ...interface{})
	Infof(c context.Context, format string, args ...interface{})
	Warningf(c context.Context, format string, args ...interface{})
	Errorf(c context.Context, format string, args ...interface{})
	Errf(c context.Context, err error, format string, args ...interface{})
	Criticalf(c context.Context, format string, args ...interface{})
	ClearErrorSamples() []string
}
