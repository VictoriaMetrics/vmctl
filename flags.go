package main

import "github.com/urfave/cli/v2"

const (
	vmAddr        = "vm-addr"
	vmAccountID   = "vm-account-id"
	vmConcurrency = "vm-concurrency"
	vmCompress    = "vm-compress"
)

var (
	vmFlags = []cli.Flag{
		&cli.StringFlag{
			Name:  vmAddr,
			Value: "http://localhost:8428",
			Usage: "VictoriaMetrics address to perform import requests. " +
				"Should be the same as --httpListenAddr value for single-node version or VMSelect component.",
		},
		&cli.IntFlag{
			Name:  vmAccountID,
			Value: -1,
			Usage: "Account(tenant) ID - for the cluster VM only",
		},
		&cli.UintFlag{
			Name:  vmConcurrency,
			Usage: "Number of workers concurrently performing import requests to VM",
			Value: 2,
		},
		&cli.BoolFlag{
			Name:  vmCompress,
			Value: true,
			Usage: "Whether to apply gzip compression to import requests",
		},
	}
)

const (
	influxAddr      = "influx-addr"
	influxUser      = "influx-user"
	influxPassword  = "influx-password"
	influxDB        = "influx-database"
	influxRetention = "influx-retention-policy"
	influxFilter    = "influx-series-filter"
	influxChunkSize = "influx-chunk-size"
)

var (
	influxFlags = []cli.Flag{
		&cli.StringFlag{
			Name:  influxAddr,
			Value: "http://localhost:8086",
			Usage: "Influx server addr",
		},
		&cli.StringFlag{
			Name:    influxUser,
			Usage:   "Influx user",
			EnvVars: []string{"INFLUX_USERNAME"},
		},
		&cli.StringFlag{
			Name:    influxPassword,
			Usage:   "Influx user password",
			EnvVars: []string{"INFLUX_PASSWORD"},
		},
		&cli.StringFlag{
			Name:     influxDB,
			Usage:    "Influx database",
			Required: true,
		},
		&cli.StringFlag{
			Name:  influxRetention,
			Usage: "Influx retention policy",
			Value: "autogen",
		},
		&cli.StringFlag{
			Name:  influxFilter,
			Usage: "Influx filter expression to select timeseries. E.g. \"FROM cpu WHERE arch='x64'\"",
		},
		&cli.IntFlag{
			Name:  influxChunkSize,
			Usage: "The chunkSize defines max amount of series to be returned in one chunk",
			Value: 10e3,
		},
	}
)
