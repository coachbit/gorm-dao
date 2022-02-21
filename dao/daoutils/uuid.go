package daoutils

import (
	"github.com/gofrs/uuid"
)

func NewUUIDV4() uuid.UUID {
	id, err := uuid.NewV4()
	if err != nil {
		panic(err)
	}
	return id
}
