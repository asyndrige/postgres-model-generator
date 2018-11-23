package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"text/template"
	"unicode"

	_ "github.com/lib/pq"
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

type DBTables map[string][]DBColumn

type DBColumn struct {
	ColumnName             string
	OrdinalPosition        int
	ColumnDefault          *string
	IsNullable             bool
	DataType               string
	UDTName                string
	CharacterMaximumLength *int
	CharacterOctetLength   *int
	NumericPrecision       *int
}

func (tables *DBTables) AsModels() []Model {
	models := make([]Model, 0, len(*tables))

	for name, columns := range *tables {
		modelFields := make([]Field, 0)

		for _, col := range columns {
			modelFields = append(modelFields, col.AsField())
		}

		models = append(models, Model{
			Name:   name,
			Fields: modelFields,
		})
	}

	fmt.Printf("+++ %#v\n\n\n", models[0])

	return models
}

type Model struct {
	Name   string
	Fields []Field
}

type Field struct {
	Name string
	Type string
	Tag  string
}

func (col *DBColumn) AsField() Field {
	var (
		tag       string
		fieldType string
		f         Field
	)
	if col.IsNullable {
		tag = fmt.Sprintf(`sql:"%s"`, col.ColumnName)
		fieldType = fmt.Sprintf("*%s", col.DataType)
	} else {
		tag = fmt.Sprintf(`sql:"%s,notnull"`, col.ColumnName)
		fieldType = col.DataType
	}
	f.Tag = tag
	f.Type = fieldType
	f.Name = toCamelCase(col.ColumnName)

	return f
}

type DB struct {
	*sql.DB
}

func MustNewDB(connStr string) *DB {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	return &DB{db}
}

func (db *DB) GetAllTables() DBTables {
	q := `
SELECT 
	c.table_name, c.column_name, c.ordinal_position, c.column_default, bool(c.is_nullable), c.data_type, c.udt_name, 
	c.character_maximum_length, c.character_octet_length, c.numeric_precision
FROM 
	information_schema.columns AS c 
JOIN
	information_schema.tables as t
ON
	t.table_name = c.table_name
WHERE 
	t.table_schema = 'public' AND t.table_type = 'BASE TABLE'
ORDER BY 
	c.table_name;
`
	tables := make(DBTables)
	rows, err := db.Query(q)
	if err != nil {
		log.Fatal(err)
	} else {
		defer rows.Close()
	}

	for rows.Next() {
		var (
			tableName string
			col       = new(DBColumn)
		)
		if err := rows.Scan(
			&tableName, &col.ColumnName, &col.OrdinalPosition, &col.ColumnDefault, &col.IsNullable, &col.DataType,
			&col.UDTName, &col.CharacterMaximumLength, &col.CharacterOctetLength, &col.NumericPrecision,
		); err != nil {
			log.Print(err)
			continue
		}
		if _, ok := tables[tableName]; !ok {
			tables[tableName] = make([]DBColumn, 0, 1)
		}
		tables[tableName] = append(tables[tableName], *col)
	}

	return tables
}

func main() {
	var (
		separateFiles bool
		configPath    string
		username      string
		password      string
		database      string
		sslMode       string
	)
	flag.BoolVar(&separateFiles, "sf", false, "generate separate file for each model")
	flag.StringVar(&username, "u", "test", "username")
	flag.StringVar(&password, "p", "test", "password")
	flag.StringVar(&database, "d", "test", "database")
	flag.StringVar(&sslMode, "ssl", "disable", "ssl mode")
	flag.StringVar(&configPath, "c", "config", "path to config file")
	flag.Parse()

	db := MustNewDB(fmt.Sprintf(
		"user=%s password=%s database=%s sslmode=%s",
		username, password, database, sslMode,
	))

	tables := db.GetAllTables()
	tables.AsModels()

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

// utils
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
