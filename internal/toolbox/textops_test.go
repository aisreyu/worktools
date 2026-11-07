package toolbox

import (
	"strings"
	"testing"
)

func TestRowsToColumns(t *testing.T) {
	in := "a1\tb1\tc1\na2\tb2\tc2\na3\tb3\tc3"
	want := "a1\ta2\ta3\nb1\tb2\tb3\nc1\tc2\tc3\n"
	if got := RowsToColumns(in, "\t"); got != want {
		t.Errorf("got %q want %q", got, want)
	}
	// ragged rows padded
	got := RowsToColumns("a\tb\n c", "\t")
	if !strings.Contains(got, "a\tc") || !strings.Contains(got, "b\t") {
		t.Errorf("ragged: %q", got)
	}
}

func TestColumnsToRows(t *testing.T) {
	in := "a1\ta2\ta3\nb1\tb2\tb3"
	want := "a1,b1,c1" // placeholder, verify below
	_ = want
	got := ColumnsToRows(in, ",")
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != 3 {
		t.Fatalf("want 3 rows, got %d: %q", len(lines), got)
	}
	if lines[0] != "a1,b1" || lines[1] != "a2,b2" || lines[2] != "a3,b3" {
		t.Errorf("rows: %q", lines)
	}
}

func TestTransposeRoundTrip(t *testing.T) {
	in := "a\tb\tc\n1\t2\t3\nx\ty\tz"
	once := RowsToColumns(in, "\t")
	back := ColumnsToRows(once, "\t")
	if back != in+"\n" {
		t.Errorf("round trip:\n%q\n%q", in+"\n", back)
	}
}

func TestJoinAndQuote(t *testing.T) {
	in := "apple\nbanana\ncherry"
	if got := JoinLines(in, ", ", "\""); got != `"apple", "banana", "cherry"` {
		t.Errorf("double quote: %q", got)
	}
	if got := QuoteLines(in, ",", "'"); got != `'apple','banana','cherry'` {
		t.Errorf("single quote: %q", got)
	}
	if got := QuoteLines(in, ",", "("); got != "(apple),(banana),(cherry)" {
		t.Errorf("bracket: %q", got)
	}
	// embedded quote escaped
	if got := QuoteLines("it's\nok", ",", "'"); got != `'it\'s','ok'` {
		t.Errorf("escape: %q", got)
	}
}

func TestDedupeLines(t *testing.T) {
	in := "b\na\nb\nc\na"
	if got := DedupeLines(in, false); got != "b\na\nc" {
		t.Errorf("dedupe: %q", got)
	}
	if got := DedupeLines(in, true); got != "a\nb\nc" {
		t.Errorf("dedupe sorted: %q", got)
	}
}

func TestTextToJSON(t *testing.T) {
	in := "name\tage\nalice\t30\nbob\t25"
	got, err := TextToJSON(in, "\t", false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"name": "alice"`) || !strings.Contains(got, `"age": "30"`) {
		t.Errorf("json: %s", got)
	}
	// no header → c1/c2 keys
	got, err = TextToJSON("x\ty", "\t", true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"c1": "x"`) || !strings.Contains(got, `"c2": "y"`) {
		t.Errorf("no-header json: %s", got)
	}
}

func TestJSONToText(t *testing.T) {
	in := `[{"name":"alice","age":30},{"name":"bob","age":25}]`
	got, err := JSONToText(in, "\t")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != 3 {
		t.Fatalf("want 3 lines, got %d: %q", len(lines), got)
	}
	if lines[0] != "age\tname" {
		t.Errorf("header: %q", lines[0])
	}
	if lines[1] != "30\talice" {
		t.Errorf("row1: %q", lines[1])
	}
}
