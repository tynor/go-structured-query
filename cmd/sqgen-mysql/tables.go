package main

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"unicode"

	"github.com/spf13/cobra"
)

// Field Types
const (
	FieldTypeBoolean = "sq.BooleanField"
	FieldTypeJSON    = "sq.JSONField"
	FieldTypeNumber  = "sq.NumberField"
	FieldTypeString  = "sq.StringField"
	FieldTypeTime    = "sq.TimeField"
	FieldTypeEnum    = "sq.EnumField"
	FieldTypeBinary  = "sq.BinaryField"

	FieldConstructorBoolean = "sq.NewBooleanField"
	FieldConstructorJSON    = "sq.NewJSONField"
	FieldConstructorNumber  = "sq.NewNumberField"
	FieldConstructorString  = "sq.NewStringField"
	FieldConstructorTime    = "sq.NewTimeField"
	FieldConstructorEnum    = "sq.NewEnumField"
	FieldConstructorBinary  = "sq.NewBinaryField"
)

var tablesCmd = &cobra.Command{
	Use:   "tables",
	Short: "Generate tables from the database",
	RunE:  tablesRun,
}

var tablesTemplate = `// Code generated by 'sqgen-mysql tables'; DO NOT EDIT.
package {{$.PackageName}}

import (
	{{- range $_, $import := $.Imports}}
	{{$import}}
	{{- end}}
)
{{- range $_, $table := $.Tables}}
{{template "table_struct_definition" $table}}
{{template "table_constructor" $table}}
{{template "table_as" $table}}
{{- end}}

{{- define "table_struct_definition"}}
{{- with $table := .}}
{{- if eq $table.RawType "BASE TABLE"}}
// {{$table.StructName.Export}} references the {{$table.Schema}}.{{$table.Name.QuoteSpace}} table.
{{- else if eq $table.RawType "VIEW"}}
// {{$table.StructName.Export}} references the {{$table.Schema}}.{{$table.Name.QuoteSpace}} view.
{{- end}}
type {{$table.StructName.Export}} struct {
	*sq.TableInfo
	{{- range $_, $field := $table.Fields}}
	{{$field.Name.Export}} {{$field.Type}}
	{{- end}}
}
{{- end}}
{{- end}}

{{- define "table_constructor"}}
{{- with $table := .}}
{{- if eq $table.RawType "BASE TABLE"}}
// {{$table.Constructor.Export}} creates an instance of the {{$table.Schema}}.{{$table.Name.QuoteSpace}} table.
{{- else if eq $table.RawType "VIEW"}}
// {{$table.Constructor.Export}} creates an instance of the {{$table.Schema}}.{{$table.Name.QuoteSpace}} view.
{{- end}}
func {{$table.Constructor.Export}}() {{$table.StructName.Export}} {
	tbl := {{$table.StructName.Export}}{TableInfo: &sq.TableInfo{
		Schema: "{{$table.Schema}}",
		Name: "{{$table.Name}}",
	},}
	{{- range $_, $field := $table.Fields}}
	tbl.{{$field.Name.Export}} = {{$field.Constructor}}("{{$field.Name}}", tbl.TableInfo)
	{{- end}}
	return tbl
}
{{- end}}
{{- end}}

{{- define "table_as"}}
{{- with $table := .}}
{{- if eq $table.RawType "BASE TABLE"}}
// As modifies the alias of the underlying table.
{{- else if eq $table.RawType "VIEW"}}
// As modifies the alias of the underlying view.
{{- end}}
func (tbl {{$table.StructName.Export}}) As(alias string) {{$table.StructName.Export}} {
	tbl.TableInfo.Alias = alias
	return tbl
}
{{- end}}
{{- end}}`

// Table represents a database table
type Table struct {
	Schema      string
	Name        String
	StructName  String
	RawType     string
	Constructor String
	Fields      []TableField
}

// TableField represents a field in a database table
type TableField struct {
	Name        String
	RawType     string
	RawTypeEx   string
	Type        string
	Constructor string
}

// String is a custom string type
type String string

// String implements fmt.Stringer
func (s String) String() string {
	return string(s)
}

// Export will make the string follow Go's export rules.
func (s String) Export() String {
	str := strings.TrimPrefix(string(s), "_")
	str = strings.ReplaceAll(str, " ", "_")
	str = strings.ToUpper(str)
	return String(str)
}

// QuoteSpace will quote the string if it contains spaces.
func (s String) QuoteSpace() String {
	if strings.Contains(string(s), " ") {
		return String("`" + string(s) + "`")
	}
	return s
}

func init() {
	sqgenCmd.AddCommand(tablesCmd)
	// Initialise flags
	tablesCmd.Flags().String("database", "", "(required) Database URL")
	tablesCmd.Flags().String("directory", filepath.Join(currdir, "tables"), "(optional) Directory to place the generated file. Can be absolute or relative filepath")
	tablesCmd.Flags().Bool("dryrun", false, "(optional) Print the list of tables to be generated without generating the file")
	tablesCmd.Flags().String("file", "tables.go", "(optional) Name of the file to be generated. If file already exists, -overwrite flag must be specified to overwrite the file")
	tablesCmd.Flags().Bool("overwrite", false, "(optional) Overwrite any files that already exist")
	tablesCmd.Flags().String("pkg", "tables", "(optional) Package name of the file to be generated")
	tablesCmd.Flags().String("schemas", "", "(required) A comma separated list of schemas (databases) that you want to generate tables for. In MySQL this is usually the database name you are using. Please don't include any spaces")
	// Mark required flags
	cobra.MarkFlagRequired(tablesCmd.LocalFlags(), "database")
	cobra.MarkFlagRequired(tablesCmd.LocalFlags(), "schemas")
}

// tablesRun is the main function to be run with the `sqgen-mysql tables`
// command
func tablesRun(cmd *cobra.Command, args []string) error {
	// Prep flag values
	database, _ := cmd.Flags().GetString("database")
	directory, _ := cmd.Flags().GetString("directory")
	dryrun, _ := cmd.Flags().GetBool("dryrun")
	file, _ := cmd.Flags().GetString("file")
	overwrite, _ := cmd.Flags().GetBool("overwrite")
	pkg, _ := cmd.Flags().GetString("pkg")
	schemasStr, _ := cmd.Flags().GetString("schemas")
	schemas := strings.FieldsFunc(schemasStr, func(r rune) bool { return r == ',' || unicode.IsSpace(r) })
	if len(schemas) == 0 {
		return fmt.Errorf("'%s' is not a valid comma separated list of schemas", schemasStr)
	}
	if !strings.HasSuffix(file, ".go") {
		file = file + ".go"
	}

	// Setup database
	db, err := sql.Open("mysql", database)
	if err != nil {
		return wrap(err)
	}
	err = db.Ping()
	if err != nil {
		return fmt.Errorf("Could not ping the database, is the database reachable via " + database + "? " + err.Error())
	}

	// Get list of tables from database
	tables, err := getTables(db, database, schemas)
	if err != nil {
		return wrap(err)
	}
	if dryrun {
		for _, table := range tables {
			fmt.Println(table)
		}
		return nil
	}

	asboluteFilePath := filepath.Join(directory, file)
	if _, err := os.Stat(asboluteFilePath); err == nil && !overwrite {
		return fmt.Errorf("%s already exists. If you wish to overwrite it, provide the --overwrite flag", asboluteFilePath)
	}

	// Write list of tables into file
	err = writeTablesToFile(tables, directory, file, pkg)
	if err != nil {
		return wrap(err)
	}
	fmt.Println("[RESULT] "+strconv.Itoa(len(tables)), "tables written into", filepath.Join(directory, file))
	return nil
}

func getTables(db *sql.DB, databaseURL string, schemas []string) ([]Table, error) {
	// Prepare the query and args
	query := "SELECT t.table_type, c.table_schema, c.table_name, c.column_name, c.data_type, c.column_type" +
		" FROM information_schema.tables AS t" +
		" JOIN information_schema.columns AS c USING (table_schema, table_name)" +
		" WHERE table_schema IN (?" + strings.Repeat(", ?", len(schemas)-1) + ")" +
		" ORDER BY c.table_schema, t.table_type, c.table_name, c.column_name"
	args := make([]interface{}, len(schemas))
	for i := range schemas {
		args[i] = schemas[i]
	}

	// Query the database and aggregate the results into a []Table slice
	rows, err := db.Query(query, args...)
	fmt.Println(query, args)
	if err != nil {
		return nil, wrap(err)
	}
	defer rows.Close()
	var tableIndices = make(map[string]int)
	var tables []Table
	for rows.Next() {
		var tableType, tableSchema, tableName, columnName, columnType, columnTypeEx string
		err := rows.Scan(&tableType, &tableSchema, &tableName, &columnName, &columnType, &columnTypeEx)
		if err != nil {
			return tables, err
		}
		fullTableName := tableSchema + "." + tableName
		if _, ok := tableIndices[fullTableName]; !ok {
			// create new table
			table := Table{
				Schema:  tableSchema,
				Name:    String(tableName),
				RawType: tableType,
			}
			tables = append(tables, table)
			tableIndices[fullTableName] = len(tables) - 1
		}
		// create new field
		field := TableField{
			Name:      String(columnName),
			RawType:   columnType,
			RawTypeEx: columnTypeEx,
		}
		index := tableIndices[fullTableName]
		tables[index].Fields = append(tables[index].Fields, field)
	}

	// Do postprocessing on the tables to fill in the struct names,
	// constructors, etc
	tables = processTables(tables)
	return tables, nil
}

func processTables(tables []Table) []Table {
	// tableNames keeps count of how many times a table name appears
	var tableNames = make(map[string]int)
	for i := range tables {
		tableNames[string(tables[i].Name)]++
	}
	for i := range tables {
		schema := string(tables[i].Schema)
		name := string(tables[i].Name)
		// Add struct type prefix to struct name. For a list of possible
		// RawTypes that can appear, consult this link (look for table_type):
		// https://www.mysqlql.org/docs/current/infoschema-tables.html
		tables[i].StructName = "TABLE_"
		if tables[i].RawType == "VIEW" {
			tables[i].StructName = "VIEW_"
		}
		// Add schema prefix to struct name and constructor if more than one
		// table share same name
		if tableNames[name] > 1 {
			tables[i].StructName += String(strings.ToUpper(schema + "__"))
			tables[i].Constructor += String(strings.ToUpper(schema + "__"))
		}
		tables[i].StructName += String(strings.ToUpper(name))
		tables[i].Constructor += String(strings.ToUpper(name))
		var field TableField
		var fields []TableField
		for j := range tables[i].Fields {
			field = tables[i].Fields[j].fillInTheBlanks() // process the field
			if field.Type == "" {
				fmt.Printf("Skipping %s.%s because type '%s' is unknown\n", tables[i].Name, field.Name, field.RawType)
				continue
			}
			fields = append(fields, field)
		}
		tables[i].Fields = fields
	}
	return tables
}

// fillInTheBlanks will fill in the .Type and .Constructor for a field based on
// the field's .RawType. For list of possible RawTypes that can appear, consult
// this link (Table 8.1): https://www.mysqlql.org/docs/current/datatype.html.
func (field TableField) fillInTheBlanks() TableField {
	// Boolean
	if field.RawTypeEx == "tinyint(1)" {
		field.Type = FieldTypeBoolean
		field.Constructor = FieldConstructorBoolean
		return field
	}

	// JSON
	if strings.HasPrefix(field.RawType, "json") {
		field.Type = FieldTypeJSON
		field.Constructor = FieldConstructorJSON
		return field
	}

	// Number
	switch field.RawType {
	case "decimal", "numeric", "float", "double": // float
		fallthrough
	case "integer", "int", "smallint", "tinyint", "mediumint", "bigint": // integer
		field.Type = FieldTypeNumber
		field.Constructor = FieldConstructorNumber
		return field
	}

	// String
	switch field.RawType {
	case "tinytext", "text", "mediumtext", "longtext", "char", "varchar":
		field.Type = FieldTypeString
		field.Constructor = FieldConstructorString
		return field
	}

	// Time
	switch field.RawType {
	case "date", "time", "datetime", "timestamp":
		field.Type = FieldTypeTime
		field.Constructor = FieldConstructorTime
		return field
	}

	// Enum
	switch field.RawType {
	case "enum":
		field.Type = FieldTypeEnum
		field.Constructor = FieldConstructorEnum
		return field
	}

	// Blob
	switch field.RawType {
	case "binary", "varbinary", "tinyblob", "blob", "mediumblob", "longblob":
		field.Type = FieldTypeBinary
		field.Constructor = FieldConstructorBinary
		return field
	}

	return field
}

// writeTablesToFile will write the tables into a file specified by
// filepath.Join(directory, file).
func writeTablesToFile(tables []Table, directory, file, packageName string) error {
	err := os.MkdirAll(directory, 0755)
	if err != nil {
		return fmt.Errorf("Could not create directory %s: %w", directory, err)
	}
	filename := filepath.Join(directory, file)
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	t, err := template.New("").Parse(tablesTemplate)
	if err != nil {
		return err
	}
	data := struct {
		PackageName string
		Imports     []string
		Tables      []Table
	}{
		PackageName: packageName,
		Imports: []string{
			`sq "github.com/bokwoon95/go-structured-query/mysql"`,
		},
		Tables: tables,
	}
	err = t.Execute(f, data)
	if err != nil {
		return err
	}
	if _, err := exec.LookPath("goimports"); err == nil {
		_ = exec.Command("goimports", "-w", filename).Run()
	} else if _, err := exec.LookPath("gofmt"); err == nil {
		_ = exec.Command("gofmt", "-w", filename).Run()
	}
	return nil
}

// String implements the fmt.Stringer interface.
func (table Table) String() string {
	var output string
	if table.Constructor != "" && table.StructName != "" {
		output += fmt.Sprintf("%s.%s => func %s() %s\n", table.Schema, table.Name, table.Constructor, table.StructName)
	} else {
		output += fmt.Sprintf("%s.%s\n", table.Schema, table.Name)
	}
	for _, field := range table.Fields {
		if field.Constructor != "" && field.Type != "" {
			output += fmt.Sprintf("    %s: %s => %s\n", field.Name, field.RawType, field.Type)
		} else {
			output += fmt.Sprintf("    %s: %s\n", field.Name, field.RawType)
		}
	}
	return output
}
