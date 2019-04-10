package models

import (
	"fmt"
	"time"

	"github.com/teachme2/gorm-dao/utils"

	"github.com/gofrs/uuid"
)

type BaseModel struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time `sql:"index"`
}

func (bm *BaseModel) GetID() uuid.UUID {
	return bm.ID
}

func (bm *BaseModel) GenerateID() {
	bm.ID = utils.NewUUIDV4()
}

func (bm *BaseModel) IsIDNil() bool {
	return bm.GetID() == uuid.Nil
}

var AllTestModels = []interface{}{
	new(User),
	new(Something),
	new(UserWithHooks),
}

type User struct {
	BaseModel

	Birth     time.Time
	Email     string
	FirstName string `gorm:"column:name"`
	LastName  string
	Phone     string

	Code int
}

type Something struct {
	BaseModel
	UserID uuid.UUID
	Code   string
}

type UserWithHooks struct {
	BaseModel

	FirstName string
	LastName  string

	Name string
}

func (p *UserWithHooks) BeforeCreateOrUpdate() map[string]interface{} {
	p.Name = fmt.Sprintf("%s %s", p.FirstName, p.LastName)
	return map[string]interface{}{
		Columns.UserWithHooks.Name: p.Name,
	}
}
