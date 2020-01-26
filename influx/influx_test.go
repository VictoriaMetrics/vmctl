package influx

import "testing"

func TestFetchQuery(t *testing.T) {
	testCases := []struct {
		s          Series
		timeFilter string
		expected   string
	}{
		{
			s: Series{
				Measurement: "cpu",
				Field:       "value",
				LabelPairs: []LabelPair{
					{
						Name:  "foo",
						Value: "bar",
					},
				},
			},
			expected: `select "value" from "cpu" where "foo"='bar'`,
		},
		{
			s: Series{
				Measurement: "cpu",
				Field:       "value",
				LabelPairs: []LabelPair{
					{
						Name:  "foo",
						Value: "bar",
					},
					{
						Name:  "baz",
						Value: "qux",
					},
				},
			},
			expected: `select "value" from "cpu" where "foo"='bar' and "baz"='qux'`,
		},
		{
			s: Series{
				Measurement: "cpu",
				Field:       "value",
				LabelPairs: []LabelPair{
					{
						Name:  "foo",
						Value: "bar",
					},
				},
			},
			timeFilter: "time >= now()",
			expected:   `select "value" from "cpu" where "foo"='bar' and time >= now()`,
		},
		{
			s: Series{
				Measurement: "cpu",
				Field:       "value",
			},
			timeFilter: "time >= now()",
			expected:   `select "value" from "cpu" where time >= now()`,
		},
		{
			s: Series{
				Measurement: "cpu",
				Field:       "value",
			},
			expected: `select "value" from "cpu"`,
		},
	}

	for _, tc := range testCases {
		query := tc.s.fetchQuery(tc.timeFilter)
		if query != tc.expected {
			t.Fatalf("got: \n%q;\nexpected: \n%q", query, tc.expected)
		}
	}
}

func TestTimeFilter(t *testing.T) {
	testCases := []struct {
		start    string
		end      string
		expected string
	}{
		{
			start:    "2020-01-01T20:07:00Z",
			end:      "2020-01-01T21:07:00Z",
			expected: "time >= '2020-01-01T20:07:00Z' and time <= '2020-01-01T21:07:00Z'",
		},
		{
			expected: "",
		},
		{
			start:    "2020-01-01T20:07:00Z",
			expected: "time >= '2020-01-01T20:07:00Z'",
		},
		{
			end:      "2020-01-01T21:07:00Z",
			expected: "time <= '2020-01-01T21:07:00Z'",
		},
	}
	for _, tc := range testCases {
		f := timeFilter(tc.start, tc.end)
		if f != tc.expected {
			t.Fatalf("got: \n%q;\nexpected: \n%q", f, tc.expected)
		}
	}
}
