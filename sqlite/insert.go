package sqlite

import (
	"fmt"
	"log"
	"strings"
)

const actionInsertUpdate = "UPDATE SET"
const actionInsertDoNothing = "NOTHING"

func (d *dbImpl) Insert(table string, row map[string]any) error {
	err := d.insertPlus(table, row, "")
	if err != nil {
		log.Printf("[ERROR] during insert: %v", err)
	}
	return err
}

func (d *dbImpl) InsertOrUpdate(table string, row map[string]any, onConflict ...string) error {
	err := d.insertPlus(table, row, actionInsertUpdate, onConflict...)
	if err != nil {
		log.Printf("[ERROR] during insert or update: %v", err)
	}
	return err
}

func (d *dbImpl) InsertIfNotExists(table string, row map[string]any, onConflict ...string) error {
	err := d.insertPlus(table, row, actionInsertDoNothing, onConflict...)
	if err != nil {
		log.Printf("[ERROR] during insert or do nothing: %v", err)
	}
	return err
}

func (d *dbImpl) insertPlus(table string, row map[string]any, action string, onConflict ...string) error {
	insert, args := generateInsertQuery(table, row)
	conflict := generateOnConflict(row, action, onConflict...)

	_, err := d.Sqlite.Exec(insert+conflict+";", args)
	return err
}

func generateInsertQuery(table string, row map[string]any) (string, []any) {
	var q strings.Builder
	q.WriteString("INSERT INTO ")
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
	q.WriteString(") VALUES (")
	q.WriteString(v.String())
	q.WriteString(")")

	return q.String(), args
}

func generateOnConflict(row map[string]any, action string, onConflict ...string) string {
	if onConflict == nil || len(onConflict) == 0 {
		return ""
	}

	var q strings.Builder

	conf := make(map[string]bool, 0)
	for key := range row {
		conf[key] = false
	}

	for _, key := range onConflict {
		conf[key] = true
	}

	q.WriteString("\nON CONFLICT (")

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
	q.WriteString(") DO ")

	return q.String() + generatePostAction(row, action, onConflict...)
}

func generatePostAction(row map[string]any, action string, onConflict ...string) string {
	switch {
	case action == actionInsertDoNothing:
		return action
	case action == actionInsertUpdate:
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
