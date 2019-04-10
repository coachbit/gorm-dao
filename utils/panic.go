package utils

import (
	"fmt"
	"os"
	"runtime/debug"
)

func CheckPanic(r interface{}) bool {
	if r != nil {
		if err, is := r.(error); is {
			_, _ = fmt.Fprintf(os.Stderr, "panic error: %+v\n", err)
		} else {
			_, _ = fmt.Fprintf(os.Stderr, "panic: %#v\n", r)
		}
		_, _ = fmt.Fprintf(os.Stderr, "trace: %s", string(debug.Stack()))
		return true
	}
	return false
}
