package dsql

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
)

var (
	// ErrPinging is an error returned when health check endpoint returns a non 200.
	ErrPinging = errors.New("druid: error fetching health info from druid")

	// ErrCancelled is an error returned when we receive a cancellation event from a context object
	ErrCancelled = errors.New("druid: cancellation received")

	// ErrRequestForm is an error returned when failing to form a request
	ErrRequestForm = errors.New("druid: error forming request")

	// ErrCreatingRequest is an error when creating a new request
	ErrCreatingRequest = errors.New("druid: error creating new request")

	// ErrMakingRequest is an error whilst making a request to the druid server itself
	ErrMakingRequest = errors.New("druid: error making request to druid server")

	ErrArgsNotImplement = errors.New("druid: args not implement")
)

func wrapErr(a, b error) error {
	return fmt.Errorf("%v: %v", a, b)
}

type connection struct {
	Client *http.Client
	Cfg    *Config
}

type queryRequest struct {
	Query        string `json:"query"`
	ResultFormat string `json:"resultFormat"`
	Header       bool   `json:"header"`
}

type queryResponse [][]interface{}

// Prepare implements db.Conn.Prepare and returns a noop statement
func (c *connection) Prepare(stmt string) (driver.Stmt, error) {
	return &statementNoop{}, driver.ErrSkip
}

// Close closes a connection.
func (c *connection) Close() (err error) {
	return
}

// Begin implements db.Conn.Prepare and is a noop
func (c *connection) Begin() (tx driver.Tx, err error) {
	tx = &transactionNoop{}
	return tx, driver.ErrSkip
}

// Ping implements db.conn.Prepare and hits the health endpoint of a broker
func (c *connection) Ping(ctx context.Context) error {
	res, err := c.Client.Get(fmt.Sprintf("%s%s", c.Cfg.BrokerAddr, c.Cfg.PingEndpoint))
	if err != nil {
		return wrapErr(ErrPinging, err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return wrapErr(ErrPinging, err)
	}

	return nil
}

func (c *connection) Query(q string, args []driver.Value) (driver.Rows, error) {
	if len(args) > 0 {
		return &rows{}, ErrArgsNotImplement
	}
	req, err := c.makeRequest(q)
	if err != nil {
		return &rows{}, err
	}
	return c.query(req)
}

func (c *connection) QueryContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	if len(args) > 0 {
		return &rows{}, ErrArgsNotImplement
	}
	req, err := c.makeRequest(q)
	if err != nil {
		return &rows{}, err
	}
	return c.query(req.WithContext(ctx))
}

func (c *connection) makeRequest(q string) (*http.Request, error) {
	queryURL := fmt.Sprintf("%s%s", c.Cfg.BrokerAddr, c.Cfg.QueryEndpoint)
	request := &queryRequest{
		Query:        q,
		ResultFormat: "arrayLines",
		Header:       true,
	}

	payload, err := json.Marshal(request)
	if err != nil {
		return nil, wrapErr(ErrRequestForm, err)
	}

	req, err := http.NewRequest(http.MethodPost, queryURL, bytes.NewReader(payload))
	if err != nil {
		return nil, wrapErr(ErrRequestForm, err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("Connection", "keep-alive")

	return req, nil
}

func (c *connection) parseJSONResponse(body *bufio.Reader) (queryResponse, error) {
	var results queryResponse

	in := bufio.NewScanner(body)

	for in.Scan() {
		line := in.Bytes()

		if len(line) > 0 {
			var row []interface{}
			err := json.Unmarshal(in.Bytes(), &row)
			if err != nil {
				return results, err
			}

			results = append(results, row)
		}
	}

	if err := in.Err(); err != nil {
		return results, err
	}

	return results, nil
}

func (c *connection) parseResponse(body *bufio.Reader) (r *rows, err error) {
	results, err := c.parseJSONResponse(body)

	if err != nil {
		return &rows{}, err
	}

	// No results returned
	if len(results) == 0 {
		return &rows{}, sql.ErrNoRows
	}

	var columnNames []string
	for _, val := range results[0] {
		columnNames = append(columnNames, val.(string))
	}

	var returnedRows [][]field
	for i := 1; i < len(results); i++ {
		var cols []field
		for _, val := range results[i] {
			cols = append(cols, field{Value: reflect.ValueOf(val), Type: reflect.TypeOf(val)})
		}
		returnedRows = append(returnedRows, cols)
	}

	resultSet := resultSet{
		columnNames: columnNames,
		rows:        returnedRows,
		currentRow:  0,
		dateField:   c.Cfg.DateField,
		dateFormat:  c.Cfg.DateFormat,
	}

	r = &rows{
		conn:      c,
		resultSet: resultSet,
	}

	return r, nil
}

func (c *connection) query(req *http.Request) (*rows, error) {
	res, err := c.Client.Do(req)
	if err != nil {
		return &rows{}, err
	}
	defer res.Body.Close()

	code := res.StatusCode
	if code != http.StatusOK {
		body, err := io.ReadAll(res.Body)
		if err != nil {
			return &rows{}, err
		}

		return &rows{}, fmt.Errorf("error making query request to druid, status code: %d, %s", code, string(body))
	}

	response, err := c.parseResponse(bufio.NewReader(res.Body))
	if err != nil {
		return &rows{}, err
	}

	return response, nil
}
