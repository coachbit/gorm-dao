package dao

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/jinzhu/gorm"
	"github.com/tkrajina/go-reflector/reflector"
)

func GenerateColumnNames(targetFile, tabledMd string, models ...interface{}) error {
	fmt.Printf("Generating %s from %#v\n", targetFile, models)
	f, err := os.Create(targetFile)
	if err != nil {
		return err
	}
	md, err := os.Create(tabledMd)
	if err != nil {
		return err
	}

	_, _ = f.WriteString("package appmodels\n")
	_, _ = f.WriteString("/* Run 'go generate' to regenerate this file */\n\n")

	_, _ = md.WriteString("# Database tables\n")
	_, _ = md.WriteString("Run 'go generate' to regenerate this file\n\n")

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
		_, _ = md.WriteString("# " + name + "\n\n")
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
				_, _ = md.WriteString("* " + field.Name() + "\n")
				declarationCode.WriteString(fmt.Sprintf("%s string\n", field.Name()))
				initializationCode.WriteString(fmt.Sprintf("Columns.%s.%s = \"%s\"\n", name, field.Name(), gormFieldName))

				modelName := strings.Split(obj.Type().String(), ".")[1]
				code.WriteString(fmt.Sprintf(`func (m *%s) Column%s() (string, interface{}) { return "%s", m.%s }
`, modelName, field.Name(), gormFieldName, field.Name()))
			}
		}
		declarationCode.WriteString("}\n")
		_, _ = md.WriteString("\n")
	}
	_, _ = f.WriteString("var Columns struct {\n")
	_, _ = f.WriteString(declarationCode.String())
	_, _ = f.WriteString("}\n")
	_, _ = f.WriteString("// nolint\n")
	_, _ = f.WriteString("func init() {\n")
	_, _ = f.WriteString(initializationCode.String())
	_, _ = f.WriteString("}\n")
	_, _ = f.WriteString(code.String())

	return nil
}
