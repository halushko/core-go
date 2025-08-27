package sqlite

import (
	"strings"
)

func (d *dbImpl) Update(table string, set map[string]any, where map[string]any) error {
	u := d.generateUpdate(table)
	s, sa := d.generateSet(set)
	w, wa := d.generateWhere(table, where)
	query := strings.Join([]string{u, s, w}, "\n") + ";"
	sa = append(sa, wa)
	_, err := d.Sqlite.Exec(query, sa)
	return err
}

func (d *dbImpl) generateUpdate(table string) string {
	return strings.Join([]string{"UPDATE", table, "\n"}, " ")
}

func (d *dbImpl) generateSet(set map[string]any) (string, []any) {
	var s strings.Builder
	s.WriteString("SET ")

	args := make([]any, 0)

	for key, value := range set {
		s.WriteString(key)
		s.WriteString(" = ?\n")
		args = append(args, value)
	}

	return s.String(), args
}

func (d *dbImpl) generateWhere(table string, where map[string]any) (string, []any) {
	var w strings.Builder
	w.WriteString("WHERE ")

	args := make([]any, 0)

	for key, value := range where {
		w.WriteString(table)
		w.WriteString(".")
		w.WriteString(key)
		w.WriteString(" = ?\n")
		args = append(args, value)
	}

	return w.String(), args
}
