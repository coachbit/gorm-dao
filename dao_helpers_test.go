package dao

import (
	"context"
	"fmt"
	"testing"

	"github.com/teachme2/gorm-dao/models"
	"github.com/teachme2/gorm-dao/utils"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
)

func TestOrderBy(t *testing.T) {
	t.Parallel()
	c := context.Background()
	n := 20
	rand := utils.RandomNumeric(10)
	var insertedUsers []models.User
	for i := 0; i < n; i++ {
		User := models.User{
			FirstName: rand,
			Email:     fmt.Sprintf("%05d-%s@jkljkl.com", i, rand),
			Phone:     utils.RandomNumeric(10),
		}
		assert.Nil(t, dao.Create(c, &User))
		insertedUsers = append(insertedUsers, User)
	}
	{
		var Users []models.User
		err := dao.Query(c).
			Filter(models.Columns.User.FirstName, "=", rand).
			OrderBy(models.Columns.User.CreatedAt, "asc").
			All(&Users)
		assert.Nil(t, err)
		assert.Equal(t, len(Users), len(insertedUsers))
		assert.Equal(t, Users[0].FirstName, insertedUsers[0].FirstName)
		assert.Equal(t, Users[len(Users)-1].FirstName, insertedUsers[len(insertedUsers)-1].FirstName)
	}
	if t.Failed() {
		t.FailNow()
	}
	{
		var Users []models.User
		err := dao.Query(c).
			Filter(models.Columns.User.FirstName, "=", rand).
			OrderBy(models.Columns.User.CreatedAt, "desc").
			All(&Users)
		assert.Nil(t, err)
		assert.Equal(t, len(Users), len(insertedUsers))
		assert.Equal(t, Users[0].FirstName, insertedUsers[len(insertedUsers)-1].FirstName)
		assert.Equal(t, Users[len(Users)-1].FirstName, insertedUsers[0].FirstName)
	}
	if t.Failed() {
		t.FailNow()
	}
	{
		var User models.User
		err := dao.Query(c).
			Filter(models.Columns.User.FirstName, "=", rand).
			OrderBy(models.Columns.User.CreatedAt, "desc").
			First(&User)
		assert.Nil(t, err)
		assert.Equal(t, User.FirstName, insertedUsers[len(insertedUsers)-1].FirstName)
	}
	if t.Failed() {
		t.FailNow()
	}
	{
		var User models.User
		err := dao.Query(c).
			Filter(models.Columns.User.FirstName, "=", rand).
			OrderBy(models.Columns.User.CreatedAt, "asc").
			First(&User)
		assert.Nil(t, err)
		assert.Equal(t, User.FirstName, insertedUsers[0].FirstName)
	}
}

func TestCount(t *testing.T) {
	t.Parallel()
	c := context.Background()
	n := 77
	rand := utils.RandomNumeric(10)
	for i := 0; i < n; i++ {
		User := models.User{
			FirstName: rand,
			Email:     fmt.Sprintf("%d@%s.com", i, rand),
			Phone:     utils.RandomNumeric(10),
		}
		assert.Nil(t, dao.Create(c, &User))
	}
	count, err := dao.Query(c).
		Filter(models.Columns.User.FirstName, "=", rand).
		Count(new(models.User))
	assert.Nil(t, err)
	assert.Equal(t, n, count)
}

func testQuery(t *testing.T, n int, botStartCode string) {
	c := context.Background()
	firstName := ""
	firstEmail := ""
	for i := 0; i < n; i++ {
		User := models.User{
			FirstName: botStartCode,
			Email:     fmt.Sprintf("%s@jkljkl.com", utils.RandomAlphanumeric(10)),
			Phone:     utils.RandomNumeric(10),
		}
		if i == 0 {
			firstName = User.FirstName
			firstEmail = User.Email
		}
		assert.Nil(t, dao.Create(c, &User))
	}

	{
		q := dao.Query(c)
		var User models.User
		assert.True(t, uuid.Nil == User.GetID())
		assert.Nil(t, q.Filter(models.Columns.User.FirstName, "=", firstName).First(&User))
		assert.False(t, uuid.Nil == User.GetID())
		assert.NotEmpty(t, User.FirstName)
		assert.NotEmpty(t, User.Email)
	}
	{
		q := dao.Query(c)
		var User models.User
		assert.True(t, uuid.Nil == User.GetID())
		err := q.Filter(models.Columns.User.FirstName, "=", firstName).Filter(models.Columns.User.Email, "=", "???").First(&User)
		assert.NotNil(t, err)
		assert.True(t, IsRecordNotFound(err))
		assert.Empty(t, User.FirstName)
		assert.Empty(t, User.Email)
	}
	{
		q := dao.Query(c)
		var User models.User
		assert.True(t, uuid.Nil == User.GetID())
		err := q.Filter(models.Columns.User.FirstName, "=", firstName).Filter(models.Columns.User.Email, "=", firstEmail).First(&User)
		assert.Nil(t, err)
		assert.False(t, IsRecordNotFound(err))
		assert.Equal(t, firstEmail, User.Email)
		assert.Equal(t, firstName, User.FirstName)
	}
}

func TestQuery10(t *testing.T) {
	t.Parallel()
	c := context.Background()
	botStartCode := utils.RandomAlphanumeric(200)
	n := 10
	testQuery(t, n, botStartCode)
	if t.Failed() {
		t.FailNow()
	}
	{
		q := dao.Query(c)
		var Users []models.User
		err := q.Filter(models.Columns.User.FirstName, "=", botStartCode).All(&Users)
		assert.Nil(t, err)
		assert.Equal(t, n, len(Users))
	}
}

func TestQuery100(t *testing.T) {
	t.Parallel()
	c := context.Background()
	botStartCode := utils.RandomAlphanumeric(200)
	n := 100
	testQuery(t, n, botStartCode)
	if t.Failed() {
		t.FailNow()
	}
	{
		q := dao.Query(c)
		var Users []models.User
		err := q.Filter(models.Columns.User.FirstName, "=", botStartCode).
			WithDefaultPageSize().
			All(&Users)
		assert.Nil(t, err)
		assert.Equal(t, defaultPageSize, len(Users))
	}
	{
		q := dao.Query(c)
		var Users []models.User
		err := q.Filter(models.Columns.User.FirstName, "=", botStartCode).
			WithPageSize(n * 2).
			All(&Users)
		assert.Nil(t, err)
		assert.Equal(t, n, len(Users))
	}
}
