package sqlite

import (
	"fmt"
)

func buildMacrosQuery(query string, params map[string]any) (string, []any, error) {
	if query == "" {
		return "", nil, fmt.Errorf("query is empty")
	}

	args := make([]any, 0)

	matches := macroRegexp.FindAllStringSubmatchIndex(query, -1)
	if len(matches) == 0 {
		return query, args, nil
	}

	result := make([]byte, 0, len(query))
	last := 0

	for _, m := range matches {
		fullStart, fullEnd := m[0], m[1]
		nameStart, nameEnd := m[2], m[3]

		result = append(result, query[last:fullStart]...)

		name := query[nameStart:nameEnd]
		value, ok := params[name]
		if !ok {
			return "", nil, fmt.Errorf("missing macro value for key %q", name)
		}

		result = append(result, '?')
		args = append(args, value)

		last = fullEnd
	}

	result = append(result, query[last:]...)

	return string(result), args, nil
}
