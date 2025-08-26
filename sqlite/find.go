package sqlite

import (
	"fmt"
	"strings"
)

func (d *dbImpl) Find(table Table, by Column, byValue any, whatColumns ...Column) ([]map[string]any, error) {
	resultColumns := "*"
	if whatColumns != nil && len(whatColumns) > 0 {
		addComa := false
		var q strings.Builder

		for _, wc := range whatColumns {
			if !addComa {
				addComa = true
			} else {
				q.WriteString(", ")
			}
			q.WriteString(wc.Name)
		}
		resultColumns = q.String()
	}
	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s = ?;", resultColumns, table.Name, by.Name)

	rows, err := d.Sqlite.Query(query, byValue)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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
			return nil, err
		}

		rowMap := make(map[string]any)
		for i, colName := range columns {
			rowMap[colName] = values[i]
		}

		results = append(results, rowMap)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}
