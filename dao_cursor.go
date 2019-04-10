package dao

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const (
	DefaultCursorLimit = 1000
	DefaultPageSize    = 20
)

type Cursor struct {
	limit  int
	offset int
}

func NewCursor(limit, offset int) *Cursor {
	return &Cursor{limit: limit, offset: offset}
}

func NewFirstPageCursor() *Cursor {
	return &Cursor{limit: DefaultPageSize, offset: 0}
}

func (c *Cursor) NextPage() *Cursor {
	if c == nil {
		return NewFirstPageCursor()
	}
	return NewCursor(c.offset+c.limit, c.limit)
}

func (c *Cursor) Limit() int {
	if c == nil {
		return DefaultCursorLimit
	}
	return c.limit
}

func (c *Cursor) Offset() int {
	if c == nil {
		return 0
	}
	return c.offset
}

func (c *Cursor) Serialize() string {
	if c == nil {
		return ""
	}
	return hex.EncodeToString([]byte(fmt.Sprintf("%d:%d", c.Limit(), c.Offset())))
}

func (c *Cursor) Unserialize(serialized string) error {
	if serialized == "" {
		c.offset = 0
		c.limit = DefaultCursorLimit
	}

	bytes, err := hex.DecodeString(serialized)
	if err != nil {
		return errors.Wrapf(err, "serialized=%s", serialized)
	}

	parts := strings.Split(string(bytes), ":")
	if len(parts) != 2 {
		return errors.Errorf("Invalid cursor unserialized=%s", string(bytes))
	}

	limit, err := strconv.ParseInt(parts[0], 10, 32)
	if err != nil {
		return errors.Wrapf(err, "unserialized=%s", string(bytes))
	}

	offset, err := strconv.ParseInt(parts[1], 10, 32)
	if err != nil {
		return errors.Wrapf(err, "unserialized=%s", string(bytes))
	}

	c.limit = int(limit)
	c.offset = int(offset)

	return nil
}
