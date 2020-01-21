package influx

import (
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	influx "github.com/influxdata/influxdb/client/v2"
)

// Client represents a wrapper over
// influx HTTP client
type Client struct {
	influx.Client

	database  string
	filter    string
	retention string
	chunkSize int
}

// Config contains fields required
// for Client configuration
type Config struct {
	Addr      string
	Username  string
	Password  string
	Database  string
	Retention string
	Filter    string
	ChunkSize int
}

// Series holds the time series
type Series struct {
	Measurement string
	Field       string
	LabelPairs  []LabelPair
}

// LabelPair is the key-value record
// of time series label
type LabelPair struct {
	Name  string
	Value string
}

// NewClient creates and returns influx client
// configured with passed Config
func NewClient(cfg Config) (*Client, error) {
	c := influx.HTTPConfig{
		Addr:               cfg.Addr,
		Username:           cfg.Username,
		Password:           cfg.Password,
		InsecureSkipVerify: true,
	}
	hc, err := influx.NewHTTPClient(c)
	if err != nil {
		return nil, fmt.Errorf("failed to establish conn: %s", err)
	}
	if _, _, err := hc.Ping(time.Second); err != nil {
		return nil, fmt.Errorf("ping failed: %s", err)
	}

	chunkSize := cfg.ChunkSize
	if chunkSize < 1 {
		chunkSize = 10e3
	}
	client := &Client{
		Client:    hc,
		database:  cfg.Database,
		retention: cfg.Retention,
		filter:    cfg.Filter,
		chunkSize: chunkSize,
	}
	return client, nil
}

// Explore checks the existing data schema in influx
// by checking available fields and series,
// which unique combination represents all possible
// time series existing in database.
// The explore required to reduce the load on influx
// by querying field of the exact time series at once,
// instead of fetching all of the values over and over.
//
// May contain non-existing time series.
func (c *Client) Explore() ([]*Series, error) {
	log.Printf("Exploring scheme for database %q", c.database)
	fieldKeys, err := c.getFieldKeys()
	if err != nil {
		return nil, fmt.Errorf("failed to get field keys: %s", err)
	}

	series, err := c.getSeries()
	if err != nil {
		return nil, fmt.Errorf("failed to get series: %s", err)
	}

	var iSeries []*Series
	for _, s := range series {
		labelParts := strings.Split(s, ",")
		measurement := labelParts[0]
		var lp []LabelPair
		for _, pair := range labelParts[1:] {
			pairParts := strings.Split(pair, "=")
			lp = append(lp, LabelPair{
				Name:  pairParts[0],
				Value: pairParts[1],
			})
		}
		for _, field := range fieldKeys {
			is := &Series{
				Measurement: measurement,
				Field:       field,
				LabelPairs:  lp,
			}
			iSeries = append(iSeries, is)
		}
	}
	return iSeries, nil
}

// ChunkedResponse is a wrapper over influx.ChunkedResponse.
// Used for better memory usage control while iterating
// over huge time series.
type ChunkedResponse struct {
	cr    *influx.ChunkedResponse
	field string
}

// Next reads the next part/chunk of time series.
// Returns io.EOF when time series was read entirely.
func (cr *ChunkedResponse) Next() ([]int64, []interface{}, error) {
	resp, err := cr.cr.NextResponse()
	if err != nil {
		return nil, nil, err
	}

	if resp.Error() != nil {
		return nil, nil, fmt.Errorf("response error: %s", resp.Error())
	}

	if len(resp.Results) != 1 {
		return nil, nil, fmt.Errorf("unexpected number of results in response: %d", len(resp.Results))
	}

	results, err := parse(resp.Results[0])
	if err != nil {
		return nil, nil, err
	}

	const timeFiled = "time"
	timestamps, ok := results[timeFiled]
	if !ok {
		return nil, nil, fmt.Errorf("response doesn't contain field %q", timeFiled)
	}

	values, ok := results[cr.field]
	if !ok {
		return nil, nil, fmt.Errorf("response doesn't contain filed %q", cr.field)
	}

	ts := make([]int64, len(results["time"]))
	for i, v := range timestamps {
		t, err := parseDate(v.(string))
		if err != nil {
			return nil, nil, err
		}
		ts[i] = t
	}
	return ts, values, nil
}

// FetchDataPoints performs SELECT request to fetch
// datapoints for particular field.
func (c *Client) FetchDataPoints(s *Series) (*ChunkedResponse, error) {
	filter := getFilter(c.filter, s.LabelPairs)
	q := fmt.Sprintf("select %s from %s %s", s.Field, s.Measurement, filter)
	iq := influx.Query{
		Command:         q,
		Database:        c.database,
		RetentionPolicy: c.retention,
		Chunked:         true,
		ChunkSize:       1e4,
	}
	cr, err := c.QueryAsChunk(iq)
	if err != nil {
		return nil, fmt.Errorf("query %q err: %s", iq.Command, err)
	}
	return &ChunkedResponse{cr, s.Field}, nil
}

func getFilter(filter string, labelPairs []LabelPair) string {
	if filter == "" && len(labelPairs) == 0 {
		return ""
	}

	f := &strings.Builder{}
	f.WriteString("where ")
	for i, pair := range labelPairs {
		fmt.Fprintf(f, "%s='%s'", pair.Name, pair.Value)
		if i != len(labelPairs)-1 {
			f.WriteString(" and ")
		}
	}

	if filter != "" {
		if len(labelPairs) > 0 {
			fmt.Fprintf(f, " and %s", filter)
		} else {
			f.WriteString(filter)
		}
	}

	return f.String()
}

func parseDate(dateStr string) (int64, error) {
	startTime, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		return 0, fmt.Errorf("cannot parse %q: %s", dateStr, err)
	}
	return startTime.UnixNano() / 1e6, nil
}

func (c *Client) getFieldKeys() ([]string, error) {
	q := influx.Query{
		Command:         "show field keys",
		Database:        c.database,
		RetentionPolicy: c.retention,
	}
	log.Printf("fetching fields: %s", stringify(q))
	values, err := c.do(q)
	if err != nil {
		return nil, fmt.Errorf("error while executing query %q: %s", q.Command, err)
	}

	result := make([]string, len(values["fieldKey"]))
	for i, v := range values["fieldKey"] {
		result[i] = v.(string)
	}
	log.Printf("found %d fields", len(result))
	return result, nil
}

func (c *Client) getSeries() ([]string, error) {
	f := getFilter(c.filter, nil)
	com := fmt.Sprintf("show series %s", f)
	q := influx.Query{
		Command:         com,
		Database:        c.database,
		RetentionPolicy: c.retention,
		Chunked:         true,
		ChunkSize:       c.chunkSize,
	}

	log.Printf("fetching series: %s", stringify(q))
	cr, err := c.QueryAsChunk(q)
	if err != nil {
		return nil, fmt.Errorf("error while executing query %q: %s", q.Command, err)
	}

	var result []string
	for {
		resp, err := cr.NextResponse()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if resp.Error() != nil {
			return nil, fmt.Errorf("response error for query %q: %s", q.Command, resp.Error())
		}
		values, err := parse(resp.Results[0])
		if err != nil {
			return nil, err
		}
		for _, v := range values["key"] {
			result = append(result, v.(string))
		}
	}

	return result, nil
}

func (c *Client) do(q influx.Query) (map[string][]interface{}, error) {
	res, err := c.Query(q)
	if err != nil {
		return nil, fmt.Errorf("query %q err: %s", q.Command, err)
	}
	if len(res.Results) < 1 {
		return nil, fmt.Errorf("exploration query %q returned 0 results", q.Command)
	}
	return parse(res.Results[0])
}

func parse(res influx.Result) (map[string][]interface{}, error) {
	if len(res.Err) > 0 {
		return nil, fmt.Errorf("result error: %s", res.Err)
	}
	values := make(map[string][]interface{})
	for _, row := range res.Series {
		cols := make(map[int]string)
		for pos, key := range row.Columns {
			cols[pos] = key
		}
		for _, value := range row.Values {
			for idx, v := range value {
				key := cols[idx]
				values[key] = append(values[key], v)
			}
		}
	}
	return values, nil
}

func stringify(q influx.Query) string {
	return fmt.Sprintf("command: %q; database: %q; retention: %q",
		q.Command, q.Database, q.RetentionPolicy)
}
