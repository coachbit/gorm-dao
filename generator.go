package dao

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/jinzhu/gorm"
	"github.com/tkrajina/go-reflector/reflector"
)

func GenerateColumnNames(targetFile string, models ...interface{}) error {
	fmt.Printf("Generating %s from %#v\n", targetFile, models)
	f, err := os.Create(targetFile)
	if err != nil {
		return err
	}

	f.WriteString("package models\n")
	f.WriteString("/* Run 'go generate' to regenerate this file */\n\n")

	var declarationCode bytes.Buffer
	var initializationCode bytes.Buffer
	var code bytes.Buffer

	//f.WriteString("type FieldName string\n")
	for _, model := range models {
		obj := reflector.New(model)
		name := obj.Type().Name()
		if obj.IsPtr() {
			name = obj.Type().Elem().Name()
		}
		fmt.Println("Generating", name)
		declarationCode.WriteString(fmt.Sprintf("%s struct {\n", name))
		for _, field := range obj.FieldsFlattened() {
			gormTag, _ := field.Tag("gorm")
			if gormTag != "-" {
				gormFieldName := gorm.ToDBName(field.Name())
				parts := strings.Split(gormTag, ";")
				for _, part := range parts {
					parts2 := strings.Split(part, ":")
					if len(parts2) >= 2 && strings.ToLower(parts2[0]) == "column" {
						gormFieldName = parts2[1]
					}
				}
				declarationCode.WriteString(fmt.Sprintf("%s string\n", field.Name()))
				initializationCode.WriteString(fmt.Sprintf("Columns.%s.%s = \"%s\"\n", name, field.Name(), gormFieldName))

				modelName := strings.Split(obj.Type().String(), ".")[1]
				code.WriteString(fmt.Sprintf(`func (m *%s) Column%s() (string, interface{}) { return "%s", m.%s }
`, modelName, field.Name(), gormFieldName, field.Name()))
			}
		}
		declarationCode.WriteString("}\n")
	}
	f.WriteString("var Columns struct {\n")
	f.WriteString(declarationCode.String())
	f.WriteString("}\n")
	f.WriteString("// nolint\n")
	f.WriteString("func init() {\n")
	f.WriteString(initializationCode.String())
	f.WriteString("}\n")
	f.WriteString(code.String())

	return nil
}
