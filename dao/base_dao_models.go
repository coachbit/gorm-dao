package dao

import (
	"time"

	"github.com/coachbit/gorm-dao/dao/daoutils"
	"github.com/gofrs/uuid"
)

type BaseModel struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (bm *BaseModel) GetID() uuid.UUID {
	return bm.ID
}

func (bm *BaseModel) GenerateID() {
	bm.ID = daoutils.NewUUIDV4()
}

func (bm *BaseModel) IsIDNil() bool {
	return bm.GetID() == uuid.Nil
}

// DeletableBaseModel can be used for tables with `deleted_at` columns.
//
// Use GetDeletedByID() to load those entities anyway.
//
// Note that the problem with keeping those in the table are unique columns. For example, a user with `deleted_at` set
// will still have an email, and any new user can't reuse it. So, if keeping deleted values is important -- maybe it
// would be better to keep them in another table.
type DeletableBaseModel struct {
	BaseModel
	DeletedAt *time.Time `sql:"index"`
}

/*
type UserWithHooks struct {
	BaseModel

	FirstName string
	LastName  string

	Name string
}

func (p *UserWithHooks) BeforeCreateOrUpdate() map[string]interface{} {
	p.Name = fmt.Sprintf("%s %s", p.FirstName, p.LastName)
	return map[string]interface{}{
		"name": p.Name,
	}
}
*/
