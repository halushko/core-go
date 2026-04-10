package sqlite

import (
	"fmt"
	"strings"
)

func buildChecksSQL(t Table) []string {
	var parts []string

	for _, chk := range t.Checks {
		if strings.TrimSpace(chk.Expr) == "" {
			continue
		}

		part := ""
		if chk.Name != "" {
			part += fmt.Sprintf("CONSTRAINT %s ", escapeIdentifier(chk.Name))
		}
		part += fmt.Sprintf("CHECK (%s)", chk.Expr)

		parts = append(parts, part)
	}

	return parts
}
