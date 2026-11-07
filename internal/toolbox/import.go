package toolbox

import (
	"strings"
)

// ImportedCredential is one credential parsed from an external CSV export.
type ImportedCredential struct {
	Title    string `json:"title"`
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
	Note     string `json:"note"`
}

// ImportedCredentials is the outcome of parsing an external export.
type ImportedCredentials struct {
	Source string               `json:"source"`
	Items  []ImportedCredential `json:"items"`
}

// headerAliases maps known column names (lower-cased) to canonical fields.
var importAliases = map[string]string{
	"title": "title", "name": "title", "account": "title", "item": "title",
	"url": "url", "web site": "url", "website": "url", "login uri": "url", "uri": "url",
	"username": "username", "login name": "username", "user name": "username",
	"login": "username", "email": "username", "user": "username",
	"password": "password", "pass": "password",
	"note": "note", "notes": "note", "comments": "note", "comment": "note",
	"tags": "note", "extra": "note",
}

// ParseImportCSV parses a KeePass / Chrome / 1Password style CSV export. It
// auto-detects the header row and maps columns by common aliases, so a single
// implementation covers all three exporters.
func ParseImportCSV(text, source string) (ImportedCredentials, error) {
	rows, err := parseCSV(text)
	if err != nil {
		return ImportedCredentials{}, err
	}
	if len(rows) == 0 {
		return ImportedCredentials{}, nil
	}
	header := rows[0]
	fieldOf := make([]string, len(header))
	for i, h := range header {
		key := strings.ToLower(strings.TrimSpace(h))
		key = strings.TrimPrefix(key, "\ufeff")
		fieldOf[i] = importAliases[key]
	}
	out := ImportedCredentials{Source: source, Items: make([]ImportedCredential, 0)}
	for _, row := range rows[1:] {
		var c ImportedCredential
		for i, v := range row {
			if i >= len(fieldOf) {
				break
			}
			switch fieldOf[i] {
			case "title":
				c.Title = v
			case "url":
				c.URL = v
			case "username":
				c.Username = v
			case "password":
				c.Password = v
			case "note":
				c.Note = v
			}
		}
		if strings.TrimSpace(c.Title) == "" && strings.TrimSpace(c.URL) == "" && strings.TrimSpace(c.Password) == "" {
			continue
		}
		if strings.TrimSpace(c.Title) == "" {
			c.Title = firstNonEmpty(c.URL, c.Username, "imported_entry")
		}
		out.Items = append(out.Items, c)
	}
	return out, nil
}
