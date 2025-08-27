package sqlite

import (
	"fmt"
	"log"
	"strings"
)

func (d *dbImpl) ExecUpdate(query string, args ...any) error {
	q := strings.TrimSpace(query)

	if q == "" {

		return fmt.Errorf("empty query")
	}

	if !strings.HasSuffix(q, ";") {
		q = q + ";"
	}
	_, err := d.Sqlite.Query(query, args)
	return err
}

func (d *dbImpl) UpdateByAll(table string, set map[string]any, where map[string]any) error {
	err := d.updatePlus(table, set, where, sqlAND)
	if err != nil {
		log.Printf("[ERROR] UpdateByAll err: %v", err)
	}
	return err
}

func (d *dbImpl) UpdateByAny(table string, set map[string]any, where map[string]any) error {
	err := d.updatePlus(table, set, where, sqlOR)
	if err != nil {
		log.Printf("[ERROR] UpdateByAny err: %v", err)
	}
	return err
}

func (d *dbImpl) updatePlus(table string, set map[string]any, where map[string]any, condition string) error {
	u := d.generateUpdate(table)
	s, sa := d.generateSet(set)
	w, wa := d.generateWhere(table, where, condition)
	query := strings.Join([]string{u, s, w}, "\n") + ";"
	sa = append(sa, wa)
	return d.ExecUpdate(query, sa...)
}

func (d *dbImpl) generateUpdate(table string) string {
	return sqlUPDATE + " " + table + "\n"
}

func (d *dbImpl) generateSet(set map[string]any) (string, []any) {
	var s strings.Builder
	s.WriteString(sqlSET + " ")

	args := make([]any, 0)
	isFirst := true

	for key, value := range set {
		if isFirst {
			isFirst = false
		} else {
			s.WriteString(", ")
		}
		s.WriteString(key)
		s.WriteString(" = ?\n")
		args = append(args, value)
	}

	return s.String(), args
}

func (d *dbImpl) generateWhere(table string, where map[string]any, condition string) (string, []any) {
	var w strings.Builder
	w.WriteString(sqlWHERE + " ")

	args := make([]any, 0)
	isFirst := true

	for key, value := range where {
		if isFirst {
			isFirst = false
		} else {
			w.WriteString(condition + " ")
		}
		w.WriteString(table)
		w.WriteString(".")
		w.WriteString(key)
		w.WriteString(" = ?\n")
		args = append(args, value)
	}

	return w.String(), args
}
