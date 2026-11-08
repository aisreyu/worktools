package toolbox

import (
	"sort"
	"strings"
)

// SearchDoc is one indexable item for cross-module quick search.
type SearchDoc struct {
	Kind     string `json:"kind"` // url | secret | otp | note | schedule | file | clipboard | tool
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	Body     string `json:"body"`
	Route    string `json:"route"`
}

// SearchHit is a ranked search result.
type SearchHit struct {
	Kind     string `json:"kind"`
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	Route    string `json:"route"`
	Score    int    `json:"score"`
}

// SearchDocs ranks docs against a free-text query. Terms are whitespace
// separated and must all appear (AND). Title matches weigh more than subtitle,
// which weighs more than body text. When the query is empty every doc is
// returned (alphabetically) so the palette can show a default list.
func SearchDocs(query string, docs []SearchDoc, limit int) []SearchHit {
	if limit <= 0 {
		limit = 50
	}
	q := strings.ToLower(strings.TrimSpace(query))
	terms := strings.Fields(q)
	hits := make([]SearchHit, 0, len(docs))
	for _, d := range docs {
		title := strings.ToLower(d.Title)
		sub := strings.ToLower(d.Subtitle)
		body := strings.ToLower(d.Body)
		if len(terms) == 0 {
			hits = append(hits, SearchHit{Kind: d.Kind, ID: d.ID, Title: d.Title, Subtitle: d.Subtitle, Route: d.Route, Score: 1})
			continue
		}
		score := 0
		matchedAll := true
		for _, term := range terms {
			ts := 0
			if strings.Contains(title, term) {
				ts += 6
			}
			if strings.Contains(sub, term) {
				ts += 3
			}
			if strings.Contains(body, term) {
				ts += 1
			}
			if ts == 0 {
				matchedAll = false
				break
			}
			score += ts
		}
		if !matchedAll {
			continue
		}
		hits = append(hits, SearchHit{Kind: d.Kind, ID: d.ID, Title: d.Title, Subtitle: d.Subtitle, Route: d.Route, Score: score})
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return strings.ToLower(hits[i].Title) < strings.ToLower(hits[j].Title)
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits
}
