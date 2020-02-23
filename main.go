package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/urfave/cli/v2"
	"github.com/victoriametrics/vmctl/influx"
	"github.com/victoriametrics/vmctl/prometheus"
	"github.com/victoriametrics/vmctl/vm"
)

const version = "0.0.2"

func main() {
	start := time.Now()
	app := &cli.App{
		Name:    "vmctl",
		Usage:   "Victoria metrics command-line tool",
		Version: version,
		Commands: []*cli.Command{
			{
				Name:  "influx",
				Usage: "Migrate timeseries from InfluxDB",
				Flags: append(influxFlags, vmFlags...),
				Action: func(c *cli.Context) error {
					fmt.Println("InfluxDB import mode")

					iCfg := influx.Config{
						Addr:      c.String(influxAddr),
						Username:  c.String(influxUser),
						Password:  c.String(influxPassword),
						Database:  c.String(influxDB),
						Retention: c.String(influxRetention),
						Filter: influx.Filter{
							Series:    c.String(influxFilterSeries),
							TimeStart: c.String(influxFilterTimeStart),
							TimeEnd:   c.String(influxFilterTimeEnd),
						},
						ChunkSize: c.Int(influxChunkSize),
					}
					influxClient, err := influx.NewClient(iCfg)
					if err != nil {
						return fmt.Errorf("failed to create influx client: %s", err)
					}

					vmCfg := vm.Config{
						Addr:        c.String(vmAddr),
						User:        c.String(vmUser),
						Password:    c.String(vmPassword),
						Concurrency: uint8(c.Int(vmConcurrency)),
						Compress:    c.Bool(vmCompress),
						AccountID:   c.Int(vmAccountID),
					}
					importer, err := vm.NewImporter(vmCfg)
					if err != nil {
						return fmt.Errorf("failed to create VM importer: %s", err)
					}

					processor := newInfluxProcessor(influxClient, importer,
						c.Int(influxConcurrency), c.String(influxMeasurementFieldSeparator))
					return processor.run()
				},
			},
			{
				Name:  "prometheus",
				Usage: "Migrate timeseries from Prometheus",
				Flags: append(promFlags, vmFlags...),
				Action: func(c *cli.Context) error {
					fmt.Println("Prometheus import mode")

					vmCfg := vm.Config{
						Addr:        c.String(vmAddr),
						User:        c.String(vmUser),
						Password:    c.String(vmPassword),
						Concurrency: uint8(c.Int(vmConcurrency)),
						Compress:    c.Bool(vmCompress),
						AccountID:   c.Int(vmAccountID),
					}
					importer, err := vm.NewImporter(vmCfg)
					if err != nil {
						return fmt.Errorf("failed to create VM importer: %s", err)
					}
					promCfg := prometheus.Config{
						Snapshot: c.String(promSnapshot),
						Filter: prometheus.Filter{
							TimeMin:    c.String(promFilterTimeStart),
							TimeMax:    c.String(promFilterTimeEnd),
							Label:      c.String(promFilterLabel),
							LabelValue: c.String(promFilterLabelValue),
						},
					}
					cl, err := prometheus.NewClient(promCfg)
					if err != nil {
						return fmt.Errorf("failed to create prometheus client: %s", err)
					}
					pp := prometheusProcessor{
						cl: cl,
						im: importer,
						cc: c.Int(promConcurrency),
					}
					return pp.run()
				},
			},
		},
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\r- Execution cancelled")
		os.Exit(0)
	}()

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Total time: %v\n", time.Since(start))
}
