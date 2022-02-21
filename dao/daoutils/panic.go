package daoutils

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/ansel1/merry"
)

func PanicIfErr(err error) {
	if err != nil {
		panic(err)
	}
}

type PanicLogger interface {
	Errorf(c context.Context, format string, args ...interface{})
}

func panicMsg(r interface{}) string {
	stacktrace := string(debug.Stack())
	if err, is := r.(error); is {
		stacktrace = FirstNonEmpty(merry.Stacktrace(err), stacktrace)
		return fmt.Sprintf("[%T] %s: %s\n%s", r, merry.Message(err), merry.UserMessage(err), stacktrace)
	}
	return fmt.Sprintf("panic: [%T] %#v\n%s", r, r, stacktrace)
}

func CheckPanicOrLog(r interface{}, log PanicLogger) bool {
	if r != nil {
		if log == nil {
			fmt.Fprint(os.Stderr, panicMsg(r))
		} else {
			log.Errorf(context.Background(), panicMsg(r))
		}
		return true
	}
	return false
}

func CheckPanicOrStderr(r interface{}) bool {
	if r != nil {
		fmt.Fprintln(os.Stderr, panicMsg(r))
		return true
	}
	return false
}

func CheckPanicAndErr(r interface{}) error {
	if r != nil {
		stacktrace := string(debug.Stack())
		if err, is := r.(error); is {
			return merry.Wrap(err)
		}
		return merry.New("panic").Appendf("panic: [%T] %#v\n%s", r, r, stacktrace)
	}
	return nil
}
