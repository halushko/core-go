package sqlite

import (
	external "database/sql"
)

const dbPath = "/data/sqlite"

//goland:noinspection ALL
const (
	sqlAND = "AND"
	sqlOR  = "OR"

	sqlSELECT = "SELECT"
	sqlUPDATE = "UPDATE"
	sqlINSERT = "INSERT INTO"

	sqlFROM  = "FROM"
	sqlWHERE = "WHERE"

	sqlORDER = "ORDER BY"

	sqlSET      = "SET"
	sqlVALUES   = "VALUES"
	sqlON       = "ON"
	sqlDO       = "DO"
	sqlNOTHING  = "NOTHING"
	sqlCONFLICT = "CONFLICT"

	sqlDoUpdateSet = sqlDO + " " + sqlUPDATE + " " + sqlSET
	sqlDoNothing   = sqlDO + " " + sqlNOTHING
	sqlCreateTable = "CREATE TABLE IF NOT EXISTS"
)

type dbImpl struct {
	Sqlite *external.DB
}

type DBI interface {
	ExecSelect(query string, args ...any) ([]map[string]any, error)
	SelectByAll(table string, condition map[string]any, outputColumns ...string) ([]map[string]any, error)
	SelectByAny(table string, condition map[string]any, outputColumns ...string) ([]map[string]any, error)

	ExecInsert(query string, args ...any) error
	Insert(table string, row map[string]any) error
	InsertIfNotExists(table string, row map[string]any, onConflict ...string) error
	InsertOrUpdate(table string, row map[string]any, onConflict ...string) error

	ExecUpdate(query string, args ...any) error
	UpdateByAll(table string, set map[string]any, where map[string]any) error
	UpdateByAny(table string, set map[string]any, where map[string]any) error
}

type DBInfo struct {
	Name    string
	Project string
	Tables  []Table
}
type Table struct {
	Name    string
	Columns []Column
}

type Column struct {
	Name          string
	Type          Type
	IsPrimaryKey  bool
	IsNotNull     bool
	IsUnique      bool
	Default       string
	Autoincrement bool
}

type Type string

//goland:noinspection GoUnusedConst
const (
	Integer Type = "INTEGER"
	Real    Type = "REAL"
	Text    Type = "TEXT"
	Blob    Type = "BLOB"
)
