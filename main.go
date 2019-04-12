package main

import (
	"bufio"
	"bytes"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"log"
	"os"
	"sort"
	"strings"
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
	_ = time.Time{}
)
`

	modelTpl = "type {{.Name}} struct {\ntableName struct{} `sql:\"{{.TableName}}\"`\n{{range .Fields}}\t{{.Name}} {{.Type}} `{{.Tag}}`\n{{end}} }\n\n"
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
	typer := NewTypesMapping()

	for name, columns := range *tables {
		modelFields := make([]Field, 0, len(columns))

		sort.Slice(columns, func(i, j int) bool {
			return columns[i].OrdinalPosition < columns[j].OrdinalPosition
		})

		for _, col := range columns {
			modelFields = append(modelFields, col.AsField(typer))
		}

		models = append(models, Model{
			Name:      toCamelCase(name),
			TableName: name,
			Fields:    modelFields,
		})
	}

	return models
}

type Model struct {
	Name      string
	TableName string
	Fields    []Field
}

type Field struct {
	Name string
	Type string
	Tag  string
}

func (col *DBColumn) AsField(typer Typer) Field {
	var (
		tag       string
		fieldType string
		f         Field
	)

	if col.IsNullable {
		tag = fmt.Sprintf(`sql:"%s"`, col.ColumnName)
		t, err := typer.GetType(col.UDTName)
		if err != nil {
			panic(err)
		}
		fieldType = fmt.Sprintf("*%s", t)
	} else {
		tag = fmt.Sprintf(`sql:"%s,notnull"`, col.ColumnName)
		t, err := typer.GetType(col.UDTName)
		if err != nil {
			panic(err)
		}
		fieldType = t
	}
	f.Tag = tag
	f.Type = fieldType
	f.Name = toCamelCase(col.ColumnName)

	return f
}

type TypesMapping struct {
	SQLTypes map[string][]string
}

type Typer interface {
	GetType(string) (string, error)
}

func NewTypesMapping() *TypesMapping {
	return &TypesMapping{
		map[string][]string{
			"bool":   {"bool"},
			"string": {"varchar", "text", "uuid"},
			"int":    {"int2", "int4", "int8"},
			// "int64":       {"bigint"},
			"time.Time":   {"timestamp", "date"},
			"interface{}": {"jsonb", "json"},
			"[]string":    {"_text", "_varchar", "tsvector"},
			"[]int":       {"_int2", "_int4", "_int8"},
		},
	}
}

func (tm *TypesMapping) GetType(sqlType string) (string, error) {
	for goType, sqlTypes := range tm.SQLTypes {
		for _, t := range sqlTypes {
			if t == sqlType {
				return goType, nil
			}
		}
	}
	return "", errors.New("type not detected")
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
	models := tables.AsModels()
	// model := models[0]
	modelFile, err := os.Create("models/models.go")
	if err != nil {
		log.Fatal(err)
	}
	var buffer bytes.Buffer
	buf := bufio.NewWriter(&buffer)
	buf.WriteString(headerTpl)
	buf.WriteString("\n")

	for _, model := range models {
		tmpl, err := template.New("test").Parse(modelTpl)
		if err != nil {
			log.Fatal(err)
		}

		if err := tmpl.Execute(buf, model); err != nil {
			log.Fatal(err)
		}
	}

	buf.Flush()
	content, err := format.Source(buffer.Bytes())
	if err != nil {
		log.Fatal(err)
	}
	modelFile.Write(content)
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
	out = strings.NewReplacer(
		"Id", "ID",
		"Uuid", "UUID",
		"Url", "URL",
		"Html", "HTML",
	).Replace(out)
	return
}
