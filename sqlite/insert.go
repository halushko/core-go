package sqlite

import (
	"fmt"
	"log"
	"strings"

	_ "modernc.org/sqlite"
)

func (d *dbImpl) ExecInsert(query string, args ...any) error {
	q := strings.TrimSpace(query)

	if q == "" {
		return fmt.Errorf("empty query")
	}

	if !strings.HasSuffix(q, ";") {
		q = q + ";"
	}

	_, err := d.Sqlite.Exec(q, args)
	return err
}

func (d *dbImpl) Insert(table string, row map[string]any) error {
	err := d.insertPlus(table, row, "")
	if err != nil {
		log.Printf("[ERROR] during insert: %v", err)
	}
	return err
}

func (d *dbImpl) InsertOrUpdate(table string, row map[string]any, onConflict ...string) error {
	err := d.insertPlus(table, row, sqlDoUpdateSet, onConflict...)
	if err != nil {
		log.Printf("[ERROR] during insert or update: %v", err)
	}
	return err
}

func (d *dbImpl) InsertIfNotExists(table string, row map[string]any, onConflict ...string) error {
	err := d.insertPlus(table, row, sqlDoNothing, onConflict...)
	if err != nil {
		log.Printf("[ERROR] during insert or do nothing: %v", err)
	}
	return err
}

func (d *dbImpl) insertPlus(table string, row map[string]any, action string, onConflict ...string) error {
	insert, args := generateInsertQuery(table, row)
	conflict := generateOnConflict(row, action, onConflict...)

	return d.ExecInsert(insert+conflict, args)
}

func generateInsertQuery(table string, row map[string]any) (string, []any) {
	var q strings.Builder
	q.WriteString(sqlINSERT + " ")
	q.WriteString(table)
	q.WriteString(" (")

	var k strings.Builder
	var v strings.Builder
	args := make([]any, 0)

	putComma := false

	for key, value := range row {
		if putComma {
			k.WriteString(", ")
			v.WriteString(", ")
		} else {
			putComma = true
		}
		k.WriteString(key)
		v.WriteString("?")
		args = append(args, value)
	}

	q.WriteString(k.String())
	q.WriteString(") " + sqlVALUES + " (")
	q.WriteString(v.String())
	q.WriteString(")")

	return q.String(), args
}

func generateOnConflict(row map[string]any, action string, onConflict ...string) string {
	if onConflict == nil || len(onConflict) == 0 {
		return ""
	}

	var q strings.Builder

	conf := make(map[string]bool)
	for key := range row {
		conf[key] = false
	}

	for _, key := range onConflict {
		conf[key] = true
	}

	q.WriteString("\n" + sqlON + " " + sqlCONFLICT + " (")

	var conflict strings.Builder

	putOnConflictComma := false

	for key := range row {
		isOnConflict := conf[key]

		if isOnConflict {
			if putOnConflictComma {
				conflict.WriteString(", ")
			} else {
				putOnConflictComma = true
			}
		}

		if isOnConflict {
			conflict.WriteString(key)
		}
	}

	q.WriteString(conflict.String())
	q.WriteString(") ")

	return q.String() + generatePostAction(row, action, onConflict...)
}

func generatePostAction(row map[string]any, action string, onConflict ...string) string {
	switch {
	case action == sqlDoNothing:
		return action
	case action == sqlDoUpdateSet:
		if onConflict == nil || len(onConflict) == 0 {
			return ""
		}

		conf := make(map[string]bool)
		for key := range row {
			conf[key] = false
		}
		for _, key := range onConflict {
			conf[key] = true
		}

		var q strings.Builder
		var excluded strings.Builder
		lastConflict := ""

		for key := range row {
			isOnConflict := conf[key]

			if !isOnConflict {
				val := fmt.Sprintf("\t%s = excluded.%s", key, key)
				if lastConflict == "" {
					lastConflict = val
				} else {
					excluded.WriteString(val + ",\n")
				}
			}
		}
		excluded.WriteString(lastConflict)

		q.WriteString(action + "\n")
		q.WriteString(excluded.String())
		return q.String()
	default:
		return ""
	}
}
