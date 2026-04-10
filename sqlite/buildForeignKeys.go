package sqlite

import (
	"fmt"
)

func buildForeignKeysSQL(t Table) []string {
	var parts []string

	for _, fk := range t.ForeignKeys {
		if len(fk.Columns) == 0 || fk.ReferenceTable == "" || len(fk.ReferenceColumns) == 0 {
			continue
		}

		part := ""
		if fk.Name != "" {
			part += fmt.Sprintf("CONSTRAINT %s ", escapeIdentifier(fk.Name))
		}

		part += fmt.Sprintf(
			"FOREIGN KEY (%s) REFERENCES %s (%s)",
			joinEscapedIdentifiers(fk.Columns),
			escapeIdentifier(fk.ReferenceTable),
			joinEscapedIdentifiers(fk.ReferenceColumns),
		)

		if fk.OnDelete != "" {
			part += " ON DELETE " + fk.OnDelete
		}
		if fk.OnUpdate != "" {
			part += " ON UPDATE " + fk.OnUpdate
		}
		if fk.Deferrable != nil && *fk.Deferrable {
			part += " DEFERRABLE"
			if fk.InitiallyDeferred != nil && *fk.InitiallyDeferred {
				part += " INITIALLY DEFERRED"
			}
		}

		parts = append(parts, part)
	}

	return parts
}
