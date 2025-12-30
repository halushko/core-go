package sqlite

import (
	"context"
	external "database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const dbDefaultPath = "/data/sqlite"

type Client struct {
	db *external.DB
}

type DBI interface {
	Select(query string, args ...any) ([]map[string]any, error)
	Exec(query string, args ...any) error
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

// DropTable drops a table by name if it exists.
func (c *Client) DropTable(name string) error {
	if name == "" {
		return errors.New("table name is empty")
	}

	query := fmt.Sprintf("DROP TABLE IF EXISTS %q", name)
	return c.Execute(query)
}

func getDbPath() string {
	path := os.Getenv("DB_PATH")
	if path == "" {
		return dbDefaultPath
	} else {
		return path
	}
}
