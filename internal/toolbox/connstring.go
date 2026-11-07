package toolbox

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// ConnString is a parsed connection string with its scheme-specific fields.
type ConnString struct {
	Kind     string            `json:"kind"`
	Scheme   string            `json:"scheme"`
	Host     string            `json:"host"`
	Port     int               `json:"port"`
	Database string            `json:"database"`
	Username string            `json:"username"`
	Password string            `json:"password"`
	Params   map[string]string `json:"params"`
	Raw      string            `json:"raw"`
}

var connDefaultPorts = map[string]int{
	"mysql":    3306,
	"mariadb":  3306,
	"postgres": 5432,
	"postgresql": 5432,
	"redis":    6379,
	"rediss":   6379,
	"mongodb":  27017,
	"mongodb+srv": 27017,
	"kafka":    9092,
	"amqp":     5672,
	"amqps":    5671,
	"clickhouse": 9000,
	"rocketmq": 9876,
}

// ParseConnString parses a database/messaging connection URL into structured
// fields. Supported schemes: mysql, mariadb, postgres(ql), redis(s),
// mongodb(+srv), kafka, amqp(s), clickhouse, rocketmq.
func ParseConnString(raw string) (ConnString, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ConnString{}, fmt.Errorf("connstring_empty")
	}
	scheme := ""
	if i := strings.Index(raw, "://"); i > 0 {
		scheme = strings.ToLower(raw[:i])
	} else {
		return ConnString{}, fmt.Errorf("connstring_no_scheme")
	}
	kind := normalizeConnKind(scheme)
	if _, ok := connDefaultPorts[scheme]; !ok && kind == scheme {
		// Unknown but parse generically anyway.
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ConnString{}, fmt.Errorf("connstring_invalid: %s", err.Error())
	}
	out := ConnString{
		Kind:   kind,
		Scheme: scheme,
		Params: map[string]string{},
		Raw:    raw,
	}
	host := u.Hostname()
	out.Host = host
	out.Port = connDefaultPorts[scheme]
	if p := u.Port(); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			out.Port = n
		}
	}
	if u.User != nil {
		out.Username = u.User.Username()
		if pw, ok := u.User.Password(); ok {
			out.Password = pw
		}
	}
	// Path holds the database (or topic list for Kafka).
	if p := strings.TrimPrefix(u.Path, "/"); p != "" {
		out.Database = p
	}
	for k, vs := range u.Query() {
		out.Params[k] = strings.Join(vs, ",")
	}
	return out, nil
}

func normalizeConnKind(scheme string) string {
	switch scheme {
	case "postgresql":
		return "postgres"
	case "rediss":
		return "redis"
	case "mariadb":
		return "mysql"
	case "amqps":
		return "amqp"
	case "mongodb+srv":
		return "mongodb"
	default:
		return scheme
	}
}

// BuildConnString is the inverse of ParseConnString. The Kind selects the
// scheme; if Scheme is set it is used directly.
func BuildConnString(in ConnString) (string, error) {
	scheme := strings.ToLower(strings.TrimSpace(in.Scheme))
	if scheme == "" {
		scheme = strings.ToLower(strings.TrimSpace(in.Kind))
	}
	if scheme == "" {
		return "", fmt.Errorf("connstring_need_kind")
	}
	host := strings.TrimSpace(in.Host)
	if host == "" {
		return "", fmt.Errorf("connstring_need_host")
	}
	port := in.Port
	if port == 0 {
		port = connDefaultPorts[scheme]
	}
	u := url.URL{Scheme: scheme, Host: host}
	if port > 0 {
		u.Host = host + ":" + strconv.Itoa(port)
	}
	if in.Username != "" || in.Password != "" {
		u.User = url.UserPassword(in.Username, in.Password)
	}
	if in.Database != "" {
		u.Path = "/" + strings.TrimPrefix(in.Database, "/")
	}
	if len(in.Params) > 0 {
		q := url.Values{}
		keys := make([]string, 0, len(in.Params))
		for k := range in.Params {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			q.Set(k, in.Params[k])
		}
		u.RawQuery = q.Encode()
	}
	return u.String(), nil
}
