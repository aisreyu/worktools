package toolbox

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"
)

// WhoisResult is a parsed WHOIS/RDAP response.
type WhoisResult struct {
	Query          string            `json:"query"`
	Server         string            `json:"server"`
	Raw            string            `json:"raw"`
	Fields         map[string]string `json:"fields"`
	Registrar      string            `json:"registrar"`
	Created        string            `json:"created"`
	Updated        string            `json:"updated"`
	Expires        string            `json:"expires"`
	NameServers    []string          `json:"nameServers"`
	Status         []string          `json:"status"`
	Registrant     string            `json:"registrant"`
	DaysToExpiry   int               `json:"daysToExpiry"`
	ExpiryKnown    bool              `json:"expiryKnown"`
}

var whoisFieldLabels = map[string]string{
	"domain name": "domain",
	"registrar": "registrar",
	"registrar url": "registrar_url",
	"registrant organization": "registrant_org",
	"registrant": "registrant",
	"registrant name": "registrant",
	"org": "registrant_org",
	"creation date": "created",
	"created": "created",
	"created date": "created",
	"registry expiry date": "expires",
	"registrar registration expiration date": "expires",
	"expiry date": "expires",
	"expiration date": "expires",
	"expires": "expires",
	"updated date": "updated",
	"last updated": "updated",
	"domain status": "status",
	"status": "status",
	"name server": "name_server",
	"nserver": "name_server",
	"dnssec":                            "DNSSEC",
	"registrar abuse contact email": "abuse_email",
}

var (
	reNameServer = regexp.MustCompile(`(?i)^\s*nserver:\s*(.+)$`)
	reStatus     = regexp.MustCompile(`(?i)^\s*(?:domain )?status:\s*(.+)$`)
)

// Whois queries the authoritative WHOIS server for domain and parses common
// fields (registrar, key dates, name servers, status).
func Whois(ctx context.Context, domain string, timeoutMS int) (WhoisResult, error) {
	domain = strings.TrimSpace(domain)
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimPrefix(domain, "https://")
	if i := strings.IndexAny(domain, "/ "); i >= 0 {
		domain = domain[:i]
	}
	domain = strings.TrimSuffix(domain, ".")
	if domain == "" {
		return WhoisResult{}, fmt.Errorf("whois_domain_empty")
	}
	if timeoutMS <= 0 {
		timeoutMS = 10000
	}
	deadline := time.Now().Add(time.Duration(timeoutMS) * time.Millisecond)

	server := discoverWhoisServer(ctx, domain, deadline)
	raw, err := whoisQuery(ctx, server, domain, deadline)
	if err != nil {
		return WhoisResult{}, err
	}
	if server == "" {
		server = "whois.iana.org"
	}
	return parseWhois(domain, server, raw), nil
}

// discoverWhoisServer asks IANA which WHOIS server handles the TLD, falling
// back to whois.iana.org's own referral.
func discoverWhoisServer(ctx context.Context, domain string, deadline time.Time) string {
	tld := domain
	if i := strings.LastIndex(domain, "."); i >= 0 {
		tld = domain[i+1:]
	}
	raw, err := whoisQuery(ctx, "whois.iana.org", tld, deadline)
	if err != nil {
		return ""
	}
	for _, ln := range strings.Split(raw, "\n") {
		ln = strings.TrimSpace(ln)
		if strings.HasPrefix(strings.ToLower(ln), "whois:") {
			return strings.TrimSpace(ln[len("whois:"):])
		}
		if strings.HasPrefix(strings.ToLower(ln), "refer:") {
			return strings.TrimSpace(ln[len("refer:"):])
		}
	}
	return ""
}

func whoisQuery(ctx context.Context, server, query string, deadline time.Time) (string, error) {
	if server == "" {
		return "", fmt.Errorf("whois_no_server")
	}
	host := server
	if !strings.Contains(host, ":") {
		host += ":43"
	}
	d := net.Dialer{Deadline: deadline}
	conn, err := d.DialContext(ctx, "tcp", host)
	if err != nil {
		return "", fmt.Errorf("whois_connect_failed: %s: %s", server, err.Error())
	}
	defer conn.Close()
	_ = conn.SetDeadline(deadline)
	if _, err := fmt.Fprintf(conn, "%s\r\n", query); err != nil {
		return "", err
	}
	var b strings.Builder
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		b.WriteString(sc.Text())
		b.WriteString("\n")
	}
	if b.Len() == 0 {
		return "", fmt.Errorf("whois_empty_response")
	}
	return b.String(), nil
}

func parseWhois(query, server, raw string) WhoisResult {
	res := WhoisResult{
		Query:  query,
		Server: server,
		Raw:    raw,
		Fields: map[string]string{},
	}
	statusSeen := map[string]bool{}
	nsSeen := map[string]bool{}
	for _, ln := range strings.Split(raw, "\n") {
		line := strings.TrimSpace(ln)
		if line == "" || strings.HasPrefix(line, "%") || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		val = strings.TrimSpace(val)
		if val == "" {
			continue
		}
		if label, ok := whoisFieldLabels[key]; ok {
			if _, exists := res.Fields[label]; !exists {
				res.Fields[label] = val
			}
		}
		if reNameServer.MatchString(line) {
			m := reNameServer.FindStringSubmatch(line)
			if len(m) == 2 {
				ns := strings.ToLower(strings.TrimSpace(m[1]))
				if !nsSeen[ns] {
					nsSeen[ns] = true
					res.NameServers = append(res.NameServers, ns)
				}
			}
			continue
		}
		if reStatus.MatchString(line) {
			m := reStatus.FindStringSubmatch(line)
			if len(m) == 2 {
				st := strings.TrimSpace(m[1])
				if !statusSeen[st] {
					statusSeen[st] = true
					res.Status = append(res.Status, st)
				}
			}
			continue
		}
	}
	res.Registrar = firstNonEmpty(res.Fields["registrar"], res.Fields["registrar_url"])
	res.Created = res.Fields["created"]
	res.Updated = res.Fields["updated"]
	res.Expires = res.Fields["expires"]
	res.Registrant = firstNonEmpty(res.Fields["registrant_org"], res.Fields["registrant"])
	if res.Expires != "" {
		if t, ok := parseLooseDate(res.Expires); ok {
			res.ExpiryKnown = true
			res.DaysToExpiry = int(time.Until(t).Hours() / 24)
		}
	}
	return res
}

// parseLooseDate tries a handful of date layouts commonly found in WHOIS
// responses.
func parseLooseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	// Strip a possible trailing timezone label in parentheses.
	if i := strings.Index(s, "("); i > 0 {
		s = strings.TrimSpace(s[:i])
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"02-Jan-2006",
		"2006.01.02",
		"02.01.2006",
		"20060102",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
