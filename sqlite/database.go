package sqlite

import (
	external "database/sql"
)

const dbPath = "/data/sqlite"

type dbImpl struct {
	Sqlite *external.DB
}

type DBI interface {
	Select(table string, condition map[string]any, outputColumns ...string) ([]map[string]any, error)
	ExecSelect(query string, args ...any) ([]map[string]any, error)

	Insert(table string, row map[string]any) error
	InsertIfNotExists(table string, row map[string]any, onConflict ...string) error
	InsertOrUpdate(table string, row map[string]any, onConflict ...string) error
	ExecInsert(query string, args ...any) error
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
	Type          SQLiteType
	IsPrimaryKey  bool
	IsNullable    bool
	IsUnique      bool
	Default       string
	Autoincrement bool
}

type SQLiteType string

const (
	SQLiteInteger SQLiteType = "INTEGER"
	SQLiteReal    SQLiteType = "REAL"
	SQLiteText    SQLiteType = "TEXT"
	SQLiteBlob    SQLiteType = "BLOB"
)
