package sqlite

import (
	"fmt"
	"strings"
)

func buildColumnsSQL(t Table) []string {
	var parts []string

	for _, c := range t.Columns {
		part := fmt.Sprintf("%s %s", escapeIdentifier(c.Name), c.Type)

		if c.GeneratedExpr != nil && strings.TrimSpace(*c.GeneratedExpr) != "" {
			part += fmt.Sprintf(" GENERATED ALWAYS AS (\n        %s\n    )", *c.GeneratedExpr)

			if c.Stored != nil && *c.Stored {
				part += " STORED"
			} else {
				part += " VIRTUAL"
			}
		} else {
			if c.PrimaryKey != nil && *c.PrimaryKey {
				part += " PRIMARY KEY"
			}
			if c.NotNull != nil && *c.NotNull {
				part += " NOT NULL"
			}
			if c.Unique != nil && *c.Unique {
				part += " UNIQUE"
			}
			if c.Default != nil && *c.Default != "" {
				part += " DEFAULT " + *c.Default
			}
		}
		parts = append(parts, part)
	}

	return parts
}
