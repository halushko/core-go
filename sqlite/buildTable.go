package sqlite

import (
	"fmt"
	"strings"
)

func buildCreateTableSQL(t Table, ifNotExists bool) string {
	if t.Columns == nil || len(t.Columns) == 0 {
		return ""
	}
	createClause := "CREATE TABLE"
	if ifNotExists {
		createClause += " IF NOT EXISTS"
	}

	columns := buildColumnsSQL(t)
	uniqueConstraints := buildUniqueConstraintsSQL(t)
	checks := buildChecksSQL(t)
	foreignKeys := buildForeignKeysSQL(t)

	parts := append(columns, uniqueConstraints...)
	parts = append(parts, checks...)
	parts = append(parts, foreignKeys...)

	return fmt.Sprintf(
		"%s %s\n(\n    %s\n);",
		createClause,
		escapeIdentifier(t.Name),
		strings.Join(parts, ",\n    "),
	)
}
