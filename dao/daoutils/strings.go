package daoutils

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"
)

func FirstNonEmpty(strs ...string) string {
	for _, str := range strs {
		if len(str) > 0 {
			return str
		}
	}
	return ""
}

func FormatTime(duration, unit time.Duration, decimalDigits int) string {
	f := float64(duration) / float64(unit)
	format := fmt.Sprint("%.", decimalDigits, "f", getTimeDurationUnit(unit))
	return fmt.Sprintf(format, f)
}

func getTimeDurationUnit(unit time.Duration) string {
	return strings.Trim(strings.Split(unit.String(), "0")[0], "1234567890.")
}

type StringBuilder struct {
	buff *bytes.Buffer
}

func NewStringBuilder() *StringBuilder { return &StringBuilder{buff: bytes.NewBuffer([]byte{})} }

func (sb *StringBuilder) Append(a ...interface{}) *StringBuilder {
	_, _ = sb.buff.WriteString(fmt.Sprint(a...))
	return sb
}

func (sb *StringBuilder) Appendln(a ...interface{}) *StringBuilder {
	_, _ = sb.buff.WriteString(fmt.Sprintln(a...))
	return sb
}

func (sb *StringBuilder) Appendf(format string, a ...interface{}) *StringBuilder {
	_, _ = sb.buff.WriteString(fmt.Sprintf(format, a...))
	return sb
}

func (sb StringBuilder) String() string {
	return sb.buff.String()
}

func (sb StringBuilder) StringTrimmed() string {
	return strings.TrimSpace(sb.buff.String())
}

func (sb StringBuilder) Bytes() []byte {
	return sb.buff.Bytes()
}

func CloseCloser(closer io.Closer) {
	if closer != nil {
		_ = closer.Close()
	}
}
