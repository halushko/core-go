package sqlite

import (
	external "database/sql"
	"fmt"
	"log"
	"strings"

	_ "modernc.org/sqlite"
)

func (d *dbImpl) ExecSelect(query string, args ...any) ([]map[string]any, error) {
	q := strings.TrimSpace(query)

	if q == "" {
		return nil, fmt.Errorf("empty query")
	}

	if !strings.HasSuffix(q, ";") {
		q = q + ";"
	}

	rows, err := d.Sqlite.Query(query, args...)

	if rows != nil {
		defer func(rows *external.Rows) {
			e := rows.Close()
			if e != nil {
				return
			}
		}(rows)
	}

	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		} else {
			log.Printf("[ERROR] during select execution: %v", err)
			return nil, err
		}
	}

	if rows == nil {
		return nil, nil
	}

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]any

	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err = rows.Scan(valuePtrs...); err != nil {
			log.Printf("[ERROR] during select execution: %v", err)
			return nil, err
		}

		rowMap := make(map[string]any)
		for i, colName := range columns {
			rowMap[colName] = values[i]
		}

		results = append(results, rowMap)
	}

	if err = rows.Err(); err != nil {
		log.Printf("[ERROR] during select execution: %v", err)
		return nil, err
	}

	return results, nil
}

func (d *dbImpl) SelectByAll(table string, condition map[string]any, outputColumns ...string) ([]map[string]any, error) {
	res, err := d.selectPlus(table, condition, sqlAND, outputColumns...)
	if err != nil {
		log.Printf("[ERROR] during select by all: %v", err)
		return nil, err
	}

	return res, err
}

func (d *dbImpl) SelectByAny(table string, condition map[string]any, outputColumns ...string) ([]map[string]any, error) {
	res, err := d.selectPlus(table, condition, sqlOR, outputColumns...)
	if err != nil {
		log.Printf("[ERROR] during select by any: %v", err)
		return nil, err
	}

	return res, err
}

func (d *dbImpl) selectPlus(table string, condition map[string]any, conditionBy string, outputColumns ...string) ([]map[string]any, error) {
	outputColumnsStr := d.generateOutputColumns(outputColumns...)
	conditionStr, args := d.generateCondition(condition, conditionBy)

	query := strings.Join([]string{
		sqlSELECT + " " + outputColumnsStr,
		sqlFROM + " " + table,
		sqlWHERE + " " + conditionStr}, "\n")
	return d.ExecSelect(query+";", args...)
}

func (d *dbImpl) generateOutputColumns(outputColumns ...string) string {
	if outputColumns == nil || len(outputColumns) <= 0 {
		return "*"
	}

	addComa := false
	var q strings.Builder

	for _, column := range outputColumns {
		if !addComa {
			addComa = true
		} else {
			q.WriteString(", ")
		}
		q.WriteString(column)
	}
	return q.String()
}

func (d *dbImpl) generateCondition(condition map[string]any, logic string) (string, []any) {
	if condition == nil || len(condition) == 0 {
		return "", make([]any, 0)
	}

	var q strings.Builder
	args := make([]any, 0)
	isFirstCondition := true

	for k, v := range condition {
		if isFirstCondition {
			isFirstCondition = false
		} else {
			q.WriteString(logic + " ")

		}
		q.WriteString(k)
		q.WriteString(" = ")
		q.WriteString(" ?\n")
		args = append(args, v)
	}

	return q.String(), args
}
