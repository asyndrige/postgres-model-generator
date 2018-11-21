package main

import (
	"database/sql"
	"flag"
	"os"
	"unicode"

	"github.com/alecthomas/template"
)

var (
	_ = (*sql.DB)(nil)
)

const (
	headerTpl = `
package models

import (
	"time"
)

var (
	_ = time.Time
)
`

	modelTpl = `
type {{.Name}} struct {
	{{range .Fields}}{{.Name}} {{.Type}} {{.Tag}} {{end}}
}
`
)

type Model struct {
	Name   string
	Fields []Field
}

type Field struct {
	Name string
	Type string
	Tag  string
}

func main() {
	var (
		separateFiles bool
		configPath    string
	)
	flag.BoolVar(&separateFiles, "sf", false, "generate separate file for each model")
	flag.StringVar(&configPath, "c", "config", "path to config file")
	flag.Parse()

	model := Model{
		Name: toCamelCase("test_testing_model"),
		Fields: []Field{
			{
				Name: toCamelCase("field_id"),
				Type: "int64",
				Tag:  `sql:"field_id"`,
			},
		},
	}
	tmpl, err := template.New("test").Parse(modelTpl)
	if err != nil {
		panic(err)
	}

	if err := tmpl.Execute(os.Stdout, model); err != nil {
		panic(err)
	}
}

func toCamelCase(in string) (out string) {
	var toUpper bool
	for i, char := range in {
		if i == 0 || toUpper {
			out += string(unicode.ToUpper(char))
			toUpper = false
			continue
		}
		if char == '_' {
			toUpper = true
			continue
		}
		out += string(char)
	}
	return
}
