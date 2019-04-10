package dao

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/teachme2/gorm-dao/models"
	"github.com/teachme2/gorm-dao/utils"

	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"
)

var c = context.Background()

func TestCRUD(t *testing.T) {
	t.Parallel()
	botStartCode := utils.RandomAlphanumeric(20)
	{
		User := models.User{
			FirstName: botStartCode,
			Email:     fmt.Sprintf("%s@jkljkl.com", utils.RandomAlphanumeric(10)),
			Phone:     utils.RandomNumeric(10),
		}
		assert.Nil(t, dao.Create(c, &User))
		code := models.Something{
			UserID: User.GetID(),
			Code:   botStartCode,
		}
		assert.Nil(t, dao.Create(c, &code))
	}
	if t.Failed() {
		t.FailNow()
	}
	{
		var code models.Something
		err := dao.Query(c).Filter(models.Columns.Something.Code, "=", utils.RandomAlphanumeric(30)).First(&code)
		assert.True(t, code.IsIDNil())
		assert.NotNil(t, err)
		assert.True(t, IsRecordNotFound(err))
	}
	if t.Failed() {
		t.FailNow()
	}
	{
		var Users []models.Something
		assert.Nil(t, dao.Query(c).Filter(models.Columns.Something.Code, "=", botStartCode).All(&Users))
		assert.Equal(t, 1, len(Users))
		assert.Equal(t, botStartCode, Users[0].Code)
	}
	if t.Failed() {
		t.FailNow()
	}
	{
		var Users []models.User
		assert.Nil(t, dao.Query(c).All(&Users))
		assert.True(t, len(Users) > 0)
	}
}

func TestCRUDAlwaysUpdatesUpdatedAt(t *testing.T) {
	User := models.User{
		FirstName: utils.RandomAlphanumeric(10),
		Email:     fmt.Sprintf("%s@jkljkl.com", utils.RandomAlphanumeric(10)),
		Phone:     utils.RandomNumeric(10),
	}
	assert.Nil(t, dao.Create(c, &User))
	assert.Nil(t, dao.ByID(c, &User, User.GetID()))
	assert.True(t, time.Since(User.UpdatedAt) < time.Second)

	assert.Nil(t, dao.ByID(c, &User, User.GetID()))
	updatedAt := User.UpdatedAt

	////////////////////////////////////////////////////////////////////////////////////////////////////

	User.FirstName += "1"

	assert.Nil(t, dao.UpdateColumns(c, &User, User.ColumnFirstName))

	assert.Nil(t, dao.ByID(c, &User, User.GetID()))
	assert.True(t, User.FirstName[len(User.FirstName)-1] == '1')
	assert.True(t, User.UpdatedAt.After(updatedAt))

	assert.Nil(t, dao.ByID(c, &User, User.GetID()))
	updatedAt = User.UpdatedAt

	////////////////////////////////////////////////////////////////////////////////////////////////////

	assert.Nil(t, dao.DeprecatedUpdateColumns(c, &User, map[string]interface{}{
		models.Columns.User.FirstName: "aaa",
	}))

	assert.Nil(t, dao.ByID(c, &User, User.GetID()))
	assert.True(t, User.UpdatedAt.After(updatedAt))

	assert.Nil(t, dao.ByID(c, &User, User.GetID()))
	updatedAt = User.UpdatedAt

	////////////////////////////////////////////////////////////////////////////////////////////////////

	assert.Nil(t, dao.Delete(c, &User))

	assert.True(t, dao.IsRecordNotFound(dao.ByID(c, &User, User.GetID())))

	assert.Nil(t, dao.GetDeletedByID(c, &User, User.GetID()))
	assert.True(t, User.UpdatedAt.After(updatedAt))
}

func TestDeleted(t *testing.T) {
	t.Parallel()

	UserWithHooks := models.UserWithHooks{FirstName: utils.RandomAlphanumeric(10)}
	assert.Nil(t, dao.Create(c, &UserWithHooks))
	assert.Nil(t, dao.ByID(c, &models.UserWithHooks{}, UserWithHooks.GetID()))
	assert.Nil(t, dao.Delete(c, &UserWithHooks))
	assert.NotNil(t, dao.ByID(c, &models.UserWithHooks{}, UserWithHooks.GetID()))
	assert.True(t, IsRecordNotFound(dao.ByID(c, &models.UserWithHooks{}, UserWithHooks.GetID())))

	var zeroTime time.Time
	var UserWithHookss []models.UserWithHooks
	assert.Nil(t, dao.Query(c).Filter(models.Columns.UserWithHooks.DeletedAt, "!=", zeroTime).All(&UserWithHookss))
	assert.Empty(t, UserWithHookss)

	assert.Nil(t, dao.Query(c).IncludeDeleted().Filter(models.Columns.UserWithHooks.DeletedAt, "!=", zeroTime).All(&UserWithHookss))
	assert.NotEmpty(t, UserWithHookss)

	for _, t2 := range UserWithHookss {
		if t2.FirstName == UserWithHooks.FirstName {
			return
		}
	}
	assert.Fail(t, "not found: "+UserWithHooks.FirstName)
}

func TestHooks(t *testing.T) {
	t.Parallel()
	UserWithHooks := models.UserWithHooks{FirstName: "aaa", LastName: "bbb"}
	assert.Nil(t, dao.Create(c, &UserWithHooks))
	assert.Equal(t, "aaa bbb", UserWithHooks.Name)
	{
		var UserWithHooks2 models.UserWithHooks
		assert.Nil(t, dao.ByID(c, &UserWithHooks2, UserWithHooks.GetID()))
		assert.Equal(t, UserWithHooks.Name, UserWithHooks2.Name)
	}
	{
		UserWithHooks.FirstName = "xxx"
		assert.Nil(t, dao.CreateOrUpdate(c, &UserWithHooks))
		assert.Equal(t, "xxx bbb", UserWithHooks.Name)

		var UserWithHooks2 models.UserWithHooks
		assert.Nil(t, dao.ByID(c, &UserWithHooks2, UserWithHooks.GetID()))
		assert.Equal(t, UserWithHooks.Name, UserWithHooks2.Name)
	}
	{
		// With maps
		UserWithHooks.FirstName = "yyy"
		assert.Nil(t, dao.UpdateColumns(c, &UserWithHooks, UserWithHooks.ColumnFirstName))
		var UserWithHooks2 models.UserWithHooks
		assert.Nil(t, dao.ByID(c, &UserWithHooks2, UserWithHooks.GetID()))
		fmt.Printf("%#v\n", UserWithHooks2)
		assert.Equal(t, "yyy bbb", UserWithHooks2.Name)
	}
	{
		// With maps
		UserWithHooks.FirstName = "zzz"
		assert.Nil(t, dao.UpdateColumns(c, &UserWithHooks, UserWithHooks.ColumnFirstName))
		var UserWithHooks2 models.UserWithHooks
		assert.Nil(t, dao.ByID(c, &UserWithHooks2, UserWithHooks.GetID()))
		assert.Equal(t, "zzz bbb", UserWithHooks2.Name)
	}
}

func TestNotFound(t *testing.T) {
	t.Parallel()
	botStartCode := utils.RandomAlphanumeric(20)
	{
		var User models.User
		err := dao.Query(c).Filter("xxxxxx_"+models.Columns.User.FirstName, "=", botStartCode).First(&User)
		assert.NotNil(t, err)
		assert.False(t, gorm.IsRecordNotFoundError(err))
	}
	{
		var User models.User
		err := dao.Query(c).Filter(models.Columns.User.FirstName, "=", botStartCode).First(&User)
		assert.NotNil(t, err)
		assert.True(t, IsRecordNotFound(err))
	}
}

func TestUserUpdate(t *testing.T) {
	t.Parallel()
	code := models.Something{}

	assert.Nil(t, dao.Create(c, &code))

	botStartCode := utils.RandomAlphanumeric(20)
	code.Code = botStartCode
	assert.Nil(t, dao.UpdateColumns(c, &code, code.ColumnCode))
	assert.Equal(t, botStartCode, code.Code)

	var User2 models.Something
	assert.Nil(t, dao.Query(c).Filter("id", "=", code.ID).First(&User2))
	assert.Equal(t, botStartCode, User2.Code)
}

func TestRecordNotFound(t *testing.T) {
	t.Parallel()
	var User models.User
	err := dao.Query(context.Background()).Filter(models.Columns.User.Birth, "=", 1).First(&User)
	assert.NotNil(t, err)
	assert.True(t, IsRecordNotFound(err), "err=%#v", err)
}

func TestIncreaseColumnValue(t *testing.T) {
	t.Parallel()
	unread := 10
	incr := -17
	User := models.User{
		Email: fmt.Sprintf("%s@jkljkl.com", utils.RandomAlphanumeric(10)),
		Phone: utils.RandomAlphanumeric(20),
		Code:  unread,
	}
	assert.Nil(t, dao.Create(c, &User))
	assert.Nil(t, dao.IncrColumn(c, &User, models.Columns.User.Code, float64(incr)))

	assert.Nil(t, dao.ByID(c, &User, User.GetID()))
	if !assert.Equal(t, -7, User.Code) {
		t.FailNow()
	}

	var User2 models.User
	assert.Nil(t, dao.ByID(c, &User2, User.GetID()))

	assert.Equal(t, unread+incr, User2.Code)
}

/*
func TestExplicitSetInInsert(t *testing.T) {
	t.Parallel()
	var id uuid.UUID
	var title = utils.RandomAlphanumeric(20)
	{
		var User models.UserScript
		User.GenerateID()
		assert.Nil(t, dao.gormDb.Create(&User).Set(models.Columns.UserScript.Title, title).Error)
	}
	{
		var User models.UserScript
		assert.Nil(t, dao.ByID(c, &User, id))
		assert.Equal(t, title, User.Title)
	}
}
*/
