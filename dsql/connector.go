package dsql

import (
	"context"
	"database/sql/driver"
	"net/http"
)

type connector struct {
	Cfg *Config
}

// Connect implements db.Connector and sets up an http client to druid's sql endpoint
func (c *connector) Connect(ctx context.Context) (driver.Conn, error) {
	connection := &connection{
		Client: &http.Client{
			Timeout: c.Cfg.Timeout,
		},
		Cfg: c.Cfg,
	}
	return connection, nil
}

// Driver implements db.Connector and returns a druid driver
func (c *connector) Driver() (d driver.Driver) {
	return &Driver{}
}
