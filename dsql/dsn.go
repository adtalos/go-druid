package dsql

import (
	"fmt"
	"log"
	"net/url"
)

// Config represents a struct to a druid database
type Config struct {
	User          string
	Passwd        string
	BrokerAddr    string
	PingEndpoint  string
	QueryEndpoint string

	// DateFormat for the date field, i.e iso, auto etc
	DateFormat string

	// DateField field to use as the date field
	DateField string
}

// FormatDSN formats a data source name from a config struct
func (c *Config) FormatDSN() (dsn string) {
	if c.BrokerAddr == "" {
		log.Fatal("druid: you must specify a brokeraddr")
	}

	var auth string
	if c.User != "" && c.Passwd != "" {
		auth = fmt.Sprintf("%s:%s@", c.User, c.Passwd)
	}

	pingEndpoint := c.PingEndpoint
	if pingEndpoint == "" {
		pingEndpoint = "/status/health"
	}

	queryEndpoint := c.QueryEndpoint
	if queryEndpoint == "" {
		queryEndpoint = "/druid/v2/sql"
	}

	return fmt.Sprintf("druid://%s%s?pingEndpoint=%s&queryEndpoint=%s", auth, c.BrokerAddr, pingEndpoint, queryEndpoint)
}

// ParseDSN returns a config struct from a dsn string
func ParseDSN(dsn string) *Config {
	cfg := &Config{}
	u, err := url.Parse(dsn)
	if err != nil {
		log.Fatal("error parsing dsn", err)
	}

	// TODO: logic to use https if specified
	u.Scheme = "http"
	q := u.Query()

	cfg.BrokerAddr = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	cfg.PingEndpoint = q.Get("pingEndpoint")
	cfg.QueryEndpoint = q.Get("queryEndpoint")
	cfg.User = u.User.Username()
	pass, _ := u.User.Password()
	cfg.Passwd = pass

	return cfg
}
