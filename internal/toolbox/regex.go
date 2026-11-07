package toolbox

import (
	"regexp"
)

// RegexMatch is a single regular-expression match with capture groups.
type RegexMatch struct {
	Index  int      `json:"index"`
	Text   string   `json:"text"`
	Groups []string `json:"groups"`
	Named  map[string]string `json:"named,omitempty"`
}

// RegexResult is the outcome of running a pattern against input text.
type RegexResult struct {
	Matches []RegexMatch `json:"matches"`
	Count   int          `json:"count"`
	Error   string       `json:"error,omitempty"`
}

// RegexTest runs pattern against input and returns all matches. When
// ignoreCase is set the (?i) flag is prepended.
func RegexTest(pattern, input string, ignoreCase bool) RegexResult {
	if ignoreCase {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return RegexResult{Error: err.Error()}
	}
	locs := re.FindAllStringSubmatchIndex(input, -1)
	out := RegexResult{Matches: make([]RegexMatch, 0, len(locs))}
	names := re.SubexpNames()
	for i, loc := range locs {
		m := RegexMatch{Index: i + 1, Groups: make([]string, 0, len(loc)/2)}
		for g := 0; g < len(loc); g += 2 {
			start, end := loc[g], loc[g+1]
			text := ""
			if start >= 0 && end >= 0 {
				text = input[start:end]
			}
			if g == 0 {
				m.Text = text
			}
			m.Groups = append(m.Groups, text)
			if g/2 < len(names) && names[g/2] != "" {
				if m.Named == nil {
					m.Named = map[string]string{}
				}
				m.Named[names[g/2]] = text
			}
		}
		out.Matches = append(out.Matches, m)
	}
	out.Count = len(out.Matches)
	return out
}

// RegexReplace replaces all matches of pattern in input with repl.
func RegexReplace(pattern, input, repl string, ignoreCase bool) (string, error) {
	if ignoreCase {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", err
	}
	return re.ReplaceAllString(input, repl), nil
}
