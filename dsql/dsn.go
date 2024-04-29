package dsql

import (
	"fmt"
	"log"
	"net/url"
	"strconv"
	"time"
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

	// UseSSL determines whether to use SSL or not
	UseSSL bool

	Timeout time.Duration
}

// FormatDSN formats a data source name from a config struct
func (c *Config) FormatDSN() (dsn string) {
	if c.BrokerAddr == "" {
		log.Fatal("druid: you must specify a broker addr")
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

	sslEnabled := "false"
	if c.UseSSL {
		sslEnabled = "true"
	}

	return fmt.Sprintf("%s%s?pingEndpoint=%s&queryEndpoint=%s&sslenable=%s", auth, c.BrokerAddr, pingEndpoint, queryEndpoint, sslEnabled)
}

// ParseDSN returns a config struct from a dsn string
func ParseDSN(dsn string) *Config {
	cfg := &Config{}
	u, err := url.Parse(dsn)
	if err != nil {
		log.Println("dsn:", dsn)
		log.Fatal("error parsing dsn", err)
	}

	q := u.Query()

	isHttps := false
	if ssl, ok := q["sslenable"]; ok {
		if ssl[0] == "true" {
			isHttps = true
		}
	}

	u.Scheme = "http"
	if isHttps {
		u.Scheme = "https"

	}
	cfg.PingEndpoint = q.Get("pingEndpoint")
	cfg.QueryEndpoint = q.Get("queryEndpoint")
	if t := q.Get("timeout"); t != "" {
		if timeout, err := strconv.ParseInt(t, 10, 64); err != nil {
			log.Fatal("error parsing dsn timeout", err)
		} else {
			cfg.Timeout = time.Duration(timeout) * time.Second
		}
	}
	cfg.User = u.User.Username()
	pass, _ := u.User.Password()
	cfg.Passwd = pass

	credentials := ""
	if cfg.User != "" && cfg.Passwd != "" {
		credentials = fmt.Sprintf("%s:%s@", cfg.User, cfg.Passwd)
	}

	cfg.BrokerAddr = fmt.Sprintf("%s://%s%s%s", u.Scheme, credentials, u.Hostname(), u.Path)
	if u.Port() != "" {
		cfg.BrokerAddr = fmt.Sprintf("%s://%s%s:%s%s", u.Scheme, credentials, u.Hostname(), u.Port(), u.Path)
	}

	return cfg
}
