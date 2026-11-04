package sqlite

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"time"
	"unicode"
)

// TableFromStruct converts a struct (or pointer to a struct) to a Table.
// Exported fields are included unless tagged sqlite:"-".
// Supported tags: name, type, pk, autoincrement, notnull, unique,
// index[=name], unique_index[=name], unique_group=name,
// default=SQL, check=SQL, references=table(column),
// ondelete=ACTION and onupdate=ACTION.
// A bare first tag item is treated as the column name.
func TableFromStruct(name string, model any) (Table, error) {
	if strings.TrimSpace(name) == "" {
		return Table{}, fmt.Errorf("table name is empty")
	}
	if model == nil {
		return Table{}, fmt.Errorf("model is nil")
	}
	typ := reflect.TypeOf(model)
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return Table{}, fmt.Errorf("model must be a struct or pointer to struct, got %s", typ)
	}

	t := Table{Name: name}
	seen := make(map[string]bool)
	indexes := make(map[string]int)
	uniqueGroups := make(map[string]int)
	primaryKeys := 0

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}
		tag := field.Tag.Get("sqlite")
		if tag == "-" {
			continue
		}

		col := Column{Name: snakeCase(field.Name)}
		options, err := parseSQLiteTag(tag)
		if err != nil {
			return Table{}, fmt.Errorf("field %s: %w", field.Name, err)
		}
		if n, ok := options["name"]; ok {
			col.Name = n
		}
		if col.Name == "" || seen[col.Name] {
			return Table{}, fmt.Errorf("field %s: empty or duplicate column %q", field.Name, col.Name)
		}
		seen[col.Name] = true

		col.Type, err = inferSQLiteType(field.Type)
		if override, ok := options["type"]; ok {
			switch strings.ToUpper(override) {
			case "INTEGER", "TEXT", "REAL", "BLOB", "DATETIME", "NUMERIC":
				col.Type = ColumnType(strings.ToUpper(override))
				err = nil
			default:
				return Table{}, fmt.Errorf("field %s: unsupported sqlite type %q", field.Name, override)
			}
		}
		if err != nil {
			return Table{}, fmt.Errorf("field %s: %w", field.Name, err)
		}

		if _, ok := options["pk"]; ok {
			col.PrimaryKey = sqliteBool(true)
			primaryKeys++
			if primaryKeys > 1 {
				return Table{}, fmt.Errorf("multiple primary keys are not supported by struct tags")
			}
		}
		if _, ok := options["autoincrement"]; ok {
			if col.PrimaryKey == nil || col.Type != TypeInteger {
				return Table{}, fmt.Errorf("field %s: autoincrement requires INTEGER PRIMARY KEY", field.Name)
			}
			col.Autoincrement = sqliteBool(true)
		}
		if _, ok := options["notnull"]; ok {
			col.NotNull = sqliteBool(true)
		}
		if _, ok := options["unique"]; ok {
			col.Unique = sqliteBool(true)
		}
		if v, ok := options["default"]; ok {
			if v == "" {
				return Table{}, fmt.Errorf("field %s: default value is empty", field.Name)
			}
			col.Default = &v
		}
		if expr, ok := options["check"]; ok {
			if expr == "" {
				return Table{}, fmt.Errorf("field %s: empty check constraint", field.Name)
			}
			t.Checks = append(t.Checks, CheckConstraint{Expr: expr})
		}
		ref, hasRef := options["references"]
		if hasRef {
			open := strings.IndexByte(ref, '(')
			if open <= 0 || !strings.HasSuffix(ref, ")") {
				return Table{}, fmt.Errorf("field %s: references must be table(column)", field.Name)
			}
			refTable := strings.TrimSpace(ref[:open])
			refColumn := strings.TrimSpace(ref[open+1 : len(ref)-1])
			if refTable == "" || refColumn == "" || strings.Contains(refColumn, ",") {
				return Table{}, fmt.Errorf("field %s: invalid references value %q", field.Name, ref)
			}
			fk := ForeignKey{
				Columns:          []string{col.Name},
				ReferenceTable:    refTable,
				ReferenceColumns:  []string{refColumn},
			}
			if action, ok := options["ondelete"]; ok {
				fk.OnDelete, err = sqliteForeignKeyAction(action)
				if err != nil {
					return Table{}, fmt.Errorf("field %s: %w", field.Name, err)
				}
			}
			if action, ok := options["onupdate"]; ok {
				fk.OnUpdate, err = sqliteForeignKeyAction(action)
				if err != nil {
					return Table{}, fmt.Errorf("field %s: %w", field.Name, err)
				}
			}
			t.ForeignKeys = append(t.ForeignKeys, fk)
		} else if _, ok := options["ondelete"]; ok {
			return Table{}, fmt.Errorf("field %s: ondelete requires references", field.Name)
		} else if _, ok := options["onupdate"]; ok {
			return Table{}, fmt.Errorf("field %s: onupdate requires references", field.Name)
		}

		if v, ok := options["index"]; ok {
			indexName := v
			if indexName == "" {
				indexName = "idx_" + name + "_" + col.Name
			}
			addSQLiteIndex(&t, indexes, indexName, col.Name, false)
		}
		if v, ok := options["unique_index"]; ok {
			indexName := v
			if indexName == "" {
				indexName = "uidx_" + name + "_" + col.Name
			}
			if existing, ok := indexes[indexName]; ok && !t.Indexes[existing].Unique {
				return Table{}, fmt.Errorf("field %s: index %q has conflicting uniqueness", field.Name, indexName)
			}
			addSQLiteIndex(&t, indexes, indexName, col.Name, true)
		}
		if group, ok := options["unique_group"]; ok {
			if group == "" {
				return Table{}, fmt.Errorf("field %s: unique_group requires a name", field.Name)
			}
			if idx, ok := uniqueGroups[group]; ok {
				t.UniqueConstraints[idx].Columns = append(t.UniqueConstraints[idx].Columns, col.Name)
			} else {
				uniqueGroups[group] = len(t.UniqueConstraints)
				t.UniqueConstraints = append(t.UniqueConstraints, UniqueConstraint{
					Name: group, Columns: []string{col.Name},
				})
			}
		}

		t.Columns = append(t.Columns, col)
	}
	if len(t.Columns) == 0 {
		return Table{}, fmt.Errorf("model has no exported sqlite columns")
	}
	return t, nil
}

// CreateTableFromStruct uses the existing CreateTable SQL builders.
func (c *Client) CreateTableFromStruct(name string, model any) error {
	t, err := TableFromStruct(name, model)
	if err != nil {
		return err
	}
	return c.CreateTable(t)
}

func sqliteBool(v bool) *bool { return &v }

func addSQLiteIndex(t *Table, names map[string]int, name, column string, unique bool) {
	if idx, ok := names[name]; ok {
		t.Indexes[idx].Columns = append(t.Indexes[idx].Columns, column)
		return
	}
	names[name] = len(t.Indexes)
	t.Indexes = append(t.Indexes, Index{Name: name, Unique: unique, Columns: []string{column}})
}

func sqliteForeignKeyAction(action string) (string, error) {
	a := strings.ToUpper(strings.Join(strings.Fields(strings.ReplaceAll(action, "_", " ")), " "))
	switch a {
	case "CASCADE", "RESTRICT", "SET NULL", "SET DEFAULT", "NO ACTION":
		return a, nil
	default:
		return "", fmt.Errorf("invalid foreign key action %q", action)
	}
}

func inferSQLiteType(typ reflect.Type) (ColumnType, error) {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ == reflect.TypeOf(time.Time{}) || typ == reflect.TypeOf(sql.NullTime{}) {
		return TypeDatetime, nil
	}
	switch typ {
	case reflect.TypeOf(sql.NullString{}):
		return TypeText, nil
	case reflect.TypeOf(sql.NullInt16{}), reflect.TypeOf(sql.NullInt32{}),
		reflect.TypeOf(sql.NullInt64{}), reflect.TypeOf(sql.NullBool{}), reflect.TypeOf(sql.NullByte{}):
		return TypeInteger, nil
	case reflect.TypeOf(sql.NullFloat64{}):
		return TypeReal, nil
	}
	switch typ.Kind() {
	case reflect.String:
		return TypeText, nil
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return TypeInteger, nil
	case reflect.Float32, reflect.Float64:
		return TypeReal, nil
	case reflect.Slice:
		if typ.Elem().Kind() == reflect.Uint8 {
			return TypeBlob, nil
		}
	}
	return "", fmt.Errorf("unsupported Go type %s; use sqlite type=... to override", typ)
}

func snakeCase(s string) string {
	runes := []rune(s)
	var b strings.Builder
	for i, r := range runes {
		if unicode.IsUpper(r) && i > 0 &&
			(unicode.IsLower(runes[i-1]) || unicode.IsDigit(runes[i-1]) ||
				(i+1 < len(runes) && unicode.IsLower(runes[i+1]) && unicode.IsUpper(runes[i-1]))) {
			b.WriteByte('_')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

func parseSQLiteTag(tag string) (map[string]string, error) {
	result := make(map[string]string)
	if strings.TrimSpace(tag) == "" {
		return result, nil
	}
	var parts []string
	var b strings.Builder
	depth := 0
	var quote rune
	for _, r := range tag {
		switch {
		case quote != 0:
			b.WriteRune(r)
			if r == quote {
				quote = 0
			}
		case r == '\'' || r == '"':
			quote = r
			b.WriteRune(r)
		case r == '(':
			depth++
			b.WriteRune(r)
		case r == ')':
			depth--
			if depth < 0 {
				return nil, fmt.Errorf("unbalanced sqlite tag parentheses")
			}
			b.WriteRune(r)
		case r == ',' && depth == 0:
			parts = append(parts, strings.TrimSpace(b.String()))
			b.Reset()
		default:
			b.WriteRune(r)
		}
	}
	if quote != 0 || depth != 0 {
		return nil, fmt.Errorf("unbalanced sqlite tag")
	}
	parts = append(parts, strings.TrimSpace(b.String()))
	for i, part := range parts {
		if part == "" {
			return nil, fmt.Errorf("empty sqlite tag option")
		}
		key, value, hasValue := strings.Cut(part, "=")
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		switch key {
		case "name", "type", "default", "check", "references", "ondelete", "onupdate", "unique_group":
			if !hasValue {
				return nil, fmt.Errorf("sqlite tag %q requires a value", key)
			}
		case "pk", "autoincrement", "notnull", "unique":
			if hasValue {
				return nil, fmt.Errorf("sqlite tag %q does not take a value", key)
			}
		case "index", "unique_index":
		default:
			if i == 0 && !hasValue {
				key, value = "name", strings.TrimSpace(part)
			} else {
				return nil, fmt.Errorf("unknown sqlite tag option %q", key)
			}
		}
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("duplicate sqlite tag option %q", key)
		}
		result[key] = value
	}
	return result, nil
}
