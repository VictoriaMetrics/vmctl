package influx

import (
	"fmt"
	"time"

	influx "github.com/influxdata/influxdb/client/v2"
)

type queryValues struct {
	name   string
	values map[string][]interface{}
}

func parseResult(r influx.Result) ([]queryValues, error) {
	if len(r.Err) > 0 {
		return nil, fmt.Errorf("result error: %s", r.Err)
	}
	qValues := make([]queryValues, len(r.Series))
	for i, row := range r.Series {
		values := make(map[string][]interface{}, len(row.Values))
		for _, value := range row.Values {
			for idx, v := range value {
				key := row.Columns[idx]
				values[key] = append(values[key], v)
			}
		}
		qValues[i] = queryValues{
			name:   row.Name,
			values: values,
		}
	}
	return qValues, nil
}

func parseDate(dateStr string) (int64, error) {
	startTime, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		return 0, fmt.Errorf("cannot parse %q: %s", dateStr, err)
	}
	return startTime.UnixNano() / 1e6, nil
}

func stringify(q influx.Query) string {
	return fmt.Sprintf("command: %q; database: %q; retention: %q",
		q.Command, q.Database, q.RetentionPolicy)
}
