package sqlite

import (
	"fmt"
)

func buildUniqueConstraintsSQL(t Table) []string {
	var parts []string

	for _, uc := range t.UniqueConstraints {
		if len(uc.Columns) == 0 {
			continue
		}

		part := ""
		if uc.Name != "" {
			part += fmt.Sprintf("CONSTRAINT %s ", escapeIdentifier(uc.Name))
		}
		part += fmt.Sprintf("UNIQUE (%s)", joinEscapedIdentifiers(uc.Columns))

		parts = append(parts, part)
	}

	return parts
}
