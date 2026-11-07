package toolbox

import (
	"strings"
)

// DiffLine is one line of a unified-style diff.
type DiffLine struct {
	Type string `json:"type"` // "same", "add", "del"
	A    int    `json:"a"`    // 1-based line number in the original (0 if none)
	B    int    `json:"b"`    // 1-based line number in the modified (0 if none)
	Text string `json:"text"`
}

// DiffResult summarises a line diff.
type DiffResult struct {
	Lines   []DiffLine `json:"lines"`
	Added   int        `json:"added"`
	Deleted int        `json:"deleted"`
	Same    int        `json:"same"`
}

// TextDiff computes a line-level LCS diff between two texts.
func TextDiff(a, b string) DiffResult {
	al := splitLinesKeep(a)
	bl := splitLinesKeep(b)
	lcs := lcsTable(al, bl)

	res := DiffResult{Lines: make([]DiffLine, 0)}
	i, j := 0, 0
	for i < len(al) && j < len(bl) {
		switch {
		case al[i] == bl[j]:
			res.Lines = append(res.Lines, DiffLine{Type: "same", A: i + 1, B: j + 1, Text: al[i]})
			res.Same++
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			res.Lines = append(res.Lines, DiffLine{Type: "del", A: i + 1, Text: al[i]})
			res.Deleted++
			i++
		default:
			res.Lines = append(res.Lines, DiffLine{Type: "add", B: j + 1, Text: bl[j]})
			res.Added++
			j++
		}
	}
	for ; i < len(al); i++ {
		res.Lines = append(res.Lines, DiffLine{Type: "del", A: i + 1, Text: al[i]})
		res.Deleted++
	}
	for ; j < len(bl); j++ {
		res.Lines = append(res.Lines, DiffLine{Type: "add", B: j + 1, Text: bl[j]})
		res.Added++
	}
	return res
}

func splitLinesKeep(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// lcsTable builds the classic dynamic-programming LCS length table.
func lcsTable(a, b []string) [][]int {
	n, m := len(a), len(b)
	t := make([][]int, n+1)
	for i := range t {
		t[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				t[i][j] = t[i+1][j+1] + 1
			} else if t[i+1][j] >= t[i][j+1] {
				t[i][j] = t[i+1][j]
			} else {
				t[i][j] = t[i][j+1]
			}
		}
	}
	return t
}

// UnifiedDiff renders the diff in a compact unified format.
func UnifiedDiff(a, b string) string {
	r := TextDiff(a, b)
	var sb strings.Builder
	sb.WriteString("--- original\n+++ modified\n")
	for _, l := range r.Lines {
		switch l.Type {
		case "same":
			sb.WriteString("  " + l.Text + "\n")
		case "add":
			sb.WriteString("+ " + l.Text + "\n")
		case "del":
			sb.WriteString("- " + l.Text + "\n")
		}
	}
	return sb.String()
}
