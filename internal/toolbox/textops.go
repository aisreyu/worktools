package toolbox

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ---- text / data transforms ----

// SplitLines splits input into non-empty trimmed lines.
func SplitLines(input string) []string {
	var out []string
	for _, ln := range strings.Split(strings.ReplaceAll(input, "\r\n", "\n"), "\n") {
		ln = strings.TrimSpace(ln)
		if ln != "" {
			out = append(out, ln)
		}
	}
	return out
}

// RowsToColumns transposes row-oriented text (one record per line, fields
// separated by sep) into column-oriented text: field i of every record
// becomes line i. Ragged rows are padded with empty strings.
func RowsToColumns(input, sep string) string {
	rows := SplitLines(input)
	if len(rows) == 0 {
		return ""
	}
	if sep == "" {
		sep = "\t"
	}
	grid := make([][]string, len(rows))
	maxCols := 0
	for i, r := range rows {
		grid[i] = strings.Split(r, sep)
		if len(grid[i]) > maxCols {
			maxCols = len(grid[i])
		}
	}
	var b strings.Builder
	for c := 0; c < maxCols; c++ {
		cells := make([]string, len(grid))
		for i := range grid {
			if c < len(grid[i]) {
				cells[i] = strings.TrimSpace(grid[i][c])
			}
		}
		b.WriteString(strings.Join(cells, "\t"))
		b.WriteString("\n")
	}
	return b.String()
}

// ColumnsToRows is the inverse of RowsToColumns: line i holds field i of
// every record, so the output is one record per line with fields joined by
// sep.
func ColumnsToRows(input, sep string) string {
	lines := SplitLines(input)
	if len(lines) == 0 {
		return ""
	}
	if sep == "" {
		sep = "\t"
	}
	grid := make([][]string, len(lines))
	maxCols := 0
	for i, ln := range lines {
		grid[i] = strings.Split(ln, "\t")
		if len(grid[i]) > maxCols {
			maxCols = len(grid[i])
		}
	}
	var b strings.Builder
	for c := 0; c < maxCols; c++ {
		cells := make([]string, len(grid))
		for i := range grid {
			if c < len(grid[i]) {
				cells[i] = strings.TrimSpace(grid[i][c])
			}
		}
		b.WriteString(strings.Join(cells, sep))
		b.WriteString("\n")
	}
	return b.String()
}

// JoinLines joins non-empty lines with the given separator. quote wraps
// every item: "" none, "'" single, "\"" double, "`" backtick, "(" bracket,
// "[" square bracket.
func JoinLines(input, sep, quote string) string {
	items := SplitLines(input)
	for i, it := range items {
		items[i] = wrapQuote(it, quote)
	}
	return strings.Join(items, sep)
}

// QuoteLines wraps every non-empty line in the chosen quote character and
// joins them with sep (default ","). Useful to build SQL IN lists.
func QuoteLines(input, sep, quote string) string {
	if sep == "" {
		sep = ","
	}
	return JoinLines(input, sep, quote)
}

func wrapQuote(s, quote string) string {
	switch quote {
	case "'":
		return "'" + strings.ReplaceAll(s, "'", "\\'") + "'"
	case "\"":
		return "\"" + strings.ReplaceAll(s, "\"", "\\\"") + "\""
	case "`":
		return "`" + s + "`"
	case "(":
		return "(" + s + ")"
	case "[":
		return "[" + s + "]"
	default:
		return s
	}
}

// DedupeLines removes duplicate lines, keeping first occurrences. If sortOut
// is set the result is sorted.
func DedupeLines(input string, sortOut bool) string {
	items := SplitLines(input)
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, it := range items {
		if _, ok := seen[it]; ok {
			continue
		}
		seen[it] = struct{}{}
		out = append(out, it)
	}
	if sortOut {
		sortStrings(out)
	}
	return strings.Join(out, "\n")
}

// TextToJSON converts row-oriented text into a JSON array of objects. The
// first line is the header; each following line is a record. Fields are
// split by sep (default tab). If noHeader is set, keys are "c1", "c2", …
func TextToJSON(input, sep string, noHeader bool) (string, error) {
	lines := SplitLines(input)
	if len(lines) == 0 {
		return "[]", nil
	}
	if sep == "" {
		sep = "\t"
	}
	var header []string
	start := 0
	if noHeader {
		n := len(strings.Split(lines[0], sep))
		for i := 1; i <= n; i++ {
			header = append(header, fmt.Sprintf("c%d", i))
		}
	} else {
		header = strings.Split(lines[0], sep)
		for i := range header {
			header[i] = strings.TrimSpace(header[i])
		}
		start = 1
	}
	var records []map[string]string
	for _, ln := range lines[start:] {
		fields := strings.Split(ln, sep)
		rec := make(map[string]string, len(header))
		for i, h := range header {
			v := ""
			if i < len(fields) {
				v = strings.TrimSpace(fields[i])
			}
			rec[h] = v
		}
		records = append(records, rec)
	}
	b, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// JSONToText flattens a JSON array of objects (or an object) into
// row-oriented text: header line of keys, then one line per record with
// values joined by sep (default tab).
func JSONToText(input, sep string) (string, error) {
	if sep == "" {
		sep = "\t"
	}
	var data interface{}
	if err := json.Unmarshal([]byte(input), &data); err != nil {
		return "", fmt.Errorf("invalid json: %w", err)
	}
	arr, ok := data.([]interface{})
	if !ok {
		arr = []interface{}{data}
	}
	var header []string
	seen := make(map[string]struct{})
	var rows [][]string
	for _, item := range arr {
		obj, ok := item.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("json is not an array of objects")
		}
		// stable key order: first appearance
		var row []string
		for _, k := range sortedKeys(obj) {
			if _, dup := seen[k]; !dup {
				seen[k] = struct{}{}
				header = append(header, k)
			}
			v := obj[k]
			row = append(row, fmt.Sprint(v))
		}
		rows = append(rows, row)
	}
	// rebuild rows in header order
	var b strings.Builder
	b.WriteString(strings.Join(header, sep))
	b.WriteString("\n")
	for _, obj := range arr {
		m := obj.(map[string]interface{})
		cells := make([]string, len(header))
		for i, h := range header {
			if v, ok := m[h]; ok {
				cells[i] = fmt.Sprint(v)
			}
		}
		b.WriteString(strings.Join(cells, sep))
		b.WriteString("\n")
	}
	return b.String(), nil
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortStrings(keys)
	return keys
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
