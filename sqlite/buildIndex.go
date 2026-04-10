package sqlite

import (
	"fmt"
	"strings"
)

func buildIndexesSQL(t Table, ifNotExists bool) []string {
	var parts []string
	if t.Name == "" {
		return []string{}
	}

	for _, idx := range t.Indexes {
		if idx.Name == "" || len(idx.Columns) == 0 {
			continue
		}

		createClause := "CREATE INDEX"
		if idx.Unique {
			createClause = "CREATE UNIQUE INDEX"
		}
		if ifNotExists {
			createClause += " IF NOT EXISTS"
		}

		where := ""
		if strings.TrimSpace(idx.Where) != "" {
			where += "WHERE " + idx.Where
		}

		part := fmt.Sprintf(
			"%s %s ON %s (%s) %s;",
			createClause,
			escapeIdentifier(idx.Name),
			escapeIdentifier(t.Name),
			joinEscapedIdentifiers(idx.Columns),
			where,
		)

		parts = append(parts, part)
	}

	return parts
}
