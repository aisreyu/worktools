package toolbox

import (
	"encoding/csv"
	"fmt"
	"strings"
)

// parseCSV parses CSV text, tolerating ragged rows by padding them to the
// header width.
func parseCSV(s string) ([][]string, error) {
	r := csv.NewReader(strings.NewReader(strings.TrimSpace(s)))
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) > 0 {
		w := len(rows[0])
		for i, row := range rows {
			if len(row) < w {
				padded := make([]string, w)
				copy(padded, row)
				rows[i] = padded
			}
		}
	}
	return rows, nil
}

// CSVToSQL converts CSV text into INSERT statements for the given table.
// The first row is treated as the header. Columns are quoted with the
// dialect's identifier quote ("\"" for postgres, "`" for mysql).
func CSVToSQL(csvText, table, dialect string, createTable bool) (string, error) {
	table = strings.TrimSpace(table)
	if table == "" {
		return "", fmt.Errorf("table name is required")
	}
	rows, err := parseCSV(csvText)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", fmt.Errorf("no rows found")
	}
	quote := "\""
	if strings.EqualFold(dialect, "mysql") {
		quote = "`"
	}
	q := func(s string) string { return quote + s + quote }

	header := rows[0]
	cols := make([]string, len(header))
	for i, h := range header {
		cols[i] = q(strings.TrimSpace(h))
	}

	var sb strings.Builder
	if createTable {
		sb.WriteString("CREATE TABLE " + q(table) + " (\n")
		for i, c := range cols {
			comma := ","
			if i == len(cols)-1 {
				comma = ""
			}
			sb.WriteString("  " + c + " TEXT" + comma + "\n")
		}
		sb.WriteString(");\n\n")
	}

	colList := strings.Join(cols, ", ")
	for _, row := range rows[1:] {
		vals := make([]string, 0, len(row))
		for _, v := range row {
			vals = append(vals, sqlLiteral(v))
		}
		sb.WriteString("INSERT INTO " + q(table) + " (" + colList + ") VALUES (" + strings.Join(vals, ", ") + ");\n")
	}
	return strings.TrimRight(sb.String(), "\n"), nil
}

func sqlLiteral(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "NULL"
	}
	low := strings.ToLower(v)
	if low == "null" {
		return "NULL"
	}
	if _, ok := parseAllInt(v); ok {
		return v
	}
	if isNumericLiteral(v) {
		return v
	}
	escaped := strings.ReplaceAll(v, "'", "''")
	return "'" + escaped + "'"
}

func isNumericLiteral(s string) bool {
	if s == "" {
		return false
	}
	dot := false
	for i, r := range s {
		if r == '-' && i == 0 {
			continue
		}
		if r == '.' && !dot {
			dot = true
			continue
		}
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
