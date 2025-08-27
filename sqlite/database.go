package sqlite

import (
	external "database/sql"
)

const dbPath = "/data/sqlite"

type dbImpl struct {
	Sqlite *external.DB
}

type DBI interface {
	SelectByAll(table string, condition map[string]any, outputColumns ...string) ([]map[string]any, error)
	SelectByAny(table string, condition map[string]any, outputColumns ...string) ([]map[string]any, error)
	ExecSelect(query string, args ...any) ([]map[string]any, error)

	Insert(table string, row map[string]any) error
	InsertIfNotExists(table string, row map[string]any, onConflict ...string) error
	InsertOrUpdate(table string, row map[string]any, onConflict ...string) error
	ExecInsert(query string, args ...any) error

	Update(table string, set map[string]any, where map[string]any) error
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
	IsNullable    bool
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
