package google

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"

	log "github.com/sirupsen/logrus"
)

const a1FullNotation = `^(?:'([^']+)'|([^!]+))!([A-Z]+)([1-9][0-9]*):([A-Z]+)([1-9][0-9]*)$`

type Client struct {
	svc *sheets.Service
	ctx context.Context
}

// NewFromServiceAccountFile creates a client from a service account JSON file.
//
//goland:noinspection GoUnusedExportedFunction
func NewFromServiceAccountFile(ctx context.Context, jsonPath string) (*Client, error) {
	if strings.TrimSpace(jsonPath) == "" {
		return nil, errors.New("jsonPath is empty")
	}
	svc, err := sheets.NewService(
		ctx,
		option.WithCredentialsFile(jsonPath),
		option.WithScopes(sheets.SpreadsheetsReadonlyScope),
	)
	if err != nil {
		return nil, fmt.Errorf("create sheets service: %w", err)
	}
	log.Infof("Created a new Google Spreadsheet client")
	return &Client{svc: svc, ctx: ctx}, nil
}

// NewFromServiceAccountJSON creates a client directly from JSON bytes (convenient if you keep the key secret).
//
//goland:noinspection GoUnusedExportedFunction
func NewFromServiceAccountJSON(ctx context.Context, jsonBytes []byte) (*Client, error) {
	if len(jsonBytes) == 0 {
		return nil, errors.New("jsonBytes is empty")
	}
	svc, err := sheets.NewService(
		ctx,
		option.WithCredentialsJSON(jsonBytes),
		option.WithScopes(sheets.SpreadsheetsReadonlyScope),
	)
	if err != nil {
		return nil, fmt.Errorf("create sheets service: %w", err)
	}
	log.Infof("Created a new Google Spreadsheet client")
	return &Client{svc: svc, ctx: ctx}, nil
}

// ColToA1 1 -> A, 2 -> B, ... 26 -> Z, 27 -> AA ...
func ColToA1(col int) string {
	if col < 1 {
		return "?"
	}
	var b []byte
	for col > 0 {
		col-- // 1-based -> 0-based
		b = append([]byte{byte('A' + (col % 26))}, b...)
		col /= 26
	}
	return string(b)
}

// A1ToNumber 1 -> A, 2 -> B, ... 26 -> Z, 27 -> AA ...
func A1ToNumber(col string) int {
	col = strings.ToUpper(strings.TrimSpace(col))
	if col == "" {
		return 0
	}

	n := 0
	for _, r := range col {
		if r < 'A' || r > 'Z' {
			return 0
		}
		n = n*26 + int(r-'A'+1)
	}
	return n
}

// ReadByA1 reads a rectangular range in coordinates (rows/columns) and returns [][]string.
//
// INDEX: 1-based.
// startRow=1, startCol=A => A1.
// endRow/endCol — including.
//
// Example: Read(id, "Programs", A, 1, E, 10) => Programs!A1:E10
//
//goland:noinspection GoUnusedExportedFunction
func (c *Client) ReadByA1(
	spreadsheetID string, sheetName string,
	startCol string, startRow int,
	endCol string, endRow int,
) ([][]string, error) {
	return c.ReadByIndexes(spreadsheetID, sheetName, A1ToNumber(startCol), startRow, A1ToNumber(endCol), endRow)
}

// ReadByIndexes reads a rectangular range in coordinates (rows/columns) and returns [][]string.
//
// INDEX: 1-based.
// startRow=1, startCol=1 => A1.
// endRow/endCol — including.
//
// Example: Read(id, "Programs", 2, 1, 10, 5) => Programs!A2:E10
//
//goland:noinspection GoUnusedExportedFunction
func (c *Client) ReadByIndexes(
	spreadsheetID, sheetName string,
	startCol, startRow int,
	endCol, endRow int) ([][]string, error) {

	if c == nil || c.svc == nil || c.ctx == nil {
		return nil, errors.New("client is not initialized")
	}
	if strings.TrimSpace(spreadsheetID) == "" {
		return nil, errors.New("spreadsheetID is empty")
	}
	if strings.TrimSpace(sheetName) == "" {
		return nil, errors.New("sheetName is empty")
	}

	if startRow < 1 || startCol < 1 || endRow < 1 || endCol < 1 {
		return nil, errors.New("row/col indexes must be >= 1 (1-based)")
	}
	if endRow < startRow {
		return nil, fmt.Errorf("endRow (%d) < startRow (%d)", endRow, startRow)
	}
	if endCol < startCol {
		return nil, fmt.Errorf("endCol (%d) < startCol (%d)", endCol, startCol)
	}

	a1 := buildA1Range(sheetName, ColToA1(startCol), startRow, ColToA1(endCol), endRow)

	resp, err := c.svc.Spreadsheets.Values.Get(spreadsheetID, a1).Context(c.ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("values.get %s: %w", a1, err)
	}

	return normalizeToStrings(resp.Values, (endRow-startRow)+1, (endCol-startCol)+1), nil
}

// Read reads any A1 range of the form:
//
//	Sheet!A2:D
//	'Sheet name'!B1:AA20
func (c *Client) Read(
	spreadsheetID string,
	a1 string,
) ([][]string, error) {
	sheetName, startCol, startRow, endCol, endRow, err := parseA1Range(a1)
	if err != nil {
		return nil, err
	}
	return c.ReadByA1(spreadsheetID, sheetName, startCol, startRow, endCol, endRow)
}

func parseA1Range(a1 string) (string, string, int, string, int, error) {
	a1RangeRe := regexp.MustCompile(a1FullNotation)

	m := a1RangeRe.FindStringSubmatch(a1)
	if m == nil {
		return "", "", -1, "", -1, fmt.Errorf("invalid A1 range (expected format Sheet!A1:C3): %s", a1)
	}

	sheetName := m[1]
	if sheetName == "" {
		sheetName = m[2]
	}

	startCol := m[3]
	startRow, _ := strconv.Atoi(m[4])

	endCol := m[5]
	endRow, _ := strconv.Atoi(m[6])

	return sheetName, startCol, startRow, endCol, endRow, nil
}

//goland:noinspection GoUnusedExportedFunction
func buildA1Range(sheetName string, startCol string, startRow int, endCol string, endRow int) string {
	// If there are spaces/special characters, escape with apostrophes and double ' inside.
	safeSheet := strings.ReplaceAll(sheetName, "'", "''")
	safeSheet = "'" + safeSheet + "'"

	startCell := fmt.Sprintf("%s%d", startCol, startRow)
	endCell := fmt.Sprintf("%s%d", endCol, endRow)
	return fmt.Sprintf("%s!%s:%s", safeSheet, startCell, endCell)
}

// normalizeToStrings transforms [][]interface{} to [][]string (empty cells -> "").
func normalizeToStrings(values [][]interface{}, rows, cols int) [][]string {
	out := make([][]string, rows)
	for r := 0; r < rows; r++ {
		out[r] = make([]string, cols)
		for c := 0; c < cols; c++ {
			out[r][c] = ""
		}
	}

	for r := 0; r < len(values) && r < rows; r++ {
		for c := 0; c < len(values[r]) && c < cols; c++ {
			out[r][c] = fmt.Sprint(values[r][c])
		}
	}
	return out
}
