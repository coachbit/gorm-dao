package main

import (
	"github.com/teachme2/gorm-dao"
	"github.com/teachme2/gorm-dao/models"
)

func main() {
	if err := dao.GenerateColumnNames("models/columns_generated.go", models.AllTestModels...); err != nil {
		panic(err)
	}
}
