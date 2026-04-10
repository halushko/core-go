package sqlite

import (
	external "database/sql"
	"regexp"
)

const dbDefaultPath = "/data/sqlite"

var macroRegexp = regexp.MustCompile(`#\$([a-zA-Z_][a-zA-Z0-9_]*)\$#`)

type Client struct {
	db *external.DB
}

type ColumnType string

const (
	TypeInteger  ColumnType = "INTEGER"
	TypeText     ColumnType = "TEXT"
	TypeReal     ColumnType = "REAL"
	TypeBlob     ColumnType = "BLOB"
	TypeDatetime ColumnType = "DATETIME"
)

type UniqueConstraint struct {
	Name    string
	Columns []string
}

type CheckConstraint struct {
	Name string
	Expr string
}

type ForeignKey struct {
	Name              string
	Columns           []string
	ReferenceTable    string
	ReferenceColumns  []string
	OnDelete          string
	OnUpdate          string
	Deferrable        *bool
	InitiallyDeferred *bool
}

type Index struct {
	Name    string
	Unique  bool
	Columns []string
	Where   string
}

type Column struct {
	Name          string
	Type          ColumnType
	PrimaryKey    *bool
	NotNull       *bool
	Unique        *bool
	GeneratedExpr *string
	Stored        *bool
	Default       *string
}

type Table struct {
	Name              string
	Columns           []Column
	UniqueConstraints []UniqueConstraint
	Checks            []CheckConstraint
	ForeignKeys       []ForeignKey
	Indexes           []Index
}
