package sqlite

import (
	"context"
	external "database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	_ "modernc.org/sqlite"
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

type Column struct {
	Name          string
	Type          ColumnType
	PrimaryKey    *bool
	NotNull       *bool
	Unique        *bool
	GeneratedExpr *string
	Stored        *bool
}

type Table struct {
	Name    string
	Columns []Column
}

type DBI interface {
	Select(query string, args ...any) ([]map[string]any, error)
	Execute(query string, args ...any) error
	ExecuteNamed(query string, params map[string]any) error
	ExecuteSqlFile(path string, args ...any) error
	ExecSelect(query string, args ...any) ([]map[string]any, error)
	ExecSelectSqlFile(path string, args ...any) ([]map[string]any, error)
	CreateTable(t Table) error
	DropTable(name string) error

	Close() error
}

//goland:noinspection GoUnusedExportedFunction
func Open(name string) (*Client, error) {
	if name == "" {
		return nil, errors.New("db name is empty")
	}

	if err := os.MkdirAll(getDbPath(), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir db path: %w", err)
	}

	dbFile := filepath.Join(getDbPath(), name+".sqlite")

	db, err := external.Open("sqlite", dbFile)
	if err != nil {
		return nil, fmt.Errorf("sql open: %w", err)
	}

	// Fail fast
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("db ping: %w", err)
	}

	return &Client{db: db}, nil
}

func (c *Client) Close() error {
	if c == nil || c.db == nil {
		return nil
	}
	return c.db.Close()
}

func (c *Client) Execute(query string, args ...any) error {
	if c == nil || c.db == nil {
		return errors.New("db client is nil")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := c.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("execute: %w", err)
	}
	return nil
}

func (c *Client) ExecuteNamed(query string, params map[string]any) error {
	if c == nil || c.db == nil {
		return errors.New("db client is nil")
	}

	compiledQuery, args, err := compileNamedQuery(query, params)
	if err != nil {
		return fmt.Errorf("compile named query: %w", err)
	}

	return c.Execute(compiledQuery, args...)
}

func (c *Client) ExecuteSqlFile(path string, args ...any) error {
	if c == nil || c.db == nil {
		return errors.New("db client is nil")
	}

	query, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	return c.Execute(string(query), args...)
}

func (c *Client) ExecuteSqlFileNamed(path string, params map[string]any) error {
	query, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	compiledQuery, args, err := compileNamedQuery(string(query), params)
	if err != nil {
		return fmt.Errorf("compile named query: %w", err)
	}

	return c.Execute(compiledQuery, args...)
}

func (c *Client) ExecSelect(query string, args ...any) ([]map[string]any, error) {
	if c == nil || c.db == nil {
		return nil, errors.New("db client is nil")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := c.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("select query: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("columns: %w", err)
	}

	out := make([]map[string]any, 0, 16)

	for rows.Next() {
		raw := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range raw {
			ptrs[i] = &raw[i]
		}

		if err := rows.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}

		m := make(map[string]any, len(cols))
		for i, name := range cols {
			v := raw[i]
			if b, ok := v.([]byte); ok {
				m[name] = string(b)
			} else {
				m[name] = v
			}
		}
		out = append(out, m)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows err: %w", err)
	}

	return out, nil
}

func (c *Client) ExecSelectSqlFile(path string, args ...any) ([]map[string]any, error) {
	query, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return c.ExecSelect(string(query), args...)
}

func (c *Client) CreateTable(t Table) error {
	var cols []string

	for _, c := range t.Columns {
		if err := c.Type.validateColumnType(); err != nil {
			return err
		}

		col := c.Name + " " + string(c.Type)

		if c.GeneratedExpr != nil {
			col += " GENERATED ALWAYS AS (" + *c.GeneratedExpr + ")"
			if c.Stored != nil && *c.Stored {
				col += " STORED"
			} else {
				col += " VIRTUAL"
			}
		} else {
			if c.PrimaryKey != nil && *c.PrimaryKey {
				col += " PRIMARY KEY"
			}
			if c.NotNull != nil && *c.NotNull {
				col += " NOT NULL"
			}
			if c.Unique != nil {
				col += " UNIQUE"
			}
		}

		cols = append(cols, col)
	}

	query := fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s (%s);`,
		t.Name,
		strings.Join(cols, ",\n"),
	)
	return c.Execute(query)
}

func (c *Client) DropTable(name string) error {
	if name == "" {
		return errors.New("table name is empty")
	}

	query := fmt.Sprintf("DROP TABLE IF EXISTS %q", name)
	return c.Execute(query)
}

func getDbPath() string {
	if path := os.Getenv("DB_PATH"); path != "" {
		return path
	}

	return dbDefaultPath
}

func (t ColumnType) validateColumnType() error {
	switch t {
	case TypeInteger, TypeText, TypeReal, TypeBlob, TypeDatetime:
		return nil
	default:
		return fmt.Errorf("invalid column type: %s", t)
	}
}

func compileNamedQuery(query string, params map[string]any) (string, []any, error) {
	if query == "" {
		return "", nil, fmt.Errorf("query is empty")
	}

	args := make([]any, 0)

	matches := macroRegexp.FindAllStringSubmatchIndex(query, -1)
	if len(matches) == 0 {
		return query, args, nil
	}

	result := make([]byte, 0, len(query))
	last := 0

	for _, m := range matches {
		fullStart, fullEnd := m[0], m[1]
		nameStart, nameEnd := m[2], m[3]

		result = append(result, query[last:fullStart]...)

		name := query[nameStart:nameEnd]
		value, ok := params[name]
		if !ok {
			return "", nil, fmt.Errorf("missing macro value for key %q", name)
		}

		result = append(result, '?')
		args = append(args, value)

		last = fullEnd
	}

	result = append(result, query[last:]...)

	return string(result), args, nil
}
