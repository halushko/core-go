package sqlite

import (
	"fmt"
	"strings"
)

func boolPtr(v bool) *bool {
	return &v
}

func stringPtr(v string) *string {
	return &v
}

func escapeIdentifier(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func joinEscapedIdentifiers(items []string) string {
	escaped := make([]string, 0, len(items))
	for _, item := range items {
		escaped = append(escaped, escapeIdentifier(item))
	}
	return strings.Join(escaped, ", ")
}

func (t ColumnType) validateColumnType() error {
	switch t {
	case TypeInteger, TypeText, TypeReal, TypeBlob, TypeDatetime:
		return nil
	default:
		return fmt.Errorf("invalid column type: %s", t)
	}
}
