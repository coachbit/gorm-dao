package dao

import (
	"fmt"
	"github.com/teachme2/gorm-dao/models"
	"os"
	"testing"
	"time"

	// _ "github.com/jinzhu/gorm/dialects/mysql"
	// _ "github.com/jinzhu/gorm/dialects/postgres"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	// _ "github.com/jinzhu/gorm/dialects/mssql"
)

var (
	dao *Dao
)

func TestMain(m *testing.M) {
	d, err := NewDebug("sqlite3", fmt.Sprintf("tmp/db_test_%d.db", time.Now().Unix()), models.AllTestModels...)
	if err != nil {
		panic(err)
	}
	dao = d
	dao.Debug = true
	os.Exit(m.Run())
}
